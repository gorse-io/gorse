// Copyright 2020 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package master

import (
	"fmt"
	"github.com/chewxy/math32"
	"github.com/juju/errors"
	"github.com/scylladb/go-set"
	"github.com/scylladb/go-set/strset"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"sort"
	"time"
)

const (
	RankingTop10NDCG      = "NDCG@10"
	RankingTop10Precision = "Precision@10"
	RankingTop10Recall    = "Recall@10"
	ClickPrecision        = "Precision"
	ClickThroughRate      = "ClickThroughRate"

	TaskFindLatest         = "Find latest items"
	TaskFindPopular        = "Find popular items"
	TaskFindItemNeighbors  = "Find neighbors of items"
	TaskFindUserNeighbors  = "Find neighbors of users"
	TaskAnalyze            = "Analyze click-through rate"
	TaskFitRankingModel    = "Fit collaborative filtering model"
	TaskFitClickModel      = "Fit click-through rate prediction model"
	TaskSearchRankingModel = "Search collaborative filtering  model"
	TaskSearchClickModel   = "Search click-through rate prediction model"

	SimilarityShrink = 100
)

// runFindPopularItemsTask updates popular items for the database.
func (m *Master) runFindPopularItemsTask(items []data.Item, feedback []data.Feedback) {
	m.taskMonitor.Start(TaskFindPopular, 1)
	base.Logger().Info("collect popular items", zap.Int("n_cache", m.GorseConfig.Database.CacheSize))
	// create item mapping
	itemMap := make(map[string]data.Item)
	for _, item := range items {
		itemMap[item.ItemId] = item
	}
	// count feedback
	timeWindowLimit := time.Now().AddDate(0, 0, -m.GorseConfig.Recommend.PopularWindow)
	count := make(map[string]int)
	for _, fb := range feedback {
		if fb.Timestamp.After(timeWindowLimit) {
			count[fb.ItemId]++
		}
	}
	// collect pop items
	popItems := make(map[string]*base.TopKStringFilter)
	popItems[""] = base.NewTopKStringFilter(m.GorseConfig.Database.CacheSize)
	for itemId, f := range count {
		popItems[""].Push(itemId, float32(f))
		// Disable popular items under labels temporarily
		//item := itemMap[itemId]
		//for _, label := range item.Labels {
		//	if _, exists := popItems[label]; !exists {
		//		popItems[label] = base.NewTopKStringFilter(m.GorseConfig.Database.CacheSize)
		//	}
		//	popItems[label].Push(itemId, float32(f))
		//}
	}
	// write back
	for label, topItems := range popItems {
		result, scores := topItems.PopAll()
		if err := m.CacheClient.SetScores(cache.PopularItems, label, cache.CreateScoredItems(result, scores)); err != nil {
			base.Logger().Error("failed to cache popular items", zap.Error(err))
		}
	}
	if err := m.CacheClient.SetString(cache.GlobalMeta, cache.LastUpdatePopularTime, base.Now()); err != nil {
		base.Logger().Error("failed to cache popular items", zap.Error(err))
	}
	m.taskMonitor.Finish(TaskFindPopular)
}

// runFindLatestItemsTask updates the latest items.
func (m *Master) runFindLatestItemsTask(items []data.Item) {
	m.taskMonitor.Start(TaskFindLatest, 1)
	base.Logger().Info("collect latest items", zap.Int("n_cache", m.GorseConfig.Database.CacheSize))
	var err error
	latestItems := make(map[string]*base.TopKStringFilter)
	latestItems[""] = base.NewTopKStringFilter(m.GorseConfig.Database.CacheSize)
	// find the latest items
	for _, item := range items {
		if !item.Timestamp.IsZero() {
			latestItems[""].Push(item.ItemId, float32(item.Timestamp.Unix()))
			// Disable the latest items under labels temporarily
			//for _, label := range item.Labels {
			//	if _, exist := latestItems[label]; !exist {
			//		latestItems[label] = base.NewTopKStringFilter(m.GorseConfig.Database.CacheSize)
			//	}
			//	latestItems[label].Push(item.ItemId, float32(item.Timestamp.Unix()))
			//}
		}
	}
	for label, topItems := range latestItems {
		result, scores := topItems.PopAll()
		if err = m.CacheClient.SetScores(cache.LatestItems, label, cache.CreateScoredItems(result, scores)); err != nil {
			base.Logger().Error("failed to cache latest items", zap.Error(err))
		}
	}
	if err = m.CacheClient.SetString(cache.GlobalMeta, cache.LastUpdateLatestTime, base.Now()); err != nil {
		base.Logger().Error("failed to cache latest items time", zap.Error(err))
	}
	m.taskMonitor.Finish(TaskFindLatest)
}

