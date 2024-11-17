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
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chewxy/math32"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/encoding"
	"github.com/zhenghaoz/gorse/base/heap"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/parallel"
	"github.com/zhenghaoz/gorse/base/progress"
	"github.com/zhenghaoz/gorse/base/search"
	"github.com/zhenghaoz/gorse/base/sizeof"
	"github.com/zhenghaoz/gorse/base/task"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"modernc.org/sortutil"
)

const (
	PositiveFeedbackRate = "PositiveFeedbackRate"

	TaskLoadDataset            = "Load dataset"
	TaskFindItemNeighbors      = "Find neighbors of items"
	TaskFindUserNeighbors      = "Find neighbors of users"
	TaskFitRankingModel        = "Fit collaborative filtering model"
	TaskFitClickModel          = "Fit click-through rate prediction model"
	TaskSearchRankingModel     = "Search collaborative filtering  model"
	TaskSearchClickModel       = "Search click-through rate prediction model"
	TaskCacheGarbageCollection = "Collect garbage in cache"

	batchSize        = 10000
	similarityShrink = 100
)

type Task interface {
	name() string
	priority() int
	run(ctx context.Context, j *task.JobsAllocator) error
}

// runLoadDatasetTask loads dataset.
func (m *Master) runLoadDatasetTask() error {
	ctx, span := m.tracer.Start(context.Background(), "Load Dataset", 1)
	defer span.End()

	initialStartTime := time.Now()
	log.Logger().Info("load dataset",
		zap.Strings("positive_feedback_types", m.Config.Recommend.DataSource.PositiveFeedbackTypes),
		zap.Strings("read_feedback_types", m.Config.Recommend.DataSource.ReadFeedbackTypes),
		zap.Uint("item_ttl", m.Config.Recommend.DataSource.ItemTTL),
		zap.Uint("feedback_ttl", m.Config.Recommend.DataSource.PositiveFeedbackTTL))
	evaluator := NewOnlineEvaluator()
	rankingDataset, clickDataset, latestItems, popularItems, err := m.LoadDataFromDatabase(ctx, m.DataClient,
		m.Config.Recommend.DataSource.PositiveFeedbackTypes,
		m.Config.Recommend.DataSource.ReadFeedbackTypes,
		m.Config.Recommend.DataSource.ItemTTL,
		m.Config.Recommend.DataSource.PositiveFeedbackTTL,
		evaluator)
	if err != nil {
		return errors.Trace(err)
	}

	// save popular items to cache
	if err = m.CacheClient.AddScores(ctx, cache.PopularItems, "", popularItems.ToSlice()); err != nil {
		log.Logger().Error("failed to cache popular items", zap.Error(err))
	}
	if err = m.CacheClient.DeleteScores(ctx, []string{cache.PopularItems}, cache.ScoreCondition{Before: &popularItems.Timestamp}); err != nil {
		log.Logger().Error("failed to reclaim outdated items", zap.Error(err))
	}
	if err = m.CacheClient.Set(ctx, cache.Time(cache.Key(cache.GlobalMeta, cache.LastUpdatePopularItemsTime), time.Now())); err != nil {
		log.Logger().Error("failed to write latest update popular items time", zap.Error(err))
	}

	// save the latest items to cache
	if err = m.CacheClient.AddScores(ctx, cache.LatestItems, "", latestItems.ToSlice()); err != nil {
		log.Logger().Error("failed to cache latest items", zap.Error(err))
	}
	if err = m.CacheClient.DeleteScores(ctx, []string{cache.LatestItems}, cache.ScoreCondition{Before: &latestItems.Timestamp}); err != nil {
		log.Logger().Error("failed to reclaim outdated items", zap.Error(err))
	}
	if err = m.CacheClient.Set(ctx, cache.Time(cache.Key(cache.GlobalMeta, cache.LastUpdateLatestItemsTime), time.Now())); err != nil {
		log.Logger().Error("failed to write latest update latest items time", zap.Error(err))
	}

	// write statistics to database
	UsersTotal.Set(float64(rankingDataset.UserCount()))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumUsers), rankingDataset.UserCount())); err != nil {
		log.Logger().Error("failed to write number of users", zap.Error(err))
	}
	ItemsTotal.Set(float64(rankingDataset.ItemCount()))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumItems), rankingDataset.ItemCount())); err != nil {
		log.Logger().Error("failed to write number of items", zap.Error(err))
	}
	ImplicitFeedbacksTotal.Set(float64(rankingDataset.Count()))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumTotalPosFeedbacks), rankingDataset.Count())); err != nil {
		log.Logger().Error("failed to write number of positive feedbacks", zap.Error(err))
	}
	UserLabelsTotal.Set(float64(clickDataset.Index.CountUserLabels()))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumUserLabels), int(clickDataset.Index.CountUserLabels()))); err != nil {
		log.Logger().Error("failed to write number of user labels", zap.Error(err))
	}
	ItemLabelsTotal.Set(float64(clickDataset.Index.CountItemLabels()))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumItemLabels), int(clickDataset.Index.CountItemLabels()))); err != nil {
		log.Logger().Error("failed to write number of item labels", zap.Error(err))
	}
	ImplicitFeedbacksTotal.Set(float64(rankingDataset.Count()))
	PositiveFeedbacksTotal.Set(float64(clickDataset.PositiveCount))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumValidPosFeedbacks), clickDataset.PositiveCount)); err != nil {
		log.Logger().Error("failed to write number of positive feedbacks", zap.Error(err))
	}
	NegativeFeedbackTotal.Set(float64(clickDataset.NegativeCount))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumValidNegFeedbacks), clickDataset.NegativeCount)); err != nil {
		log.Logger().Error("failed to write number of negative feedbacks", zap.Error(err))
	}

	// evaluate positive feedback rate
	points := evaluator.Evaluate()
	if err = m.CacheClient.AddTimeSeriesPoints(ctx, points); err != nil {
		log.Logger().Error("failed to insert measurement", zap.Error(err))
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
	if err = m.CacheClient.SetSet(ctx, cache.ItemCategories, rankingDataset.CategorySet.ToSlice()...); err != nil {
		log.Logger().Error("failed to write categories to cache", zap.Error(err))
	}

	// split ranking dataset
	startTime := time.Now()
	m.rankingDataMutex.Lock()
	m.rankingTrainSet, m.rankingTestSet = rankingDataset.Split(0, 0)
	rankingDataset = nil
	m.rankingDataMutex.Unlock()
	LoadDatasetStepSecondsVec.WithLabelValues("split_ranking_dataset").Set(time.Since(startTime).Seconds())
	MemoryInUseBytesVec.WithLabelValues("collaborative_filtering_train_set").Set(float64(m.rankingTrainSet.Bytes()))
	MemoryInUseBytesVec.WithLabelValues("collaborative_filtering_test_set").Set(float64(m.rankingTestSet.Bytes()))

	// split click dataset
	startTime = time.Now()
	m.clickDataMutex.Lock()
	m.clickTrainSet, m.clickTestSet = clickDataset.Split(0.2, 0)
	clickDataset = nil
	m.clickDataMutex.Unlock()
	LoadDatasetStepSecondsVec.WithLabelValues("split_click_dataset").Set(time.Since(startTime).Seconds())
	MemoryInUseBytesVec.WithLabelValues("ranking_train_set").Set(float64(sizeof.DeepSize(m.clickTrainSet)))
	MemoryInUseBytesVec.WithLabelValues("ranking_test_set").Set(float64(sizeof.DeepSize(m.clickTestSet)))

	LoadDatasetTotalSeconds.Set(time.Since(initialStartTime).Seconds())
	return nil
}

// FindItemNeighborsTask updates neighbors of items.
type FindItemNeighborsTask struct {
	*Master
	lastNumItems    int
	lastNumFeedback int
}

func NewFindItemNeighborsTask(m *Master) *FindItemNeighborsTask {
	return &FindItemNeighborsTask{Master: m}
}

func (t *FindItemNeighborsTask) name() string {
	return TaskFindItemNeighbors
}

func (t *FindItemNeighborsTask) priority() int {
	return -t.rankingTrainSet.ItemCount() * t.rankingTrainSet.ItemCount()
}

