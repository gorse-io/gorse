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
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/common/expression"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/common/util"
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

type Pipeline struct {
	Config                   *config.Config
	CacheClient              cache.Database
	DataClient               data.Database
	Tracer                   *monitor.Monitor
	Jobs                     int
	MatrixFactorizationItems *logics.MatrixFactorizationItems
	MatrixFactorizationUsers *logics.MatrixFactorizationUsers
	ClickThroughRateModel    ctr.FactorizationMachines
	dontskipColdStartUsers   bool
}

func (p *Pipeline) Recommend(users []data.User, progress func(completed, throughput int)) {
	ctx := context.Background()
	startRecommendTime := time.Now()
	itemCache := NewItemCache(p.DataClient)
	log.Logger().Info("ranking recommendation",
		zap.Int("n_working_users", len(users)),
		zap.Int("n_jobs", p.Jobs),
		zap.Int("cache_size", p.Config.Recommend.CacheSize))

	// progress tracker
	completed := make(chan struct{}, 1000)
	_, span := p.Tracer.Start(context.Background(), "Generate recommendation", len(users))
	defer span.End()

	go func() {
		defer util.CheckPanic()
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
				span.Add(throughput)
				if progress != nil {
					progress(completedCount, completedCount-previousCount)
				}
				previousCount = completedCount
			}
		}
	}()

	// recommendation
	startTime := time.Now()
	var (
		updateUserCount               atomic.Float64
		collaborativeRecommendSeconds atomic.Float64
		userBasedRecommendSeconds     atomic.Float64
		itemBasedRecommendSeconds     atomic.Float64
		latestRecommendSeconds        atomic.Float64
		popularRecommendSeconds       atomic.Float64
	)

	defer MemoryInuseBytesVec.WithLabelValues("user_feedback_cache").Set(0)
	err := parallel.Parallel(len(users), p.Jobs, func(workerId, jobId int) error {
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
		if !p.dontskipColdStartUsers && recommender.IsColdStart() {
			// skip cold-start users without any positive feedback
			return nil
		}

		// Update collaborative filtering recommendation.
		if p.MatrixFactorizationUsers != nil && p.MatrixFactorizationItems != nil {
			if userEmbedding, ok := p.MatrixFactorizationUsers.Get(userId); ok {
				err = p.updateCollaborativeRecommend(p.MatrixFactorizationItems, userId, userEmbedding, recommender.ExcludeSet(), itemCache)
				if err != nil {
					log.Logger().Error("failed to recommend by collaborative filtering",
						zap.String("user_id", userId), zap.Error(err))
					return errors.Trace(err)
				}
			} else if !p.dontskipColdStartUsers {
				// skip users without collaborative filtering embeddings
				return nil
			}
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
		items, err := itemCache.GetMap(lo.Map(scores, func(score cache.Score, _ int) string {
			return score.Id
		}))
		if err != nil {
			return errors.Trace(err)
		}
		for _, score := range scores {
			if _, exist := items[score.Id]; exist {
				score.Timestamp = recommendTime
				candidates = append(candidates, score)
			}
		}

		// rank by click-through-rate
		var results []cache.Score
		if p.Config.Recommend.Ranker.Type == "fm" && p.ClickThroughRateModel != nil && !p.ClickThroughRateModel.Invalid() {
			results, err = p.rankByClickTroughRate(p.ClickThroughRateModel, &user, candidates, itemCache, recommendTime)
			if err != nil {
				log.Logger().Error("failed to rank items", zap.Error(err))
				return errors.Trace(err)
			}
		} else if p.Config.Recommend.Ranker.Type == "llm" && p.Config.Recommend.Ranker.Prompt != "" && p.Config.OpenAI.ChatCompletionModel != "" {
			ranker, err := logics.NewChatRanker(p.Config.OpenAI, p.Config.Recommend.Ranker.Prompt)
			if err != nil {
				log.Logger().Error("failed to create LLM ranker", zap.Error(err))
				return errors.Trace(err)
			}
			results, err = p.rankByLLM(ranker, &user, recommender.UserFeedback(), candidates, itemCache, recommendTime)
			if err != nil {
				log.Logger().Error("failed to rank items by LLM", zap.Error(err))
				return errors.Trace(err)
			}
		} else {
			results = candidates
		}

		if p.Config.Recommend.Replacement.EnableReplacement && p.Config.Recommend.Ranker.Type == "fm" &&
			p.ClickThroughRateModel != nil && !p.ClickThroughRateModel.Invalid() {
			results, err = p.replacement(p.ClickThroughRateModel, results, &user,
				recommender.UserFeedback(), itemCache, recommendTime)
			if err != nil {
				log.Logger().Error("failed to insert historical items into recommendation",
					zap.String("user_id", userId), zap.Error(err))
				return errors.Trace(err)
			}
		}

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
		return
	}
	log.Logger().Info("complete ranking recommendation",
		zap.String("used_time", time.Since(startTime).String()))
	UpdateUserRecommendTotal.Set(updateUserCount.Load())
	OfflineRecommendTotalSeconds.Set(time.Since(startRecommendTime).Seconds())
	OfflineRecommendStepSecondsVec.WithLabelValues("collaborative_recommend").Set(collaborativeRecommendSeconds.Load())
	OfflineRecommendStepSecondsVec.WithLabelValues("item_based_recommend").Set(itemBasedRecommendSeconds.Load())
	OfflineRecommendStepSecondsVec.WithLabelValues("user_based_recommend").Set(userBasedRecommendSeconds.Load())
	OfflineRecommendStepSecondsVec.WithLabelValues("latest_recommend").Set(latestRecommendSeconds.Load())
	OfflineRecommendStepSecondsVec.WithLabelValues("popular_recommend").Set(popularRecommendSeconds.Load())
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
	if digest != p.Config.Recommend.Hash() {
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
		timeoutTime := recommendTime.Add(p.Config.Recommend.Ranker.CacheExpire)
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
	// update categories
	itemsMap, err := itemCache.GetMap(lo.Map(scores, func(score cache.Score, _ int) string {
		return score.Id
	}))
	if err != nil {
		return errors.Trace(err)
	}
	// remove excluded items and non-existing items
	recommend := make([]cache.Score, 0, len(scores))
	for i := range scores {
		if item, exist := itemsMap[scores[i].Id]; exist && !excludeSet.Contains(item.ItemId) {
			recommend = append(recommend, cache.Score{
				Id:         scores[i].Id,
				Score:      scores[i].Score,
				Categories: item.Categories,
				// the scores use the timestamp of the ranking index, which is only refreshed every so often.
				// if we don't overwrite the timestamp here, the code below will delete all scores that were
				// just written.
				Timestamp: localStartTime,
			})
		}
	}
	if err := p.CacheClient.AddScores(ctx, cache.CollaborativeFiltering, userId, recommend); err != nil {
		log.Logger().Error("failed to cache collaborative filtering recommendation result", zap.String("user_id", userId), zap.Error(err))
		return errors.Trace(err)
	}
	if err := p.CacheClient.Set(ctx,
		cache.Time(cache.Key(cache.CollaborativeFilteringUpdateTime, userId), localStartTime),
		cache.String(cache.Key(cache.CollaborativeFilteringDigest, userId), p.Config.Recommend.Collaborative.Hash(&p.Config.Recommend)),
	); err != nil {
		log.Logger().Error("failed to cache collaborative filtering recommendation time", zap.String("user_id", userId), zap.Error(err))
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
	items, err := itemCache.GetSlice(lo.Map(candidates, func(score cache.Score, _ int) string {
		return score.Id
	}))
	if err != nil {
		return nil, errors.Trace(err)
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
		output := batchPredictor.BatchPredict(inputs, p.Jobs)
		for i, score := range output {
			topItems = append(topItems, cache.Score{
				Id:         items[i].ItemId,
				Score:      float64(score),
				Categories: items[i].Categories,
				Timestamp:  recommendTime,
			})
		}
	} else {
		for _, item := range items {
			topItems = append(topItems, cache.Score{
				Id:         item.ItemId,
				Score:      float64(predictor.Predict(user.UserId, item.ItemId, ctr.ConvertLabels(user.Labels), ctr.ConvertLabels(item.Labels))),
				Categories: item.Categories,
				Timestamp:  recommendTime,
			})
		}
	}
	cache.SortDocuments(topItems)
	return topItems, nil
}

func (p *Pipeline) rankByLLM(
	ranker *logics.ChatRanker,
	user *data.User,
	feedback []data.Feedback,
	candidates []cache.Score,
	itemCache *ItemCache,
	recommendTime time.Time,
) ([]cache.Score, error) {
	// download items
	items, err := itemCache.GetSlice(lo.Map(candidates, func(score cache.Score, _ int) string {
		return score.Id
	}))
	if err != nil {
		return nil, errors.Trace(err)
	}
	// convert feedback
	itemMap, err := itemCache.GetMap(lo.Map(feedback, func(fb data.Feedback, _ int) string {
		return fb.ItemId
	}))
	if err != nil {
		return nil, errors.Trace(err)
	}
	feedbackItems := make([]*logics.FeedbackItem, 0, len(feedback))
	for _, fb := range feedback {
		if item, exist := itemMap[fb.ItemId]; exist {
			feedbackItems = append(feedbackItems, &logics.FeedbackItem{
				FeedbackType: fb.FeedbackType,
				Item:         *item,
			})
		}
	}
	// rank by LLM
	parsed, err := ranker.Rank(user, feedbackItems, items)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// construct scores
	var topItems []cache.Score
	for rank, itemId := range parsed {
		topItems = append(topItems, cache.Score{
			Id:        itemId,
			Score:     float64(len(parsed)-rank) / float64(len(parsed)), // normalize score to [0, 1]
			Timestamp: recommendTime,
		})
	}
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

	items, err := itemCache.GetSlice(distinctItems.ToSlice())
	if err != nil {
		return nil, errors.Trace(err)
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
		output := batchPredictor.BatchPredict(inputs, p.Jobs)
		for i, score := range output {
			scoredItems = append(scoredItems, cache.Score{
				Id:         items[i].ItemId,
				Score:      float64(score),
				Categories: items[i].Categories,
				Timestamp:  recommendTime,
			})
		}
	} else {
		for _, item := range items {
			scoredItems = append(scoredItems, cache.Score{
				Id:         item.ItemId,
				Score:      float64(predictor.Predict(user.UserId, item.ItemId, ctr.ConvertLabels(user.Labels), ctr.ConvertLabels(item.Labels))),
				Categories: item.Categories,
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
	Client data.Database
	Data   sync.Map
}

// NewItemCache creates a new ItemCache.
func NewItemCache(client data.Database) *ItemCache {
	return &ItemCache{
		Client: client,
		Data:   sync.Map{},
	}
}

func (c *ItemCache) GetSlice(itemIds []string) ([]*data.Item, error) {
	requests := make([]string, 0, len(itemIds))
	for _, itemId := range itemIds {
		if _, exist := c.Data.Load(itemId); !exist {
			requests = append(requests, itemId)
		}
	}
	response, err := c.Client.BatchGetItems(context.Background(), requests)
	if err != nil {
		return nil, errors.Trace(err)
	}
	for _, item := range response {
		c.Data.Store(item.ItemId, &item)
	}
	items := make([]*data.Item, 0, len(itemIds))
	for _, itemId := range itemIds {
		if val, exist := c.Data.Load(itemId); exist {
			item := val.(*data.Item)
			if !item.IsHidden {
				items = append(items, item)
			}
		}
	}
	return items, nil
}

func (c *ItemCache) GetMap(itemIds []string) (map[string]*data.Item, error) {
	items, err := c.GetSlice(itemIds)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return lo.SliceToMap(items, func(item *data.Item) (string, *data.Item) {
		return item.ItemId, item
	}), nil
}