// runFindItemNeighborsTask updates neighbors of items.
func (m *Master) runFindItemNeighborsTask(dataset *ranking.DataSet) {
	m.taskMonitor.Start(TaskFindItemNeighbors, dataset.ItemCount())
	base.Logger().Info("start collecting neighbors of items", zap.Int("n_cache", m.GorseConfig.Database.CacheSize))
	// create progress tracker
	completed := make(chan []interface{}, 1000)
	go func() {
		completedCount := 0
		ticker := time.NewTicker(time.Second)
		for {
			select {
			case _, ok := <-completed:
				if !ok {
					return
				}
				completedCount++
			case <-ticker.C:
				m.taskMonitor.Update(TaskFindItemNeighbors, completedCount)
				base.Logger().Debug("collecting neighbors of items",
					zap.Int("n_complete_items", completedCount),
					zap.Int("n_items", dataset.ItemCount()))
			}
		}
	}()

	userIDF := make([]float32, dataset.UserCount())
	if m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeRelated ||
		m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeAuto {
		for _, feedbacks := range dataset.ItemFeedback {
			sort.Ints(feedbacks)
		}
		// inverse document frequency of users
		for i := range dataset.UserFeedback {
			userIDF[i] = math32.Log(float32(dataset.ItemCount()) / float32(len(dataset.UserFeedback[i])))
		}
	}
	labeledItems := make([][]int, dataset.NumItemLabels)
	labelIDF := make([]float32, dataset.NumItemLabels)
	if m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeSimilar ||
		m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeAuto {
		for i, itemLabels := range dataset.ItemLabels {
			sort.Ints(itemLabels)
			for _, label := range itemLabels {
				labeledItems[label] = append(labeledItems[label], i)
			}
		}
		// inverse document frequency of labels
		for i := range labeledItems {
			labelIDF[i] = math32.Log(float32(dataset.ItemCount()) / float32(len(labeledItems[i])))
		}
	}

	if err := base.Parallel(dataset.ItemCount(), m.GorseConfig.Master.NumJobs, func(workerId, itemId int) error {
		nearItems := base.NewTopKFilter(m.GorseConfig.Database.CacheSize)

		if m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeSimilar ||
			(m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeAuto) {
			labels := dataset.ItemLabels[itemId]
			itemSet := set.NewIntSet()
			for _, label := range labels {
				itemSet.Add(labeledItems[label]...)
			}
			for j := range itemSet.List() {
				if j != itemId {
					commonLabels := commonElements(dataset.ItemLabels[itemId], dataset.ItemLabels[j], labelIDF)
					if commonLabels > 0 {
						score := commonLabels * commonLabels /
							math32.Sqrt(weightedSum(dataset.ItemLabels[itemId], labelIDF)) /
							math32.Sqrt(weightedSum(dataset.ItemLabels[j], labelIDF)) /
							(commonLabels + SimilarityShrink)
						nearItems.Push(j, score)
					}
				}
			}
		}

		if m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeRelated ||
			(m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeAuto && nearItems.Len() == 0) {
			users := dataset.ItemFeedback[itemId]
			itemSet := set.NewIntSet()
			for _, u := range users {
				itemSet.Add(dataset.UserFeedback[u]...)
			}
			for j := range itemSet.List() {
				if j != itemId {
					commonUsers := commonElements(dataset.ItemFeedback[itemId], dataset.ItemFeedback[j], userIDF)
					if commonUsers > 0 {
						score := commonUsers * commonUsers /
							math32.Sqrt(weightedSum(dataset.ItemFeedback[itemId], userIDF)) /
							math32.Sqrt(weightedSum(dataset.ItemFeedback[j], userIDF)) /
							(commonUsers + SimilarityShrink)
						nearItems.Push(j, score)
					}
				}
			}
		}
		elem, scores := nearItems.PopAll()
		recommends := make([]string, len(elem))
		for i := range recommends {
			recommends[i] = dataset.ItemIndex.ToName(elem[i])
		}
		if err := m.CacheClient.SetScores(cache.ItemNeighbors, dataset.ItemIndex.ToName(itemId), cache.CreateScoredItems(recommends, scores)); err != nil {
			return errors.Trace(err)
		}
		completed <- nil
		return nil
	}); err != nil {
		base.Logger().Error("failed to cache neighbors of items", zap.Error(err))
	}
	close(completed)
	if err := m.CacheClient.SetString(cache.GlobalMeta, cache.LastUpdateItemNeighborsTime, base.Now()); err != nil {
		base.Logger().Error("failed to cache neighbors of items", zap.Error(err))
	}
	base.Logger().Info("finish collecting neighbors of items")
	m.taskMonitor.Finish(TaskFindItemNeighbors)
}