func (t *FindItemNeighborsTask) run(ctx context.Context, j *task.JobsAllocator) error {
	t.rankingDataMutex.RLock()
	defer t.rankingDataMutex.RUnlock()
	dataset := t.rankingTrainSet
	numItems := dataset.ItemCount()
	numFeedback := dataset.Count()

	newCtx, span := t.tracer.Start(ctx, "Find Item Neighbors", dataset.ItemCount())
	defer span.End()

	if numItems == 0 {
		return nil
	} else if numItems == t.lastNumItems && numFeedback == t.lastNumFeedback {
		log.Logger().Info("No item neighbors need to be updated.")
		return nil
	}

	startTaskTime := time.Now()
	log.Logger().Info("start searching neighbors of items",
		zap.Int("n_cache", t.Config.Recommend.CacheSize))
	// create progress tracker
	completed := make(chan struct{}, 1000)
	go func() {
		completedCount, previousCount := 0, 0
		ticker := time.NewTicker(time.Second * 10)
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
					log.Logger().Debug("searching neighbors of items",
						zap.Int("n_complete_items", completedCount),
						zap.Int("n_items", dataset.ItemCount()),
						zap.Int("throughput", throughput/10))
					span.Add(throughput)
				}
			}
		}
	}()

	userIDF := make([]float32, dataset.UserCount())
	if t.Config.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeRelated ||
		t.Config.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeAuto {
		for _, feedbacks := range dataset.ItemFeedback {
			sort.Sort(sortutil.Int32Slice(feedbacks))
		}
		// inverse document frequency of users
		for i := range dataset.UserFeedback {
			if dataset.ItemCount() == len(dataset.UserFeedback[i]) {
				userIDF[i] = 1
			} else {
				userIDF[i] = math32.Log(float32(dataset.ItemCount()) / float32(len(dataset.UserFeedback[i])))
			}
		}
	}
	labeledItems := make([][]int32, dataset.NumItemLabels)
	labelIDF := make([]float32, dataset.NumItemLabels)
	if t.Config.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeSimilar ||
		t.Config.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeAuto {
		for i, itemLabels := range dataset.ItemFeatures {
			sort.Slice(itemLabels, func(i, j int) bool {
				return itemLabels[i].A < itemLabels[j].A
			})
			for _, label := range itemLabels {
				labeledItems[label.A] = append(labeledItems[label.A], int32(i))
			}
		}
		// inverse document frequency of labels
		for i := range labeledItems {
			labeledItems[i] = lo.Uniq(labeledItems[i])
			if dataset.ItemCount() == len(labeledItems[i]) {
				labelIDF[i] = 1
			} else {
				labelIDF[i] = math32.Log(float32(dataset.ItemCount()) / float32(len(labeledItems[i])))
			}
		}
	}

	start := time.Now()
	var err error
	if t.Config.Recommend.ItemNeighbors.EnableIndex {
		err = t.findItemNeighborsIVF(dataset, labelIDF, userIDF, completed, j)
	} else {
		err = t.findItemNeighborsBruteForce(dataset, labeledItems, labelIDF, userIDF, completed, j)
	}
	searchTime := time.Since(start)

	close(completed)
	if err != nil {
		log.Logger().Error("failed to searching neighbors of items", zap.Error(err))
		progress.Fail(newCtx, err)
		FindItemNeighborsTotalSeconds.Set(0)
	} else {
		if err := t.CacheClient.Set(ctx, cache.Time(cache.Key(cache.GlobalMeta, cache.LastUpdateItemNeighborsTime), time.Now())); err != nil {
			log.Logger().Error("failed to set neighbors of items update time", zap.Error(err))
		}
		log.Logger().Info("complete searching neighbors of items",
			zap.String("search_time", searchTime.String()))
		FindItemNeighborsTotalSeconds.Set(time.Since(startTaskTime).Seconds())
	}

	t.lastNumItems = numItems
	t.lastNumFeedback = numFeedback
	return nil
}

func (m *Master) findItemNeighborsBruteForce(dataset *ranking.DataSet, labeledItems [][]int32,
	labelIDF, userIDF []float32, completed chan struct{}, j *task.JobsAllocator) error {
	ctx := context.Background()
	var (
		updateItemCount     atomic.Float64
		findNeighborSeconds atomic.Float64
	)

	var vector VectorsInterface
	switch m.Config.Recommend.ItemNeighbors.NeighborType {
	case config.NeighborTypeSimilar:
		vector = NewVectors(lo.Map(dataset.ItemFeatures, func(features []lo.Tuple2[int32, float32], _ int) []int32 {
			indices, _ := lo.Unzip2(features)
			return indices
		}), labeledItems, labelIDF)
	case config.NeighborTypeRelated:
		vector = NewVectors(dataset.ItemFeedback, dataset.UserFeedback, userIDF)
	case config.NeighborTypeAuto:
		vector = NewDualVectors(
			NewVectors(lo.Map(dataset.ItemFeatures, func(features []lo.Tuple2[int32, float32], _ int) []int32 {
				indices, _ := lo.Unzip2(features)
				return indices
			}), labeledItems, labelIDF),
			NewVectors(dataset.ItemFeedback, dataset.UserFeedback, userIDF))
	default:
		return errors.NotImplementedf("item neighbor type `%v`", m.Config.Recommend.ItemNeighbors.NeighborType)
	}

	err := parallel.DynamicParallel(dataset.ItemCount(), j, func(workerId, itemIndex int) error {
		defer func() {
			completed <- struct{}{}
		}()
		startSearchTime := time.Now()
		itemId := dataset.ItemIndex.ToName(int32(itemIndex))
		if !m.checkItemNeighborCacheTimeout(itemId, dataset.CategorySet.ToSlice()) {
			return nil
		}
		updateItemCount.Add(1)
		startTime := time.Now()
		nearItemsFilters := make(map[string]*heap.TopKFilter[int32, float64])
		nearItemsFilters[""] = heap.NewTopKFilter[int32, float64](m.Config.Recommend.CacheSize)
		for _, category := range dataset.CategorySet.ToSlice() {
			nearItemsFilters[category] = heap.NewTopKFilter[int32, float64](m.Config.Recommend.CacheSize)
		}

		adjacencyItems := vector.Neighbors(itemIndex)
		for _, j := range adjacencyItems {
			if j != int32(itemIndex) && !dataset.HiddenItems[j] {
				score := vector.Distance(itemIndex, int(j))
				if score > 0 {
					nearItemsFilters[""].Push(j, float64(score))
					for _, category := range dataset.ItemCategories[j] {
						nearItemsFilters[category].Push(j, float64(score))
					}
				}
			}
		}

		aggregator := cache.NewDocumentAggregator(startSearchTime)
		for category, nearItemsFilter := range nearItemsFilters {
			elem, scores := nearItemsFilter.PopAll()
			recommends := make([]string, len(elem))
			for i := range recommends {
				recommends[i] = dataset.ItemIndex.ToName(elem[i])
			}
			aggregator.Add(category, recommends, scores)
		}
		if err := m.CacheClient.AddScores(ctx, cache.ItemNeighbors, itemId, aggregator.ToSlice()); err != nil {
			return errors.Trace(err)
		}
		if err := m.CacheClient.DeleteScores(ctx, []string{cache.ItemNeighbors}, cache.ScoreCondition{
			Subset: proto.String(itemId),
			Before: &aggregator.Timestamp,
		}); err != nil {
			return errors.Trace(err)
		}
		if err := m.CacheClient.Set(
			ctx,
			cache.Time(cache.Key(cache.LastUpdateItemNeighborsTime, itemId), time.Now()),
			cache.String(cache.Key(cache.ItemNeighborsDigest, itemId), m.Config.ItemNeighborDigest())); err != nil {
			return errors.Trace(err)
		}
		findNeighborSeconds.Add(time.Since(startTime).Seconds())
		return nil
	})
	if err != nil {
		return errors.Trace(err)
	}
	UpdateItemNeighborsTotal.Set(updateItemCount.Load())
	FindItemNeighborsSecondsVec.WithLabelValues("find_item_neighbors").Set(findNeighborSeconds.Load())
	FindItemNeighborsSecondsVec.WithLabelValues("build_index").Set(0)
	ItemNeighborIndexRecall.Set(1)
	return nil
}

