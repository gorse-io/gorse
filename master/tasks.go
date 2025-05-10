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

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/encoding"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/progress"
	"github.com/zhenghaoz/gorse/base/task"
	"github.com/zhenghaoz/gorse/common/parallel"
	"github.com/zhenghaoz/gorse/common/sizeof"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/dataset"
	"github.com/zhenghaoz/gorse/logics"
	"github.com/zhenghaoz/gorse/model/cf"
	"github.com/zhenghaoz/gorse/model/ctr"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

const (
	PositiveFeedbackRate = "PositiveFeedbackRate"

	TaskSearchRankingModel     = "Search collaborative filtering  model"
	TaskSearchClickModel       = "Search click-through rate prediction model"
	TaskCacheGarbageCollection = "Collect garbage in cache"

	batchSize = 10000
)

type Task interface {
	name() string
	priority() int
	run(ctx context.Context, j *task.JobsAllocator) error
}

func (m *Master) loadDataset() (*ctr.Dataset, *dataset.Dataset, error) {
	ctx, span := m.tracer.Start(context.Background(), "Load Dataset", 1)
	defer span.End()

	// Build non-personalized recommenders
	initialStartTime := time.Now()
	nonPersonalizedRecommenders := []*logics.NonPersonalized{
		logics.NewLatest(m.Config.Recommend.CacheSize, initialStartTime),
		logics.NewPopular(m.Config.Recommend.Popular.PopularWindow, m.Config.Recommend.CacheSize, initialStartTime),
	}
	for _, cfg := range m.Config.Recommend.NonPersonalized {
		recommender, err := logics.NewNonPersonalized(cfg, m.Config.Recommend.CacheSize, initialStartTime)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		nonPersonalizedRecommenders = append(nonPersonalizedRecommenders, recommender)
	}

	log.Logger().Info("load dataset",
		zap.Strings("positive_feedback_types", m.Config.Recommend.DataSource.PositiveFeedbackTypes),
		zap.Strings("read_feedback_types", m.Config.Recommend.DataSource.ReadFeedbackTypes),
		zap.Uint("item_ttl", m.Config.Recommend.DataSource.ItemTTL),
		zap.Uint("feedback_ttl", m.Config.Recommend.DataSource.PositiveFeedbackTTL))
	evaluator := NewOnlineEvaluator()
	clickDataset, dataSet, err := m.LoadDataFromDatabase(ctx, m.DataClient,
		m.Config.Recommend.DataSource.PositiveFeedbackTypes,
		m.Config.Recommend.DataSource.ReadFeedbackTypes,
		m.Config.Recommend.DataSource.ItemTTL,
		m.Config.Recommend.DataSource.PositiveFeedbackTTL,
		evaluator,
		nonPersonalizedRecommenders)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	// save non-personalized recommenders to cache
	for _, recommender := range nonPersonalizedRecommenders {
		scores := recommender.PopAll()
		if err = m.CacheClient.AddScores(ctx, cache.NonPersonalized, recommender.Name(), scores); err != nil {
			log.Logger().Error("failed to cache non-personalized recommenders", zap.Error(err))
		}
		if err = m.CacheClient.DeleteScores(ctx, []string{cache.NonPersonalized},
			cache.ScoreCondition{
				Subset: proto.String(recommender.Name()),
				Before: lo.ToPtr(recommender.Timestamp()),
			}); err != nil {
			log.Logger().Error("failed to reclaim outdated items", zap.Error(err))
		}
		if err = m.CacheClient.Set(ctx, cache.Time(cache.Key(cache.NonPersonalizedUpdateTime, recommender.Name()), recommender.Timestamp())); err != nil {
			log.Logger().Error("failed to write meta", zap.Error(err))
		}
	}

	// write statistics to database
	UsersTotal.Set(float64(dataSet.CountUsers()))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumUsers), dataSet.CountUsers())); err != nil {
		log.Logger().Error("failed to write number of users", zap.Error(err))
	}
	ItemsTotal.Set(float64(dataSet.CountItems()))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumItems), dataSet.CountItems())); err != nil {
		log.Logger().Error("failed to write number of items", zap.Error(err))
	}
	ImplicitFeedbacksTotal.Set(float64(dataSet.CountFeedback()))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumTotalPosFeedbacks), dataSet.CountFeedback())); err != nil {
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
	ImplicitFeedbacksTotal.Set(float64(dataSet.CountFeedback()))
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
	for _, userFeedback := range dataSet.GetUserFeedback() {
		if len(userFeedback) > 0 {
			activeUsers++
		} else {
			inactiveUsers++
		}
	}
	for _, itemFeedback := range dataSet.GetItemFeedback() {
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
	if err = m.CacheClient.SetSet(ctx, cache.ItemCategories, dataSet.GetCategories()...); err != nil {
		log.Logger().Error("failed to write categories to cache", zap.Error(err))
	}

	// split ranking dataset
	startTime := time.Now()
	m.rankingDataMutex.Lock()
	m.rankingTrainSet, m.rankingTestSet = dataSet.SplitCF(0, 0)
	m.rankingDataMutex.Unlock()
	LoadDatasetStepSecondsVec.WithLabelValues("split_ranking_dataset").Set(time.Since(startTime).Seconds())
	MemoryInUseBytesVec.WithLabelValues("collaborative_filtering_train_set").Set(float64(sizeof.DeepSize(m.rankingTrainSet)))
	MemoryInUseBytesVec.WithLabelValues("collaborative_filtering_test_set").Set(float64(sizeof.DeepSize(m.rankingTestSet)))

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
	return clickDataset, dataSet, nil
}

// runLoadDatasetTask loads dataset.
func (m *Master) runLoadDatasetTask() error {
	_, dataSet, err := m.loadDataset()
	if err != nil {
		return errors.Trace(err)
	}
	if err = m.updateUserToUser(dataSet); err != nil {
		log.Logger().Error("failed to update user-to-user recommendation", zap.Error(err))
	}
	if err = m.updateItemToItem(dataSet); err != nil {
		log.Logger().Error("failed to update item-to-item recommendation", zap.Error(err))
	}
	if err = m.trainCollaborativeFiltering(m.rankingTrainSet, m.rankingTestSet); err != nil {
		log.Logger().Error("failed to train collaborative filtering model", zap.Error(err))
	}
	if err = m.trainClickThroughRatePrediction(m.clickTrainSet, m.clickTestSet); err != nil {
		log.Logger().Error("failed to train click-through rate prediction model", zap.Error(err))
	}
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
	return -t.rankingTrainSet.CountFeedback()
}

func (t *SearchRankingModelTask) run(ctx context.Context, j *task.JobsAllocator) error {
	log.Logger().Info("start searching ranking model")
	t.rankingDataMutex.RLock()
	defer t.rankingDataMutex.RUnlock()
	if t.rankingTrainSet == nil {
		log.Logger().Debug("dataset has not been loaded")
		return nil
	}
	numUsers := t.rankingTrainSet.CountUsers()
	numItems := t.rankingTrainSet.CountItems()
	numFeedback := t.rankingTrainSet.CountFeedback()

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
	err := t.collaborativeFilteringSearcher.Fit(ctx, t.rankingTrainSet, t.rankingTestSet, nil)
	if err != nil {
		log.Logger().Error("failed to search collaborative filtering model", zap.Error(err))
		return nil
	}
	CollaborativeFilteringSearchSeconds.Set(time.Since(startTime).Seconds())
	_, _, bestScore := t.collaborativeFilteringSearcher.GetBestModel()
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
	numUsers := t.clickTrainSet.CountUsers()
	numItems := t.clickTrainSet.CountItems()
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
	return -t.rankingTrainSet.CountUsers() - t.rankingTrainSet.CountItems()
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
		case cache.UserToUser, cache.UserToUserDigest,
			cache.OfflineRecommend, cache.OfflineRecommendDigest, cache.CollaborativeRecommend,
			cache.LastModifyUserTime, cache.UserToUserUpdateTime, cache.LastUpdateUserRecommendTime:
			userId := splits[1]
			// check user in dataset
			if t.rankingTrainSet != nil && t.rankingTrainSet.GetUserDict().Id(userId) >= 0 {
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
			case cache.UserToUserDigest, cache.OfflineRecommendDigest,
				cache.LastModifyUserTime, cache.UserToUserUpdateTime, cache.LastUpdateUserRecommendTime:
				err = t.CacheClient.Delete(ctx, s)
			}
			if err != nil {
				return errors.Trace(err)
			}
			reclaimCount++
		case cache.ItemToItem, cache.ItemToItemDigest, cache.ItemToItemUpdateTime, cache.LastModifyItemTime:
			itemId := splits[1]
			// check item in dataset
			if t.rankingTrainSet != nil && t.rankingTrainSet.GetItemDict().Id(itemId) >= 0 {
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
			case cache.ItemToItemDigest, cache.ItemToItemUpdateTime, cache.LastModifyItemTime:
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
func (m *Master) LoadDataFromDatabase(
	ctx context.Context,
	database data.Database,
	posFeedbackTypes, readTypes []string,
	itemTTL, positiveFeedbackTTL uint,
	evaluator *OnlineEvaluator,
	nonPersonalizedRecommenders []*logics.NonPersonalized,
) (clickDataset *ctr.Dataset, dataSet *dataset.Dataset, err error) {
	var rankingDataset *cf.DataSet
	// Estimate the number of users, items, and feedbacks
	estimatedNumUsers, err := m.DataClient.CountUsers(context.Background())
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	estimatedNumItems, err := m.DataClient.CountItems(context.Background())
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	estimatedNumFeedbacks, err := m.DataClient.CountFeedback(context.Background())
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	dataSet = dataset.NewDataset(time.Now(), estimatedNumUsers, estimatedNumItems)

	newCtx, span := progress.Start(ctx, "LoadDataFromDatabase",
		estimatedNumUsers+estimatedNumItems+estimatedNumFeedbacks)
	defer span.End()

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
	rankingDataset = cf.NewMapIndexDataset()

	// STEP 1: pull users
	userLabelCount := make(map[string]int)
	userLabelFirst := make(map[string]int32)
	userLabelIndex := base.NewMapIndex()
	start := time.Now()
	userChan, errChan := database.GetUserStream(newCtx, batchSize)
	for users := range userChan {
		for _, user := range users {
			dataSet.AddUser(user)
			userIndex := dataSet.GetUserDict().Id(user.UserId)
			if len(rankingDataset.UserFeatures) == int(userIndex) {
				rankingDataset.UserFeatures = append(rankingDataset.UserFeatures, nil)
			}
			features := ctr.ConvertLabelsToFeatures(user.Labels)
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
		span.Add(len(users))
	}
	if err = <-errChan; err != nil {
		return nil, nil, errors.Trace(err)
	}
	log.Logger().Debug("pulled users from database",
		zap.Int("n_users", dataSet.CountUsers()),
		zap.Int32("n_user_labels", userLabelIndex.Len()),
		zap.Duration("used_time", time.Since(start)))
	LoadDatasetStepSecondsVec.WithLabelValues("load_users").Set(time.Since(start).Seconds())

	// STEP 2: pull items
	var items []data.Item
	itemLabelCount := make(map[string]int)
	itemLabelFirst := make(map[string]int32)
	itemLabelIndex := base.NewMapIndex()
	start = time.Now()
	itemChan, errChan := database.GetItemStream(newCtx, batchSize, itemTimeLimit)
	for batchItems := range itemChan {
		items = append(items, batchItems...)
		for _, item := range batchItems {
			dataSet.AddItem(item)
			itemIndex := dataSet.GetItemDict().Id(item.ItemId)
			if len(rankingDataset.ItemFeatures) == int(itemIndex) {
				rankingDataset.ItemFeatures = append(rankingDataset.ItemFeatures, nil)
			}
			features := ctr.ConvertLabelsToFeatures(item.Labels)
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
		}
		span.Add(len(batchItems))
	}
	if err = <-errChan; err != nil {
		return nil, nil, errors.Trace(err)
	}
	log.Logger().Debug("pulled items from database",
		zap.Int("n_items", dataSet.CountItems()),
		zap.Int32("n_item_labels", itemLabelIndex.Len()),
		zap.Duration("used_time", time.Since(start)))
	LoadDatasetStepSecondsVec.WithLabelValues("load_items").Set(time.Since(start).Seconds())

	// create positive set
	positiveSet := make([]mapset.Set[int32], dataSet.CountUsers())
	for i := range positiveSet {
		positiveSet[i] = mapset.NewSet[int32]()
	}

	// split item groups
	sort.Slice(items, func(i, j int) bool {
		return items[i].ItemId < items[j].ItemId
	})
	itemGroups := parallel.Split(items, m.Config.Master.NumJobs)

	// STEP 3: pull positive feedback
	var mu sync.Mutex
	var posFeedbackCount int
	start = time.Now()
	err = parallel.Parallel(len(itemGroups), m.Config.Master.NumJobs, func(_, i int) error {
		var itemFeedback []data.Feedback
		var itemGroupIndex int
		itemHasFeedback := make([]bool, len(itemGroups[i]))
		feedbackChan, errChan := database.GetFeedbackStream(newCtx, batchSize,
			data.WithBeginItemId(itemGroups[i][0].ItemId),
			data.WithEndItemId(itemGroups[i][len(itemGroups[i])-1].ItemId),
			feedbackTimeLimit,
			data.WithEndTime(*m.Config.Now()),
			data.WithFeedbackTypes(posFeedbackTypes...),
			data.WithOrderByItemId())
		for feedback := range feedbackChan {
			for _, f := range feedback {
				// convert user and item id to index
				userIndex := dataSet.GetUserDict().Id(f.UserId)
				if userIndex == base.NotId {
					continue
				}
				itemIndex := dataSet.GetItemDict().Id(f.ItemId)
				if itemIndex == base.NotId {
					continue
				}
				// insert feedback to positive set
				positiveSet[userIndex].Add(itemIndex)

				mu.Lock()
				posFeedbackCount++
				// insert feedback to evaluator
				evaluator.Positive(f.FeedbackType, userIndex, itemIndex, f.Timestamp)
				mu.Unlock()

				// append item feedback
				if len(itemFeedback) == 0 || itemFeedback[len(itemFeedback)-1].ItemId == f.ItemId {
					itemFeedback = append(itemFeedback, f)
				} else {
					// add item to non-personalized recommenders
					itemHasFeedback[itemGroupIndex] = true
					for _, recommender := range nonPersonalizedRecommenders {
						recommender.Push(itemGroups[i][itemGroupIndex], itemFeedback)
					}
					itemFeedback = itemFeedback[:0]
					itemFeedback = append(itemFeedback, f)
				}
				// find item group index
				for itemGroupIndex = 0; itemGroupIndex < len(itemGroups[i]); itemGroupIndex++ {
					if itemGroups[i][itemGroupIndex].ItemId == f.ItemId {
						break
					}
				}
				dataSet.AddFeedback(f.UserId, f.ItemId)
			}
			span.Add(len(feedback))
		}

		// add item to non-personalized recommenders
		if len(itemFeedback) > 0 {
			itemHasFeedback[itemGroupIndex] = true
			for _, recommender := range nonPersonalizedRecommenders {
				recommender.Push(itemGroups[i][itemGroupIndex], itemFeedback)
			}
		}
		for index, hasFeedback := range itemHasFeedback {
			if !hasFeedback {
				for _, recommender := range nonPersonalizedRecommenders {
					recommender.Push(itemGroups[i][index], nil)
				}
			}
		}
		if err = <-errChan; err != nil {
			return errors.Trace(err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	log.Logger().Debug("pulled positive feedback from database",
		zap.Int("n_positive_feedback", posFeedbackCount),
		zap.Duration("used_time", time.Since(start)))
	LoadDatasetStepSecondsVec.WithLabelValues("load_positive_feedback").Set(time.Since(start).Seconds())

	// create negative set
	negativeSet := make([]mapset.Set[int32], dataSet.CountUsers())
	for i := range negativeSet {
		negativeSet[i] = mapset.NewSet[int32]()
	}

	// STEP 4: pull negative feedback
	start = time.Now()
	var negativeFeedbackCount float64
	err = parallel.Parallel(len(itemGroups), m.Config.Master.NumJobs, func(_, i int) error {
		feedbackChan, errChan := database.GetFeedbackStream(newCtx, batchSize,
			data.WithBeginItemId(itemGroups[i][0].ItemId),
			data.WithEndItemId(itemGroups[i][len(itemGroups[i])-1].ItemId),
			feedbackTimeLimit,
			data.WithEndTime(*m.Config.Now()),
			data.WithFeedbackTypes(readTypes...))
		for feedback := range feedbackChan {
			for _, f := range feedback {
				userIndex := dataSet.GetUserDict().Id(f.UserId)
				if userIndex == base.NotId {
					continue
				}
				itemIndex := dataSet.GetItemDict().Id(f.ItemId)
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
			span.Add(len(feedback))
		}
		if err = <-errChan; err != nil {
			return errors.Trace(err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	log.Logger().Debug("pulled negative feedback from database",
		zap.Int("n_negative_feedback", int(negativeFeedbackCount)),
		zap.Duration("used_time", time.Since(start)))
	LoadDatasetStepSecondsVec.WithLabelValues("load_negative_feedback").Set(time.Since(start).Seconds())

	// STEP 5: create click dataset
	start = time.Now()
	unifiedIndex := base.NewUnifiedMapIndexBuilder()
	unifiedIndex.ItemIndex = dataSet.GetItemDict().ToIndex()
	unifiedIndex.UserIndex = dataSet.GetUserDict().ToIndex()
	unifiedIndex.ItemLabelIndex = itemLabelIndex
	unifiedIndex.UserLabelIndex = userLabelIndex
	clickDataset = &ctr.Dataset{
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
	return clickDataset, dataSet, nil
}

func (m *Master) updateItemToItem(dataset *dataset.Dataset) error {
	if len(m.Config.Recommend.ItemToItem) == 0 {
		return nil
	}
	ctx, span := m.tracer.Start(context.Background(), "Generate item-to-item recommendation",
		len(dataset.GetItems())*(len(m.Config.Recommend.ItemToItem))*2)
	defer span.End()

	// Build item-to-item recommenders
	itemToItemRecommenders := make([]logics.ItemToItem, 0, len(m.Config.Recommend.ItemToItem))
	for _, cfg := range m.Config.Recommend.ItemToItem {
		recommender, err := logics.NewItemToItem(cfg, m.Config.Recommend.CacheSize, dataset.GetTimestamp(), &logics.ItemToItemOptions{
			TagsIDF:      dataset.GetItemColumnValuesIDF(),
			UsersIDF:     dataset.GetUserIDF(),
			OpenAIConfig: m.Config.OpenAI,
		})
		if err != nil {
			return errors.Trace(err)
		}
		itemToItemRecommenders = append(itemToItemRecommenders, recommender)
	}

	// Push items to item-to-item recommenders
	for i, item := range dataset.GetItems() {
		if !item.IsHidden {
			for _, recommender := range itemToItemRecommenders {
				recommender.Push(&item, dataset.GetItemFeedback()[i])
				span.Add(1)
			}
		}
	}

	// Save item-to-item recommendations to cache
	for i, recommender := range itemToItemRecommenders {
		pool := recommender.Pool()
		for j, item := range recommender.Items() {
			itemToItemConfig := m.Config.Recommend.ItemToItem[i]
			if m.needUpdateItemToItem(item.ItemId, itemToItemConfig) {
				pool.Run(func() {
					defer span.Add(1)
					score := recommender.PopAll(j)
					if score == nil {
						return
					}
					log.Logger().Debug("update item-to-item recommendation",
						zap.String("item_id", item.ItemId),
						zap.String("name", itemToItemConfig.Name),
						zap.Int("n_recommendations", len(score)))
					// Save item-to-item recommendation to cache
					if err := m.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key(itemToItemConfig.Name, item.ItemId), score); err != nil {
						log.Logger().Error("failed to save item-to-item recommendation to cache",
							zap.String("item_id", item.ItemId), zap.Error(err))
						return
					}
					// Save item-to-item digest and last update time to cache
					if err := m.CacheClient.Set(ctx,
						cache.String(cache.Key(cache.ItemToItemDigest, itemToItemConfig.Name, item.ItemId), itemToItemConfig.Hash()),
						cache.Time(cache.Key(cache.ItemToItemUpdateTime, itemToItemConfig.Name, item.ItemId), time.Now()),
					); err != nil {
						log.Logger().Error("failed to save item-to-item digest to cache",
							zap.String("item_id", item.ItemId), zap.Error(err))
						return
					}
					// Remove stale item-to-item recommendation
					if err := m.CacheClient.DeleteScores(ctx, []string{cache.ItemToItem}, cache.ScoreCondition{
						Subset: lo.ToPtr(cache.Key(itemToItemConfig.Name, item.ItemId)),
						Before: lo.ToPtr(recommender.Timestamp()),
					}); err != nil {
						log.Logger().Error("failed to remove stale item-to-item recommendation",
							zap.String("item_id", item.ItemId), zap.Error(err))
						return
					}
				})
			} else {
				span.Add(1)
			}
		}
		pool.Wait()
	}
	return nil
}

// needUpdateItemToItem checks if item-to-item recommendation needs to be updated.
func (m *Master) needUpdateItemToItem(itemId string, itemToItemConfig config.ItemToItemConfig) bool {
	ctx := context.Background()

	// check cache
	items, err := m.CacheClient.SearchScores(ctx, cache.ItemToItem,
		cache.Key(itemToItemConfig.Name, itemId), nil, 0, -1)
	if err != nil {
		log.Logger().Error("failed to fetch item-to-item recommendation",
			zap.String("item_id", itemId), zap.Error(err))
		return true
	} else if len(items) == 0 {
		return true
	}

	// check digest
	digest, err := m.CacheClient.Get(ctx, cache.Key(cache.ItemToItemDigest, itemToItemConfig.Name, itemId)).String()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read item-to-item digest", zap.Error(err))
		}
		return true
	}
	if digest != itemToItemConfig.Hash() {
		return true
	}

	// check update time
	updateTime, err := m.CacheClient.Get(ctx, cache.Key(cache.ItemToItemUpdateTime, itemToItemConfig.Name, itemId)).Time()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read last update item neighbors time", zap.Error(err))
		}
		return true
	}
	return updateTime.Before(time.Now().Add(-m.Config.Recommend.CacheExpire))
}

func (m *Master) updateUserToUser(dataset *dataset.Dataset) error {
	if len(m.Config.Recommend.UserToUser) == 0 {
		return nil
	}
	ctx, span := m.tracer.Start(context.Background(), "Generate user-to-user recommendation",
		len(dataset.GetUsers())*(len(m.Config.Recommend.UserToUser))*2)
	defer span.End()

	userToUserRecommenders := make([]logics.UserToUser, 0, len(m.Config.Recommend.UserToUser))
	for _, cfg := range m.Config.Recommend.UserToUser {
		recommender, err := logics.NewUserToUser(cfg, m.Config.Recommend.CacheSize, dataset.GetTimestamp(), &logics.UserToUserOptions{
			TagsIDF:  dataset.GetUserColumnValuesIDF(),
			ItemsIDF: dataset.GetItemIDF(),
		})
		if err != nil {
			return errors.Trace(err)
		}
		userToUserRecommenders = append(userToUserRecommenders, recommender)
	}

	// Push users to user-to-user recommender
	for i, user := range dataset.GetUsers() {
		for _, recommender := range userToUserRecommenders {
			recommender.Push(&user, dataset.GetUserFeedback()[i])
			span.Add(1)
		}
	}

	// Save user-to-user recommendations to cache
	for i, recommender := range userToUserRecommenders {
		for j, user := range recommender.Users() {
			userToUserConfig := m.Config.Recommend.UserToUser[i]
			if m.needUpdateUserToUser(user.UserId, userToUserConfig) {
				score := recommender.PopAll(j)
				if score == nil {
					continue
				}
				log.Logger().Debug("update user neighbors",
					zap.String("user_id", user.UserId),
					zap.Int("n_recommendations", len(score)))
				// Save user-to-user recommendations to cache
				if err := m.CacheClient.AddScores(ctx, cache.UserToUser, cache.Key(userToUserConfig.Name, user.UserId), score); err != nil {
					log.Logger().Error("failed to save user neighbors to cache", zap.String("user_id", user.UserId), zap.Error(err))
					continue
				}
				// Save user-to-user digest and last update time to cache
				if err := m.CacheClient.Set(ctx,
					cache.String(cache.Key(cache.UserToUserDigest, cache.Key(userToUserConfig.Name, user.UserId)), userToUserConfig.Hash()),
					cache.Time(cache.Key(cache.UserToUserUpdateTime, cache.Key(userToUserConfig.Name, user.UserId)), time.Now()),
				); err != nil {
					log.Logger().Error("failed to save user neighbors digest to cache", zap.String("user_id", user.UserId), zap.Error(err))
					continue
				}
			}
			span.Add(1)
		}
	}
	return nil
}

// needUpdateUserToUser checks if user-to-user recommendation needs to be updated.
func (m *Master) needUpdateUserToUser(userId string, userToUserConfig config.UserToUserConfig) bool {
	ctx := context.Background()

	// check cache
	if items, err := m.CacheClient.SearchScores(ctx, cache.UserToUser, cache.Key(userToUserConfig.Name, userId), nil, 0, -1); err != nil {
		log.Logger().Error("failed to load user neighbors", zap.String("user_id", userId), zap.Error(err))
		return true
	} else if len(items) == 0 {
		return true
	}

	// read digest
	cacheDigest, err := m.CacheClient.Get(ctx, cache.Key(cache.UserToUserDigest, cache.Key(userToUserConfig.Name, userId))).String()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read user neighbors digest", zap.Error(err))
		}
		return true
	}
	if cacheDigest != userToUserConfig.Hash() {
		return true
	}

	// check update time
	updateTime, err := m.CacheClient.Get(ctx, cache.Key(cache.UserToUserUpdateTime, cache.Key(userToUserConfig.Name, userId))).Time()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read last update user neighbors time", zap.Error(err))
		}
		return true
	}
	return updateTime.Before(time.Now().Add(-m.Config.Recommend.CacheExpire))
}

func (m *Master) trainCollaborativeFiltering(trainSet, testSet dataset.CFSplit) error {
	newCtx, span := m.tracer.Start(context.Background(), "Train Collaborative Filtering Model", 1)
	defer span.End()

	if trainSet.CountUsers() == 0 {
		span.Fail(errors.New("No user found."))
		return nil
	} else if trainSet.CountItems() == 0 {
		span.Fail(errors.New("No item found."))
		return nil
	} else if trainSet.CountFeedback() == 0 {
		span.Fail(errors.New("No feedback found."))
		return nil
	} else if trainSet.CountFeedback() == m.collaborativeFilteringTrainSetSize {
		log.Logger().Info("collaborative filtering dataset not changed")
		return nil
	}

	bestModelName, bestModel, bestModelScore := m.collaborativeFilteringSearcher.GetBestModel()
	m.collaborativeFilteringModelMutex.Lock()
	if bestModel != nil && !bestModel.Invalid() &&
		(bestModelName != m.collaborativeFilteringModelName ||
			bestModel.GetParams().ToString() != m.CollaborativeFilteringModel.GetParams().ToString()) &&
		(bestModelScore.NDCG > m.collaborativeFilteringModelScore.NDCG) {
		// 1. best ranking model must have been found.
		// 2. best ranking model must be different from current model
		// 3. best ranking model must perform better than current model
		m.CollaborativeFilteringModel = bestModel
		m.collaborativeFilteringModelName = bestModelName
		m.collaborativeFilteringModelScore = bestModelScore
		log.Logger().Info("find better collaborative filtering model",
			zap.Any("score", bestModelScore),
			zap.String("name", bestModelName),
			zap.Any("params", m.CollaborativeFilteringModel.GetParams()))
	}
	collaborativeFilteringModel := cf.Clone(m.CollaborativeFilteringModel)
	m.collaborativeFilteringModelMutex.Unlock()

	startFitTime := time.Now()
	score := collaborativeFilteringModel.Fit(newCtx, trainSet, testSet, cf.NewFitConfig())
	CollaborativeFilteringFitSeconds.Set(time.Since(startFitTime).Seconds())

	// update ranking model
	m.collaborativeFilteringModelMutex.Lock()
	m.CollaborativeFilteringModel = collaborativeFilteringModel
	m.CollaborativeFilteringModelVersion++
	m.collaborativeFilteringTrainSetSize = trainSet.CountFeedback()
	m.collaborativeFilteringModelScore = score
	m.collaborativeFilteringModelMutex.Unlock()
	log.Logger().Info("fit collaborative filtering model completed",
		zap.String("version", fmt.Sprintf("%x", m.CollaborativeFilteringModelVersion)))
	CollaborativeFilteringNDCG10.Set(float64(score.NDCG))
	CollaborativeFilteringRecall10.Set(float64(score.Recall))
	CollaborativeFilteringPrecision10.Set(float64(score.Precision))
	MemoryInUseBytesVec.WithLabelValues("collaborative_filtering_model").Set(float64(sizeof.DeepSize(m.CollaborativeFilteringModel)))
	if err := m.CacheClient.Set(context.Background(), cache.Time(cache.Key(cache.GlobalMeta, cache.LastFitMatchingModelTime), time.Now())); err != nil {
		log.Logger().Error("failed to write meta", zap.Error(err))
	}

	// caching model
	m.collaborativeFilteringModelMutex.RLock()
	m.localCache.CollaborativeFilteringModelName = m.collaborativeFilteringModelName
	m.localCache.CollaborativeFilteringModelVersion = m.CollaborativeFilteringModelVersion
	m.localCache.CollaborativeFilteringModel = collaborativeFilteringModel
	m.localCache.CollaborativeFilteringModelScore = score
	m.collaborativeFilteringModelMutex.RUnlock()
	if m.localCache.ClickModel == nil || m.localCache.ClickModel.Invalid() {
		log.Logger().Info("wait click model")
	} else if err := m.localCache.WriteLocalCache(); err != nil {
		log.Logger().Error("failed to write local cache", zap.Error(err))
	} else {
		log.Logger().Info("write model to local cache",
			zap.String("collaborative_filtering_model_name", m.localCache.CollaborativeFilteringModelName),
			zap.String("collaborative_filtering_model_version", encoding.Hex(m.localCache.CollaborativeFilteringModelVersion)),
			zap.Float32("collaborative_filtering_model_score", m.localCache.CollaborativeFilteringModelScore.NDCG),
			zap.Any("collaborative_filtering_model_params", m.localCache.CollaborativeFilteringModel.GetParams()))
	}
	return nil
}

func (m *Master) trainClickThroughRatePrediction(trainSet, testSet *ctr.Dataset) error {
	newCtx, span := m.tracer.Start(context.Background(), "Train Click-Through Rate Prediction Model", 1)
	defer span.End()

	if trainSet.CountUsers() == 0 {
		span.Fail(errors.New("No user found."))
		return nil
	} else if trainSet.CountItems() == 0 {
		span.Fail(errors.New("No item found."))
		return nil
	} else if trainSet.Count() == 0 {
		span.Fail(errors.New("No feedback found."))
		return nil
	} else if trainSet.Count() == m.clickTrainSetSize {
		log.Logger().Info("click dataset not changed")
		return nil
	}

	bestModel, bestScore := m.clickModelSearcher.GetBestModel()
	m.clickModelMutex.Lock()
	if bestModel != nil && !bestModel.Invalid() &&
		bestModel.GetParams().ToString() != m.ClickModel.GetParams().ToString() &&
		bestScore.Precision > m.clickScore.Precision {
		// 1. best click model must have been found.
		// 2. best click model must be different from current model
		// 3. best click model must perform better than current model
		m.ClickModel = bestModel
		m.clickScore = bestScore
		log.Logger().Info("find better click model",
			zap.Float32("Precision", bestScore.Precision),
			zap.Float32("Recall", bestScore.Recall),
			zap.Any("params", m.ClickModel.GetParams()))
	}
	clickModel := ctr.Clone(m.ClickModel)
	m.clickModelMutex.Unlock()

	startFitTime := time.Now()
	score := clickModel.Fit(newCtx, trainSet, testSet, ctr.NewFitConfig())
	RankingFitSeconds.Set(time.Since(startFitTime).Seconds())

	// update match model
	m.clickModelMutex.Lock()
	m.ClickModel = clickModel
	m.clickTrainSetSize = trainSet.Count()
	m.clickScore = score
	m.ClickModelVersion++
	m.clickModelMutex.Unlock()
	log.Logger().Info("fit click model complete",
		zap.String("version", fmt.Sprintf("%x", m.ClickModelVersion)))
	RankingPrecision.Set(float64(score.Precision))
	RankingRecall.Set(float64(score.Recall))
	RankingAUC.Set(float64(score.AUC))
	MemoryInUseBytesVec.WithLabelValues("ranking_model").Set(float64(sizeof.DeepSize(m.ClickModel)))
	if err := m.CacheClient.Set(context.Background(), cache.Time(cache.Key(cache.GlobalMeta, cache.LastFitRankingModelTime), time.Now())); err != nil {
		log.Logger().Error("failed to write meta", zap.Error(err))
	}

	// caching model
	m.clickModelMutex.RLock()
	m.localCache.ClickModelScore = m.clickScore
	m.localCache.ClickModelVersion = m.ClickModelVersion
	m.localCache.ClickModel = m.ClickModel
	m.clickModelMutex.RUnlock()
	if m.localCache.CollaborativeFilteringModel == nil || m.localCache.CollaborativeFilteringModel.Invalid() {
		log.Logger().Info("wait collaborative filtering model")
	} else if err := m.localCache.WriteLocalCache(); err != nil {
		log.Logger().Error("failed to write local cache", zap.Error(err))
	} else {
		log.Logger().Info("write model to local cache",
			zap.String("click_model_version", encoding.Hex(m.localCache.ClickModelVersion)),
			zap.Float32("click_model_score", score.Precision),
			zap.Any("click_model_params", m.localCache.ClickModel.GetParams()))
	}
	return nil
}