// runFindUserNeighborsTask updates neighbors of users.
func (m *Master) runFindUserNeighborsTask(dataset *ranking.DataSet) {
	m.taskMonitor.Start(TaskFindUserNeighbors, dataset.UserCount())
	base.Logger().Info("start collecting neighbors of users", zap.Int("n_cache", m.GorseConfig.Database.CacheSize))
	// create progress tracker
	completed := make(chan []interface{}, 1000)
	go func() {
		completedCount := 0
		ticker := time.NewTicker(time.Second)
		for {
			select {
			case _, ok := <-completed:
				if !ok {
					return
				}
				completedCount++
			case <-ticker.C:
				m.taskMonitor.Update(TaskFindUserNeighbors, completedCount)
				base.Logger().Debug("collecting neighbors of users",
					zap.Int("n_complete_users", completedCount),
					zap.Int("n_users", dataset.UserCount()))
			}
		}
	}()

	itemIDF := make([]float32, dataset.ItemCount())
	if m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeRelated ||
		m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeAuto {
		for _, feedbacks := range dataset.UserFeedback {
			sort.Ints(feedbacks)
		}
		// inverse document frequency of items
		for i := range dataset.ItemFeedback {
			itemIDF[i] = math32.Log(float32(dataset.UserCount()) / float32(len(dataset.ItemFeedback[i])))
		}
	}
	labeledUsers := make([][]int, dataset.NumUserLabels)
	labelIDF := make([]float32, dataset.NumUserLabels)
	if m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeSimilar ||
		m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeAuto {
		for i, userLabels := range dataset.UserLabels {
			sort.Ints(userLabels)
			for _, label := range userLabels {
				labeledUsers[label] = append(labeledUsers[label], i)
			}
		}
		// inverse document frequency of labels
		for i := range labeledUsers {
			labelIDF[i] = math32.Log(float32(dataset.UserCount()) / float32(len(labeledUsers[i])))
		}
	}

	if err := base.Parallel(dataset.UserCount(), m.GorseConfig.Master.NumJobs, func(workerId, userId int) error {
		nearUsers := base.NewTopKFilter(m.GorseConfig.Database.CacheSize)

		if m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeSimilar ||
			(m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeAuto) {
			labels := dataset.UserLabels[userId]
			userSet := set.NewIntSet()
			for _, label := range labels {
				userSet.Add(labeledUsers[label]...)
			}
			for j := range userSet.List() {
				if j != userId {
					commonLabels := commonElements(dataset.UserLabels[userId], dataset.UserLabels[j], labelIDF)
					if commonLabels > 0 {
						score := commonLabels * commonLabels /
							math32.Sqrt(weightedSum(dataset.UserLabels[userId], labelIDF)) /
							math32.Sqrt(weightedSum(dataset.UserLabels[j], labelIDF)) /
							(commonLabels + SimilarityShrink)
						nearUsers.Push(j, score)
					}
				}
			}
		}

		if m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeRelated ||
			(m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeAuto && nearUsers.Len() == 0) {
			users := dataset.UserFeedback[userId]
			userSet := set.NewIntSet()
			for _, u := range users {
				userSet.Add(dataset.UserFeedback[u]...)
			}
			for j := range userSet.List() {
				if j != userId {
					commonItems := commonElements(dataset.UserFeedback[userId], dataset.UserFeedback[j], itemIDF)
					if commonItems > 0 {
						score := commonItems * commonItems /
							math32.Sqrt(weightedSum(dataset.UserFeedback[userId], itemIDF)) /
							math32.Sqrt(weightedSum(dataset.UserFeedback[j], itemIDF)) /
							(commonItems + SimilarityShrink)
						nearUsers.Push(j, score)
					}
				}
			}
		}
		elem, scores := nearUsers.PopAll()
		recommends := make([]string, len(elem))
		for i := range recommends {
			recommends[i] = dataset.UserIndex.ToName(elem[i])
		}
		if err := m.CacheClient.SetScores(cache.UserNeighbors, dataset.UserIndex.ToName(userId), cache.CreateScoredItems(recommends, scores)); err != nil {
			return errors.Trace(err)
		}
		completed <- nil
		return nil
	}); err != nil {
		base.Logger().Error("failed to cache neighbors of users", zap.Error(err))
	}
	close(completed)
	if err := m.CacheClient.SetString(cache.GlobalMeta, cache.LastUpdateUserNeighborsTime, base.Now()); err != nil {
		base.Logger().Error("failed to cache neighbors of users", zap.Error(err))
	}
	base.Logger().Info("finish collecting neighbors of users")
	m.taskMonitor.Finish(TaskFindUserNeighbors)
}