func (m *Master) findItemNeighborsIVF(dataset *ranking.DataSet, labelIDF, userIDF []float32, completed chan struct{}, j *task.JobsAllocator) error {
	var (
		updateItemCount     atomic.Float64
		findNeighborSeconds atomic.Float64
		buildIndexSeconds   atomic.Float64
	)
	ctx := context.Background()

	// build index
	buildStart := time.Now()
	var index search.VectorIndex
	var vectors []search.Vector
	switch m.Config.Recommend.ItemNeighbors.NeighborType {
	case config.NeighborTypeSimilar:
		vectors = lo.Map(dataset.ItemFeatures, func(_ []lo.Tuple2[int32, float32], i int) search.Vector {
			indices, _ := lo.Unzip2(dataset.ItemFeatures[i])
			return search.NewDictionaryVector(indices, labelIDF, dataset.ItemCategories[i], dataset.HiddenItems[i])
		})
	case config.NeighborTypeRelated:
		vectors = lo.Map(dataset.ItemFeatures, func(_ []lo.Tuple2[int32, float32], i int) search.Vector {
			return search.NewDictionaryVector(dataset.ItemFeedback[i], userIDF, dataset.ItemCategories[i], dataset.HiddenItems[i])
		})
	case config.NeighborTypeAuto:
		vectors = lo.Map(dataset.ItemFeatures, func(_ []lo.Tuple2[int32, float32], i int) search.Vector {
			indices, _ := lo.Unzip2(dataset.ItemFeatures[i])
			return NewDualDictionaryVector(indices, labelIDF, dataset.ItemFeedback[i], userIDF, dataset.ItemCategories[i], dataset.HiddenItems[i])
		})
	default:
		return errors.NotImplementedf("item neighbor type `%v`", m.Config.Recommend.ItemNeighbors.NeighborType)
	}

	builder := search.NewIVFBuilder(vectors, m.Config.Recommend.CacheSize,
		search.SetIVFJobsAllocator(j))
	var recall float32
	index, recall = builder.Build(m.Config.Recommend.ItemNeighbors.IndexRecall,
		m.Config.Recommend.ItemNeighbors.IndexFitEpoch,
		true)
	ItemNeighborIndexRecall.Set(float64(recall))
	if err := m.CacheClient.Set(ctx, cache.String(cache.Key(cache.GlobalMeta, cache.ItemNeighborIndexRecall), encoding.FormatFloat32(recall))); err != nil {
		return errors.Trace(err)
	}
	buildIndexSeconds.Add(time.Since(buildStart).Seconds())

	err := parallel.DynamicParallel(dataset.ItemCount(), j, func(workerId, itemIndex int) error {
		defer func() {
			completed <- struct{}{}
		}()
		startSearchTime := time.Now()
		itemId := dataset.ItemIndex.ToName(int32(itemIndex))
		if !m.checkItemNeighborCacheTimeout(itemId, dataset.CategorySet.ToSlice()) {
			return nil
		}
		updateItemCount.Add(1)
		startTime := time.Now()
		var neighbors map[string][]int32
		var scores map[string][]float32
		if m.Config.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeSimilar ||
			m.Config.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeAuto {
			neighbors, scores = index.MultiSearch(vectors[itemIndex], dataset.CategorySet.ToSlice(),
				m.Config.Recommend.CacheSize, true)
		}
		if m.Config.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeRelated ||
			m.Config.Recommend.ItemNeighbors.NeighborType == config.NeighborTypeAuto && len(neighbors[""]) == 0 {
			neighbors, scores = index.MultiSearch(vectors[itemIndex], dataset.CategorySet.ToSlice(),
				m.Config.Recommend.CacheSize, true)
		}
		aggregator := cache.NewDocumentAggregator(startSearchTime)
		for category := range neighbors {
			if categoryNeighbors, exist := neighbors[category]; exist && len(categoryNeighbors) > 0 {
				resultValues, resultScores := make([]string, len(neighbors[category])), make([]float64, len(neighbors[category]))
				for i := range scores[category] {
					resultValues[i] = dataset.ItemIndex.ToName(neighbors[category][i])
					resultScores[i] = float64(-scores[category][i])
				}
				aggregator.Add(category, resultValues, resultScores)
			}
		}
		if err := m.CacheClient.AddScores(ctx, cache.ItemNeighbors, itemId, aggregator.ToSlice()); err != nil {
			return errors.Trace(err)
		}
		if err := m.CacheClient.DeleteScores(ctx, []string{cache.ItemNeighbors}, cache.ScoreCondition{
			Subset: proto.String(itemId),
			Before: &aggregator.Timestamp,
		}); err != nil {
			return errors.Trace(err)
		}
		if err := m.CacheClient.Set(
			ctx,
			cache.Time(cache.Key(cache.LastUpdateItemNeighborsTime, itemId), time.Now()),
			cache.String(cache.Key(cache.ItemNeighborsDigest, itemId), m.Config.ItemNeighborDigest())); err != nil {
			return errors.Trace(err)
		}
		findNeighborSeconds.Add(time.Since(startTime).Seconds())
		return nil
	})
	if err != nil {
		return errors.Trace(err)
	}
	UpdateItemNeighborsTotal.Set(updateItemCount.Load())
	FindItemNeighborsSecondsVec.WithLabelValues("find_item_neighbors").Set(findNeighborSeconds.Load())
	FindItemNeighborsSecondsVec.WithLabelValues("build_index").Set(buildIndexSeconds.Load())
	return nil
}

// FindUserNeighborsTask updates neighbors of users.
type FindUserNeighborsTask struct {
	*Master
	lastNumUsers    int
	lastNumFeedback int
}

func NewFindUserNeighborsTask(m *Master) *FindUserNeighborsTask {
	return &FindUserNeighborsTask{Master: m}
}

func (t *FindUserNeighborsTask) name() string {
	return TaskFindUserNeighbors
}

func (t *FindUserNeighborsTask) priority() int {
	return -t.rankingTrainSet.UserCount() * t.rankingTrainSet.UserCount()
}

func (t *FindUserNeighborsTask) run(ctx context.Context, j *task.JobsAllocator) error {
	t.rankingDataMutex.RLock()
	defer t.rankingDataMutex.RUnlock()
	dataset := t.rankingTrainSet
	numUsers := dataset.UserCount()
	numFeedback := dataset.Count()

	newCtx, span := t.tracer.Start(ctx, "Find User Neighbors", dataset.UserCount())
	defer span.End()

	if numUsers == 0 {
		return nil
	} else if numUsers == t.lastNumUsers && numFeedback == t.lastNumFeedback {
		log.Logger().Info("No update of user neighbors needed.")
		return nil
	}

	startTaskTime := time.Now()
	log.Logger().Info("start searching neighbors of users",
		zap.Int("n_cache", t.Config.Recommend.CacheSize))
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
					log.Logger().Debug("searching neighbors of users",
						zap.Int("n_complete_users", completedCount),
						zap.Int("n_users", dataset.UserCount()),
						zap.Int("throughput", throughput))
					span.Add(throughput)
				}
			}
		}
	}()

	itemIDF := make([]float32, dataset.ItemCount())
	if t.Config.Recommend.UserNeighbors.NeighborType == config.NeighborTypeRelated ||
		t.Config.Recommend.UserNeighbors.NeighborType == config.NeighborTypeAuto {
		for _, feedbacks := range dataset.UserFeedback {
			sort.Sort(sortutil.Int32Slice(feedbacks))
		}
		// inverse document frequency of items
		for i := range dataset.ItemFeedback {
			if dataset.UserCount() == len(dataset.ItemFeedback[i]) {
				itemIDF[i] = 1
			} else {
				itemIDF[i] = math32.Log(float32(dataset.UserCount()) / float32(len(dataset.ItemFeedback[i])))
			}
		}
	}
	labeledUsers := make([][]int32, dataset.NumUserLabels)
	labelIDF := make([]float32, dataset.NumUserLabels)
	if t.Config.Recommend.UserNeighbors.NeighborType == config.NeighborTypeSimilar ||
		t.Config.Recommend.UserNeighbors.NeighborType == config.NeighborTypeAuto {
		for i, userLabels := range dataset.UserFeatures {
			sort.Slice(userLabels, func(i, j int) bool {
				return userLabels[i].A < userLabels[j].A
			})
			for _, label := range userLabels {
				labeledUsers[label.A] = append(labeledUsers[label.A], int32(i))
			}
		}
		// inverse document frequency of labels
		for i := range labeledUsers {
			labeledUsers[i] = lo.Uniq(labeledUsers[i])
			if dataset.UserCount() == len(labeledUsers[i]) {
				labelIDF[i] = 1
			} else {
				labelIDF[i] = math32.Log(float32(dataset.UserCount()) / float32(len(labeledUsers[i])))
			}
		}
	}

	start := time.Now()
	var err error
	if t.Config.Recommend.UserNeighbors.EnableIndex {
		err = t.findUserNeighborsIVF(newCtx, dataset, labelIDF, itemIDF, completed, j)
	} else {
		err = t.findUserNeighborsBruteForce(newCtx, dataset, labeledUsers, labelIDF, itemIDF, completed, j)
	}
	searchTime := time.Since(start)

	close(completed)
	if err != nil {
		log.Logger().Error("failed to searching neighbors of users", zap.Error(err))
		progress.Fail(newCtx, err)
		FindUserNeighborsTotalSeconds.Set(0)
	} else {
		if err := t.CacheClient.Set(ctx, cache.Time(cache.Key(cache.GlobalMeta, cache.LastUpdateUserNeighborsTime), time.Now())); err != nil {
			log.Logger().Error("failed to set neighbors of users update time", zap.Error(err))
		}
		log.Logger().Info("complete searching neighbors of users",
			zap.String("search_time", searchTime.String()))
		FindUserNeighborsTotalSeconds.Set(time.Since(startTaskTime).Seconds())
	}

	t.lastNumUsers = numUsers
	t.lastNumFeedback = numFeedback
	return nil
}

