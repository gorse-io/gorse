// Copyright 2025 gorse Project Authors
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

package worker

import (
	"context"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/common/expression"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/common/sizeof"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/logics"
	"github.com/gorse-io/gorse/model/ctr"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// Tracer is an alias for monitor.Monitor to avoid circular dependencies.
type Tracer interface {
	Start(context.Context, string, int) (context.Context, *monitor.Span)
}

type Pipeline struct {
	Config      *config.Config
	CacheClient cache.Database
	DataClient  data.Database
	jobs        int
}

// RecommendForUsers generates recommendations for a list of users.
func (p *Pipeline) RecommendForUsers(ctx context.Context, users []data.User, itemCache *ItemCache, jobs int, tracer Tracer) error {
	log.Logger().Info("ranking recommendation",
		zap.Int("n_users", len(users)),
		zap.Int("n_jobs", jobs),
		zap.Int("cache_size", p.Config.Recommend.CacheSize))

	MemoryInuseBytesVec.WithLabelValues("item_cache").Set(float64(sizeof.DeepSize(itemCache)))
	defer MemoryInuseBytesVec.WithLabelValues("item_cache").Set(0)

	// progress tracker
	completed := make(chan struct{}, 1000)
	_, span := tracer.Start(context.Background(), "Generate Offline Recommend", len(users))
	defer span.End()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Logger().Error("panic in progress tracker", zap.Any("error", err))
			}
		}()
		completedCount, previousCount := 0, 0
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
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
					span.Add(throughput)
					log.Logger().Info("ranking recommendation",
						zap.Int("n_complete_users", completedCount),
						zap.Int("n_users", len(users)),
						zap.Int("throughput", throughput))
				}
			}
		}
	}()

	// recommendation
	startTime := time.Now()
	var updateUserCount atomic.Float64

	defer MemoryInuseBytesVec.WithLabelValues("user_feedback_cache").Set(0)
	err := parallel.Parallel(len(users), jobs, func(workerId, jobId int) error {
		defer func() {
			completed <- struct{}{}
		}()
		user := users[jobId]
		userId := user.UserId
		// skip inactive users before max recommend period
		if !p.checkUserActiveTime(ctx, userId) || !p.checkRecommendCacheOutOfDate(ctx, userId) {
			return nil
		}
		updateUserCount.Add(1)

		recommendTime := time.Now()
		recommender, err := logics.NewRecommender(p.Config.Recommend, p.CacheClient, p.DataClient, false, userId, nil)
		if err != nil {
			return errors.Trace(err)
		}

		// Generate recommendation from recommenders.
		var (
			scores           []cache.Score
			digest           string
			recommenderNames []string
		)
		if len(p.Config.Recommend.Ranker.Recommenders) > 0 {
			recommenderNames = p.Config.Recommend.Ranker.Recommenders
		} else {
			recommenderNames = p.Config.Recommend.ListRecommenders()
		}
		scores, digest, err = recommender.RecommendSequential(context.Background(), scores, 0, recommenderNames...)
		if err != nil {
			return errors.Trace(err)
		}

		candidates := make([]cache.Score, 0, len(scores))
		for _, score := range scores {
			if itemCache.IsAvailable(score.Id) {
				score.Timestamp = recommendTime
				candidates = append(candidates, score)
			}
		}

		// rank by click-through-rate (simplified version without model)
		results := candidates

		// cache recommendation
		if err = p.CacheClient.AddScores(ctx, cache.Recommend, userId, results); err != nil {
			log.Logger().Error("failed to cache recommendation", zap.Error(err))
			return errors.Trace(err)
		}
		if err = p.CacheClient.Set(ctx,
			cache.Time(cache.Key(cache.RecommendUpdateTime, userId), recommendTime),
			cache.String(cache.Key(cache.RecommendDigest, userId), digest),
		); err != nil {
			log.Logger().Error("failed to cache recommendation time", zap.Error(err))
		}
		return nil
	})
	close(completed)
	if err != nil {
		log.Logger().Error("failed to continue offline recommendation", zap.Error(err))
		return errors.Trace(err)
	}
	log.Logger().Info("complete ranking recommendation",
		zap.String("used_time", time.Since(startTime).String()))
	UpdateUserRecommendTotal.Set(updateUserCount.Load())
	return nil
}