func commonElements(a, b []int, weights []float32) float32 {
	i, j, sum := 0, 0, float32(0)
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			sum += weights[a[i]]
			i++
			j++
		} else if a[i] < b[j] {
			i++
		} else if a[i] > b[j] {
			j++
		}
	}
	return sum
}

func weightedSum(a []int, weights []float32) float32 {
	var sum float32
	for _, i := range a {
		sum += weights[i]
	}
	return sum
}

// fitRankingModel fits ranking model using passed dataset. After model fitted, following states are changed:
// 1. Ranking model version are increased.
// 2. Ranking model score are updated.
// 3. Ranking model, version and score are persisted to local cache.
func (m *Master) runRankingRelatedTasks(
	lastNumUsers, lastNumItems, lastNumFeedback int,
) (numUsers, numItems, numFeedback int, err error) {
	base.Logger().Info("start fitting ranking model", zap.Int("n_jobs", m.GorseConfig.Master.NumJobs))
	m.rankingDataMutex.RLock()
	defer m.rankingDataMutex.RUnlock()
	numUsers = m.rankingTrainSet.UserCount()
	numItems = m.rankingTrainSet.ItemCount()
	numFeedback = m.rankingTrainSet.Count()

	if numUsers == 0 && numItems == 0 && numFeedback == 0 {
		base.Logger().Warn("empty ranking dataset",
			zap.Strings("positive_feedback_type", m.GorseConfig.Database.PositiveFeedbackType))
		return
	}
	numFeedbackChanged := numFeedback != lastNumFeedback
	numUsersChanged := numUsers != lastNumUsers
	numItemsChanged := numItems != lastNumItems

	var modelChanged bool
	bestRankingName, bestRankingModel, bestRankingScore := m.rankingModelSearcher.GetBestModel()
	m.rankingModelMutex.Lock()
	if bestRankingModel != nil && !bestRankingModel.Invalid() &&
		(bestRankingName != m.rankingModelName || bestRankingModel.GetParams().ToString() != m.rankingModel.GetParams().ToString()) &&
		(bestRankingScore.NDCG > m.rankingScore.NDCG) {
		// 1. best ranking model must have been found.
		// 2. best ranking model must be different from current model
		// 3. best ranking model must perform better than current model
		m.rankingModel = bestRankingModel
		m.rankingModelName = bestRankingName
		m.rankingScore = bestRankingScore
		modelChanged = true
		base.Logger().Info("find better ranking model",
			zap.Any("score", bestRankingScore),
			zap.String("name", bestRankingName),
			zap.Any("params", m.rankingModel.GetParams()))
	}
	rankingModel := m.rankingModel
	m.rankingModelMutex.Unlock()

	// update user index
	if numUsersChanged {
		m.userIndexMutex.Lock()
		m.userIndex = m.rankingTrainSet.UserIndex
		m.userIndexVersion++
		m.userIndexMutex.Unlock()
	}
	// collect popular items
	if numFeedback == 0 {
		m.taskMonitor.Fail(TaskFindPopular, "No feedback found.")
	} else if numFeedbackChanged {
		m.runFindPopularItemsTask(m.rankingItems, m.rankingFeedbacks)
	}
	// collect latest items
	if numItems == 0 {
		m.taskMonitor.Fail(TaskFindLatest, "No item found.")
	} else if numItemsChanged {
		m.runFindLatestItemsTask(m.rankingItems)
	}
	// collect neighbors of items
	if numItems == 0 {
		m.taskMonitor.Fail(TaskFindItemNeighbors, "No item found.")
	} else if numItemsChanged || numFeedbackChanged {
		m.runFindItemNeighborsTask(m.rankingFullSet)
	}
	// collect neighbors of users
	if numUsers == 0 {
		m.taskMonitor.Fail(TaskFindUserNeighbors, "No user found.")
	} else if numUsersChanged || numFeedbackChanged {
		m.runFindUserNeighborsTask(m.rankingFullSet)
	}
	// release dataset
	m.rankingFeedbacks = nil
	m.rankingItems = nil
	m.rankingFullSet = nil

	// training model
	if numFeedback == 0 {
		m.taskMonitor.Fail(TaskFitRankingModel, "No feedback found.")
		return
	} else if !numFeedbackChanged && !modelChanged {
		base.Logger().Info("nothing changed")
		return
	}
	m.runFitRankingModelTask(rankingModel)
	return
}