func (m *Master) findUserNeighborsBruteForce(ctx context.Context, dataset *ranking.DataSet, labeledUsers [][]int32, labelIDF, itemIDF []float32, completed chan struct{}, j *task.JobsAllocator) error {
	var (
		updateUserCount     atomic.Float64
		findNeighborSeconds atomic.Float64
	)

	var vectors VectorsInterface
	switch m.Config.Recommend.UserNeighbors.NeighborType {
	case config.NeighborTypeSimilar:
		vectors = NewVectors(lo.Map(dataset.UserFeatures, func(features []lo.Tuple2[int32, float32], _ int) []int32 {
			indices, _ := lo.Unzip2(features)
			return indices
		}), labeledUsers, labelIDF)
	case config.NeighborTypeRelated:
		vectors = NewVectors(dataset.UserFeedback, dataset.ItemFeedback, itemIDF)
	case config.NeighborTypeAuto:
		vectors = NewDualVectors(
			NewVectors(lo.Map(dataset.UserFeatures, func(features []lo.Tuple2[int32, float32], _ int) []int32 {
				indices, _ := lo.Unzip2(features)
				return indices
			}), labeledUsers, labelIDF),
			NewVectors(dataset.UserFeedback, dataset.ItemFeedback, itemIDF))
	default:
		return errors.NotImplementedf("user neighbor type `%v`", m.Config.Recommend.UserNeighbors.NeighborType)
	}

	err := parallel.DynamicParallel(dataset.UserCount(), j, func(workerId, userIndex int) error {
		defer func() {
			completed <- struct{}{}
		}()
		startSearchTime := time.Now()
		userId := dataset.UserIndex.ToName(int32(userIndex))
		if !m.checkUserNeighborCacheTimeout(userId) {
			return nil
		}
		updateUserCount.Add(1)
		startTime := time.Now()
		nearUsers := heap.NewTopKFilter[int32, float64](m.Config.Recommend.CacheSize)

		adjacencyUsers := vectors.Neighbors(userIndex)
		for _, j := range adjacencyUsers {
			if j != int32(userIndex) {
				score := vectors.Distance(userIndex, int(j))
				if score > 0 {
					nearUsers.Push(j, float64(score))
				}
			}
		}

		elem, scores := nearUsers.PopAll()
		recommends := make([]string, len(elem))
		for i := range recommends {
			recommends[i] = dataset.UserIndex.ToName(elem[i])
		}
		aggregator := cache.NewDocumentAggregator(startSearchTime)
		aggregator.Add("", recommends, scores)
		if err := m.CacheClient.AddScores(ctx, cache.UserNeighbors, userId, aggregator.ToSlice()); err != nil {
			return errors.Trace(err)
		}
		if err := m.CacheClient.DeleteScores(ctx, []string{cache.UserNeighbors}, cache.ScoreCondition{
			Subset: proto.String(userId),
			Before: &aggregator.Timestamp,
		}); err != nil {
			return errors.Trace(err)
		}
		if err := m.CacheClient.Set(
			ctx,
			cache.Time(cache.Key(cache.LastUpdateUserNeighborsTime, userId), time.Now()),
			cache.String(cache.Key(cache.UserNeighborsDigest, userId), m.Config.UserNeighborDigest())); err != nil {
			return errors.Trace(err)
		}
		findNeighborSeconds.Add(time.Since(startTime).Seconds())
		return nil
	})
	if err != nil {
		return errors.Trace(err)
	}
	UpdateUserNeighborsTotal.Set(updateUserCount.Load())
	FindUserNeighborsSecondsVec.WithLabelValues("find_item_neighbors").Set(findNeighborSeconds.Load())
	FindUserNeighborsSecondsVec.WithLabelValues("build_index").Set(0)
	UserNeighborIndexRecall.Set(1)
	return nil
}

func (m *Master) findUserNeighborsIVF(ctx context.Context, dataset *ranking.DataSet, labelIDF, itemIDF []float32, completed chan struct{}, j *task.JobsAllocator) error {
	var (
		updateUserCount     atomic.Float64
		buildIndexSeconds   atomic.Float64
		findNeighborSeconds atomic.Float64
	)
	// build index
	buildStart := time.Now()
	var index search.VectorIndex
	var vectors []search.Vector
	switch m.Config.Recommend.UserNeighbors.NeighborType {
	case config.NeighborTypeSimilar:
		vectors = lo.Map(dataset.UserFeatures, func(features []lo.Tuple2[int32, float32], _ int) search.Vector {
			indices, _ := lo.Unzip2(features)
			return search.NewDictionaryVector(indices, labelIDF, nil, false)
		})
	case config.NeighborTypeRelated:
		vectors = lo.Map(dataset.UserFeedback, func(indices []int32, _ int) search.Vector {
			return search.NewDictionaryVector(indices, itemIDF, nil, false)
		})
	case config.NeighborTypeAuto:
		vectors = make([]search.Vector, dataset.UserCount())
		for i := range vectors {
			indices, _ := lo.Unzip2(dataset.UserFeatures[i])
			vectors[i] = NewDualDictionaryVector(indices, labelIDF, dataset.UserFeedback[i], itemIDF, nil, false)
		}
	default:
		return errors.NotImplementedf("user neighbor type `%v`", m.Config.Recommend.UserNeighbors.NeighborType)
	}

	builder := search.NewIVFBuilder(vectors, m.Config.Recommend.CacheSize,
		search.SetIVFJobsAllocator(j))
	var recall float32
	index, recall = builder.Build(
		m.Config.Recommend.UserNeighbors.IndexRecall,
		m.Config.Recommend.UserNeighbors.IndexFitEpoch,
		true)
	UserNeighborIndexRecall.Set(float64(recall))
	if err := m.CacheClient.Set(ctx, cache.String(cache.Key(cache.GlobalMeta, cache.UserNeighborIndexRecall), encoding.FormatFloat32(recall))); err != nil {
		return errors.Trace(err)
	}
	buildIndexSeconds.Add(time.Since(buildStart).Seconds())

	err := parallel.DynamicParallel(dataset.UserCount(), j, func(workerId, userIndex int) error {
		defer func() {
			completed <- struct{}{}
		}()
		startSearchTime := time.Now()
		userId := dataset.UserIndex.ToName(int32(userIndex))
		if !m.checkUserNeighborCacheTimeout(userId) {
			return nil
		}
		updateUserCount.Add(1)
		startTime := time.Now()
		var neighbors []int32
		var scores []float32
		neighbors, scores = index.Search(vectors[userIndex], m.Config.Recommend.CacheSize, true)
		resultValues, resultScores := make([]string, len(neighbors)), make([]float64, len(neighbors))
		for i := range scores {
			resultValues[i] = dataset.UserIndex.ToName(neighbors[i])
			resultScores[i] = float64(-scores[i])
		}
		aggregator := cache.NewDocumentAggregator(startSearchTime)
		aggregator.Add("", resultValues, resultScores)
		if err := m.CacheClient.AddScores(ctx, cache.UserNeighbors, userId, aggregator.ToSlice()); err != nil {
			return errors.Trace(err)
		}
		if err := m.CacheClient.DeleteScores(ctx, []string{cache.UserNeighbors}, cache.ScoreCondition{
			Subset: proto.String(userId),
			Before: &aggregator.Timestamp,
		}); err != nil {
			return errors.Trace(err)
		}
		if err := m.CacheClient.Set(
			ctx,
			cache.Time(cache.Key(cache.LastUpdateUserNeighborsTime, userId), time.Now()),
			cache.String(cache.Key(cache.UserNeighborsDigest, userId), m.Config.UserNeighborDigest())); err != nil {
			return errors.Trace(err)
		}
		findNeighborSeconds.Add(time.Since(startTime).Seconds())
		return nil
	})
	if err != nil {
		return errors.Trace(err)
	}
	UpdateUserNeighborsTotal.Set(updateUserCount.Load())
	FindUserNeighborsSecondsVec.WithLabelValues("find_item_neighbors").Set(findNeighborSeconds.Load())
	FindUserNeighborsSecondsVec.WithLabelValues("build_index").Set(buildIndexSeconds.Load())
	return nil
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
	ctx := context.Background()
	// check cache
	if items, err := m.CacheClient.SearchScores(ctx, cache.UserNeighbors, userId, []string{""}, 0, -1); err != nil {
		log.Logger().Error("failed to load user neighbors", zap.String("user_id", userId), zap.Error(err))
		return true
	} else if len(items) == 0 {
		return true
	}
	// read digest
	cacheDigest, err = m.CacheClient.Get(ctx, cache.Key(cache.UserNeighborsDigest, userId)).String()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read user neighbors digest", zap.Error(err))
		}
		return true
	}
	if cacheDigest != m.Config.UserNeighborDigest() {
		return true
	}
	// read modified time
	modifiedTime, err = m.CacheClient.Get(ctx, cache.Key(cache.LastModifyUserTime, userId)).Time()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read last modify user time", zap.Error(err))
		}
		return true
	}
	// read update time
	updateTime, err = m.CacheClient.Get(ctx, cache.Key(cache.LastUpdateUserNeighborsTime, userId)).Time()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read last update user neighbors time", zap.Error(err))
		}
		return true
	}
	// check cache expire
	if updateTime.Before(time.Now().Add(-m.Config.Recommend.CacheExpire)) {
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
	ctx := context.Background()

	// check cache
	for _, category := range append([]string{""}, categories...) {
		items, err := m.CacheClient.SearchScores(ctx, cache.ItemNeighbors, itemId, []string{category}, 0, -1)
		if err != nil {
			log.Logger().Error("failed to load item neighbors", zap.String("item_id", itemId), zap.Error(err))
			return true
		} else if len(items) == 0 {
			return true
		}
	}
	// read digest
	cacheDigest, err = m.CacheClient.Get(ctx, cache.Key(cache.ItemNeighborsDigest, itemId)).String()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read item neighbors digest", zap.Error(err))
		}
		return true
	}
	if cacheDigest != m.Config.ItemNeighborDigest() {
		return true
	}
	// read modified time
	modifiedTime, err = m.CacheClient.Get(ctx, cache.Key(cache.LastModifyItemTime, itemId)).Time()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read last modify item time", zap.Error(err))
		}
		return true
	}
	// read update time
	updateTime, err = m.CacheClient.Get(ctx, cache.Key(cache.LastUpdateItemNeighborsTime, itemId)).Time()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read last update item neighbors time", zap.Error(err))
		}
		return true
	}
	// check cache expire
	if updateTime.Before(time.Now().Add(-m.Config.Recommend.CacheExpire)) {
		return true
	}
	// check time
	return updateTime.Unix() <= modifiedTime.Unix()
}

