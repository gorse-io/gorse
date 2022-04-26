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
	"github.com/zhenghaoz/gorse/base/heap"
	"github.com/zhenghaoz/gorse/base/parallel"
	"github.com/zhenghaoz/gorse/base/search"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"math"
	"modernc.org/sortutil"
	"sort"
	"strings"
	"time"
)

const (
	PositiveFeedbackRate = "PositiveFeedbackRate"

	TaskLoadDataset            = "Load dataset"
	TaskFindItemNeighbors      = "Find neighbors of items"
	TaskFindUserNeighbors      = "Find neighbors of users"
	TaskAnalyze                = "Analyze click-through rate"
	TaskFitRankingModel        = "Fit collaborative filtering model"
	TaskFitClickModel          = "Fit click-through rate prediction model"
	TaskSearchRankingModel     = "Search collaborative filtering  model"
	TaskSearchClickModel       = "Search click-through rate prediction model"
	TaskCacheGarbageCollection = "Collect garbage in cache"

	batchSize        = 10000
	similarityShrink = 100
)

// runLoadDatasetTask loads dataset.
func (m *Master) runLoadDatasetTask() error {
	startTime := time.Now()

	base.Logger().Info("load dataset",
		zap.Strings("positive_feedback_types", m.GorseConfig.Recommend.DataSource.PositiveFeedbackTypes),
		zap.Strings("read_feedback_types", m.GorseConfig.Recommend.DataSource.ReadFeedbackTypes),
		zap.Uint("item_ttl", m.GorseConfig.Recommend.DataSource.ItemTTL),
		zap.Uint("feedback_ttl", m.GorseConfig.Recommend.DataSource.PositiveFeedbackTTL))
	rankingDataset, clickDataset, latestItems, popularItems, err := m.LoadDataFromDatabase(m.DataClient,
		m.GorseConfig.Recommend.DataSource.PositiveFeedbackTypes,
		m.GorseConfig.Recommend.DataSource.ReadFeedbackTypes,
		m.GorseConfig.Recommend.DataSource.ItemTTL,
		m.GorseConfig.Recommend.DataSource.PositiveFeedbackTTL)
	if err != nil {
		return errors.Trace(err)
	}

	// save popular items to cache
	for category, items := range popularItems {
		if err = m.CacheClient.SetSorted(cache.Key(cache.PopularItems, category), items); err != nil {
			base.Logger().Error("failed to cache popular items", zap.Error(err))
		}
	}
	if err = m.CacheClient.Set(cache.Time(cache.Key(cache.GlobalMeta, cache.LastUpdatePopularItemsTime), time.Now())); err != nil {
		base.Logger().Error("failed to write latest update popular items time", zap.Error(err))
	}

	// save the latest items to cache
	for category, items := range latestItems {
		if err = m.CacheClient.AddSorted(cache.Sorted(cache.Key(cache.LatestItems, category), items)); err != nil {
			base.Logger().Error("failed to cache latest items", zap.Error(err))
		}
		// reclaim outdated items
		if len(items) > 0 {
			threshold := items[len(items)-1].Score - 1
			if err = m.CacheClient.RemSortedByScore(cache.Key(cache.LatestItems, category), math.Inf(-1), threshold); err != nil {
				base.Logger().Error("failed to reclaim outdated items", zap.Error(err))
			}
		}
	}
	if err = m.CacheClient.Set(cache.Time(cache.Key(cache.GlobalMeta, cache.LastUpdateLatestItemsTime), time.Now())); err != nil {
		base.Logger().Error("failed to write latest update latest items time", zap.Error(err))
	}

	// write statistics to database
	UsersTotal.Set(float64(rankingDataset.UserCount()))
	if err = m.CacheClient.Set(cache.Integer(cache.Key(cache.GlobalMeta, cache.NumUsers), rankingDataset.UserCount())); err != nil {
		base.Logger().Error("failed to write number of users", zap.Error(err))
	}
	ItemsTotal.Set(float64(rankingDataset.ItemCount()))
	if err = m.CacheClient.Set(cache.Integer(cache.Key(cache.GlobalMeta, cache.NumItems), rankingDataset.ItemCount())); err != nil {
		base.Logger().Error("failed to write number of items", zap.Error(err))
	}
	FeedbacksTotal.Set(float64(rankingDataset.Count()))
	if err = m.CacheClient.Set(cache.Integer(cache.Key(cache.GlobalMeta, cache.NumTotalPosFeedbacks), rankingDataset.Count())); err != nil {
		base.Logger().Error("failed to write number of positive feedbacks", zap.Error(err))
	}
	UserLabelsTotal.Set(float64(clickDataset.Index.CountUserLabels()))
	if err = m.CacheClient.Set(cache.Integer(cache.Key(cache.GlobalMeta, cache.NumUserLabels), int(clickDataset.Index.CountUserLabels()))); err != nil {
		base.Logger().Error("failed to write number of user labels", zap.Error(err))
	}
	ItemLabelsTotal.Set(float64(clickDataset.Index.CountItemLabels()))
	if err = m.CacheClient.Set(cache.Integer(cache.Key(cache.GlobalMeta, cache.NumItemLabels), int(clickDataset.Index.CountItemLabels()))); err != nil {
		base.Logger().Error("failed to write number of item labels", zap.Error(err))
	}
	ImplicitFeedbacksTotal.Set(float64(rankingDataset.Count()))
	PositiveFeedbacksTotal.Set(float64(clickDataset.PositiveCount))
	if err = m.CacheClient.Set(cache.Integer(cache.Key(cache.GlobalMeta, cache.NumValidPosFeedbacks), clickDataset.PositiveCount)); err != nil {
		base.Logger().Error("failed to write number of positive feedbacks", zap.Error(err))
	}
	NegativeFeedbackTotal.Set(float64(clickDataset.NegativeCount))
	if err = m.CacheClient.Set(cache.Integer(cache.Key(cache.GlobalMeta, cache.NumValidNegFeedbacks), clickDataset.NegativeCount)); err != nil {
		base.Logger().Error("failed to write number of negative feedbacks", zap.Error(err))
	}

	// collect active users and items
	activeUsers, activeItems, inactiveUsers, inactiveItems := 0, 0, 0, 0
	for _, userFeedback := range rankingDataset.UserFeedback {
		if len(userFeedback) > 0 {
			activeUsers++
		} else {
			inactiveUsers++
		}
	}
	for _, itemFeedback := range rankingDataset.ItemFeedback {
		if len(itemFeedback) > 0 {
			activeItems++
		} else {
			inactiveItems++
		}
	}
	ActiveUsersTotal.Set(float64(activeUsers))
	ActiveItemsTotal.Set(float64(activeItems))
	InactiveUsersTotal.Set(float64(inactiveUsers))
	InactiveItemsTotal.Set(float64(inactiveItems))

	// write categories to cache
	if err = m.CacheClient.SetSet(cache.ItemCategories, rankingDataset.CategorySet.List()...); err != nil {
		base.Logger().Error("failed to write categories to cache", zap.Error(err))
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

	LoadDatasetSeconds.Observe(time.Since(startTime).Seconds())
	return nil
}

// runFindItemNeighborsTask updates neighbors of items.
func (m *Master) runFindItemNeighborsTask(dataset *ranking.DataSet) {
	m.taskMonitor.Start(TaskFindItemNeighbors, dataset.ItemCount())
	base.Logger().Info("start searching neighbors of items",
		zap.Int("n_cache", m.GorseConfig.Recommend.CacheSize))
	// create progress tracker
	completed := make(chan struct{}, 1000)
	go func() {
		completedCount, previousCount := 0, 0
		ticker := time.NewTicker(time.Second)
		for {
			select {
			case _, ok := <-completed:
				if !ok {
					return
				}
				completedCount++
			case <-ticker.C:
				throughput := completedCount - previousCount
				previousCount = completedCount
				if throughput > 0 {
					m.taskMonitor.Update(TaskFindItemNeighbors, completedCount)
					base.Logger().Debug("searching neighbors of items",
						zap.Int("n_complete_items", completedCount),
						zap.Int("n_items", dataset.ItemCount()),
						zap.Int("throughput", throughput))
				}
			}
		}
	}()

	userIDF := make([]float32, dataset.UserCount())
	if m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeRelated ||
		m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeAuto {
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
	if m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeSimilar ||
		m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeAuto {
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

	start := time.Now()
	var err error
	if m.GorseConfig.Recommend.ItemNeighbors.EnableIndex {
		err = m.findItemNeighborsIVF(dataset, labelIDF, userIDF, completed)
	} else {
		err = m.findItemNeighborsBruteForce(dataset, labeledItems, labelIDF, userIDF, completed)
	}
	searchTime := time.Since(start)

	close(completed)
	if err != nil {
		base.Logger().Error("failed to searching neighbors of items", zap.Error(err))
		m.taskMonitor.Fail(TaskFindItemNeighbors, err.Error())
	} else {
		if err := m.CacheClient.Set(cache.Time(cache.Key(cache.GlobalMeta, cache.LastUpdateItemNeighborsTime), time.Now())); err != nil {
			base.Logger().Error("failed to set neighbors of items update time", zap.Error(err))
		}
		base.Logger().Info("complete searching neighbors of items",
			zap.String("search_time", searchTime.String()))
		m.taskMonitor.Finish(TaskFindItemNeighbors)
	}
}

func (m *Master) findItemNeighborsBruteForce(dataset *ranking.DataSet, labeledItems [][]int32,
	labelIDF, userIDF []float32, completed chan struct{}) error {
	return parallel.Parallel(dataset.ItemCount(), m.GorseConfig.Master.NumJobs, func(workerId, itemIndex int) error {
		defer func() {
			completed <- struct{}{}
		}()
		itemId := dataset.ItemIndex.ToName(int32(itemIndex))
		if !m.checkItemNeighborCacheTimeout(itemId, dataset.CategorySet.List()) {
			return nil
		}
		startTime := time.Now()
		nearItemsFilters := make(map[string]*heap.TopKFilter)
		nearItemsFilters[""] = heap.NewTopKFilter(m.GorseConfig.Recommend.CacheSize)
		for _, category := range dataset.CategorySet.List() {
			nearItemsFilters[category] = heap.NewTopKFilter(m.GorseConfig.Recommend.CacheSize)
		}

		if m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeSimilar ||
			(m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeAuto) {
			labels := dataset.ItemLabels[itemIndex]
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
				if j != int32(itemIndex) && !dataset.HiddenItems[j] {
					commonSum, commonCount := commonElements(dataset.ItemLabels[itemIndex], dataset.ItemLabels[j], labelIDF)
					if commonSum > 0 {
						score := commonSum * commonCount /
							math32.Sqrt(weightedSum(dataset.ItemLabels[itemIndex], labelIDF)) /
							math32.Sqrt(weightedSum(dataset.ItemLabels[j], labelIDF)) /
							(commonCount + similarityShrink)
						nearItemsFilters[""].Push(j, score)
						for _, category := range dataset.ItemCategories[j] {
							nearItemsFilters[category].Push(j, score)
						}
					}
				}
			}
		}

		if m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeRelated ||
			(m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeAuto && nearItemsFilters[""].Len() == 0) {
			users := dataset.ItemFeedback[itemIndex]
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
				if j != int32(itemIndex) && !dataset.HiddenItems[j] {
					commonSum, commonCount := commonElements(dataset.ItemFeedback[itemIndex], dataset.ItemFeedback[j], userIDF)
					if commonSum > 0 {
						score := commonSum * commonCount /
							math32.Sqrt(weightedSum(dataset.ItemFeedback[itemIndex], userIDF)) /
							math32.Sqrt(weightedSum(dataset.ItemFeedback[j], userIDF)) /
							(commonCount + similarityShrink)
						nearItemsFilters[""].Push(j, score)
						for _, category := range dataset.ItemCategories[j] {
							nearItemsFilters[category].Push(j, score)
						}
					}
				}
			}
		}
		for category, nearItemsFilter := range nearItemsFilters {
			elem, scores := nearItemsFilter.PopAll()
			recommends := make([]string, len(elem))
			for i := range recommends {
				recommends[i] = dataset.ItemIndex.ToName(elem[i])
			}
			if err := m.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, itemId, category),
				cache.CreateScoredItems(recommends, scores)); err != nil {
				return errors.Trace(err)
			}
		}
		if err := m.CacheClient.Set(
			cache.Time(cache.Key(cache.LastUpdateItemNeighborsTime, itemId), time.Now()),
			cache.String(cache.Key(cache.ItemNeighborsDigest, itemId), m.GorseConfig.ItemNeighborDigest())); err != nil {
			return errors.Trace(err)
		}
		FindItemNeighborsSeconds.Observe(time.Since(startTime).Seconds())
		return nil
	})
}