// checkUserActiveTime checks if a user is active based on their last modification time.
func (p *Pipeline) checkUserActiveTime(ctx context.Context, userId string) bool {
	if p.Config.Recommend.ActiveUserTTL == 0 {
		return true
	}
	// read active time
	activeTime, err := p.CacheClient.Get(ctx, cache.Key(cache.LastModifyUserTime, userId)).Time()
	if err != nil {
		log.Logger().Error("failed to read last modify user time", zap.String("user_id", userId), zap.Error(err))
		return true
	}
	if activeTime.IsZero() {
		return true
	}
	// check active time
	if time.Since(activeTime) < time.Duration(p.Config.Recommend.ActiveUserTTL*24)*time.Hour {
		return true
	}
	// remove recommend cache for inactive users
	if err := p.CacheClient.DeleteScores(ctx, []string{cache.Recommend},
		cache.ScoreCondition{Subset: proto.String(userId)}); err != nil {
		log.Logger().Error("failed to delete recommend cache", zap.String("user_id", userId), zap.Error(err))
	}
	return false
}

// checkRecommendCacheOutOfDate checks if recommend cache stale.
func (p *Pipeline) checkRecommendCacheOutOfDate(ctx context.Context, userId string) bool {
	var (
		activeTime    time.Time
		recommendTime time.Time
		err           error
	)

	// 1. If cache is empty, stale.
	items, err := p.CacheClient.SearchScores(ctx, cache.Recommend, userId, nil, 0, -1)
	if err != nil {
		log.Logger().Error("failed to load offline recommendation", zap.String("user_id", userId), zap.Error(err))
		return true
	} else if len(items) == 0 {
		return true
	}

	// 2. If digest is empty or not match, stale.
	digest, err := p.CacheClient.Get(ctx, cache.Key(cache.RecommendDigest, userId)).String()
	if err != nil {
		log.Logger().Error("failed to read offline recommendation digest", zap.String("user_id", userId), zap.Error(err))
		return true
	}
	if digest == "" {
		return true
	}
	// read active time
	activeTime, err = p.CacheClient.Get(ctx, cache.Key(cache.LastModifyUserTime, userId)).Time()
	if err != nil {
		log.Logger().Error("failed to read last modify user time", zap.String("user_id", userId), zap.Error(err))
	}

	// 3. If update time is empty, stale.
	recommendTime, err = p.CacheClient.Get(ctx, cache.Key(cache.RecommendUpdateTime, userId)).Time()
	if err != nil {
		log.Logger().Error("failed to read last update user recommend time", zap.Error(err))
		return true
	}

	// 4. If update time + cache expire > current time, not stale.
	if recommendTime.Before(time.Now().Add(-p.Config.Recommend.CacheExpire)) {
		return true
	}

	// 5. If active time > recommend time, not stale.
	if activeTime.Before(recommendTime) {
		timeoutTime := recommendTime.Add(p.Config.Recommend.Ranker.RefreshRecommendPeriod)
		return timeoutTime.Before(time.Now())
	}
	return true
}