type FitRankingModelTask struct {
	*Master
	lastNumFeedback int
}

func NewFitRankingModelTask(m *Master) *FitRankingModelTask {
	return &FitRankingModelTask{Master: m}
}

func (t *FitRankingModelTask) name() string {
	return TaskFitRankingModel
}

func (t *FitRankingModelTask) priority() int {
	return -t.rankingTrainSet.Count()
}

func (t *FitRankingModelTask) run(ctx context.Context, j *task.JobsAllocator) error {
	newCtx, span := t.Master.tracer.Start(ctx, "Fit Embedding", 1)
	defer span.End()

	t.rankingDataMutex.RLock()
	defer t.rankingDataMutex.RUnlock()
	dataset := t.rankingTrainSet
	numFeedback := dataset.Count()

	var modelChanged bool
	bestRankingName, bestRankingModel, bestRankingScore := t.rankingModelSearcher.GetBestModel()
	t.rankingModelMutex.Lock()
	if bestRankingModel != nil && !bestRankingModel.Invalid() &&
		(bestRankingName != t.rankingModelName || bestRankingModel.GetParams().ToString() != t.RankingModel.GetParams().ToString()) &&
		(bestRankingScore.NDCG > t.rankingScore.NDCG) {
		// 1. best ranking model must have been found.
		// 2. best ranking model must be different from current model
		// 3. best ranking model must perform better than current model
		t.RankingModel = bestRankingModel
		t.rankingModelName = bestRankingName
		t.rankingScore = bestRankingScore
		modelChanged = true
		log.Logger().Info("find better ranking model",
			zap.Any("score", bestRankingScore),
			zap.String("name", bestRankingName),
			zap.Any("params", t.RankingModel.GetParams()))
	}
	rankingModel := ranking.Clone(t.RankingModel)
	t.rankingModelMutex.Unlock()

	if numFeedback == 0 {
		// t.taskMonitor.Fail(TaskFitRankingModel, "No feedback found.")
		return nil
	} else if numFeedback == t.lastNumFeedback && !modelChanged {
		log.Logger().Info("nothing changed")
		return nil
	}

	startFitTime := time.Now()
	score := rankingModel.Fit(newCtx, t.rankingTrainSet, t.rankingTestSet, ranking.NewFitConfig().SetJobsAllocator(j))
	CollaborativeFilteringFitSeconds.Set(time.Since(startFitTime).Seconds())

	// update ranking model
	t.rankingModelMutex.Lock()
	t.RankingModel = rankingModel
	t.RankingModelVersion++
	t.rankingScore = score
	t.rankingModelMutex.Unlock()
	log.Logger().Info("fit ranking model complete",
		zap.String("version", fmt.Sprintf("%x", t.RankingModelVersion)))
	CollaborativeFilteringNDCG10.Set(float64(score.NDCG))
	CollaborativeFilteringRecall10.Set(float64(score.Recall))
	CollaborativeFilteringPrecision10.Set(float64(score.Precision))
	MemoryInUseBytesVec.WithLabelValues("collaborative_filtering_model").Set(float64(t.RankingModel.Bytes()))
	if err := t.CacheClient.Set(ctx, cache.Time(cache.Key(cache.GlobalMeta, cache.LastFitMatchingModelTime), time.Now())); err != nil {
		log.Logger().Error("failed to write meta", zap.Error(err))
	}

	// caching model
	t.rankingModelMutex.RLock()
	t.localCache.RankingModelName = t.rankingModelName
	t.localCache.RankingModelVersion = t.RankingModelVersion
	t.localCache.RankingModel = rankingModel
	t.localCache.RankingModelScore = score
	t.rankingModelMutex.RUnlock()
	if t.localCache.ClickModel == nil || t.localCache.ClickModel.Invalid() {
		log.Logger().Info("wait click model")
	} else if err := t.localCache.WriteLocalCache(); err != nil {
		log.Logger().Error("failed to write local cache", zap.Error(err))
	} else {
		log.Logger().Info("write model to local cache",
			zap.String("ranking_model_name", t.localCache.RankingModelName),
			zap.String("ranking_model_version", encoding.Hex(t.localCache.RankingModelVersion)),
			zap.Float32("ranking_model_score", t.localCache.RankingModelScore.NDCG),
			zap.Any("ranking_model_params", t.localCache.RankingModel.GetParams()))
	}

	// t.taskMonitor.Finish(TaskFitRankingModel)
	t.lastNumFeedback = numFeedback
	return nil
}

// FitClickModelTask fits click model using latest data. After model fitted, following states are changed:
// 1. Click model version are increased.
// 2. Click model score are updated.
// 3. Click model, version and score are persisted to local cache.
type FitClickModelTask struct {
	*Master
	lastNumUsers    int
	lastNumItems    int
	lastNumFeedback int
}

func NewFitClickModelTask(m *Master) *FitClickModelTask {
	return &FitClickModelTask{Master: m}
}

func (t *FitClickModelTask) name() string {
	return TaskFitClickModel
}

func (t *FitClickModelTask) priority() int {
	return -t.clickTrainSet.Count()
}

