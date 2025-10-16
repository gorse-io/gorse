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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/c-bata/goptuna"
	"github.com/c-bata/goptuna/tpe"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/common/expression"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/common/sizeof"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/logics"
	"github.com/gorse-io/gorse/model"
	"github.com/gorse-io/gorse/model/cf"
	"github.com/gorse-io/gorse/model/ctr"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/meta"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

const (
	PositiveFeedbackRate = "PositiveFeedbackRate"
	batchSize            = 10000
)

func (m *Master) loadDataset() (datasets Datasets, err error) {
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
			return Datasets{}, errors.Trace(err)
		}
		nonPersonalizedRecommenders = append(nonPersonalizedRecommenders, recommender)
	}

	log.Logger().Info("load dataset",
		zap.Any("positive_feedback_types", m.Config.Recommend.DataSource.PositiveFeedbackTypes),
		zap.Any("read_feedback_types", m.Config.Recommend.DataSource.ReadFeedbackTypes),
		zap.Uint("item_ttl", m.Config.Recommend.DataSource.ItemTTL),
		zap.Uint("feedback_ttl", m.Config.Recommend.DataSource.PositiveFeedbackTTL))
	evaluator := NewOnlineEvaluator()
	datasets.clickDataset, datasets.rankingDataset, err = m.LoadDataFromDatabase(ctx, m.DataClient,
		m.Config.Recommend.DataSource.PositiveFeedbackTypes,
		m.Config.Recommend.DataSource.ReadFeedbackTypes,
		m.Config.Recommend.DataSource.ItemTTL,
		m.Config.Recommend.DataSource.PositiveFeedbackTTL,
		evaluator,
		nonPersonalizedRecommenders)
	if err != nil {
		return Datasets{}, errors.Trace(err)
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
	if err = m.CacheClient.AddTimeSeriesPoints(ctx, []cache.TimeSeriesPoint{
		{Name: cache.NumUsers, Value: float64(datasets.rankingDataset.CountUsers()), Timestamp: datasets.rankingDataset.GetTimestamp()},
		{Name: cache.NumItems, Value: float64(datasets.rankingDataset.CountItems()), Timestamp: datasets.rankingDataset.GetTimestamp()},
		{Name: cache.NumFeedback, Value: float64(len(datasets.clickDataset.Target)), Timestamp: datasets.rankingDataset.GetTimestamp()},
		{Name: cache.NumPosFeedbacks, Value: float64(datasets.clickDataset.PositiveCount), Timestamp: datasets.rankingDataset.GetTimestamp()},
		{Name: cache.NumNegFeedbacks, Value: float64(datasets.clickDataset.NegativeCount), Timestamp: datasets.rankingDataset.GetTimestamp()},
	}); err != nil {
		log.Logger().Error("failed to write timeseries points", zap.Error(err))
	}
	UsersTotal.Set(float64(datasets.rankingDataset.CountUsers()))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumUsers), datasets.rankingDataset.CountUsers())); err != nil {
		log.Logger().Error("failed to write number of users", zap.Error(err))
	}
	ItemsTotal.Set(float64(datasets.rankingDataset.CountItems()))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumItems), datasets.rankingDataset.CountItems())); err != nil {
		log.Logger().Error("failed to write number of items", zap.Error(err))
	}
	ImplicitFeedbacksTotal.Set(float64(datasets.rankingDataset.CountFeedback()))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumTotalPosFeedbacks), datasets.rankingDataset.CountFeedback())); err != nil {
		log.Logger().Error("failed to write number of positive feedbacks", zap.Error(err))
	}
	UserLabelsTotal.Set(float64(datasets.clickDataset.Index.CountUserLabels()))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumUserLabels), int(datasets.clickDataset.Index.CountUserLabels()))); err != nil {
		log.Logger().Error("failed to write number of user labels", zap.Error(err))
	}
	ItemLabelsTotal.Set(float64(datasets.clickDataset.Index.CountItemLabels()))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumItemLabels), int(datasets.clickDataset.Index.CountItemLabels()))); err != nil {
		log.Logger().Error("failed to write number of item labels", zap.Error(err))
	}
	ImplicitFeedbacksTotal.Set(float64(datasets.rankingDataset.CountFeedback()))
	PositiveFeedbacksTotal.Set(float64(datasets.clickDataset.PositiveCount))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumValidPosFeedbacks), datasets.clickDataset.PositiveCount)); err != nil {
		log.Logger().Error("failed to write number of positive feedbacks", zap.Error(err))
	}
	NegativeFeedbackTotal.Set(float64(datasets.clickDataset.NegativeCount))
	if err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.NumValidNegFeedbacks), datasets.clickDataset.NegativeCount)); err != nil {
		log.Logger().Error("failed to write number of negative feedbacks", zap.Error(err))
	}

	// evaluate positive feedback rate
	points := evaluator.Evaluate()
	if err = m.CacheClient.AddTimeSeriesPoints(ctx, points); err != nil {
		log.Logger().Error("failed to insert measurement", zap.Error(err))
	}

	// collect active users and items
	activeUsers, activeItems, inactiveUsers, inactiveItems := 0, 0, 0, 0
	for _, userFeedback := range datasets.rankingDataset.GetUserFeedback() {
		if len(userFeedback) > 0 {
			activeUsers++
		} else {
			inactiveUsers++
		}
	}
	for _, itemFeedback := range datasets.rankingDataset.GetItemFeedback() {
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
	if err = m.CacheClient.SetSet(ctx, cache.ItemCategories, datasets.rankingDataset.GetCategories()...); err != nil {
		log.Logger().Error("failed to write categories to cache", zap.Error(err))
	}

	// split ranking dataset
	startTime := time.Now()
	datasets.rankingTrainSet, datasets.rankingTestSet = datasets.rankingDataset.SplitCF(0, 0)
	LoadDatasetStepSecondsVec.WithLabelValues("split_ranking_dataset").Set(time.Since(startTime).Seconds())
	MemoryInUseBytesVec.WithLabelValues("collaborative_filtering_train_set").Set(float64(sizeof.DeepSize(datasets.rankingTrainSet)))
	MemoryInUseBytesVec.WithLabelValues("collaborative_filtering_test_set").Set(float64(sizeof.DeepSize(datasets.rankingTestSet)))

	// split click dataset
	startTime = time.Now()
	datasets.clickTrainSet, datasets.clickTestSet = datasets.clickDataset.Split(0.2, 0)
	datasets.clickDataset = nil
	LoadDatasetStepSecondsVec.WithLabelValues("split_click_dataset").Set(time.Since(startTime).Seconds())
	MemoryInUseBytesVec.WithLabelValues("ranking_train_set").Set(float64(sizeof.DeepSize(datasets.clickTrainSet)))
	MemoryInUseBytesVec.WithLabelValues("ranking_test_set").Set(float64(sizeof.DeepSize(datasets.clickTestSet)))

	LoadDatasetTotalSeconds.Set(time.Since(initialStartTime).Seconds())
	return
}