func (m *Master) runFitRankingModelTask(rankingModel ranking.Model) {
	score := rankingModel.Fit(m.rankingTrainSet, m.rankingTestSet, ranking.NewFitConfig().
		SetJobs(m.GorseConfig.Master.NumJobs).
		SetTracker(m.taskMonitor.NewTaskTracker(TaskFitRankingModel)))

	// update ranking model
	m.rankingModelMutex.Lock()
	m.rankingModel = rankingModel
	m.rankingModelVersion++
	m.rankingScore = score
	m.rankingModelMutex.Unlock()
	base.Logger().Info("fit ranking model complete",
		zap.String("version", fmt.Sprintf("%x", m.rankingModelVersion)))
	if err := m.DataClient.InsertMeasurement(data.Measurement{Name: RankingTop10NDCG, Value: score.NDCG, Timestamp: time.Now()}); err != nil {
		base.Logger().Error("failed to insert measurement", zap.Error(err))
	}
	if err := m.DataClient.InsertMeasurement(data.Measurement{Name: RankingTop10Recall, Value: score.Recall, Timestamp: time.Now()}); err != nil {
		base.Logger().Error("failed to insert measurement", zap.Error(err))
	}
	if err := m.DataClient.InsertMeasurement(data.Measurement{Name: RankingTop10Precision, Value: score.Precision, Timestamp: time.Now()}); err != nil {
		base.Logger().Error("failed to insert measurement", zap.Error(err))
	}
	if err := m.CacheClient.SetString(cache.GlobalMeta, cache.LastFitRankingModelTime, base.Now()); err != nil {
		base.Logger().Error("failed to write meta", zap.Error(err))
	}
	if err := m.CacheClient.SetString(cache.GlobalMeta, cache.LastRankingModelVersion, fmt.Sprintf("%x", m.rankingModelVersion)); err != nil {
		base.Logger().Error("failed to write meta", zap.Error(err))
	}

	// caching model
	m.rankingModelMutex.RLock()
	m.localCache.RankingModelName = m.rankingModelName
	m.localCache.RankingModelVersion = m.rankingModelVersion
	m.localCache.RankingModel = rankingModel
	m.localCache.RankingModelScore = score
	m.rankingModelMutex.RUnlock()
	m.userIndexMutex.RLock()
	m.localCache.UserIndex = m.userIndex
	m.localCache.UserIndexVersion = m.userIndexVersion
	m.userIndexMutex.RUnlock()
	if m.localCache.ClickModel == nil || m.localCache.ClickModel.Invalid() {
		base.Logger().Info("wait click model")
	} else if err := m.localCache.WriteLocalCache(); err != nil {
		base.Logger().Error("failed to write local cache", zap.Error(err))
	} else {
		base.Logger().Info("write model to local cache",
			zap.String("ranking_model_name", m.localCache.RankingModelName),
			zap.String("ranking_model_version", base.Hex(m.localCache.RankingModelVersion)),
			zap.Float32("ranking_model_score", m.localCache.RankingModelScore.NDCG),
			zap.Any("ranking_model_params", m.localCache.RankingModel.GetParams()))
	}
}