func (t *FitClickModelTask) run(ctx context.Context, j *task.JobsAllocator) error {
	newCtx, span := t.tracer.Start(ctx, "Fit Ranker", 1)
	defer span.End()

	log.Logger().Info("prepare to fit click model", zap.Int("n_jobs", t.Config.Master.NumJobs))
	t.clickDataMutex.RLock()
	defer t.clickDataMutex.RUnlock()
	numUsers := t.clickTrainSet.UserCount()
	numItems := t.clickTrainSet.ItemCount()
	numFeedback := t.clickTrainSet.Count()
	var shouldFit bool

	if t.clickTrainSet == nil || numUsers == 0 || numItems == 0 || numFeedback == 0 {
		log.Logger().Warn("empty ranking dataset",
			zap.Strings("positive_feedback_type", t.Config.Recommend.DataSource.PositiveFeedbackTypes))
		return nil
	} else if numUsers != t.lastNumUsers ||
		numItems != t.lastNumItems ||
		numFeedback != t.lastNumFeedback {
		shouldFit = true
	}

	bestClickModel, bestClickScore := t.clickModelSearcher.GetBestModel()
	t.clickModelMutex.Lock()
	if bestClickModel != nil && !bestClickModel.Invalid() &&
		bestClickModel.GetParams().ToString() != t.ClickModel.GetParams().ToString() &&
		bestClickScore.Precision > t.clickScore.Precision {
		// 1. best click model must have been found.
		// 2. best click model must be different from current model
		// 3. best click model must perform better than current model
		t.ClickModel = bestClickModel
		t.clickScore = bestClickScore
		shouldFit = true
		log.Logger().Info("find better click model",
			zap.Float32("Precision", bestClickScore.Precision),
			zap.Float32("Recall", bestClickScore.Recall),
			zap.Any("params", t.ClickModel.GetParams()))
	}
	clickModel := click.Clone(t.ClickModel)
	t.clickModelMutex.Unlock()

	// training model
	if !shouldFit {
		log.Logger().Info("nothing changed")
		return nil
	}
	startFitTime := time.Now()
	score := clickModel.Fit(newCtx, t.clickTrainSet, t.clickTestSet, click.NewFitConfig().
		SetJobsAllocator(j))
	RankingFitSeconds.Set(time.Since(startFitTime).Seconds())

	// update match model
	t.clickModelMutex.Lock()
	t.ClickModel = clickModel
	t.clickScore = score
	t.ClickModelVersion++
	t.clickModelMutex.Unlock()
	log.Logger().Info("fit click model complete",
		zap.String("version", fmt.Sprintf("%x", t.ClickModelVersion)))
	RankingPrecision.Set(float64(score.Precision))
	RankingRecall.Set(float64(score.Recall))
	RankingAUC.Set(float64(score.AUC))
	MemoryInUseBytesVec.WithLabelValues("ranking_model").Set(float64(sizeof.DeepSize(t.ClickModel)))
	if err := t.CacheClient.Set(ctx, cache.Time(cache.Key(cache.GlobalMeta, cache.LastFitRankingModelTime), time.Now())); err != nil {
		log.Logger().Error("failed to write meta", zap.Error(err))
	}

	// caching model
	t.clickModelMutex.RLock()
	t.localCache.ClickModelScore = t.clickScore
	t.localCache.ClickModelVersion = t.ClickModelVersion
	t.localCache.ClickModel = t.ClickModel
	t.clickModelMutex.RUnlock()
	if t.localCache.RankingModel == nil || t.localCache.RankingModel.Invalid() {
		log.Logger().Info("wait ranking model")
	} else if err := t.localCache.WriteLocalCache(); err != nil {
		log.Logger().Error("failed to write local cache", zap.Error(err))
	} else {
		log.Logger().Info("write model to local cache",
			zap.String("click_model_version", encoding.Hex(t.localCache.ClickModelVersion)),
			zap.Float32("click_model_score", score.Precision),
			zap.Any("click_model_params", t.localCache.ClickModel.GetParams()))
	}

	t.lastNumItems = numItems
	t.lastNumUsers = numUsers
	t.lastNumFeedback = numFeedback
	return nil
}

// SearchRankingModelTask searches best hyper-parameters for ranking models.
// It requires read lock on the ranking dataset.
type SearchRankingModelTask struct {
	*Master
	lastNumUsers    int
	lastNumItems    int
	lastNumFeedback int
}

func NewSearchRankingModelTask(m *Master) *SearchRankingModelTask {
	return &SearchRankingModelTask{Master: m}
}

func (t *SearchRankingModelTask) name() string {
	return TaskSearchRankingModel
}

func (t *SearchRankingModelTask) priority() int {
	return -t.rankingTrainSet.Count()
}

func (t *SearchRankingModelTask) run(ctx context.Context, j *task.JobsAllocator) error {
	log.Logger().Info("start searching ranking model")
	t.rankingDataMutex.RLock()
	defer t.rankingDataMutex.RUnlock()
	if t.rankingTrainSet == nil {
		log.Logger().Debug("dataset has not been loaded")
		return nil
	}
	numUsers := t.rankingTrainSet.UserCount()
	numItems := t.rankingTrainSet.ItemCount()
	numFeedback := t.rankingTrainSet.Count()

	if numUsers == 0 || numItems == 0 || numFeedback == 0 {
		log.Logger().Warn("empty ranking dataset",
			zap.Strings("positive_feedback_type", t.Config.Recommend.DataSource.PositiveFeedbackTypes))
		// t.taskMonitor.Fail(TaskSearchRankingModel, "No feedback found.")
		return nil
	} else if numUsers == t.lastNumUsers &&
		numItems == t.lastNumItems &&
		numFeedback == t.lastNumFeedback {
		log.Logger().Info("ranking dataset not changed")
		return nil
	}

	startTime := time.Now()
	err := t.rankingModelSearcher.Fit(ctx, t.rankingTrainSet, t.rankingTestSet, nil)
	if err != nil {
		log.Logger().Error("failed to search collaborative filtering model", zap.Error(err))
		return nil
	}
	CollaborativeFilteringSearchSeconds.Set(time.Since(startTime).Seconds())
	_, _, bestScore := t.rankingModelSearcher.GetBestModel()
	CollaborativeFilteringSearchPrecision10.Set(float64(bestScore.Precision))

	t.lastNumItems = numItems
	t.lastNumUsers = numUsers
	t.lastNumFeedback = numFeedback
	return nil
}

// SearchClickModelTask searches best hyper-parameters for factorization machines.
// It requires read lock on the click dataset.
type SearchClickModelTask struct {
	*Master
	lastNumUsers    int
	lastNumItems    int
	lastNumFeedback int
}

func NewSearchClickModelTask(m *Master) *SearchClickModelTask {
	return &SearchClickModelTask{Master: m}
}

func (t *SearchClickModelTask) name() string {
	return TaskSearchClickModel
}

func (t *SearchClickModelTask) priority() int {
	return -t.clickTrainSet.Count()
}

func (t *SearchClickModelTask) run(ctx context.Context, j *task.JobsAllocator) error {
	log.Logger().Info("start searching click model")
	t.clickDataMutex.RLock()
	defer t.clickDataMutex.RUnlock()
	if t.clickTrainSet == nil {
		log.Logger().Debug("dataset has not been loaded")
		return nil
	}
	numUsers := t.clickTrainSet.UserCount()
	numItems := t.clickTrainSet.ItemCount()
	numFeedback := t.clickTrainSet.Count()

	if numUsers == 0 || numItems == 0 || numFeedback == 0 {
		log.Logger().Warn("empty click dataset",
			zap.Strings("positive_feedback_type", t.Config.Recommend.DataSource.PositiveFeedbackTypes))
		return nil
	} else if numUsers == t.lastNumUsers &&
		numItems == t.lastNumItems &&
		numFeedback == t.lastNumFeedback {
		log.Logger().Info("click dataset not changed")
		return nil
	}

	startTime := time.Now()
	err := t.clickModelSearcher.Fit(context.Background(), t.clickTrainSet, t.clickTestSet, j)
	if err != nil {
		log.Logger().Error("failed to search ranking model", zap.Error(err))
		return nil
	}
	RankingSearchSeconds.Set(time.Since(startTime).Seconds())
	_, bestScore := t.clickModelSearcher.GetBestModel()
	RankingSearchPrecision.Set(float64(bestScore.Precision))

	t.lastNumItems = numItems
	t.lastNumUsers = numUsers
	t.lastNumFeedback = numFeedback
	return nil
}

type CacheGarbageCollectionTask struct {
	*Master
}

func NewCacheGarbageCollectionTask(m *Master) *CacheGarbageCollectionTask {
	return &CacheGarbageCollectionTask{m}
}

func (t *CacheGarbageCollectionTask) name() string {
	return TaskCacheGarbageCollection
}

func (t *CacheGarbageCollectionTask) priority() int {
	return -t.rankingTrainSet.UserCount() - t.rankingTrainSet.ItemCount()
}