// runLoadDatasetTask loads dataset.
func (m *Master) runLoadDatasetTask() error {
	ctx, cancel := context.WithTimeout(context.Background(), m.Config.Recommend.Collaborative.ModelFitPeriod)
	defer cancel()
	datasets, err := m.loadDataset()
	if err != nil {
		return errors.Trace(err)
	}
	if err = m.updateUserToUser(datasets.rankingDataset); err != nil {
		log.Logger().Error("failed to update user-to-user recommendation", zap.Error(err))
	}
	if err = m.updateItemToItem(datasets.rankingDataset); err != nil {
		log.Logger().Error("failed to update item-to-item recommendation", zap.Error(err))
	}
	if err = m.trainCollaborativeFiltering(datasets.rankingTrainSet, datasets.rankingTestSet); err != nil {
		log.Logger().Error("failed to train collaborative filtering model", zap.Error(err))
	}
	if err = m.trainClickThroughRatePrediction(datasets.clickTrainSet, datasets.clickTestSet); err != nil {
		log.Logger().Error("failed to train click-through rate prediction model", zap.Error(err))
	}
	if err = m.collectGarbage(ctx, datasets.rankingDataset); err != nil {
		log.Logger().Error("failed to collect garbage in cache", zap.Error(err))
	}
	if err = m.optimizeCollaborativeFiltering(datasets.rankingTrainSet, datasets.rankingTestSet); err != nil {
		log.Logger().Error("failed to optimize collaborative filtering model", zap.Error(err))
	}
	if err = m.optimizeClickThroughRatePrediction(datasets.clickTrainSet, datasets.clickTestSet); err != nil {
		log.Logger().Error("failed to optimize click-through rate prediction model", zap.Error(err))
	}
	return nil
}