func (m *Master) runAnalyzeTask() error {
	m.taskScheduler.Lock(TaskAnalyze)
	defer m.taskScheduler.UnLock(TaskAnalyze)
	base.Logger().Info("start analyzing click-through-rate")
	m.taskMonitor.Start(TaskAnalyze, 30)
	// pull existed click-through rates
	clickThroughRates, err := m.DataClient.GetMeasurements(ClickThroughRate, 30)
	if err != nil {
		return errors.Trace(err)
	}
	existed := strset.New()
	for _, clickThroughRate := range clickThroughRates {
		existed.Add(clickThroughRate.Timestamp.String())
	}
	// update click-through rate
	for i := 1; i <= 30; i++ {
		dateTime := time.Now().AddDate(0, 0, -i)
		date := time.Date(dateTime.Year(), dateTime.Month(), dateTime.Day(), 0, 0, 0, 0, time.UTC)
		if !existed.Has(date.String()) {
			// click through clickThroughRate
			startTime := time.Now()
			clickThroughRate, err := m.DataClient.GetClickThroughRate(date, m.GorseConfig.Database.PositiveFeedbackType, m.GorseConfig.Database.ReadFeedbackType)
			if err != nil {
				return errors.Trace(err)
			}
			err = m.DataClient.InsertMeasurement(data.Measurement{
				Name:      ClickThroughRate,
				Timestamp: date,
				Value:     float32(clickThroughRate),
			})
			if err != nil {
				return errors.Trace(err)
			}
			base.Logger().Info("update click through rate",
				zap.String("date", date.String()),
				zap.Duration("time_used", time.Since(startTime)),
				zap.Float64("click_through_rate", clickThroughRate))
		}
		m.taskMonitor.Update(TaskAnalyze, i)
	}
	base.Logger().Info("complete analyzing click-through-rate")
	m.taskMonitor.Finish(TaskAnalyze)
	return nil
}

// runFitClickModelTask fits click model using latest data. After model fitted, following states are changed:
// 1. Click model version are increased.
// 2. Click model score are updated.
// 3. Click model, version and score are persisted to local cache.
func (m *Master) runFitClickModelTask(
	lastNumUsers, lastNumItems, lastNumFeedback int,
) (numUsers, numItems, numFeedback int, err error) {
	base.Logger().Info("prepare to fit click model", zap.Int("n_jobs", m.GorseConfig.Master.NumJobs))
	m.clickDataMutex.RLock()
	defer m.clickDataMutex.RUnlock()
	numUsers = m.clickTrainSet.UserCount()
	numItems = m.clickTrainSet.ItemCount()
	numFeedback = m.clickTrainSet.Count()
	var shouldFit bool

	if numUsers == 0 || numItems == 0 || numFeedback == 0 {
		base.Logger().Warn("empty ranking dataset",
			zap.Strings("positive_feedback_type", m.GorseConfig.Database.PositiveFeedbackType))
		m.taskMonitor.Fail(TaskFitClickModel, "No feedback found.")
		return
	} else if numUsers != lastNumUsers ||
		numItems != lastNumItems ||
		numFeedback != lastNumFeedback {
		shouldFit = true
	}

	bestClickModel, bestClickScore := m.clickModelSearcher.GetBestModel()
	m.clickModelMutex.Lock()
	if bestClickModel != nil && !bestClickModel.Invalid() &&
		bestClickModel.GetParams().ToString() != m.clickModel.GetParams().ToString() &&
		bestClickScore.Precision > m.clickScore.Precision {
		// 1. best click model must have been found.
		// 2. best click model must be different from current model
		// 3. best click model must perform better than current model
		m.clickModel = bestClickModel
		m.clickScore = bestClickScore
		shouldFit = true
		base.Logger().Info("find better click model",
			zap.Float32("Precision", bestClickScore.Precision),
			zap.Float32("Recall", bestClickScore.Recall),
			zap.Any("params", m.clickModel.GetParams()))
	}
	clickModel := m.clickModel
	m.clickModelMutex.Unlock()

	// training model
	if !shouldFit {
		base.Logger().Info("nothing changed")
		return
	}
	score := clickModel.Fit(m.clickTrainSet, m.clickTestSet, click.NewFitConfig().
		SetJobs(m.GorseConfig.Master.NumJobs).
		SetTracker(m.taskMonitor.NewTaskTracker(TaskFitClickModel)))

	// update match model
	m.clickModelMutex.Lock()
	m.clickModel = clickModel
	m.clickScore = score
	m.clickModelVersion++
	m.clickModelMutex.Unlock()
	base.Logger().Info("fit click model complete",
		zap.String("version", fmt.Sprintf("%x", m.clickModelVersion)))
	if err := m.DataClient.InsertMeasurement(data.Measurement{Name: ClickPrecision, Value: score.Precision, Timestamp: time.Now()}); err != nil {
		base.Logger().Error("failed to insert measurement", zap.Error(err))
	}

	// caching model
	m.clickModelMutex.RLock()
	m.localCache.ClickModelScore = m.clickScore
	m.localCache.ClickModelVersion = m.clickModelVersion
	m.localCache.ClickModel = m.clickModel
	m.clickModelMutex.RUnlock()
	if m.localCache.RankingModel == nil || m.localCache.RankingModel.Invalid() {
		base.Logger().Info("wait ranking model")
	} else if err = m.localCache.WriteLocalCache(); err != nil {
		base.Logger().Error("failed to write local cache", zap.Error(err))
	} else {
		base.Logger().Info("write model to local cache",
			zap.String("click_model_version", base.Hex(m.localCache.ClickModelVersion)),
			zap.Float32("click_model_score", score.Precision),
			zap.Any("click_model_params", m.localCache.ClickModel.GetParams()))
	}
	return
}