func (t *CacheGarbageCollectionTask) run(ctx context.Context, j *task.JobsAllocator) error {
	if t.rankingTrainSet == nil {
		log.Logger().Debug("dataset has not been loaded")
		return nil
	}

	log.Logger().Info("start cache garbage collection")
	var scanCount, reclaimCount int
	start := time.Now()
	err := t.CacheClient.Scan(func(s string) error {
		splits := strings.Split(s, "/")
		if len(splits) <= 1 {
			return nil
		}
		scanCount++
		switch splits[0] {
		case cache.UserNeighbors, cache.UserNeighborsDigest,
			cache.OfflineRecommend, cache.OfflineRecommendDigest, cache.CollaborativeRecommend,
			cache.LastModifyUserTime, cache.LastUpdateUserNeighborsTime, cache.LastUpdateUserRecommendTime:
			userId := splits[1]
			// check user in dataset
			if t.rankingTrainSet != nil && t.rankingTrainSet.UserIndex.ToNumber(userId) != base.NotId {
				return nil
			}
			// check user in database
			_, err := t.DataClient.GetUser(ctx, userId)
			if !errors.Is(err, errors.NotFound) {
				if err != nil {
					log.Logger().Error("failed to load user", zap.String("user_id", userId), zap.Error(err))
				}
				return err
			}
			// delete user cache
			switch splits[0] {
			case cache.UserNeighborsDigest, cache.OfflineRecommendDigest,
				cache.LastModifyUserTime, cache.LastUpdateUserNeighborsTime, cache.LastUpdateUserRecommendTime:
				err = t.CacheClient.Delete(ctx, s)
			}
			if err != nil {
				return errors.Trace(err)
			}
			reclaimCount++
		case cache.ItemNeighbors, cache.ItemNeighborsDigest, cache.LastModifyItemTime, cache.LastUpdateItemNeighborsTime:
			itemId := splits[1]
			// check item in dataset
			if t.rankingTrainSet != nil && t.rankingTrainSet.ItemIndex.ToNumber(itemId) != base.NotId {
				return nil
			}
			// check item in database
			_, err := t.DataClient.GetItem(ctx, itemId)
			if !errors.Is(err, errors.NotFound) {
				if err != nil {
					log.Logger().Error("failed to load item", zap.String("item_id", itemId), zap.Error(err))
				}
				return err
			}
			// delete item cache
			switch splits[0] {
			case cache.ItemNeighborsDigest, cache.LastModifyItemTime, cache.LastUpdateItemNeighborsTime:
				err = t.CacheClient.Delete(ctx, s)
			}
			if err != nil {
				return errors.Trace(err)
			}
			reclaimCount++
		}
		return nil
	})
	CacheScannedTotal.Set(float64(scanCount))
	CacheReclaimedTotal.Set(float64(reclaimCount))
	CacheScannedSeconds.Set(time.Since(start).Seconds())
	return errors.Trace(err)
}