func (m *Master) findItemNeighborsIVF(dataset *ranking.DataSet, labelIDF, userIDF []float32, completed chan struct{}) error {
	var similarItemNeighbors, relatedItemNeighbors search.VectorIndex
	var itemLabelVectors, itemFeedbackVectors []search.Vector
	if m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeSimilar ||
		m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeAuto {
		itemLabelVectors = make([]search.Vector, dataset.ItemCount())
		for i := range itemLabelVectors {
			itemLabelVectors[i] = search.NewDictionaryVector(dataset.ItemLabels[i], labelIDF, dataset.ItemCategories[i], dataset.HiddenItems[i])
		}
		builder := search.NewIVFBuilder(itemLabelVectors, m.GorseConfig.Recommend.CacheSize, 1000,
			search.SetIVFNumJobs(m.GorseConfig.Master.NumJobs))
		var recall float32
		similarItemNeighbors, recall = builder.Build(m.GorseConfig.Recommend.ItemNeighbors.IndexRecall,
			m.GorseConfig.Recommend.ItemNeighbors.IndexFitEpoch, true)
		ItemNeighborIndexRecall.Set(float64(recall))
		if err := m.CacheClient.Set(cache.String(cache.Key(cache.GlobalMeta, cache.ItemNeighborIndexRecall), base.FormatFloat32(recall))); err != nil {
			return errors.Trace(err)
		}
	}
	if m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeRelated ||
		m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeAuto {
		itemFeedbackVectors = make([]search.Vector, dataset.ItemCount())
		for i := range itemFeedbackVectors {
			itemFeedbackVectors[i] = search.NewDictionaryVector(dataset.ItemFeedback[i], userIDF, dataset.ItemCategories[i], dataset.HiddenItems[i])
		}
		builder := search.NewIVFBuilder(itemFeedbackVectors, m.GorseConfig.Recommend.CacheSize, 1000,
			search.SetIVFNumJobs(m.GorseConfig.Master.NumJobs))
		relatedItemNeighbors, _ = builder.Build(m.GorseConfig.Recommend.ItemNeighbors.IndexRecall,
			m.GorseConfig.Recommend.ItemNeighbors.IndexFitEpoch, true)
	}
	return parallel.Parallel(dataset.ItemCount(), m.GorseConfig.Master.NumJobs, func(workerId, itemIndex int) error {
		defer func() {
			completed <- struct{}{}
		}()
		itemId := dataset.ItemIndex.ToName(int32(itemIndex))
		if !m.checkItemNeighborCacheTimeout(itemId, dataset.CategorySet.List()) {
			return nil
		}
		startTime := time.Now()
		var neighbors map[string][]int32
		var scores map[string][]float32
		if m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeSimilar ||
			m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeAuto {
			neighbors, scores = similarItemNeighbors.MultiSearch(itemLabelVectors[itemIndex], dataset.CategorySet.List(),
				m.GorseConfig.Recommend.CacheSize, true)
		}
		if m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeRelated ||
			m.GorseConfig.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeAuto && len(neighbors[""]) == 0 {
			neighbors, scores = relatedItemNeighbors.MultiSearch(itemFeedbackVectors[itemIndex], dataset.CategorySet.List(),
				m.GorseConfig.Recommend.CacheSize, true)
		}
		for category := range neighbors {
			if categoryNeighbors, exist := neighbors[category]; exist && len(categoryNeighbors) > 0 {
				itemScores := make([]cache.Scored, len(neighbors[category]))
				for i := range scores[category] {
					itemScores[i].Id = dataset.ItemIndex.ToName(neighbors[category][i])
					itemScores[i].Score = float64(-scores[category][i])
				}
				if err := m.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, itemId, category), itemScores); err != nil {
					return errors.Trace(err)
				}
			}
		}
		if err := m.CacheClient.Set(
			cache.Time(cache.Key(cache.LastUpdateItemNeighborsTime, itemId), time.Now()),
			cache.String(cache.Key(cache.ItemNeighborsDigest, itemId), m.GorseConfig.ItemNeighborDigest())); err != nil {
			return errors.Trace(err)
		}
		FindItemNeighborsSeconds.Observe(time.Since(startTime).Seconds())
		return nil
	})
}