// runSearchRankingModelTask searches best hyper-parameters for ranking models.
// It requires read lock on the ranking dataset.
func (m *Master) runSearchRankingModelTask(
	lastNumUsers, lastNumItems, lastNumFeedback int,
) (numUsers, numItems, numFeedback int, err error) {
	base.Logger().Info("start searching ranking model")
	m.rankingDataMutex.RLock()
	defer m.rankingDataMutex.RUnlock()
	numUsers = m.rankingTrainSet.UserCount()
	numItems = m.rankingTrainSet.ItemCount()
	numFeedback = m.rankingTrainSet.Count()

	if numUsers == 0 || numItems == 0 || numFeedback == 0 {
		base.Logger().Warn("empty ranking dataset",
			zap.Strings("positive_feedback_type", m.GorseConfig.Database.PositiveFeedbackType))
		m.taskMonitor.Fail(TaskSearchRankingModel, "No feedback found.")
		return
	} else if numUsers == lastNumUsers &&
		numItems == lastNumItems &&
		numFeedback == lastNumFeedback {
		base.Logger().Info("ranking dataset not changed")
		return
	}

	err = m.rankingModelSearcher.Fit(m.rankingTrainSet, m.rankingTestSet,
		m.taskMonitor.NewTaskTracker(TaskSearchRankingModel), m.taskScheduler.NewRunner(TaskSearchRankingModel))
	return
}

// runSearchClickModelTask searches best hyper-parameters for factorization machines.
// It requires read lock on the click dataset.
func (m *Master) runSearchClickModelTask(
	lastNumUsers, lastNumItems, lastNumFeedback int,
) (numUsers, numItems, numFeedback int, err error) {
	base.Logger().Info("start searching click model")
	m.clickDataMutex.RLock()
	defer m.clickDataMutex.RUnlock()
	numUsers = m.clickTrainSet.UserCount()
	numItems = m.clickTrainSet.ItemCount()
	numFeedback = m.clickTrainSet.Count()

	if numUsers == 0 || numItems == 0 || numFeedback == 0 {
		base.Logger().Warn("empty click dataset",
			zap.Strings("positive_feedback_type", m.GorseConfig.Database.PositiveFeedbackType))
		m.taskMonitor.Fail(TaskSearchClickModel, "No feedback found.")
		return
	} else if numUsers == lastNumUsers &&
		numItems == lastNumItems &&
		numFeedback == lastNumFeedback {
		base.Logger().Info("click dataset not changed")
		return
	}

	err = m.clickModelSearcher.Fit(m.clickTrainSet, m.clickTestSet,
		m.taskMonitor.NewTaskTracker(TaskSearchClickModel), m.taskScheduler.NewRunner(TaskSearchClickModel))
	return
}
