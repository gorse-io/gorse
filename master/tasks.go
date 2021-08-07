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
	ActiveUsersYesterday  = "ActiveUsersYesterday"
	ActiveUsersMonthly    = "ActiveUsersMonthly"
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

// runFindNeighborsTask updates neighbors for the database.
func (m *Master) runFindNeighborsTask(dataset *ranking.DataSet) {
	m.taskMonitor.Start(TaskFindNeighbor, dataset.ItemCount())
	base.Logger().Info("start collecting similar items", zap.Int("n_cache", m.GorseConfig.Database.CacheSize))
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
				m.taskMonitor.Update(TaskFindNeighbor, completedCount)
				base.Logger().Debug("collect similar items",
					zap.Int("n_complete_items", completedCount),
					zap.Int("n_items", dataset.ItemCount()))
			}
		}
	}()

	if m.GorseConfig.Recommend.NeighborType == config.NeighborTypeRelated ||
		m.GorseConfig.Recommend.NeighborType == config.NeighborTypeAuto {
		for _, feedbacks := range dataset.ItemFeedback {
			sort.Ints(feedbacks)
		}
	}
	labeledItems := make([][]int, dataset.NumItemLabels)
	if m.GorseConfig.Recommend.NeighborType == config.NeighborTypeSimilar ||
		m.GorseConfig.Recommend.NeighborType == config.NeighborTypeAuto {
		for i, itemLabels := range dataset.ItemLabels {
			sort.Ints(itemLabels)
			for _, label := range itemLabels {
				labeledItems[label] = append(labeledItems[label], i)
			}
		}
	}

	if err := base.Parallel(dataset.ItemCount(), m.GorseConfig.Master.FitJobs, func(workerId, itemId int) error {
		nearItems := base.NewTopKFilter(m.GorseConfig.Database.CacheSize)

		if m.GorseConfig.Recommend.NeighborType == config.NeighborTypeSimilar ||
			(m.GorseConfig.Recommend.NeighborType == config.NeighborTypeAuto) {
			labels := dataset.ItemLabels[itemId]
			itemSet := set.NewIntSet()
			for _, label := range labels {
				itemSet.Add(labeledItems[label]...)
			}
			for j := range itemSet.List() {
				if j != itemId {
					score := commonElements(dataset.ItemLabels[itemId], dataset.ItemLabels[j])
					if score > 0 {
						nearItems.Push(j, score)
					}
				}
			}
		}

		if m.GorseConfig.Recommend.NeighborType == config.NeighborTypeRelated ||
			(m.GorseConfig.Recommend.NeighborType == config.NeighborTypeAuto && nearItems.Len() == 0) {
			users := dataset.ItemFeedback[itemId]
			itemSet := set.NewIntSet()
			for _, u := range users {
				itemSet.Add(dataset.UserFeedback[u]...)
			}
			for j := range itemSet.List() {
				if j != itemId {
					score := commonElements(dataset.ItemFeedback[itemId], dataset.ItemFeedback[j])
					if score > 0 {
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
		if err := m.CacheClient.SetScores(cache.SimilarItems, dataset.ItemIndex.ToName(itemId), cache.CreateScoredItems(recommends, scores)); err != nil {
			return errors.Trace(err)
		}
		completed <- nil
		return nil
	}); err != nil {
		base.Logger().Error("failed to cache similar items", zap.Error(err))
	}
	close(completed)
	if err := m.CacheClient.SetString(cache.GlobalMeta, cache.LastUpdateNeighborTime, base.Now()); err != nil {
		base.Logger().Error("failed to cache similar items", zap.Error(err))
	}
	base.Logger().Info("finish collecting similar items")
	m.taskMonitor.Finish(TaskFindNeighbor)
}

func commonElements(a, b []int) float32 {
	i, j, sum := 0, 0, float32(0)
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			i++
			j++
			sum++
		} else if a[i] < b[j] {
			i++
		} else if a[i] > b[j] {
			j++
		}
	}
	return sum
}

// fitRankingModel fits ranking model using passed dataset. After model fitted, following states are changed:
// 1. Ranking model version are increased.
// 2. Ranking model score are updated.
// 3. Ranking model, version and score are persisted to local cache.
func (m *Master) fitRankingModelAndNonPersonalized(
	lastNumUsers, lastNumItems, lastNumFeedback int,
) (numUsers, numItems, numFeedback int, err error) {
	base.Logger().Info("prepare to fit ranking model", zap.Int("n_jobs", m.GorseConfig.Master.FitJobs))
	m.rankingDataMutex.RLock()
	defer m.rankingDataMutex.RUnlock()
	numUsers = m.rankingTrainSet.UserCount()
	numItems = m.rankingTrainSet.ItemCount()
	numFeedback = m.rankingTrainSet.Count()
	var dataChanged bool
	var modelChanged bool

	if numUsers == 0 || numItems == 0 || numFeedback == 0 {
		base.Logger().Warn("empty ranking dataset",
			zap.Strings("positive_feedback_type", m.GorseConfig.Database.PositiveFeedbackType))
		return
	} else if numUsers != lastNumUsers ||
		numItems != lastNumItems ||
		numFeedback != lastNumFeedback {
		dataChanged = true
	}

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

	if dataChanged {
		// update user index
		m.userIndexMutex.Lock()
		m.userIndex = m.rankingTrainSet.UserIndex
		m.userIndexVersion++
		m.userIndexMutex.Unlock()
		// collect popular items
		m.runFindPopularItemsTask(m.rankingItems, m.rankingFeedbacks)
		// collect latest items
		m.runFindLatestItemsTask(m.rankingItems)
		// collect similar items
		m.runFindNeighborsTask(m.rankingFullSet)
		// release dataset
		m.rankingFeedbacks = nil
		m.rankingItems = nil
		m.rankingFullSet = nil
	}

	// training model
	if !dataChanged && !modelChanged {
		base.Logger().Info("nothing changed")
		return
	}
	score := rankingModel.Fit(m.rankingTrainSet, m.rankingTestSet, ranking.NewFitConfig().
		SetJobs(m.GorseConfig.Master.FitJobs).
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
	return
}

func (m *Master) runAnalyzeTask() error {
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
			clickThroughRate, err := m.DataClient.GetClickThroughRate(date, m.GorseConfig.Database.ClickFeedbackTypes, m.GorseConfig.Database.ReadFeedbackType)
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

	// pull existed active users
	yesterdayActiveUsers, err := m.DataClient.GetMeasurements(ActiveUsersYesterday, 1)
	if err != nil {
		return errors.Trace(err)
	}
	yesterdayDatetime := time.Now().AddDate(0, 0, -1)
	yesterdayDate := time.Date(yesterdayDatetime.Year(), yesterdayDatetime.Month(), yesterdayDatetime.Day(), 0, 0, 0, 0, time.UTC)
	if len(yesterdayActiveUsers) == 0 || !yesterdayActiveUsers[0].Timestamp.Equal(yesterdayDate) {
		startTime := time.Now()
		activeUsers, err := m.DataClient.CountActiveUsers(time.Now().AddDate(0, 0, -1))
		if err != nil {
			return errors.Trace(err)
		}
		err = m.DataClient.InsertMeasurement(data.Measurement{
			Name:      ActiveUsersYesterday,
			Timestamp: yesterdayDate,
			Value:     float32(activeUsers),
		})
		if err != nil {
			return errors.Trace(err)
		}
		base.Logger().Info("update active users of yesterday",
			zap.String("date", yesterdayDate.String()),
			zap.Duration("time_used", time.Since(startTime)),
			zap.Int("active_users", activeUsers))
	}

	monthActiveUsers, err := m.DataClient.GetMeasurements(ActiveUsersMonthly, 1)
	if err != nil {
		return errors.Trace(err)
	}
	if len(monthActiveUsers) == 0 || !monthActiveUsers[0].Timestamp.Equal(yesterdayDate) {
		startTime := time.Now()
		activeUsers, err := m.DataClient.CountActiveUsers(time.Now().AddDate(0, 0, -30))
		if err != nil {
			return errors.Trace(err)
		}
		err = m.DataClient.InsertMeasurement(data.Measurement{
			Name:      ActiveUsersMonthly,
			Timestamp: yesterdayDate,
			Value:     float32(activeUsers),
		})
		if err != nil {
			return errors.Trace(err)
		}
		base.Logger().Info("update active users of this month",
			zap.String("date", yesterdayDate.String()),
			zap.Duration("time_used", time.Since(startTime)),
			zap.Int("active_users", activeUsers))
	}

	m.taskMonitor.Finish(TaskAnalyze)
	return nil
}

// fitClickModel fits click model using latest data. After model fitted, following states are changed:
// 1. Click model version are increased.
// 2. Click model score are updated.
// 3. Click model, version and score are persisted to local cache.
func (m *Master) fitClickModel(
	lastNumUsers, lastNumItems, lastNumFeedback int,
) (numUsers, numItems, numFeedback int, err error) {
	base.Logger().Info("prepare to fit click model", zap.Int("n_jobs", m.GorseConfig.Master.FitJobs))
	m.clickDataMutex.RLock()
	defer m.clickDataMutex.RUnlock()
	numUsers = m.clickTrainSet.UserCount()
	numItems = m.clickTrainSet.ItemCount()
	numFeedback = m.clickTrainSet.Count()
	var shouldFit bool

	if numUsers == 0 || numItems == 0 || numFeedback == 0 {
		base.Logger().Warn("empty ranking dataset",
			zap.Strings("positive_feedback_type", m.GorseConfig.Database.PositiveFeedbackType))
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
			zap.Float32("score", bestClickScore.Precision),
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
		SetJobs(m.GorseConfig.Master.FitJobs).
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
	base.Logger().Info("prepare to search ranking model")
	m.rankingDataMutex.RLock()
	defer m.rankingDataMutex.RUnlock()
	numUsers = m.rankingTrainSet.UserCount()
	numItems = m.rankingTrainSet.ItemCount()
	numFeedback = m.rankingTrainSet.Count()

	if numUsers == 0 || numItems == 0 || numFeedback == 0 {
		base.Logger().Warn("empty ranking dataset",
			zap.Strings("positive_feedback_type", m.GorseConfig.Database.PositiveFeedbackType))
		return
	} else if numUsers == lastNumUsers &&
		numItems == lastNumItems &&
		numFeedback == lastNumFeedback {
		base.Logger().Info("ranking dataset not changed")
		return
	}

	err = m.rankingModelSearcher.Fit(m.rankingTrainSet, m.rankingTestSet,
		m.taskMonitor.NewTaskTracker(TaskSearchRankingModel))
	return
}

// runSearchClickModelTask searches best hyper-parameters for factorization machines.
// It requires read lock on the click dataset.
func (m *Master) runSearchClickModelTask(
	lastNumUsers, lastNumItems, lastNumFeedback int,
) (numUsers, numItems, numFeedback int, err error) {
	base.Logger().Info("prepare to search click model")
	m.clickDataMutex.RLock()
	defer m.clickDataMutex.RUnlock()
	numUsers = m.clickTrainSet.UserCount()
	numItems = m.clickTrainSet.ItemCount()
	numFeedback = m.clickTrainSet.Count()

	if numUsers == 0 || numItems == 0 || numFeedback == 0 {
		base.Logger().Warn("empty click dataset",
			zap.Strings("click_feedback_type", m.GorseConfig.Database.ClickFeedbackTypes))
		return
	} else if numUsers == lastNumUsers &&
		numItems == lastNumItems &&
		numFeedback == lastNumFeedback {
		base.Logger().Info("click dataset not changed")
		return
	}

	err = m.clickModelSearcher.Fit(m.clickTrainSet, m.clickTestSet,
		m.taskMonitor.NewTaskTracker(TaskSearchClickModel))
	return
}