// runFindUserNeighborsTask updates neighbors of users.
func (m *Master) runFindUserNeighborsTask(dataset *ranking.DataSet) {
	m.taskMonitor.Start(TaskFindUserNeighbors, dataset.UserCount())
	base.Logger().Info("start searching neighbors of users",
		zap.Int("n_cache", m.GorseConfig.Recommend.CacheSize))
	// create progress tracker
	completed := make(chan struct{}, 1000)
	go func() {
		completedCount, previousCount := 0, 0
		ticker := time.NewTicker(time.Second)
		for {
			select {
			case _, ok := <-completed:
				if !ok {
					return
				}
				completedCount++
			case <-ticker.C:
				throughput := completedCount - previousCount
				previousCount = completedCount
				if throughput > 0 {
					m.taskMonitor.Update(TaskFindUserNeighbors, completedCount)
					base.Logger().Debug("searching neighbors of users",
						zap.Int("n_complete_users", completedCount),
						zap.Int("n_users", dataset.UserCount()),
						zap.Int("throughput", throughput))
				}
			}
		}
	}()

	itemIDF := make([]float32, dataset.ItemCount())
	if m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeRelated ||
		m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeAuto {
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
	if m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeSimilar ||
		m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeAuto {
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

	start := time.Now()
	var err error
	if m.GorseConfig.Recommend.UserNeighbors.EnableIndex {
		err = m.findUserNeighborsIVF(dataset, labelIDF, itemIDF, completed)
	} else {
		err = m.findUserNeighborsBruteForce(dataset, labeledUsers, labelIDF, itemIDF, completed)
	}
	searchTime := time.Since(start)

	close(completed)
	if err != nil {
		base.Logger().Error("failed to searching neighbors of users", zap.Error(err))
		m.taskMonitor.Fail(TaskFindUserNeighbors, err.Error())
	} else {
		if err := m.CacheClient.Set(cache.Time(cache.Key(cache.GlobalMeta, cache.LastUpdateUserNeighborsTime), time.Now())); err != nil {
			base.Logger().Error("failed to set neighbors of users update time", zap.Error(err))
		}
		base.Logger().Info("complete searching neighbors of users",
			zap.String("search_time", searchTime.String()))
	}
	m.taskMonitor.Finish(TaskFindUserNeighbors)
}

func (m *Master) findUserNeighborsBruteForce(dataset *ranking.DataSet, labeledUsers [][]int32, labelIDF, itemIDF []float32, completed chan struct{}) error {
	return parallel.Parallel(dataset.UserCount(), m.GorseConfig.Master.NumJobs, func(workerId, userIndex int) error {
		defer func() {
			completed <- struct{}{}
		}()
		userId := dataset.UserIndex.ToName(int32(userIndex))
		if !m.checkUserNeighborCacheTimeout(userId) {
			return nil
		}
		startTime := time.Now()
		nearUsers := heap.NewTopKFilter(m.GorseConfig.Recommend.CacheSize)

		if m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeSimilar ||
			(m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeAuto) {
			labels := dataset.UserLabels[userIndex]
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
				if j != int32(userIndex) {
					commonSum, commonCount := commonElements(dataset.UserLabels[userIndex], dataset.UserLabels[j], labelIDF)
					if commonSum > 0 {
						score := commonSum * commonCount /
							math32.Sqrt(weightedSum(dataset.UserLabels[userIndex], labelIDF)) /
							math32.Sqrt(weightedSum(dataset.UserLabels[j], labelIDF)) /
							(commonCount + similarityShrink)
						nearUsers.Push(j, score)
					}
				}
			}
		}

		if m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeRelated ||
			(m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeAuto && nearUsers.Len() == 0) {
			items := dataset.UserFeedback[userIndex]
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
				if j != int32(userIndex) {
					commonSum, commonCount := commonElements(dataset.UserFeedback[userIndex], dataset.UserFeedback[j], itemIDF)
					if commonSum > 0 {
						score := commonSum * commonCount /
							math32.Sqrt(weightedSum(dataset.UserFeedback[userIndex], itemIDF)) /
							math32.Sqrt(weightedSum(dataset.UserFeedback[j], itemIDF)) /
							(commonCount + similarityShrink)
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
		if err := m.CacheClient.SetSorted(cache.Key(cache.UserNeighbors, userId),
			cache.CreateScoredItems(recommends, scores)); err != nil {
			return errors.Trace(err)
		}
		if err := m.CacheClient.Set(
			cache.Time(cache.Key(cache.LastUpdateUserNeighborsTime, userId), time.Now()),
			cache.String(cache.Key(cache.UserNeighborsDigest, userId), m.GorseConfig.UserNeighborDigest())); err != nil {
			return errors.Trace(err)
		}
		FindUserNeighborsSeconds.Observe(time.Since(startTime).Seconds())
		return nil
	})
}

func (m *Master) findUserNeighborsIVF(dataset *ranking.DataSet, labelIDF, itemIDF []float32, completed chan struct{}) error {
	var similarUserNeighbors, relatedUserNeighbors search.VectorIndex
	var userLabelVectors, userFeedbackVectors []search.Vector
	if m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeSimilar ||
		m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeAuto {
		userLabelVectors = make([]search.Vector, dataset.UserCount())
		for i := range userLabelVectors {
			userLabelVectors[i] = search.NewDictionaryVector(dataset.UserLabels[i], labelIDF, nil, false)
		}
		builder := search.NewIVFBuilder(userLabelVectors, m.GorseConfig.Recommend.CacheSize, 1000,
			search.SetIVFNumJobs(m.GorseConfig.Master.NumJobs))
		var recall float32
		similarUserNeighbors, recall = builder.Build(
			m.GorseConfig.Recommend.UserNeighbors.IndexRecall,
			m.GorseConfig.Recommend.UserNeighbors.IndexFitEpoch, true)
		UserNeighborIndexRecall.Set(float64(recall))
		if err := m.CacheClient.Set(cache.String(cache.Key(cache.GlobalMeta, cache.UserNeighborIndexRecall), base.FormatFloat32(recall))); err != nil {
			return errors.Trace(err)
		}
	}
	if m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeRelated ||
		m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeAuto {
		userFeedbackVectors = make([]search.Vector, dataset.UserCount())
		for i := range userFeedbackVectors {
			userFeedbackVectors[i] = search.NewDictionaryVector(dataset.UserFeedback[i], itemIDF, nil, false)
		}
		builder := search.NewIVFBuilder(userFeedbackVectors, m.GorseConfig.Recommend.CacheSize, 1000,
			search.SetIVFNumJobs(m.GorseConfig.Master.NumJobs))
		relatedUserNeighbors, _ = builder.Build(
			m.GorseConfig.Recommend.UserNeighbors.IndexRecall,
			m.GorseConfig.Recommend.UserNeighbors.IndexFitEpoch, true)
	}
	return parallel.Parallel(dataset.UserCount(), m.GorseConfig.Master.NumJobs, func(workerId, userIndex int) error {
		defer func() {
			completed <- struct{}{}
		}()
		userId := dataset.UserIndex.ToName(int32(userIndex))
		if !m.checkUserNeighborCacheTimeout(userId) {
			return nil
		}
		startTime := time.Now()
		var neighbors []int32
		var scores []float32
		if m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeSimilar ||
			m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeAuto {
			neighbors, scores = similarUserNeighbors.Search(userLabelVectors[userIndex], m.GorseConfig.Recommend.CacheSize, true)
		}
		if m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeRelated ||
			m.GorseConfig.Recommend.UserNeighbors.NeighborType == config.NeighborTypeAuto && len(neighbors) == 0 {
			neighbors, scores = relatedUserNeighbors.Search(userFeedbackVectors[userIndex], m.GorseConfig.Recommend.CacheSize, true)
		}
		itemScores := make([]cache.Scored, len(neighbors))
		for i := range scores {
			itemScores[i].Id = dataset.UserIndex.ToName(neighbors[i])
			itemScores[i].Score = float64(-scores[i])
		}
		if err := m.CacheClient.SetSorted(cache.Key(cache.UserNeighbors, userId), itemScores); err != nil {
			return errors.Trace(err)
		}
		if err := m.CacheClient.Set(
			cache.Time(cache.Key(cache.LastUpdateUserNeighborsTime, userId), time.Now()),
			cache.String(cache.Key(cache.UserNeighborsDigest, userId), m.GorseConfig.UserNeighborDigest())); err != nil {
			return errors.Trace(err)
		}
		FindUserNeighborsSeconds.Observe(time.Since(startTime).Seconds())
		return nil
	})
}

func commonElements(a, b []int32, weights []float32) (float32, float32) {
	i, j, sum, count := 0, 0, float32(0), float32(0)
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			sum += weights[a[i]]
			count++
			i++
			j++
		} else if a[i] < b[j] {
			i++
		} else if a[i] > b[j] {
			j++
		}
	}
	return sum, count
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
	var (
		modifiedTime time.Time
		updateTime   time.Time
		cacheDigest  string
		err          error
	)
	// check cache
	if items, err := m.CacheClient.GetSorted(cache.Key(cache.UserNeighbors, userId), 0, -1); err != nil {
		base.Logger().Error("failed to load user neighbors", zap.String("user_id", userId), zap.Error(err))
		return true
	} else if len(items) == 0 {
		return true
	}
	// read digest
	cacheDigest, err = m.CacheClient.Get(cache.Key(cache.UserNeighborsDigest, userId)).String()
	if err != nil {
		if !errors.IsNotFound(err) {
			base.Logger().Error("failed to read user neighbors digest", zap.Error(err))
		}
		return true
	}
	if cacheDigest != m.GorseConfig.UserNeighborDigest() {
		return true
	}
	// read modified time
	modifiedTime, err = m.CacheClient.Get(cache.Key(cache.LastModifyUserTime, userId)).Time()
	if err != nil {
		if !errors.IsNotFound(err) {
			base.Logger().Error("failed to read last modify user time", zap.Error(err))
		}
		return true
	}
	// read update time
	updateTime, err = m.CacheClient.Get(cache.Key(cache.LastUpdateUserNeighborsTime, userId)).Time()
	if err != nil {
		if !errors.IsNotFound(err) {
			base.Logger().Error("failed to read last update user neighbors time", zap.Error(err))
		}
		return true
	}
	// check time
	return updateTime.Unix() <= modifiedTime.Unix()
}

// checkItemNeighborCacheTimeout checks if item neighbor cache stale.
// 1. if cache is empty, stale.
// 2. if modified time > update time, stale.
func (m *Master) checkItemNeighborCacheTimeout(itemId string, categories []string) bool {
	var (
		modifiedTime time.Time
		updateTime   time.Time
		cacheDigest  string
		err          error
	)
	// check cache
	for _, category := range append([]string{""}, categories...) {
		items, err := m.CacheClient.GetSorted(cache.Key(cache.ItemNeighbors, itemId, category), 0, -1)
		if err != nil {
			base.Logger().Error("failed to load item neighbors", zap.String("item_id", itemId), zap.Error(err))
			return true
		} else if len(items) == 0 {
			return true
		}
	}
	// read digest
	cacheDigest, err = m.CacheClient.Get(cache.Key(cache.ItemNeighborsDigest, itemId)).String()
	if err != nil {
		if !errors.IsNotFound(err) {
			base.Logger().Error("failed to read item neighbors digest", zap.Error(err))
		}
		return true
	}
	if cacheDigest != m.GorseConfig.ItemNeighborDigest() {
		return true
	}
	// read modified time
	modifiedTime, err = m.CacheClient.Get(cache.Key(cache.LastModifyItemTime, itemId)).Time()
	if err != nil {
		if !errors.IsNotFound(err) {
			base.Logger().Error("failed to read last modify item time", zap.Error(err))
		}
		return true
	}
	// read update time
	updateTime, err = m.CacheClient.Get(cache.Key(cache.LastUpdateItemNeighborsTime, itemId)).Time()
	if err != nil {
		if !errors.IsNotFound(err) {
			base.Logger().Error("failed to read last update item neighbors time", zap.Error(err))
		}
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
			zap.Strings("positive_feedback_type", m.GorseConfig.Recommend.DataSource.PositiveFeedbackTypes))
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
	CollaborativeFilteringNDCG10.Set(float64(score.NDCG))
	CollaborativeFilteringRecall10.Set(float64(score.Recall))
	CollaborativeFilteringPrecision10.Set(float64(score.Precision))
	if err := m.CacheClient.Set(cache.Time(cache.Key(cache.GlobalMeta, cache.LastFitMatchingModelTime), time.Now())); err != nil {
		base.Logger().Error("failed to write meta", zap.Error(err))
	}

	// caching model
	m.rankingModelMutex.RLock()
	m.localCache.RankingModelName = m.rankingModelName
	m.localCache.RankingModelVersion = m.rankingModelVersion
	m.localCache.RankingModel = rankingModel
	m.localCache.RankingModelScore = score
	m.rankingModelMutex.RUnlock()
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
	m.taskMonitor.Start(TaskAnalyze, 30*len(m.GorseConfig.Recommend.DataSource.PositiveFeedbackTypes))
	for j, feedbackType := range m.GorseConfig.Recommend.DataSource.PositiveFeedbackTypes {
		measurement := cache.Key(PositiveFeedbackRate, feedbackType)
		// pull existed click-through rates
		clickThroughRates, err := m.DataClient.GetMeasurements(measurement, 30)
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
				clickThroughRate, err := m.DataClient.GetClickThroughRate(date, []string{feedbackType},
					m.GorseConfig.Recommend.DataSource.ReadFeedbackTypes)
				if err != nil {
					return errors.Trace(err)
				}
				PositiveFeedbackRateVec.WithLabelValues(feedbackType).Set(float64(len(clickThroughRates)))
				err = m.DataClient.InsertMeasurement(data.Measurement{
					Name:      measurement,
					Timestamp: date,
					Value:     float32(clickThroughRate),
				})
				if err != nil {
					return errors.Trace(err)
				}
				base.Logger().Info("update click through rate",
					zap.String("date", date.String()),
					zap.Duration("time_used", time.Since(startTime)),
					zap.String("positive_feedback_type", feedbackType),
					zap.Float64("positive_feedback_rate", clickThroughRate))
			}
			m.taskMonitor.Update(TaskAnalyze, i+j*30)
		}
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
			zap.Strings("positive_feedback_type", m.GorseConfig.Recommend.DataSource.PositiveFeedbackTypes))
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
	RankingPrecision.Set(float64(score.Precision))
	RankingRecall.Set(float64(score.Recall))
	RankingAUC.Set(float64(score.AUC))
	if err = m.CacheClient.Set(cache.Time(cache.Key(cache.GlobalMeta, cache.LastFitRankingModelTime), time.Now())); err != nil {
		base.Logger().Error("failed to write meta", zap.Error(err))
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
			zap.Strings("positive_feedback_type", m.GorseConfig.Recommend.DataSource.PositiveFeedbackTypes))
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
			zap.Strings("positive_feedback_type", m.GorseConfig.Recommend.DataSource.PositiveFeedbackTypes))
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