// LoadDataFromDatabase loads dataset from data store.
func (m *Master) LoadDataFromDatabase(ctx context.Context, database data.Database, posFeedbackTypes, readTypes []string, itemTTL, positiveFeedbackTTL uint, evaluator *OnlineEvaluator) (
	rankingDataset *ranking.DataSet, clickDataset *click.Dataset, latestItems *cache.DocumentAggregator, popularItems *cache.DocumentAggregator, err error) {
	newCtx, span := progress.Start(ctx, "LoadDataFromDatabase", 4)
	defer span.End()

	startLoadTime := time.Now()
	// setup time limit
	var feedbackTimeLimit data.ScanOption
	var itemTimeLimit *time.Time
	if itemTTL > 0 {
		temp := time.Now().AddDate(0, 0, -int(itemTTL))
		itemTimeLimit = &temp
	}
	if positiveFeedbackTTL > 0 {
		temp := time.Now().AddDate(0, 0, -int(positiveFeedbackTTL))
		feedbackTimeLimit = data.WithBeginTime(temp)
	}
	timeWindowLimit := time.Time{}
	if m.Config.Recommend.Popular.PopularWindow > 0 {
		timeWindowLimit = time.Now().Add(-m.Config.Recommend.Popular.PopularWindow)
	}
	rankingDataset = ranking.NewMapIndexDataset()

	// create filers for latest items
	latestItemsFilters := make(map[string]*heap.TopKFilter[string, float64])
	latestItemsFilters[""] = heap.NewTopKFilter[string, float64](m.Config.Recommend.CacheSize)

	// STEP 1: pull users
	userLabelCount := make(map[string]int)
	userLabelFirst := make(map[string]int32)
	userLabelIndex := base.NewMapIndex()
	start := time.Now()
	userChan, errChan := database.GetUserStream(newCtx, batchSize)
	for users := range userChan {
		for _, user := range users {
			rankingDataset.AddUser(user.UserId)
			userIndex := rankingDataset.UserIndex.ToNumber(user.UserId)
			if len(rankingDataset.UserFeatures) == int(userIndex) {
				rankingDataset.UserFeatures = append(rankingDataset.UserFeatures, nil)
			}
			features := click.ConvertLabelsToFeatures(user.Labels)
			rankingDataset.NumUserLabelUsed += len(features)
			rankingDataset.UserFeatures[userIndex] = make([]lo.Tuple2[int32, float32], 0, len(features))
			for _, feature := range features {
				userLabelCount[feature.Name]++
				// Memorize the first occurrence.
				if userLabelCount[feature.Name] == 1 {
					userLabelFirst[feature.Name] = userIndex
				}
				// Add the label to the index in second occurrence.
				if userLabelCount[feature.Name] == 2 {
					userLabelIndex.Add(feature.Name)
					firstUserIndex := userLabelFirst[feature.Name]
					rankingDataset.UserFeatures[firstUserIndex] = append(rankingDataset.UserFeatures[firstUserIndex], lo.Tuple2[int32, float32]{
						A: userLabelIndex.ToNumber(feature.Name),
						B: feature.Value,
					})
				}
				// Add the label to the user.
				if userLabelCount[feature.Name] > 1 {
					rankingDataset.UserFeatures[userIndex] = append(rankingDataset.UserFeatures[userIndex], lo.Tuple2[int32, float32]{
						A: userLabelIndex.ToNumber(feature.Name),
						B: feature.Value,
					})
				}
			}
		}
	}
	if err = <-errChan; err != nil {
		return nil, nil, nil, nil, errors.Trace(err)
	}
	rankingDataset.NumUserLabels = userLabelIndex.Len()
	log.Logger().Debug("pulled users from database",
		zap.Int("n_users", rankingDataset.UserCount()),
		zap.Int32("n_user_labels", userLabelIndex.Len()),
		zap.Duration("used_time", time.Since(start)))
	LoadDatasetStepSecondsVec.WithLabelValues("load_users").Set(time.Since(start).Seconds())
	span.Add(1)

	// STEP 2: pull items
	itemLabelCount := make(map[string]int)
	itemLabelFirst := make(map[string]int32)
	itemLabelIndex := base.NewMapIndex()
	start = time.Now()
	itemChan, errChan := database.GetItemStream(newCtx, batchSize, itemTimeLimit)
	for items := range itemChan {
		for _, item := range items {
			rankingDataset.AddItem(item.ItemId)
			itemIndex := rankingDataset.ItemIndex.ToNumber(item.ItemId)
			if len(rankingDataset.ItemFeatures) == int(itemIndex) {
				rankingDataset.ItemFeatures = append(rankingDataset.ItemFeatures, nil)
				rankingDataset.HiddenItems = append(rankingDataset.HiddenItems, false)
				rankingDataset.ItemCategories = append(rankingDataset.ItemCategories, item.Categories)
				rankingDataset.CategorySet.Append(item.Categories...)
			}
			features := click.ConvertLabelsToFeatures(item.Labels)
			rankingDataset.NumItemLabelUsed += len(features)
			rankingDataset.ItemFeatures[itemIndex] = make([]lo.Tuple2[int32, float32], 0, len(features))
			for _, feature := range features {
				itemLabelCount[feature.Name]++
				// Memorize the first occurrence.
				if itemLabelCount[feature.Name] == 1 {
					itemLabelFirst[feature.Name] = itemIndex
				}
				// Add the label to the index in second occurrence.
				if itemLabelCount[feature.Name] == 2 {
					itemLabelIndex.Add(feature.Name)
					firstItemIndex := itemLabelFirst[feature.Name]
					rankingDataset.ItemFeatures[firstItemIndex] = append(rankingDataset.ItemFeatures[firstItemIndex], lo.Tuple2[int32, float32]{
						A: itemLabelIndex.ToNumber(feature.Name),
						B: feature.Value,
					})
				}
				// Add the label to the item.
				if itemLabelCount[feature.Name] > 1 {
					rankingDataset.ItemFeatures[itemIndex] = append(rankingDataset.ItemFeatures[itemIndex], lo.Tuple2[int32, float32]{
						A: itemLabelIndex.ToNumber(feature.Name),
						B: feature.Value,
					})
				}
			}
			if item.IsHidden { // set hidden flag
				rankingDataset.HiddenItems[itemIndex] = true
			} else if !item.Timestamp.IsZero() { // add items to the latest items filter
				latestItemsFilters[""].Push(item.ItemId, float64(item.Timestamp.Unix()))
				for _, category := range item.Categories {
					if _, exist := latestItemsFilters[category]; !exist {
						latestItemsFilters[category] = heap.NewTopKFilter[string, float64](m.Config.Recommend.CacheSize)
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
	log.Logger().Debug("pulled items from database",
		zap.Int("n_items", rankingDataset.ItemCount()),
		zap.Int32("n_item_labels", itemLabelIndex.Len()),
		zap.Duration("used_time", time.Since(start)))
	LoadDatasetStepSecondsVec.WithLabelValues("load_items").Set(time.Since(start).Seconds())
	span.Add(1)

	// create positive set
	popularCount := make([]int32, rankingDataset.ItemCount())
	positiveSet := make([]mapset.Set[int32], rankingDataset.UserCount())
	for i := range positiveSet {
		positiveSet[i] = mapset.NewSet[int32]()
	}

	// split user groups
	users := rankingDataset.UserIndex.GetNames()
	sort.Strings(users)
	userGroups := parallel.Split(users, m.Config.Master.NumJobs)

	// STEP 3: pull positive feedback
	var mu sync.Mutex
	var posFeedbackCount int
	start = time.Now()
	err = parallel.Parallel(len(userGroups), m.Config.Master.NumJobs, func(_, userIndex int) error {
		feedbackChan, errChan := database.GetFeedbackStream(newCtx, batchSize,
			data.WithBeginUserId(userGroups[userIndex][0]),
			data.WithEndUserId(userGroups[userIndex][len(userGroups[userIndex])-1]),
			feedbackTimeLimit,
			data.WithEndTime(*m.Config.Now()),
			data.WithFeedbackTypes(posFeedbackTypes...))
		for feedback := range feedbackChan {
			for _, f := range feedback {
				// convert user and item id to index
				userIndex := rankingDataset.UserIndex.ToNumber(f.UserId)
				if userIndex == base.NotId {
					continue
				}
				itemIndex := rankingDataset.ItemIndex.ToNumber(f.ItemId)
				if itemIndex == base.NotId {
					continue
				}
				// insert feedback to positive set
				positiveSet[userIndex].Add(itemIndex)

				mu.Lock()
				posFeedbackCount++
				// insert feedback to ranking dataset
				rankingDataset.AddFeedback(f.UserId, f.ItemId, false)
				// insert feedback to popularity counter
				if f.Timestamp.After(timeWindowLimit) && !rankingDataset.HiddenItems[itemIndex] {
					popularCount[itemIndex]++
				}
				// insert feedback to evaluator
				evaluator.Positive(f.FeedbackType, userIndex, itemIndex, f.Timestamp)
				mu.Unlock()
			}
		}
		if err = <-errChan; err != nil {
			return errors.Trace(err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, nil, errors.Trace(err)
	}
	log.Logger().Debug("pulled positive feedback from database",
		zap.Int("n_positive_feedback", posFeedbackCount),
		zap.Duration("used_time", time.Since(start)))
	LoadDatasetStepSecondsVec.WithLabelValues("load_positive_feedback").Set(time.Since(start).Seconds())
	span.Add(1)

	// create negative set
	negativeSet := make([]mapset.Set[int32], rankingDataset.UserCount())
	for i := range negativeSet {
		negativeSet[i] = mapset.NewSet[int32]()
	}

	// STEP 4: pull negative feedback
	start = time.Now()
	var negativeFeedbackCount float64
	err = parallel.Parallel(len(userGroups), m.Config.Master.NumJobs, func(_, userIndex int) error {
		feedbackChan, errChan := database.GetFeedbackStream(newCtx, batchSize,
			data.WithBeginUserId(userGroups[userIndex][0]),
			data.WithEndUserId(userGroups[userIndex][len(userGroups[userIndex])-1]),
			feedbackTimeLimit,
			data.WithEndTime(*m.Config.Now()),
			data.WithFeedbackTypes(readTypes...))
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
				if !positiveSet[userIndex].Contains(itemIndex) {
					negativeSet[userIndex].Add(itemIndex)
				}

				mu.Lock()
				negativeFeedbackCount++
				evaluator.Read(userIndex, itemIndex, f.Timestamp)
				mu.Unlock()
			}
		}
		if err = <-errChan; err != nil {
			return errors.Trace(err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, nil, errors.Trace(err)
	}
	log.Logger().Debug("pulled negative feedback from database",
		zap.Int("n_negative_feedback", int(negativeFeedbackCount)),
		zap.Duration("used_time", time.Since(start)))
	LoadDatasetStepSecondsVec.WithLabelValues("load_negative_feedback").Set(time.Since(start).Seconds())
	span.Add(1)

	// STEP 5: create click dataset
	start = time.Now()
	unifiedIndex := click.NewUnifiedMapIndexBuilder()
	unifiedIndex.ItemIndex = rankingDataset.ItemIndex
	unifiedIndex.UserIndex = rankingDataset.UserIndex
	unifiedIndex.ItemLabelIndex = itemLabelIndex
	unifiedIndex.UserLabelIndex = userLabelIndex
	clickDataset = &click.Dataset{
		Index:        unifiedIndex.Build(),
		UserFeatures: rankingDataset.UserFeatures,
		ItemFeatures: rankingDataset.ItemFeatures,
	}
	for userIndex := range positiveSet {
		if positiveSet[userIndex].Cardinality() == 0 || negativeSet[userIndex].Cardinality() == 0 {
			// release positive set and negative set
			positiveSet[userIndex] = nil
			negativeSet[userIndex] = nil
			continue
		}
		// insert positive feedback
		for _, itemIndex := range positiveSet[userIndex].ToSlice() {
			clickDataset.Users.Append(int32(userIndex))
			clickDataset.Items.Append(itemIndex)
			clickDataset.Target.Append(1)
			clickDataset.PositiveCount++
		}
		// insert negative feedback
		for _, itemIndex := range negativeSet[userIndex].ToSlice() {
			clickDataset.Users.Append(int32(userIndex))
			clickDataset.Items.Append(itemIndex)
			clickDataset.Target.Append(-1)
			clickDataset.NegativeCount++
		}
		// release positive set and negative set
		positiveSet[userIndex] = nil
		negativeSet[userIndex] = nil
	}
	log.Logger().Debug("created ranking dataset",
		zap.Int("n_valid_positive", clickDataset.PositiveCount),
		zap.Int("n_valid_negative", clickDataset.NegativeCount),
		zap.Duration("used_time", time.Since(start)))
	LoadDatasetStepSecondsVec.WithLabelValues("create_ranking_dataset").Set(time.Since(start).Seconds())

	// collect latest items
	latestItems = cache.NewDocumentAggregator(startLoadTime)
	for category, latestItemsFilter := range latestItemsFilters {
		items, scores := latestItemsFilter.PopAll()
		latestItems.Add(category, items, scores)
	}

	// collect popular items
	popularItemFilters := make(map[string]*heap.TopKFilter[string, float64])
	popularItemFilters[""] = heap.NewTopKFilter[string, float64](m.Config.Recommend.CacheSize)
	for itemIndex, val := range popularCount {
		itemId := rankingDataset.ItemIndex.ToName(int32(itemIndex))
		popularItemFilters[""].Push(itemId, float64(val))
		for _, category := range rankingDataset.ItemCategories[itemIndex] {
			if _, exist := popularItemFilters[category]; !exist {
				popularItemFilters[category] = heap.NewTopKFilter[string, float64](m.Config.Recommend.CacheSize)
			}
			popularItemFilters[category].Push(itemId, float64(val))
		}
	}
	popularItems = cache.NewDocumentAggregator(startLoadTime)
	for category, popularItemFilter := range popularItemFilters {
		items, scores := popularItemFilter.PopAll()
		popularItems.Add(category, items, scores)
	}

	return rankingDataset, clickDataset, latestItems, popularItems, nil
}