func (p *Pipeline) updateCollaborativeRecommend(
	items *logics.MatrixFactorizationItems,
	userId string,
	userEmbedding []float32,
	excludeSet mapset.Set[string],
	itemCache *ItemCache,
) error {
	ctx := context.Background()
	localStartTime := time.Now()
	scores := items.Search(userEmbedding, p.Config.Recommend.CacheSize+excludeSet.Cardinality())
	// remove excluded items
	scores = lo.Filter(scores, func(score cache.Score, _ int) bool {
		return !excludeSet.Contains(score.Id)
	})
	// update categories
	for i := range scores {
		scores[i].Categories = itemCache.GetCategory(scores[i].Id)
		// the scores use the timestamp of the ranking index, which is only refreshed every so often.
		// if we don't overwrite the timestamp here, the code below will delete all scores that were
		// just written.
		scores[i].Timestamp = localStartTime
	}
	if err := p.CacheClient.AddScores(ctx, cache.CollaborativeFiltering, userId, scores); err != nil {
		log.Logger().Error("failed to cache collaborative filtering recommendation result", zap.String("user_id", userId), zap.Error(err))
		return errors.Trace(err)
	}
	if err := p.CacheClient.DeleteScores(ctx, []string{cache.CollaborativeFiltering}, cache.ScoreCondition{Before: &localStartTime, Subset: proto.String(userId)}); err != nil {
		log.Logger().Error("failed to delete stale collaborative filtering recommendation result", zap.String("user_id", userId), zap.Error(err))
		return errors.Trace(err)
	}
	return nil
}

// rankByClickTroughRate ranks items by predicted click-through-rate.
func (p *Pipeline) rankByClickTroughRate(
	predictor ctr.FactorizationMachines,
	user *data.User,
	candidates []cache.Score,
	itemCache *ItemCache,
	recommendTime time.Time,
) ([]cache.Score, error) {
	// download items
	items := make([]*data.Item, 0, len(candidates))
	for _, candidate := range candidates {
		if item, exist := itemCache.Get(candidate.Id); exist {
			items = append(items, item)
		} else {
			log.Logger().Warn("item doesn't exists in database", zap.String("item_id", candidate.Id))
		}
	}
	// rank by CTR
	topItems := make([]cache.Score, 0, len(items))
	if batchPredictor, ok := predictor.(ctr.BatchInference); ok {
		inputs := make([]lo.Tuple4[string, string, []ctr.Label, []ctr.Label], len(items))
		for i, item := range items {
			inputs[i].A = user.UserId
			inputs[i].B = item.ItemId
			inputs[i].C = ctr.ConvertLabels(user.Labels)
			inputs[i].D = ctr.ConvertLabels(item.Labels)
		}
		output := batchPredictor.BatchPredict(inputs, p.jobs)
		for i, score := range output {
			topItems = append(topItems, cache.Score{
				Id:         items[i].ItemId,
				Score:      float64(score),
				Categories: itemCache.GetCategory(items[i].ItemId),
				Timestamp:  recommendTime,
			})
		}
	} else {
		for _, item := range items {
			topItems = append(topItems, cache.Score{
				Id:         item.ItemId,
				Score:      float64(predictor.Predict(user.UserId, item.ItemId, ctr.ConvertLabels(user.Labels), ctr.ConvertLabels(item.Labels))),
				Categories: itemCache.GetCategory(item.ItemId),
				Timestamp:  recommendTime,
			})
		}
	}
	cache.SortDocuments(topItems)
	return topItems, nil
}