func (m *Master) runCacheGarbageCollectionTask() error {
	if m.rankingTrainSet == nil {
		return nil
	}
	base.Logger().Info("start cache garbage collection")
	m.taskMonitor.Start(TaskCacheGarbageCollection, m.rankingTrainSet.UserCount()*9+m.rankingTrainSet.ItemCount()*4)
	var scanCount int
	err := m.CacheClient.Scan(func(s string) error {
		splits := strings.Split(s, "/")
		if len(splits) <= 1 {
			return nil
		}
		scanCount++
		m.taskMonitor.Update(TaskCacheGarbageCollection, scanCount)
		switch splits[0] {
		case cache.UserNeighbors, cache.UserNeighborsDigest, cache.IgnoreItems,
			cache.OfflineRecommend, cache.OfflineRecommendDigest, cache.CollaborativeRecommend,
			cache.LastModifyUserTime, cache.LastUpdateUserNeighborsTime, cache.LastUpdateUserRecommendTime:
			userId := splits[1]
			// check user in dataset
			if m.rankingTrainSet != nil && m.rankingTrainSet.UserIndex.ToNumber(userId) != base.NotId {
				return nil
			}
			// check user in database
			_, err := m.DataClient.GetUser(userId)
			if !errors.IsNotFound(err) {
				if err != nil {
					base.Logger().Error("failed to load user", zap.String("user_id", userId), zap.Error(err))
				}
				return err
			}
			// delete user cache
			switch splits[0] {
			case cache.UserNeighbors, cache.IgnoreItems, cache.CollaborativeRecommend, cache.OfflineRecommend:
				err = m.CacheClient.SetSorted(s, nil)
			case cache.UserNeighborsDigest, cache.OfflineRecommendDigest,
				cache.LastModifyUserTime, cache.LastUpdateUserNeighborsTime, cache.LastUpdateUserRecommendTime:
				err = m.CacheClient.Delete(s)
			}
			if err != nil {
				return errors.Trace(err)
			}
		case cache.ItemNeighbors, cache.ItemNeighborsDigest, cache.LastModifyItemTime, cache.LastUpdateItemNeighborsTime:
			itemId := splits[1]
			// check item in dataset
			if m.rankingTrainSet != nil && m.rankingTrainSet.ItemIndex.ToNumber(itemId) != base.NotId {
				return nil
			}
			// check item in database
			_, err := m.DataClient.GetItem(itemId)
			if !errors.IsNotFound(err) {
				if err != nil {
					base.Logger().Error("failed to load item", zap.String("item_id", itemId), zap.Error(err))
				}
				return err
			}
			// delete item cache
			switch splits[0] {
			case cache.ItemNeighbors:
				err = m.CacheClient.SetSorted(s, nil)
			case cache.ItemNeighborsDigest, cache.LastModifyItemTime, cache.LastUpdateItemNeighborsTime:
				err = m.CacheClient.Delete(s)
			}
			if err != nil {
				return errors.Trace(err)
			}
		}
		return nil
	})
	m.taskMonitor.Finish(TaskCacheGarbageCollection)
	return errors.Trace(err)
}

