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
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/common/sizeof"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/logics"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/juju/errors"
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
}

// RecommendForUsers generates recommendations for a list of users.
func (r *Pipeline) RecommendForUsers(ctx context.Context, users []data.User, itemCache *ItemCache, jobs int, tracer Tracer) error {
	log.Logger().Info("ranking recommendation",
		zap.Int("n_users", len(users)),
		zap.Int("n_jobs", jobs),
		zap.Int("cache_size", r.Config.Recommend.CacheSize))

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
		if !r.checkUserActiveTime(ctx, userId) || !r.checkRecommendCacheOutOfDate(ctx, userId) {
			return nil
		}
		updateUserCount.Add(1)

		recommendTime := time.Now()
		recommender, err := logics.NewRecommender(r.Config.Recommend, r.CacheClient, r.DataClient, false, userId, nil)
		if err != nil {
			return errors.Trace(err)
		}

		// Generate recommendation from recommenders.
		var (
			scores           []cache.Score
			digest           string
			recommenderNames []string
		)
		if len(r.Config.Recommend.Ranker.Recommenders) > 0 {
			recommenderNames = r.Config.Recommend.Ranker.Recommenders
		} else {
			recommenderNames = r.Config.Recommend.ListRecommenders()
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
		if err = r.CacheClient.AddScores(ctx, cache.Recommend, userId, results); err != nil {
			log.Logger().Error("failed to cache recommendation", zap.Error(err))
			return errors.Trace(err)
		}
		if err = r.CacheClient.Set(ctx,
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
func (r *Pipeline) checkUserActiveTime(ctx context.Context, userId string) bool {
	if r.Config.Recommend.ActiveUserTTL == 0 {
		return true
	}
	// read active time
	activeTime, err := r.CacheClient.Get(ctx, cache.Key(cache.LastModifyUserTime, userId)).Time()
	if err != nil {
		log.Logger().Error("failed to read last modify user time", zap.String("user_id", userId), zap.Error(err))
		return true
	}
	if activeTime.IsZero() {
		return true
	}
	// check active time
	if time.Since(activeTime) < time.Duration(r.Config.Recommend.ActiveUserTTL*24)*time.Hour {
		return true
	}
	// remove recommend cache for inactive users
	if err := r.CacheClient.DeleteScores(ctx, []string{cache.Recommend},
		cache.ScoreCondition{Subset: proto.String(userId)}); err != nil {
		log.Logger().Error("failed to delete recommend cache", zap.String("user_id", userId), zap.Error(err))
	}
	return false
}

// checkRecommendCacheOutOfDate checks if recommend cache stale.
func (r *Pipeline) checkRecommendCacheOutOfDate(ctx context.Context, userId string) bool {
	var (
		activeTime    time.Time
		recommendTime time.Time
		err           error
	)

	// 1. If cache is empty, stale.
	items, err := r.CacheClient.SearchScores(ctx, cache.Recommend, userId, nil, 0, -1)
	if err != nil {
		log.Logger().Error("failed to load offline recommendation", zap.String("user_id", userId), zap.Error(err))
		return true
	} else if len(items) == 0 {
		return true
	}

	// 2. If digest is empty or not match, stale.
	digest, err := r.CacheClient.Get(ctx, cache.Key(cache.RecommendDigest, userId)).String()
	if err != nil {
		log.Logger().Error("failed to read offline recommendation digest", zap.String("user_id", userId), zap.Error(err))
		return true
	}
	if digest == "" {
		return true
	}
	// read active time
	activeTime, err = r.CacheClient.Get(ctx, cache.Key(cache.LastModifyUserTime, userId)).Time()
	if err != nil {
		log.Logger().Error("failed to read last modify user time", zap.String("user_id", userId), zap.Error(err))
	}

	// 3. If update time is empty, stale.
	recommendTime, err = r.CacheClient.Get(ctx, cache.Key(cache.RecommendUpdateTime, userId)).Time()
	if err != nil {
		log.Logger().Error("failed to read last update user recommend time", zap.Error(err))
		return true
	}

	// 4. If update time + cache expire > current time, not stale.
	if recommendTime.Before(time.Now().Add(-r.Config.Recommend.CacheExpire)) {
		return true
	}

	// 5. If active time > recommend time, not stale.
	if activeTime.Before(recommendTime) {
		timeoutTime := recommendTime.Add(r.Config.Recommend.Ranker.RefreshRecommendPeriod)
		return timeoutTime.Before(time.Now())
	}
	return true
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
func (r *Pipeline) PullItems(ctx context.Context) (*ItemCache, []string, error) {
	// pull items from database
	itemCache := NewItemCache()
	itemCategories := mapset.NewSet[string]()
	itemChan, errChan := r.DataClient.GetItemStream(ctx, batchSize, nil)
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