// replacement inserts historical items back to recommendation.
func (p *Pipeline) replacement(
	predictor ctr.FactorizationMachines,
	recommend []cache.Score,
	user *data.User,
	feedbacks []data.Feedback,
	itemCache *ItemCache,
	recommendTime time.Time,
) ([]cache.Score, error) {
	recommendItems := mapset.NewSet[string]()
	positiveItems := mapset.NewSet[string]()
	distinctItems := mapset.NewSet[string]()
	for _, r := range recommend {
		recommendItems.Add(r.Id)
	}
	newRecommend := make([]cache.Score, 0, len(recommend))
	newRecommend = append(newRecommend, recommend...)
	for _, feedback := range feedbacks {
		if expression.MatchFeedbackTypeExpressions(p.Config.Recommend.DataSource.PositiveFeedbackTypes, feedback.FeedbackType, feedback.Value) {
			positiveItems.Add(feedback.ItemId)
			distinctItems.Add(feedback.ItemId)
		} else if expression.MatchFeedbackTypeExpressions(p.Config.Recommend.DataSource.ReadFeedbackTypes, feedback.FeedbackType, feedback.Value) {
			distinctItems.Add(feedback.ItemId)
		}
	}
	negativeItems := distinctItems.Difference(positiveItems)

	items := make([]*data.Item, 0, distinctItems.Cardinality())
	for itemId := range distinctItems.Iter() {
		if item, exist := itemCache.Get(itemId); exist {
			items = append(items, item)
		}
	}
	scoredItems := make([]cache.Score, 0, len(items))
	if batchPredictor, ok := predictor.(ctr.BatchInference); ok {
		inputs := make([]lo.Tuple4[string, string, []ctr.Label, []ctr.Label], len(items))
		for i, item := range items {
			inputs[i].A = user.UserId
			inputs[i].B = item.ItemId
			inputs[i].C = ctr.ConvertLabels(user.Labels)
			inputs[i].D = ctr.ConvertLabels(item.Labels)
		}
		output := batchPredictor.BatchPredict(inputs, p.jobs)
		for i, score := range output {
			scoredItems = append(scoredItems, cache.Score{
				Id:         items[i].ItemId,
				Score:      float64(score),
				Categories: itemCache.GetCategory(items[i].ItemId),
				Timestamp:  recommendTime,
			})
		}
	} else {
		for _, item := range items {
			scoredItems = append(scoredItems, cache.Score{
				Id:         item.ItemId,
				Score:      float64(predictor.Predict(user.UserId, item.ItemId, ctr.ConvertLabels(user.Labels), ctr.ConvertLabels(item.Labels))),
				Categories: itemCache.GetCategory(item.ItemId),
				Timestamp:  recommendTime,
			})
		}
	}

	for _, scoredItem := range scoredItems {
		if recommendItems.Contains(scoredItem.Id) {
			continue
		}
		if positiveItems.Contains(scoredItem.Id) {
			scoredItem.Score *= p.Config.Recommend.Replacement.PositiveReplacementDecay
		} else if negativeItems.Contains(scoredItem.Id) {
			scoredItem.Score *= p.Config.Recommend.Replacement.ReadReplacementDecay
		} else {
			continue
		}
		newRecommend = append(newRecommend, scoredItem)
	}

	// rank items
	cache.SortDocuments(newRecommend)
	return newRecommend, nil
}

// ItemCache is alias of map[string]data.Item.
type ItemCache struct {
	Data map[string]*data.Item
}

// NewItemCache creates a new ItemCache.
func NewItemCache() *ItemCache {
	return &ItemCache{Data: make(map[string]*data.Item)}
}

// Len returns the number of items in the cache.
func (c *ItemCache) Len() int {
	return len(c.Data)
}

// Set adds an item to the cache.
func (c *ItemCache) Set(itemId string, item data.Item) {
	if _, exist := c.Data[itemId]; !exist {
		c.Data[itemId] = &item
	}
}

// Get retrieves an item from the cache.
func (c *ItemCache) Get(itemId string) (*data.Item, bool) {
	item, exist := c.Data[itemId]
	return item, exist
}

// GetCategory gets the categories of an item.
func (c *ItemCache) GetCategory(itemId string) []string {
	if item, exist := c.Data[itemId]; exist {
		return item.Categories
	} else {
		return nil
	}
}

// IsAvailable means the item exists in database and is not hidden.
func (c *ItemCache) IsAvailable(itemId string) bool {
	if item, exist := c.Data[itemId]; exist {
		return !item.IsHidden
	} else {
		return false
	}
}

// PullItems pulls all items from the data store.
func (p *Pipeline) PullItems(ctx context.Context) (*ItemCache, []string, error) {
	// pull items from database
	itemCache := NewItemCache()
	itemCategories := mapset.NewSet[string]()
	itemChan, errChan := p.DataClient.GetItemStream(ctx, batchSize, nil)
	for batchItems := range itemChan {
		for _, item := range batchItems {
			itemCache.Set(item.ItemId, item)
			itemCategories.Append(item.Categories...)
		}
	}
	if err := <-errChan; err != nil {
		return nil, nil, errors.Trace(err)
	}
	return itemCache, itemCategories.ToSlice(), nil
}