// LoadDataFromDatabase loads dataset from data store.
func (m *Master) LoadDataFromDatabase(database data.Database, posFeedbackTypes, readTypes []string, itemTTL, positiveFeedbackTTL uint) (
	rankingDataset *ranking.DataSet, clickDataset *click.Dataset, latestItems map[string][]cache.Scored, popularItems map[string][]cache.Scored, err error) {
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
	if m.GorseConfig.Recommend.Popular.PopularWindow > 0 {
		timeWindowLimit = time.Now().Add(-m.GorseConfig.Recommend.Popular.PopularWindow)
	}
	rankingDataset = ranking.NewMapIndexDataset()

	// create filers for latest items
	latestItemsFilters := make(map[string]*heap.TopKStringFilter)
	latestItemsFilters[""] = heap.NewTopKStringFilter(m.GorseConfig.Recommend.CacheSize)

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
	itemLabelIndex := base.NewMapIndex()
	start = time.Now()
	itemChan, errChan := database.GetItemStream(batchSize, itemTimeLimit)
	for items := range itemChan {
		for _, item := range items {
			rankingDataset.AddItem(item.ItemId)
			itemIndex := rankingDataset.ItemIndex.ToNumber(item.ItemId)
			if len(rankingDataset.ItemLabels) == int(itemIndex) {
				rankingDataset.ItemLabels = append(rankingDataset.ItemLabels, nil)
				rankingDataset.HiddenItems = append(rankingDataset.HiddenItems, false)
				rankingDataset.ItemCategories = append(rankingDataset.ItemCategories, item.Categories)
				rankingDataset.CategorySet.Add(item.Categories...)
			}
			rankingDataset.ItemLabels[itemIndex] = make([]int32, len(item.Labels))
			for i, label := range item.Labels {
				itemLabelIndex.Add(label)
				rankingDataset.ItemLabels[itemIndex][i] = itemLabelIndex.ToNumber(label)
			}
			if item.IsHidden { // set hidden flag
				rankingDataset.HiddenItems[itemIndex] = true
			} else if !item.Timestamp.IsZero() { // add items to the latest items filter
				latestItemsFilters[""].Push(item.ItemId, float64(item.Timestamp.Unix()))
				for _, category := range item.Categories {
					if _, exist := latestItemsFilters[category]; !exist {
						latestItemsFilters[category] = heap.NewTopKStringFilter(m.GorseConfig.Recommend.CacheSize)
					}
					latestItemsFilters[category].Push(item.ItemId, float64(item.Timestamp.Unix()))
				}
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
			if f.Timestamp.After(timeWindowLimit) && !rankingDataset.HiddenItems[itemIndex] {
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

	// collect latest items
	latestItems = make(map[string][]cache.Scored)
	for category, latestItemsFilter := range latestItemsFilters {
		items, scores := latestItemsFilter.PopAll()
		latestItems[category] = cache.CreateScoredItems(items, scores)
	}

	// collect popular items
	popularItemFilters := make(map[string]*heap.TopKStringFilter)
	popularItemFilters[""] = heap.NewTopKStringFilter(m.GorseConfig.Recommend.CacheSize)
	for itemIndex, val := range popularCount {
		itemId := rankingDataset.ItemIndex.ToName(int32(itemIndex))
		popularItemFilters[""].Push(itemId, float64(val))
		for _, category := range rankingDataset.ItemCategories[itemIndex] {
			if _, exist := popularItemFilters[category]; !exist {
				popularItemFilters[category] = heap.NewTopKStringFilter(m.GorseConfig.Recommend.CacheSize)
			}
			popularItemFilters[category].Push(itemId, float64(val))
		}
	}
	popularItems = make(map[string][]cache.Scored)
	for category, popularItemFilter := range popularItemFilters {
		items, scores := popularItemFilter.PopAll()
		popularItems[category] = cache.CreateScoredItems(items, scores)
	}

	m.taskMonitor.Finish(TaskLoadDataset)
	return rankingDataset, clickDataset, latestItems, popularItems, nil
}
