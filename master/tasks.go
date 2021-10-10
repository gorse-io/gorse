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
	"github.com/bits-and-blooms/bitset"
	"github.com/chewxy/math32"
	"github.com/juju/errors"
	"github.com/scylladb/go-set/i32set"
	"github.com/scylladb/go-set/strset"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"modernc.org/sortutil"
	"sort"
	"time"
)

const (
	RankingTop10NDCG      = "NDCG@10"
	RankingTop10Precision = "Precision@10"
	RankingTop10Recall    = "Recall@10"
	ClickPrecision        = "Precision"
	ClickThroughRate      = "ClickThroughRate"

	TaskLoadDataset        = "Load dataset"
	TaskFindItemNeighbors  = "Find neighbors of items"
	TaskFindUserNeighbors  = "Find neighbors of users"
	TaskAnalyze            = "Analyze click-through rate"
	TaskFitRankingModel    = "Fit collaborative filtering model"
	TaskFitClickModel      = "Fit click-through rate prediction model"
	TaskSearchRankingModel = "Search collaborative filtering  model"
	TaskSearchClickModel   = "Search click-through rate prediction model"

	batchSize        = 10000
	similarityShrink = 100
)

// runLoadDatasetTask loads dataset.
func (m *Master) runLoadDatasetTask() error {

	base.Logger().Info("load dataset",
		zap.Strings("positive_feedback_types", m.GorseConfig.Database.PositiveFeedbackType),
		zap.Strings("read_feedback_types", m.GorseConfig.Database.ReadFeedbackTypes),
		zap.Uint("item_ttl", m.GorseConfig.Database.ItemTTL),
		zap.Uint("feedback_ttl", m.GorseConfig.Database.PositiveFeedbackTTL),
		zap.Strings("positive_feedback_types", m.GorseConfig.Database.PositiveFeedbackType))
	rankingDataset, clickDataset, latestItems, popularItems, err := m.LoadDataFromDatabase(m.DataClient, m.GorseConfig.Database.PositiveFeedbackType,
		m.GorseConfig.Database.ReadFeedbackTypes, m.GorseConfig.Database.ItemTTL, m.GorseConfig.Database.PositiveFeedbackTTL)
	if err != nil {
		return errors.Trace(err)
	}

	// save popular items to cache
	if err = m.CacheClient.SetScores(cache.PopularItems, "", popularItems); err != nil {
		base.Logger().Error("failed to cache popular items", zap.Error(err))
	}

	// save the latest items to cache
	if err = m.CacheClient.SetScores(cache.LatestItems, "", latestItems); err != nil {
		base.Logger().Error("failed to cache latest items", zap.Error(err))
	}

	// write statistics to database
	loadDataTime := base.DateNow()
	if err = m.DataClient.InsertMeasurement(data.Measurement{
		Name: NumUsers, Timestamp: loadDataTime, Value: float32(rankingDataset.UserCount()),
	}); err != nil {
		base.Logger().Error("failed to write number of users", zap.Error(err))
	}
	if err = m.DataClient.InsertMeasurement(data.Measurement{
		Name: NumItems, Timestamp: loadDataTime, Value: float32(rankingDataset.ItemCount()),
	}); err != nil {
		base.Logger().Error("failed to write number of items", zap.Error(err))
	}
	if err = m.DataClient.InsertMeasurement(data.Measurement{
		Name: NumTotalPosFeedbacks, Timestamp: loadDataTime, Value: float32(rankingDataset.Count()),
	}); err != nil {
		base.Logger().Error("failed to write number of positive feedbacks", zap.Error(err))
	}
	if err = m.DataClient.InsertMeasurement(data.Measurement{
		Name: NumUserLabels, Timestamp: loadDataTime, Value: float32(clickDataset.Index.CountUserLabels()),
	}); err != nil {
		base.Logger().Error("failed to write number of user labels", zap.Error(err))
	}
	if err = m.DataClient.InsertMeasurement(data.Measurement{
		Name: NumItemLabels, Timestamp: loadDataTime, Value: float32(clickDataset.Index.CountItemLabels()),
	}); err != nil {
		base.Logger().Error("failed to write number of item labels", zap.Error(err))
	}
	if err = m.DataClient.InsertMeasurement(data.Measurement{
		Name: NumValidPosFeedbacks, Timestamp: loadDataTime, Value: float32(clickDataset.PositiveCount),
	}); err != nil {
		base.Logger().Error("failed to write number of positive feedbacks", zap.Error(err))
	}
	if err = m.DataClient.InsertMeasurement(data.Measurement{
		Name: NumValidNegFeedbacks, Timestamp: loadDataTime, Value: float32(clickDataset.NegativeCount),
	}); err != nil {
		base.Logger().Error("failed to write number of negative feedbacks", zap.Error(err))
	}

	// split ranking dataset
	m.rankingModelMutex.Lock()
	m.rankingTrainSet, m.rankingTestSet = rankingDataset.Split(0, 0)
	rankingDataset = nil
	m.rankingModelMutex.Unlock()

	// split click dataset
	m.clickModelMutex.Lock()
	m.clickTrainSet, m.clickTestSet = clickDataset.Split(0.2, 0)
	clickDataset = nil
	m.clickModelMutex.Unlock()
	return nil
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
		start := time.Now()
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
					zap.Int("n_items", dataset.ItemCount()),
					zap.Float64("throughput", float64(completedCount)/time.Since(start).Seconds()))
			}
		}
	}()

	userIDF := make([]float32, dataset.UserCount())
	if m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeRelated ||
		m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeAuto {
		for _, feedbacks := range dataset.ItemFeedback {
			sort.Sort(sortutil.Int32Slice(feedbacks))
		}
		// inverse document frequency of users
		for i := range dataset.UserFeedback {
			userIDF[i] = math32.Log(float32(dataset.ItemCount()) / float32(len(dataset.UserFeedback[i])))
		}
	}
	labeledItems := make([][]int32, dataset.NumItemLabels)
	labelIDF := make([]float32, dataset.NumItemLabels)
	if m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeSimilar ||
		m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeAuto {
		for i, itemLabels := range dataset.ItemLabels {
			sort.Sort(sortutil.Int32Slice(itemLabels))
			for _, label := range itemLabels {
				labeledItems[label] = append(labeledItems[label], int32(i))
			}
		}
		// inverse document frequency of labels
		for i := range labeledItems {
			labelIDF[i] = math32.Log(float32(dataset.ItemCount()) / float32(len(labeledItems[i])))
		}
	}

	if err := base.Parallel(dataset.ItemCount(), m.GorseConfig.Master.NumJobs, func(workerId, itemId int) error {
		if !m.checkItemNeighborCacheTimeout(dataset.ItemIndex.ToName(int32(itemId))) {
			return nil
		}
		nearItems := base.NewTopKFilter(m.GorseConfig.Database.CacheSize)

		if m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeSimilar ||
			(m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeAuto) {
			labels := dataset.ItemLabels[itemId]
			itemSet := bitset.New(uint(dataset.ItemCount()))
			var adjacencyItems []int32
			for _, label := range labels {
				for _, adjacencyItemId := range labeledItems[label] {
					if !itemSet.Test(uint(adjacencyItemId)) {
						itemSet.Set(uint(adjacencyItemId))
						adjacencyItems = append(adjacencyItems, adjacencyItemId)
					}
				}
			}
			for _, j := range adjacencyItems {
				if j != int32(itemId) {
					commonLabels := commonElements(dataset.ItemLabels[itemId], dataset.ItemLabels[j], labelIDF)
					if commonLabels > 0 {
						score := commonLabels * commonLabels /
							math32.Sqrt(weightedSum(dataset.ItemLabels[itemId], labelIDF)) /
							math32.Sqrt(weightedSum(dataset.ItemLabels[j], labelIDF)) /
							(commonLabels + similarityShrink)
						nearItems.Push(j, score)
					}
				}
			}
		}

		if m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeRelated ||
			(m.GorseConfig.Recommend.ItemNeighborType == config.NeighborTypeAuto && nearItems.Len() == 0) {
			users := dataset.ItemFeedback[itemId]
			itemSet := bitset.New(uint(dataset.ItemCount()))
			var adjacencyItems []int32
			for _, u := range users {
				for _, adjacencyItemId := range dataset.UserFeedback[u] {
					if !itemSet.Test(uint(adjacencyItemId)) {
						itemSet.Set(uint(adjacencyItemId))
						adjacencyItems = append(adjacencyItems, adjacencyItemId)
					}
				}
			}
			for _, j := range adjacencyItems {
				if j != int32(itemId) {
					commonUsers := commonElements(dataset.ItemFeedback[itemId], dataset.ItemFeedback[j], userIDF)
					if commonUsers > 0 {
						score := commonUsers * commonUsers /
							math32.Sqrt(weightedSum(dataset.ItemFeedback[itemId], userIDF)) /
							math32.Sqrt(weightedSum(dataset.ItemFeedback[j], userIDF)) /
							(commonUsers + similarityShrink)
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
		if err := m.CacheClient.SetScores(cache.ItemNeighbors, dataset.ItemIndex.ToName(int32(itemId)), cache.CreateScoredItems(recommends, scores)); err != nil {
			return errors.Trace(err)
		}
		if err := m.CacheClient.SetTime(cache.LastUpdateItemNeighborsTime, dataset.ItemIndex.ToName(int32(itemId)), time.Now()); err != nil {
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
		start := time.Now()
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
					zap.Int("n_users", dataset.UserCount()),
					zap.Float64("throughput", float64(completedCount)/time.Since(start).Seconds()))
			}
		}
	}()

	itemIDF := make([]float32, dataset.ItemCount())
	if m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeRelated ||
		m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeAuto {
		for _, feedbacks := range dataset.UserFeedback {
			sort.Sort(sortutil.Int32Slice(feedbacks))
		}
		// inverse document frequency of items
		for i := range dataset.ItemFeedback {
			itemIDF[i] = math32.Log(float32(dataset.UserCount()) / float32(len(dataset.ItemFeedback[i])))
		}
	}
	labeledUsers := make([][]int32, dataset.NumUserLabels)
	labelIDF := make([]float32, dataset.NumUserLabels)
	if m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeSimilar ||
		m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeAuto {
		for i, userLabels := range dataset.UserLabels {
			sort.Sort(sortutil.Int32Slice(userLabels))
			for _, label := range userLabels {
				labeledUsers[label] = append(labeledUsers[label], int32(i))
			}
		}
		// inverse document frequency of labels
		for i := range labeledUsers {
			labelIDF[i] = math32.Log(float32(dataset.UserCount()) / float32(len(labeledUsers[i])))
		}
	}

	if err := base.Parallel(dataset.UserCount(), m.GorseConfig.Master.NumJobs, func(workerId, userId int) error {
		if !m.checkUserNeighborCacheTimeout(dataset.UserIndex.ToName(int32(userId))) {
			return nil
		}
		nearUsers := base.NewTopKFilter(m.GorseConfig.Database.CacheSize)

		if m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeSimilar ||
			(m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeAuto) {
			labels := dataset.UserLabels[userId]
			userSet := bitset.New(uint(dataset.UserCount()))
			var adjacencyUsers []int32
			for _, label := range labels {
				for _, adjacencyUserId := range labeledUsers[label] {
					if !userSet.Test(uint(adjacencyUserId)) {
						userSet.Set(uint(adjacencyUserId))
						adjacencyUsers = append(adjacencyUsers, adjacencyUserId)
					}
				}
			}
			for _, j := range adjacencyUsers {
				if j != int32(userId) {
					commonLabels := commonElements(dataset.UserLabels[userId], dataset.UserLabels[j], labelIDF)
					if commonLabels > 0 {
						score := commonLabels * commonLabels /
							math32.Sqrt(weightedSum(dataset.UserLabels[userId], labelIDF)) /
							math32.Sqrt(weightedSum(dataset.UserLabels[j], labelIDF)) /
							(commonLabels + similarityShrink)
						nearUsers.Push(j, score)
					}
				}
			}
		}

		if m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeRelated ||
			(m.GorseConfig.Recommend.UserNeighborType == config.NeighborTypeAuto && nearUsers.Len() == 0) {
			items := dataset.UserFeedback[userId]
			userSet := bitset.New(uint(dataset.UserCount()))
			var adjacencyUsers []int32
			for _, item := range items {
				for _, adjacencyUserId := range dataset.ItemFeedback[item] {
					if !userSet.Test(uint(adjacencyUserId)) {
						userSet.Set(uint(adjacencyUserId))
						adjacencyUsers = append(adjacencyUsers, adjacencyUserId)
					}
				}
			}
			for _, j := range adjacencyUsers {
				if j != int32(userId) {
					commonItems := commonElements(dataset.UserFeedback[userId], dataset.UserFeedback[j], itemIDF)
					if commonItems > 0 {
						score := commonItems * commonItems /
							math32.Sqrt(weightedSum(dataset.UserFeedback[userId], itemIDF)) /
							math32.Sqrt(weightedSum(dataset.UserFeedback[j], itemIDF)) /
							(commonItems + similarityShrink)
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
		if err := m.CacheClient.SetScores(cache.UserNeighbors, dataset.UserIndex.ToName(int32(userId)), cache.CreateScoredItems(recommends, scores)); err != nil {
			return errors.Trace(err)
		}
		if err := m.CacheClient.SetTime(cache.LastUpdateUserNeighborsTime, dataset.UserIndex.ToName(int32(userId)), time.Now()); err != nil {
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

func commonElements(a, b []int32, weights []float32) float32 {
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

func weightedSum(a []int32, weights []float32) float32 {
	var sum float32
	for _, i := range a {
		sum += weights[i]
	}
	return sum
}

// checkUserNeighborCacheTimeout checks if user neighbor cache stale.
// 1. if cache is empty, stale.
// 2. if modified time > update time, stale.
func (m *Master) checkUserNeighborCacheTimeout(userId string) bool {
	var modifiedTime, updateTime time.Time
	var err error
	// read modified time
	modifiedTime, err = m.CacheClient.GetTime(cache.LastModifyUserTime, userId)
	if err != nil {
		base.Logger().Error("failed to read meta", zap.Error(err))
		return true
	}
	// read update time
	updateTime, err = m.CacheClient.GetTime(cache.LastUpdateUserNeighborsTime, userId)
	if err != nil {
		base.Logger().Error("failed to read meta", zap.Error(err))
		return true
	}
	// check time
	return updateTime.Unix() <= modifiedTime.Unix()
}

// checkItemNeighborCacheTimeout checks if item neighbor cache stale.
// 1. if cache is empty, stale.
// 2. if modified time > update time, stale.
func (m *Master) checkItemNeighborCacheTimeout(itemId string) bool {
	var modifiedTime, updateTime time.Time
	var err error
	// read modified time
	modifiedTime, err = m.CacheClient.GetTime(cache.LastModifyItemTime, itemId)
	if err != nil {
		base.Logger().Error("failed to read meta", zap.Error(err))
		return true
	}
	// read update time
	updateTime, err = m.CacheClient.GetTime(cache.LastUpdateItemNeighborsTime, itemId)
	if err != nil {
		base.Logger().Error("failed to read meta", zap.Error(err))
		return true
	}
	// check time
	return updateTime.Unix() <= modifiedTime.Unix()
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
	// collect neighbors of items
	if numItems == 0 {
		m.taskMonitor.Fail(TaskFindItemNeighbors, "No item found.")
	} else if numItemsChanged || numFeedbackChanged {
		m.runFindItemNeighborsTask(m.rankingTrainSet)
	}
	// collect neighbors of users
	if numUsers == 0 {
		m.taskMonitor.Fail(TaskFindUserNeighbors, "No user found.")
	} else if numUsersChanged || numFeedbackChanged {
		m.runFindUserNeighborsTask(m.rankingTrainSet)
	}

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
			clickThroughRate, err := m.DataClient.GetClickThroughRate(date, m.GorseConfig.Database.PositiveFeedbackType, m.GorseConfig.Database.ReadFeedbackTypes)
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

// LoadDataFromDatabase loads dataset from data store.
func (m *Master) LoadDataFromDatabase(database data.Database, posFeedbackTypes, readTypes []string, itemTTL, positiveFeedbackTTL uint) (
	rankingDataset *ranking.DataSet, clickDataset *click.Dataset, latestItems []cache.Scored, popularItems []cache.Scored, err error) {
	m.taskMonitor.Start(TaskLoadDataset, 5)

	// setup time limit
	var itemTimeLimit, feedbackTimeLimit *time.Time
	if itemTTL > 0 {
		temp := time.Now().AddDate(0, 0, -int(itemTTL))
		itemTimeLimit = &temp
	}
	if positiveFeedbackTTL > 0 {
		temp := time.Now().AddDate(0, 0, -int(positiveFeedbackTTL))
		feedbackTimeLimit = &temp
	}
	timeWindowLimit := time.Time{}
	if m.GorseConfig.Recommend.PopularWindow > 0 {
		timeWindowLimit = time.Now().AddDate(0, 0, -m.GorseConfig.Recommend.PopularWindow)
	}
	rankingDataset = ranking.NewMapIndexDataset()

	// STEP 1: pull users
	userLabelIndex := base.NewMapIndex()
	start := time.Now()
	userChan, errChan := database.GetUserStream(batchSize)
	for users := range userChan {
		for _, user := range users {
			rankingDataset.AddUser(user.UserId)
			userIndex := rankingDataset.UserIndex.ToNumber(user.UserId)
			if len(rankingDataset.UserLabels) == int(userIndex) {
				rankingDataset.UserLabels = append(rankingDataset.UserLabels, nil)
			}
			rankingDataset.UserLabels[userIndex] = make([]int32, len(user.Labels))
			for i, label := range user.Labels {
				userLabelIndex.Add(label)
				rankingDataset.UserLabels[userIndex][i] = userLabelIndex.ToNumber(label)
			}
		}
	}
	if err = <-errChan; err != nil {
		return nil, nil, nil, nil, errors.Trace(err)
	}
	rankingDataset.NumUserLabels = userLabelIndex.Len()
	m.taskMonitor.Update(TaskLoadDataset, 1)
	base.Logger().Debug("pulled users from database",
		zap.Int("n_users", rankingDataset.UserCount()),
		zap.Int32("n_user_labels", userLabelIndex.Len()),
		zap.Duration("used_time", time.Since(start)))

	// STEP 2: pull items
	latestItemsFilter := base.NewTopKStringFilter(m.GorseConfig.Database.CacheSize)
	itemLabelIndex := base.NewMapIndex()
	start = time.Now()
	itemChan, errChan := database.GetItemStream(batchSize, itemTimeLimit)
	for items := range itemChan {
		for _, item := range items {
			rankingDataset.AddItem(item.ItemId)
			itemIndex := rankingDataset.ItemIndex.ToNumber(item.ItemId)
			if len(rankingDataset.ItemLabels) == int(itemIndex) {
				rankingDataset.ItemLabels = append(rankingDataset.ItemLabels, nil)
			}
			rankingDataset.ItemLabels[itemIndex] = make([]int32, len(item.Labels))
			for i, label := range item.Labels {
				itemLabelIndex.Add(label)
				rankingDataset.ItemLabels[itemIndex][i] = itemLabelIndex.ToNumber(label)
			}
			// add items to the latest items filter
			if !item.Timestamp.IsZero() {
				latestItemsFilter.Push(item.ItemId, float32(item.Timestamp.Unix()))
			}
		}
	}
	if err = <-errChan; err != nil {
		return nil, nil, nil, nil, errors.Trace(err)
	}
	rankingDataset.NumItemLabels = itemLabelIndex.Len()
	m.taskMonitor.Update(TaskLoadDataset, 2)
	base.Logger().Debug("pulled items from database",
		zap.Int("n_items", rankingDataset.ItemCount()),
		zap.Int32("n_item_labels", itemLabelIndex.Len()),
		zap.Duration("used_time", time.Since(start)))

	// create positive set
	popularCount := make([]int32, rankingDataset.ItemCount())
	positiveSet := make([]*i32set.Set, rankingDataset.UserCount())
	for i := range positiveSet {
		positiveSet[i] = i32set.New()
	}

	// STEP 3: pull positive feedback
	start = time.Now()
	feedbackChan, errChan := database.GetFeedbackStream(batchSize, feedbackTimeLimit, posFeedbackTypes...)
	for feedback := range feedbackChan {
		for _, f := range feedback {
			rankingDataset.AddFeedback(f.UserId, f.ItemId, false)
			// insert feedback to positive set
			userIndex := rankingDataset.UserIndex.ToNumber(f.UserId)
			if userIndex == base.NotId {
				continue
			}
			itemIndex := rankingDataset.ItemIndex.ToNumber(f.ItemId)
			if itemIndex == base.NotId {
				continue
			}
			positiveSet[userIndex].Add(itemIndex)
			// insert feedback to popularity counter
			if f.Timestamp.After(timeWindowLimit) {
				popularCount[itemIndex]++
			}
		}
	}
	if err = <-errChan; err != nil {
		return nil, nil, nil, nil, errors.Trace(err)
	}
	m.taskMonitor.Update(TaskLoadDataset, 3)
	base.Logger().Debug("pulled positive feedback from database",
		zap.Int("n_positive_feedback", rankingDataset.Count()),
		zap.Duration("used_time", time.Since(start)))

	// create negative set
	negativeSet := make([]*i32set.Set, rankingDataset.UserCount())
	for i := range negativeSet {
		negativeSet[i] = i32set.New()
	}

	// STEP 4: pull negative feedback
	start = time.Now()
	feedbackChan, errChan = database.GetFeedbackStream(batchSize, feedbackTimeLimit, readTypes...)
	for feedback := range feedbackChan {
		for _, f := range feedback {
			userIndex := rankingDataset.UserIndex.ToNumber(f.UserId)
			if userIndex == base.NotId {
				continue
			}
			itemIndex := rankingDataset.ItemIndex.ToNumber(f.ItemId)
			if itemIndex == base.NotId {
				continue
			}
			if !positiveSet[userIndex].Has(itemIndex) {
				negativeSet[userIndex].Add(itemIndex)
			}
		}
	}
	if err = <-errChan; err != nil {
		return nil, nil, nil, nil, errors.Trace(err)
	}
	m.taskMonitor.Update(TaskLoadDataset, 4)

	// STEP 5: create click dataset
	unifiedIndex := click.NewUnifiedMapIndexBuilder()
	unifiedIndex.ItemIndex = rankingDataset.ItemIndex
	unifiedIndex.UserIndex = rankingDataset.UserIndex
	unifiedIndex.ItemLabelIndex = itemLabelIndex
	unifiedIndex.UserLabelIndex = userLabelIndex
	clickDataset = &click.Dataset{
		Index:        unifiedIndex.Build(),
		UserFeatures: rankingDataset.UserLabels,
		ItemFeatures: rankingDataset.ItemLabels,
	}
	for userIndex := range positiveSet {
		if positiveSet[userIndex].IsEmpty() || negativeSet[userIndex].IsEmpty() {
			// release positive set and negative set
			positiveSet[userIndex] = nil
			negativeSet[userIndex] = nil
			continue
		}
		// insert positive feedback
		for _, itemIndex := range positiveSet[userIndex].List() {
			clickDataset.Users.Append(int32(userIndex))
			clickDataset.Items.Append(itemIndex)
			clickDataset.NormValues.Append(1 / math32.Sqrt(float32(len(clickDataset.UserFeatures[userIndex])+len(clickDataset.ItemFeatures[itemIndex]))))
			clickDataset.Target.Append(1)
			clickDataset.PositiveCount++
		}
		// insert negative feedback
		for _, itemIndex := range negativeSet[userIndex].List() {
			clickDataset.Users.Append(int32(userIndex))
			clickDataset.Items.Append(itemIndex)
			clickDataset.NormValues.Append(1 / math32.Sqrt(float32(len(clickDataset.UserFeatures[userIndex])+len(clickDataset.ItemFeatures[itemIndex]))))
			clickDataset.Target.Append(-1)
			clickDataset.NegativeCount++
		}
		// release positive set and negative set
		positiveSet[userIndex] = nil
		negativeSet[userIndex] = nil
	}
	base.Logger().Debug("pulled negative feedback from database",
		zap.Int("n_valid_positive", clickDataset.PositiveCount),
		zap.Int("n_valid_negative", clickDataset.NegativeCount),
		zap.Duration("used_time", time.Since(start)))
	m.taskMonitor.Update(TaskLoadDataset, 5)

	// collect latest items and popular items
	items, scores := latestItemsFilter.PopAll()
	latestItems = cache.CreateScoredItems(items, scores)
	popularItemsFilter := base.NewTopKStringFilter(m.GorseConfig.Database.CacheSize)
	for itemIndex, val := range popularCount {
		popularItemsFilter.Push(rankingDataset.ItemIndex.ToName(int32(itemIndex)), float32(val))
	}
	items, scores = popularItemsFilter.PopAll()
	popularItems = cache.CreateScoredItems(items, scores)

	m.taskMonitor.Finish(TaskLoadDataset)
	return rankingDataset, clickDataset, latestItems, popularItems, nil
}