// LoadDataFromDatabase loads dataset from data store.
func (m *Master) LoadDataFromDatabase(
	ctx context.Context,
	database data.Database,
	posFeedbackTypes, readTypes []expression.FeedbackTypeExpression,
	itemTTL, positiveFeedbackTTL uint,
	evaluator *OnlineEvaluator,
	nonPersonalizedRecommenders []*logics.NonPersonalized,
) (ctrDataset *ctr.Dataset, dataSet *dataset.Dataset, err error) {
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

	newCtx, span := monitor.Start(ctx, "LoadDataFromDatabase",
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

	// STEP 1: pull users
	userLabelCount := make(map[string]int)
	userLabelFirst := make(map[string]int32)
	userLabelIndex := dataset.NewMapIndex()
	userLabels := make([][]lo.Tuple2[int32, float32], 0, estimatedNumUsers)
	start := time.Now()
	userChan, errChan := database.GetUserStream(newCtx, batchSize)
	for users := range userChan {
		for _, user := range users {
			dataSet.AddUser(user)
			userIndex := dataSet.GetUserDict().Id(user.UserId)
			if len(userLabels) == int(userIndex) {
				userLabels = append(userLabels, nil)
			}
			features := ctr.ConvertLabels(user.Labels)
			userLabels[userIndex] = make([]lo.Tuple2[int32, float32], 0, len(features))
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
					userLabels[firstUserIndex] = append(userLabels[firstUserIndex], lo.Tuple2[int32, float32]{
						A: userLabelIndex.ToNumber(feature.Name),
						B: feature.Value,
					})
				}
				// Add the label to the user.
				if userLabelCount[feature.Name] > 1 {
					userLabels[userIndex] = append(userLabels[userIndex], lo.Tuple2[int32, float32]{
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
	itemLabelIndex := dataset.NewMapIndex()
	itemLabels := make([][]lo.Tuple2[int32, float32], 0, estimatedNumItems)
	itemEmbeddingIndexer := dataset.NewMapIndex()
	itemEmbeddingDimension := make([]map[int]int, 0)
	itemEmbeddings := make([][][]float32, 0, estimatedNumItems)
	start = time.Now()
	itemChan, errChan := database.GetItemStream(newCtx, batchSize, itemTimeLimit)
	for batchItems := range itemChan {
		items = append(items, batchItems...)
		for _, item := range batchItems {
			dataSet.AddItem(item)
			itemIndex := dataSet.GetItemDict().Id(item.ItemId)
			if len(itemLabels) == int(itemIndex) {
				itemLabels = append(itemLabels, nil)
			}
			if len(itemEmbeddings) == int(itemIndex) {
				itemEmbeddings = append(itemEmbeddings, nil)
			}
			// load labels
			labels := ctr.ConvertLabels(item.Labels)
			itemLabels[itemIndex] = make([]lo.Tuple2[int32, float32], 0, len(labels))
			for _, feature := range labels {
				itemLabelCount[feature.Name]++
				// Memorize the first occurrence.
				if itemLabelCount[feature.Name] == 1 {
					itemLabelFirst[feature.Name] = itemIndex
				}
				// Add the label to the index in second occurrence.
				if itemLabelCount[feature.Name] == 2 {
					itemLabelIndex.Add(feature.Name)
					firstItemIndex := itemLabelFirst[feature.Name]
					itemLabels[firstItemIndex] = append(itemLabels[firstItemIndex], lo.Tuple2[int32, float32]{
						A: itemLabelIndex.ToNumber(feature.Name),
						B: feature.Value,
					})
				}
				// Add the label to the item.
				if itemLabelCount[feature.Name] > 1 {
					itemLabels[itemIndex] = append(itemLabels[itemIndex], lo.Tuple2[int32, float32]{
						A: itemLabelIndex.ToNumber(feature.Name),
						B: feature.Value,
					})
				}
			}
			// load embeddings
			embeddings := ctr.ConvertEmbeddings(item.Labels)
			itemEmbeddings[itemIndex] = make([][]float32, 0, len(embeddings))
			for _, embedding := range embeddings {
				itemEmbeddingIndexer.Add(embedding.Name)
				itemEmbeddingIndex := itemEmbeddingIndexer.ToNumber(embedding.Name)
				for len(itemEmbeddings[itemIndex]) <= int(itemEmbeddingIndex) {
					itemEmbeddings[itemIndex] = append(itemEmbeddings[itemIndex], nil)
				}
				itemEmbeddings[itemIndex][itemEmbeddingIndex] = embedding.Value
				for len(itemEmbeddingDimension) <= int(itemEmbeddingIndex) {
					itemEmbeddingDimension = append(itemEmbeddingDimension, make(map[int]int))
				}
				itemEmbeddingDimension[itemEmbeddingIndex][len(itemEmbeddings[itemIndex][itemEmbeddingIndex])]++
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
				if userIndex == dataset.NotId {
					continue
				}
				itemIndex := dataSet.GetItemDict().Id(f.ItemId)
				if itemIndex == dataset.NotId {
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
				if userIndex == dataset.NotId {
					continue
				}
				itemIndex := dataSet.GetItemDict().Id(f.ItemId)
				if itemIndex == dataset.NotId {
					continue
				}
				negativeSet[userIndex].Add(itemIndex)
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

	// STEP 5: create click-through rate dataset
	start = time.Now()
	unifiedIndex := dataset.NewUnifiedMapIndexBuilder()
	unifiedIndex.ItemIndex = dataSet.GetItemDict().ToIndex()
	unifiedIndex.UserIndex = dataSet.GetUserDict().ToIndex()
	unifiedIndex.ItemLabelIndex = itemLabelIndex
	unifiedIndex.UserLabelIndex = userLabelIndex
	ctrDataset = &ctr.Dataset{
		Index:      unifiedIndex.Build(),
		UserLabels: userLabels,
		ItemLabels: itemLabels,
		Users:      make([]int32, 0, estimatedNumFeedbacks),
		Items:      make([]int32, 0, estimatedNumFeedbacks),
		Target:     make([]float32, 0, estimatedNumFeedbacks),
	}
	ctrDataset.ItemEmbeddingIndex = itemEmbeddingIndexer
	ctrDataset.ItemEmbeddingDimension = make([]int, len(itemEmbeddingDimension))
	for i, dimension := range itemEmbeddingDimension {
		for dim, cnt := range dimension {
			if cnt > itemEmbeddingDimension[i][ctrDataset.ItemEmbeddingDimension[i]] {
				ctrDataset.ItemEmbeddingDimension[i] = dim
			}
		}
	}
	for i, embeddings := range itemEmbeddings {
		for j, embedding := range embeddings {
			if len(embedding) != ctrDataset.ItemEmbeddingDimension[j] {
				itemEmbeddings[i][j] = nil
			}
		}
	}
	ctrDataset.ItemEmbeddings = itemEmbeddings
	for userIndex := range positiveSet {
		// insert positive feedback
		for _, itemIndex := range positiveSet[userIndex].ToSlice() {
			ctrDataset.Users = append(ctrDataset.Users, int32(userIndex))
			ctrDataset.Items = append(ctrDataset.Items, itemIndex)
			ctrDataset.Target = append(ctrDataset.Target, 1)
			ctrDataset.PositiveCount++
		}
		// insert negative feedback
		for _, itemIndex := range negativeSet[userIndex].ToSlice() {
			ctrDataset.Users = append(ctrDataset.Users, int32(userIndex))
			ctrDataset.Items = append(ctrDataset.Items, itemIndex)
			ctrDataset.Target = append(ctrDataset.Target, -1)
			ctrDataset.NegativeCount++
		}
		// release positive set and negative set
		positiveSet[userIndex] = nil
		negativeSet[userIndex] = nil
	}
	log.Logger().Debug("created ranking dataset",
		zap.Int("n_valid_positive", ctrDataset.PositiveCount),
		zap.Int("n_valid_negative", ctrDataset.NegativeCount),
		zap.Duration("used_time", time.Since(start)))
	LoadDatasetStepSecondsVec.WithLabelValues("create_ranking_dataset").Set(time.Since(start).Seconds())
	return ctrDataset, dataSet, nil
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
	parallel.ForEach(dataset.GetItems(), m.Config.Master.NumJobs, func(i int, item data.Item) {
		if !item.IsHidden {
			for _, recommender := range itemToItemRecommenders {
				recommender.Push(&item, dataset.GetItemFeedback()[i])
				span.Add(1)
			}
		}
	})

	// Save item-to-item recommendations to cache
	for i, recommender := range itemToItemRecommenders {
		pool := recommender.Pool()
		parallel.ForEach(recommender.Items(), m.Config.Master.NumJobs, func(j int, item *data.Item) {
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
		})
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
	parallel.ForEach(dataset.GetUsers(), m.Config.Master.NumJobs, func(i int, user data.User) {
		for _, recommender := range userToUserRecommenders {
			recommender.Push(&user, dataset.GetUserFeedback()[i])
			span.Add(1)
		}
	})

	// Save user-to-user recommendations to cache
	for i, recommender := range userToUserRecommenders {
		parallel.ForEach(recommender.Users(), m.Config.Master.NumJobs, func(j int, user *data.User) {
			userToUserConfig := m.Config.Recommend.UserToUser[i]
			if m.needUpdateUserToUser(user.UserId, userToUserConfig) {
				score := recommender.PopAll(j)
				if score == nil {
					return
				}
				log.Logger().Debug("update user neighbors",
					zap.String("user_id", user.UserId),
					zap.Int("n_recommendations", len(score)))
				// Save user-to-user recommendations to cache
				if err := m.CacheClient.AddScores(ctx, cache.UserToUser, cache.Key(userToUserConfig.Name, user.UserId), score); err != nil {
					log.Logger().Error("failed to save user neighbors to cache", zap.String("user_id", user.UserId), zap.Error(err))
					return
				}
				// Save user-to-user digest and last update time to cache
				if err := m.CacheClient.Set(ctx,
					cache.String(cache.Key(cache.UserToUserDigest, cache.Key(userToUserConfig.Name, user.UserId)), userToUserConfig.Hash()),
					cache.Time(cache.Key(cache.UserToUserUpdateTime, cache.Key(userToUserConfig.Name, user.UserId)), time.Now()),
				); err != nil {
					log.Logger().Error("failed to save user neighbors digest to cache", zap.String("user_id", user.UserId), zap.Error(err))
					return
				}
				// Delete stale user-to-user recommendations
				if err := m.CacheClient.DeleteScores(ctx, []string{cache.UserToUser}, cache.ScoreCondition{
					Subset: lo.ToPtr(cache.Key(userToUserConfig.Name, user.UserId)),
					Before: lo.ToPtr(recommender.Timestamp()),
				}); err != nil {
					log.Logger().Error("failed to remove stale user neighbors", zap.String("user_id", user.UserId), zap.Error(err))
				}
			}
			span.Add(1)
		})
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
	newCtx, span := m.tracer.Start(context.Background(), "Train Collaborative Filtering Model", 2)
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

	m.collaborativeFilteringModelMutex.Lock()
	collaborativeFilteringType := m.collaborativeFilteringMeta.Type
	collaborativeFilteringParams := m.collaborativeFilteringMeta.Params
	if m.collaborativeFilteringTarget.Score.NDCG > 0 &&
		(!m.collaborativeFilteringTarget.Equal(m.collaborativeFilteringMeta)) &&
		(m.collaborativeFilteringTarget.Score.NDCG > m.collaborativeFilteringMeta.Score.NDCG) {
		// 1. best ranking model must have been found.
		// 2. best ranking model must be different from current model
		// 3. best ranking model must perform better than current model
		collaborativeFilteringType = m.collaborativeFilteringTarget.Type
		collaborativeFilteringParams = m.collaborativeFilteringTarget.Params
		log.Logger().Info("find better collaborative filtering model",
			zap.Any("score", m.collaborativeFilteringTarget.Score),
			zap.String("type", collaborativeFilteringType),
			zap.Any("params", collaborativeFilteringParams))
	}
	m.collaborativeFilteringModelMutex.Unlock()

	startFitTime := time.Now()
	fitCtx, fitSpan := monitor.Start(newCtx, "Fit", 1)
	collaborativeFilteringModel := m.newCollaborativeFilteringModel(collaborativeFilteringType, collaborativeFilteringParams)
	score := collaborativeFilteringModel.Fit(fitCtx, trainSet, testSet,
		cf.NewFitConfig().
			SetJobs(m.Config.Master.NumJobs).
			SetPatience(m.Config.Recommend.Collaborative.EarlyStopping.Patience))
	CollaborativeFilteringFitSeconds.Set(time.Since(startFitTime).Seconds())
	span.Add(1)
	fitSpan.End()

	_, indexSpan := monitor.Start(newCtx, "Index", trainSet.CountItems())
	matrixFactorizationItems := logics.NewMatrixFactorizationItems(time.Now())
	parallel.For(trainSet.CountItems(), m.Config.Master.NumJobs, func(i int) {
		defer indexSpan.Add(1)
		if itemId, ok := trainSet.GetItemDict().String(int32(i)); ok && collaborativeFilteringModel.IsItemPredictable(int32(i)) {
			matrixFactorizationItems.Add(itemId, collaborativeFilteringModel.GetItemFactor(int32(i)))
		}
	})
	span.Add(1)
	indexSpan.End()

	matrixFactorizationUsers := logics.NewMatrixFactorizationUsers()
	for i := 0; i < trainSet.CountUsers(); i++ {
		if userId, ok := trainSet.GetUserDict().String(int32(i)); ok && collaborativeFilteringModel.IsUserPredictable(int32(i)) {
			matrixFactorizationUsers.Add(userId, collaborativeFilteringModel.GetUserFactor(int32(i)))
		}
	}

	// update ranking model
	m.collaborativeFilteringModelMutex.Lock()
	m.collaborativeFilteringTrainSetSize = trainSet.CountFeedback()
	m.collaborativeFilteringModelMutex.Unlock()
	collaborativeFilteringModelId := time.Now().Unix()
	log.Logger().Info("fit collaborative filtering model completed",
		zap.Int64("id", collaborativeFilteringModelId))
	CollaborativeFilteringNDCG10.Set(float64(score.NDCG))
	CollaborativeFilteringRecall10.Set(float64(score.Recall))
	CollaborativeFilteringPrecision10.Set(float64(score.Precision))
	if err := m.CacheClient.Set(context.Background(), cache.Time(cache.Key(cache.GlobalMeta, cache.LastFitMatchingModelTime), time.Now())); err != nil {
		log.Logger().Error("failed to write meta", zap.Error(err))
	}

	// upload model
	w, done, err := m.blobStore.Create(strconv.FormatInt(collaborativeFilteringModelId, 10))
	if err != nil {
		log.Logger().Error("failed to create blob for collaborative filtering model",
			zap.Int64("id", collaborativeFilteringModelId), zap.Error(err))
		return err
	}
	if err = matrixFactorizationItems.Marshal(w); err != nil {
		log.Logger().Error("failed to matrix factorization items",
			zap.Int64("id", collaborativeFilteringModelId), zap.Error(err))
		return err
	}
	if err = matrixFactorizationUsers.Marshal(w); err != nil {
		log.Logger().Error("failed to matrix factorization users",
			zap.Int64("id", collaborativeFilteringModelId), zap.Error(err))
		return err
	}
	if err = w.Close(); err != nil {
		log.Logger().Error("failed to close blob for collaborative filtering model",
			zap.Int64("id", collaborativeFilteringModelId), zap.Error(err))
		return err
	}
	<-done

	// update meta
	m.collaborativeFilteringModelMutex.RLock()
	m.collaborativeFilteringMeta.ID = collaborativeFilteringModelId
	m.collaborativeFilteringMeta.Type = collaborativeFilteringType
	m.collaborativeFilteringMeta.Params = collaborativeFilteringParams
	m.collaborativeFilteringMeta.Score = score
	m.collaborativeFilteringModelMutex.RUnlock()
	if err = m.metaStore.Put(meta.COLLABORATIVE_FILTERING_MODEL, m.collaborativeFilteringMeta.ToJSON()); err != nil {
		log.Logger().Error("failed to write collaborative filtering model meta", zap.Error(err))
		return err
	} else {
		log.Logger().Info("write collaborative filtering model meta",
			zap.Int64("id", collaborativeFilteringModelId),
			zap.Float32("ndcg", score.NDCG),
			zap.Float32("recall", score.Recall),
			zap.Float32("precision", score.Precision))
	}

	// update statistics
	if err = m.CacheClient.AddTimeSeriesPoints(context.Background(), []cache.TimeSeriesPoint{
		{Name: cache.CFNDCG, Value: float64(score.NDCG), Timestamp: time.Now()},
		{Name: cache.CFPrecision, Value: float64(score.Precision), Timestamp: time.Now()},
		{Name: cache.CFRecall, Value: float64(score.Recall), Timestamp: time.Now()},
	}); err != nil {
		log.Logger().Error("failed to write time series points", zap.Error(err))
		return nil
	}

	m.removeOutOfDateModels()
	return nil
}

func (m *Master) newCollaborativeFilteringModel(modelType string, params model.Params) cf.MatrixFactorization {
	switch modelType {
	case "BPR":
		return cf.NewBPR(params)
	case "ALS":
		return cf.NewALS(params)
	default:
		return cf.NewBPR(params)
	}
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
	} else if trainSet.Count() == m.clickThroughRateTrainSetSize {
		log.Logger().Info("click dataset not changed")
		return nil
	}

	m.clickThroughRateModelMutex.Lock()
	clickThroughRateType := m.clickThroughRateMeta.Type
	clickThroughRateParams := m.clickThroughRateMeta.Params
	if m.clickThroughRateTarget.Score.AUC > 0 &&
		(!m.clickThroughRateTarget.Equal(m.clickThroughRateMeta)) &&
		(m.clickThroughRateTarget.Score.AUC > m.clickThroughRateMeta.Score.AUC) {
		// 1. best click model must have been found.
		// 2. best click model must be different from current model
		// 3. best click model must perform better than current model
		clickThroughRateType = m.clickThroughRateTarget.Type
		clickThroughRateParams = m.clickThroughRateTarget.Params
		log.Logger().Info("find better click model",
			zap.Float32("Precision", m.clickThroughRateTarget.Score.Precision),
			zap.Float32("Recall", m.clickThroughRateTarget.Score.Recall),
			zap.Any("params", clickThroughRateParams))
	}
	clickModel := ctr.NewFMV2(clickThroughRateParams)
	m.clickThroughRateModelMutex.Unlock()

	startFitTime := time.Now()
	score := clickModel.Fit(newCtx, trainSet, testSet,
		ctr.NewFitConfig().
			SetJobs(m.Config.Master.NumJobs).
			SetPatience(m.Config.Recommend.Offline.EarlyStopping.Patience))
	RankingFitSeconds.Set(time.Since(startFitTime).Seconds())

	// update match model
	m.clickThroughRateModelMutex.Lock()
	m.clickThroughRateTrainSetSize = trainSet.Count()
	clickThroughRateModelId := time.Now().Unix()
	m.clickThroughRateModelMutex.Unlock()
	log.Logger().Info("fit click model complete",
		zap.Int64("id", clickThroughRateModelId))
	RankingPrecision.Set(float64(score.Precision))
	RankingRecall.Set(float64(score.Recall))
	RankingAUC.Set(float64(score.AUC))
	if err := m.CacheClient.Set(context.Background(), cache.Time(cache.Key(cache.GlobalMeta, cache.LastFitRankingModelTime), time.Now())); err != nil {
		log.Logger().Error("failed to write meta", zap.Error(err))
	}

	// upload model
	w, done, err := m.blobStore.Create(strconv.FormatInt(clickThroughRateModelId, 10))
	if err != nil {
		log.Logger().Error("failed to create blob for click-through rate model",
			zap.Int64("id", clickThroughRateModelId), zap.Error(err))
		return err
	}
	if err = ctr.MarshalModel(w, clickModel); err != nil {
		log.Logger().Error("failed to marshal click-through rate model",
			zap.Int64("id", clickThroughRateModelId), zap.Error(err))
		return err
	}
	if err = w.Close(); err != nil {
		log.Logger().Error("failed to close blob for click-through rate model",
			zap.Int64("id", clickThroughRateModelId), zap.Error(err))
		return err
	}
	<-done

	// update meta
	m.clickThroughRateModelMutex.RLock()
	m.clickThroughRateMeta.ID = clickThroughRateModelId
	m.clickThroughRateMeta.Type = clickThroughRateType
	m.clickThroughRateMeta.Params = clickThroughRateParams
	m.clickThroughRateMeta.Score = score
	m.clickThroughRateModelMutex.RUnlock()
	if err = m.metaStore.Put(meta.CLICK_THROUGH_RATE_MODEL, m.clickThroughRateMeta.ToJSON()); err != nil {
		log.Logger().Error("failed to write click-through rate model meta", zap.Error(err))
		return err
	} else {
		log.Logger().Info("write click-through rate model meta",
			zap.Int64("id", clickThroughRateModelId),
			zap.Float32("precision", score.Precision),
			zap.Float32("recall", score.Recall),
			zap.Float32("auc", score.AUC),
			zap.Any("params", clickThroughRateParams))
	}

	// update statistics
	if err = m.CacheClient.AddTimeSeriesPoints(context.Background(), []cache.TimeSeriesPoint{
		{Name: cache.CTRPrecision, Value: float64(score.Precision), Timestamp: time.Now()},
		{Name: cache.CTRRecall, Value: float64(score.Recall), Timestamp: time.Now()},
		{Name: cache.CTRAUC, Value: float64(score.AUC), Timestamp: time.Now()},
	}); err != nil {
		log.Logger().Error("failed to write time series points", zap.Error(err))
		return err
	}

	m.removeOutOfDateModels()
	return nil
}

func (m *Master) removeOutOfDateModels() {
	m.collaborativeFilteringModelMutex.RLock()
	m.clickThroughRateModelMutex.RLock()
	timestamp := min(m.collaborativeFilteringMeta.ID, m.clickThroughRateMeta.ID)
	m.clickThroughRateModelMutex.RUnlock()
	m.collaborativeFilteringModelMutex.RUnlock()

	files, err := m.blobStore.List()
	if err != nil {
		log.Logger().Error("failed to list models in blob store", zap.Error(err))
		return
	}
	for _, file := range files {
		id, err := strconv.ParseInt(file, 10, 64)
		if err != nil {
			log.Logger().Info("failed to parse model id", zap.String("file", file), zap.Error(err))
			continue
		}
		if id < timestamp {
			if err = m.blobStore.Remove(file); err != nil {
				log.Logger().Error("failed to delete model from blob store", zap.Int64("id", id), zap.Error(err))
			} else {
				log.Logger().Info("deleted out-of-date model from blob store", zap.Int64("id", id))
			}
		}
	}
}

func (m *Master) collectGarbage(ctx context.Context, dataSet *dataset.Dataset) error {
	_, span := m.tracer.Start(context.Background(), "Collect Garbage in Cache", 1)
	defer span.End()
	err := m.CacheClient.ScanScores(ctx, func(collection, id, subset string, timestamp time.Time) error {
		switch collection {
		case cache.NonPersonalized:
			if subset != cache.Popular && subset != cache.Latest && !lo.ContainsBy(m.Config.Recommend.NonPersonalized, func(cfg config.NonPersonalizedConfig) bool {
				return cfg.Name == subset
			}) {
				return m.CacheClient.DeleteScores(ctx, []string{cache.NonPersonalized}, cache.ScoreCondition{
					Subset: lo.ToPtr(subset),
				})
			}
		case cache.UserToUser:
			splits := strings.Split(subset, "/")
			if len(splits) != 2 {
				log.Logger().Error("invalid subset", zap.String("subset", subset))
				return nil
			}
			if dataSet.GetUserDict().Id(splits[1]) == dataset.NotId || !lo.ContainsBy(m.Config.Recommend.UserToUser, func(cfg config.UserToUserConfig) bool {
				return cfg.Name == splits[0]
			}) {
				return m.CacheClient.DeleteScores(ctx, []string{cache.UserToUser}, cache.ScoreCondition{
					Subset: lo.ToPtr(subset),
					Before: lo.ToPtr(dataSet.GetTimestamp()),
				})
			}
		case cache.ItemToItem:
			splits := strings.Split(subset, "/")
			if len(splits) != 2 {
				log.Logger().Error("invalid subset", zap.String("subset", subset))
				return nil
			}
			if dataSet.GetItemDict().Id(splits[1]) == dataset.NotId || !lo.ContainsBy(m.Config.Recommend.ItemToItem, func(cfg config.ItemToItemConfig) bool {
				return cfg.Name == splits[0]
			}) {
				return m.CacheClient.DeleteScores(ctx, []string{cache.ItemToItem}, cache.ScoreCondition{
					Subset: lo.ToPtr(subset),
					Before: lo.ToPtr(dataSet.GetTimestamp()),
				})
			}
		case cache.CollaborativeFiltering:
			if dataSet.GetUserDict().Id(subset) == dataset.NotId {
				return m.CacheClient.DeleteScores(ctx, []string{cache.CollaborativeFiltering}, cache.ScoreCondition{
					Subset: lo.ToPtr(subset),
					Before: lo.ToPtr(dataSet.GetTimestamp()),
				})
			}
		}
		return nil
	})
	return errors.Trace(err)
}

func (m *Master) optimizeCollaborativeFiltering(trainSet, testSet dataset.CFSplit) error {
	ctx, span := m.tracer.Start(context.Background(), "Optimize Collaborative Filtering Model", m.Config.Recommend.Collaborative.ModelSearchTrials)
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
	}

	search := cf.NewModelSearch(map[string]cf.ModelCreator{
		"BPR": func() cf.MatrixFactorization {
			return cf.NewBPR(nil)
		},
		"ALS": func() cf.MatrixFactorization {
			return cf.NewALS(nil)
		},
	}, trainSet, testSet,
		cf.NewFitConfig().
			SetJobs(m.Config.Master.NumJobs).
			SetPatience(m.Config.Recommend.Collaborative.EarlyStopping.Patience)).
		WithContext(ctx).
		WithSpan(span)

	study, err := goptuna.CreateStudy("optimizeCollaborativeFiltering",
		goptuna.StudyOptionDirection(goptuna.StudyDirectionMaximize),
		goptuna.StudyOptionSampler(tpe.NewSampler()),
		goptuna.StudyOptionLogger(log.NewOptunaLogger(log.Logger())))
	if err != nil {
		return errors.Trace(err)
	}
	if err = study.Optimize(search.Objective, m.Config.Recommend.Collaborative.ModelSearchTrials); err != nil {
		return errors.Trace(err)
	}
	m.collaborativeFilteringModelMutex.Lock()
	m.collaborativeFilteringTarget = search.Result()
	m.collaborativeFilteringModelMutex.Unlock()
	log.Logger().Info("optimize collaborative filtering model completed",
		zap.Any("score", m.collaborativeFilteringTarget.Score),
		zap.String("type", m.collaborativeFilteringTarget.Type),
		zap.Any("params", m.collaborativeFilteringTarget.Params))
	return nil
}

func (m *Master) optimizeClickThroughRatePrediction(trainSet, testSet *ctr.Dataset) error {
	ctx, span := m.tracer.Start(context.Background(), "Optimize Click-Through Rate Prediction Model", m.Config.Recommend.Collaborative.ModelSearchTrials)
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
	}

	search := ctr.NewModelSearch(map[string]ctr.ModelCreator{
		"FM": func() ctr.FactorizationMachines {
			return ctr.NewFMV2(nil)
		},
	}, trainSet, testSet,
		ctr.NewFitConfig().
			SetJobs(m.Config.Master.NumJobs).
			SetPatience(m.Config.Recommend.Offline.EarlyStopping.Patience)).
		WithContext(ctx).
		WithSpan(span)

	study, err := goptuna.CreateStudy("optimizeClickThroughRatePrediction",
		goptuna.StudyOptionDirection(goptuna.StudyDirectionMaximize),
		goptuna.StudyOptionSampler(tpe.NewSampler()),
		goptuna.StudyOptionLogger(log.NewOptunaLogger(log.Logger())))
	if err != nil {
		return errors.Trace(err)
	}
	if err = study.Optimize(search.Objective, m.Config.Recommend.Collaborative.ModelSearchTrials); err != nil {
		return errors.Trace(err)
	}
	m.clickThroughRateModelMutex.Lock()
	m.clickThroughRateTarget = search.Result()
	m.clickThroughRateModelMutex.Unlock()
	log.Logger().Info("optimize click-through rate model completed",
		zap.Any("score", m.clickThroughRateTarget.Score),
		zap.String("type", m.clickThroughRateTarget.Type),
		zap.Any("params", m.clickThroughRateTarget.Params))
	return nil
}
