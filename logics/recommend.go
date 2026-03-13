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

package logics

import (
	"context"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/common/expression"
	"github.com/gorse-io/gorse/common/heap"
	"github.com/gorse-io/gorse/common/util"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/juju/errors"
	"github.com/samber/lo"
)

const (
	LatestRecommender          = "latest"
	NonPersonalizedRecommender = "non-personalized/"
	ItemToItemRecommender      = "item-to-item/"
	UserToUserRecommender      = "user-to-user/"
	ExternalRecommender        = "external/"
	CollaborativeRecommender   = "collaborative"
)

type Recommender struct {
	config      config.RecommendConfig
	cacheClient cache.Database
	dataClient  data.Database

	online       bool
	coldstart    bool
	userId       string
	userFeedback []data.Feedback
	categories   []string
	excludeSet   mapset.Set[string]
}

type RecommenderFunc func(ctx context.Context) ([]cache.Score, string, error)

func NewRecommender(config config.RecommendConfig, cacheClient cache.Database, dataClient data.Database, online bool, userId string, categories []string) (*Recommender, error) {
	// Load user feedback
	userFeedback, err := dataClient.GetUserFeedback(context.Background(), userId, lo.ToPtr(time.Now()))
	if err != nil {
		return nil, errors.Trace(err)
	}
	excludeSet := mapset.NewSet[string]()
	coldstart := true
	for _, feedback := range userFeedback {
		if !config.Replacement.EnableReplacement || !online {
			excludeSet.Add(feedback.ItemId)
		}
		if expression.MatchFeedbackTypeExpressions(config.DataSource.PositiveFeedbackTypes, feedback.FeedbackType, feedback.Value) {
			coldstart = false
		}
	}
	return &Recommender{
		config:       config,
		cacheClient:  cacheClient,
		dataClient:   dataClient,
		userId:       userId,
		userFeedback: userFeedback,
		online:       online,
		coldstart:    coldstart,
		categories:   categories,
		excludeSet:   excludeSet,
	}, nil
}

func (r *Recommender) ExcludeSet() mapset.Set[string] {
	return r.excludeSet
}

func (r *Recommender) UserFeedback() []data.Feedback {
	return r.userFeedback
}

func (r *Recommender) IsColdStart() bool {
	return r.coldstart
}

func (r *Recommender) Recommend(ctx context.Context, limit int) (result []cache.Score, err error) {
	if !strings.EqualFold(r.config.Ranker.Type, "none") {
		scores, err := r.searchAvailableScores(ctx, cache.Recommend, r.userId, r.categories)
		if err != nil {
			return nil, errors.Trace(err)
		}
		result = make([]cache.Score, 0, len(scores))
		for _, score := range scores {
			if !r.excludeSet.Contains(score.Id) {
				r.excludeSet.Add(score.Id)
				result = append(result, score)
			}
		}
	} else {
		result, _, err = r.RecommendSequential(ctx, result, r.config.CacheSize, r.config.Ranker.Recommenders...)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	if len(result) >= limit && limit > 0 {
		return result[:limit], nil
	}
	result, _, err = r.RecommendSequential(ctx, result, limit, r.config.Fallback.Recommenders...)
	return result, errors.Trace(err)
}

// RecommendSequential recommend items from multiple recommenders sequentially util reaching the limit.
// If limit <= 0, all recommendations are returned.
func (r *Recommender) RecommendSequential(ctx context.Context, result []cache.Score, limit int, names ...string) ([]cache.Score, string, error) {
	var digests []string
	for _, name := range names {
		recommenderFunc, err := r.parse(name)
		if err != nil {
			return nil, "", errors.Trace(err)
		}
		scores, digest, err := recommenderFunc(ctx)
		if err != nil {
			return nil, "", errors.Trace(err)
		}
		for _, score := range scores {
			r.excludeSet.Add(score.Id)
		}
		result = append(result, scores...)
		digests = append(digests, digest)
		if limit > 0 && len(result) >= limit {
			return result[:limit], util.MD5(digests...), nil
		}
	}
	return result, util.MD5(digests...), nil
}

func (r *Recommender) expireBefore() time.Time {
	if r.config.DataSource.ItemTTL == 0 {
		return time.Time{}
	}
	return time.Now().AddDate(0, 0, -int(r.config.DataSource.ItemTTL))
}

func availableItem(item data.Item, expireBefore time.Time) bool {
	if item.IsHidden {
		return false
	}
	return expireBefore.IsZero() || item.Timestamp.IsZero() || !item.Timestamp.Before(expireBefore)
}

func (r *Recommender) filterAvailableItems(items []data.Item) []data.Item {
	expireBefore := r.expireBefore()
	return lo.Filter(items, func(item data.Item, _ int) bool {
		return availableItem(item, expireBefore)
	})
}

func (r *Recommender) batchGetItemsMap(ctx context.Context, ids []string) (map[string]data.Item, error) {
	if len(ids) == 0 {
		return map[string]data.Item{}, nil
	}
	items, err := r.dataClient.BatchGetItems(ctx, ids)
	if err != nil {
		return nil, errors.Trace(err)
	}
	itemsMap := make(map[string]data.Item, len(items))
	for _, item := range items {
		itemsMap[item.ItemId] = item
	}
	return itemsMap, nil
}

func (r *Recommender) filterAvailableScores(ctx context.Context, scores []cache.Score) ([]cache.Score, error) {
	itemsMap, err := r.batchGetItemsMap(ctx, lo.Map(scores, func(score cache.Score, _ int) string {
		return score.Id
	}))
	if err != nil {
		return nil, errors.Trace(err)
	}
	expireBefore := r.expireBefore()
	return lo.Filter(scores, func(score cache.Score, _ int) bool {
		item, ok := itemsMap[score.Id]
		return !ok || availableItem(item, expireBefore)
	}), nil
}

func (r *Recommender) searchAvailableScores(ctx context.Context, collection, subset string, categories []string) ([]cache.Score, error) {
	if r.config.CacheSize <= 0 {
		scores, err := r.cacheClient.SearchScores(ctx, collection, subset, categories, 0, -1)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return r.filterAvailableScores(ctx, scores)
	}
	results := make([]cache.Score, 0, r.config.CacheSize)
	for begin := 0; ; begin += r.config.CacheSize {
		scores, err := r.cacheClient.SearchScores(ctx, collection, subset, categories, begin, begin+r.config.CacheSize)
		if err != nil {
			return nil, errors.Trace(err)
		}
		pageSize := len(scores)
		if pageSize == 0 {
			return results, nil
		}
		scores, err = r.filterAvailableScores(ctx, scores)
		if err != nil {
			return nil, errors.Trace(err)
		}
		results = append(results, scores...)
		if len(results) >= r.config.CacheSize {
			return results[:r.config.CacheSize], nil
		}
		if pageSize < r.config.CacheSize {
			return results, nil
		}
	}
}

func (r *Recommender) getLatestAvailableItems(ctx context.Context) ([]data.Item, error) {
	if r.config.CacheSize <= 0 {
		items, err := r.dataClient.GetLatestItems(ctx, 0, r.categories)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return r.filterAvailableItems(items), nil
	}
	for n := r.config.CacheSize; ; n += r.config.CacheSize {
		items, err := r.dataClient.GetLatestItems(ctx, n, r.categories)
		if err != nil {
			return nil, errors.Trace(err)
		}
		m := len(items)
		items = r.filterAvailableItems(items)
		if len(items) >= r.config.CacheSize {
			return items[:r.config.CacheSize], nil
		}
		if m < n {
			return items, nil
		}
	}
}

func (r *Recommender) parse(fullname string) (RecommenderFunc, error) {
	if fullname == CollaborativeRecommender {
		return r.recommendCollaborative, nil
	} else if fullname == LatestRecommender {
		return r.recommendLatest, nil
	} else if strings.HasPrefix(fullname, NonPersonalizedRecommender) {
		name := strings.TrimPrefix(fullname, NonPersonalizedRecommender)
		return r.recommendNonPersonalized(name), nil
	} else if strings.HasPrefix(fullname, ItemToItemRecommender) {
		name := strings.TrimPrefix(fullname, ItemToItemRecommender)
		return r.recommendItemToItem(name), nil
	} else if strings.HasPrefix(fullname, UserToUserRecommender) {
		name := strings.TrimPrefix(fullname, UserToUserRecommender)
		return r.recommendUserToUser(name), nil
	} else if strings.HasPrefix(fullname, ExternalRecommender) {
		name := strings.TrimPrefix(fullname, ExternalRecommender)
		return r.recommendExternal(name), nil
	} else {
		return nil, errors.Errorf("unknown recommender: %s", fullname)
	}
}

func (r *Recommender) recommendLatest(ctx context.Context) ([]cache.Score, string, error) {
	items, err := r.getLatestAvailableItems(ctx)
	if err != nil {
		return nil, "", errors.Trace(err)
	}
	scores := make([]cache.Score, 0, len(items))
	for _, item := range items {
		if !r.excludeSet.Contains(item.ItemId) {
			scores = append(scores, cache.Score{
				Id:         item.ItemId,
				Score:      float64(item.Timestamp.Unix()),
				Categories: item.Categories,
			})
		}
	}
	return scores, "latest", nil
}

func (r *Recommender) recommendNonPersonalized(name string) RecommenderFunc {
	return func(ctx context.Context) ([]cache.Score, string, error) {
		var categories []string
		if len(r.categories) == 0 {
			categories = []string{""}
		} else {
			categories = r.categories
		}
		items, err := r.searchAvailableScores(ctx, cache.NonPersonalized, name, categories)
		if err != nil {
			return nil, "", errors.Trace(err)
		}
		// read digest
		digest, err := r.cacheClient.Get(ctx, cache.Key(cache.NonPersonalizedDigest, name)).String()
		if err != nil {
			return nil, "", errors.Trace(err)
		}
		// remove excluded items
		return lo.Filter(items, func(item cache.Score, _ int) bool {
			return !r.excludeSet.Contains(item.Id)
		}), digest, nil
	}
}

func (r *Recommender) recommendCollaborative(ctx context.Context) ([]cache.Score, string, error) {
	items, err := r.searchAvailableScores(ctx, cache.CollaborativeFiltering, r.userId, r.categories)
	if err != nil {
		return nil, "", errors.Trace(err)
	}
	// read digest
	digest, err := r.cacheClient.Get(ctx, cache.Key(cache.CollaborativeFilteringDigest, r.userId)).String()
	if err != nil {
		return nil, "", errors.Trace(err)
	}
	// remove excluded items
	return lo.Filter(items, func(item cache.Score, _ int) bool {
		return !r.excludeSet.Contains(item.Id)
	}), digest, nil
}

func (r *Recommender) recommendItemToItem(name string) RecommenderFunc {
	return func(ctx context.Context) ([]cache.Score, string, error) {
		// filter positive feedbacks
		data.SortFeedbacks(r.userFeedback)
		userFeedback := make([]data.Feedback, 0, r.config.CacheSize)
		for _, feedback := range r.userFeedback {
			if r.online && r.config.ContextSize <= len(userFeedback) {
				break
			}
			if expression.MatchFeedbackTypeExpressions(r.config.DataSource.PositiveFeedbackTypes, feedback.FeedbackType, feedback.Value) {
				userFeedback = append(userFeedback, feedback)
			}
		}
		// collect scores
		scores := make(map[string]float64)
		categories := make(map[string][]string)
		digests := mapset.NewSet[string]()
		for _, feedback := range userFeedback {
			similarItems, err := r.searchAvailableScores(ctx, cache.ItemToItem, cache.Key(name, feedback.ItemId), r.categories)
			if err != nil {
				return nil, "", errors.Trace(err)
			}
			digest, err := r.cacheClient.Get(ctx, cache.Key(cache.ItemToItemDigest, name, feedback.ItemId)).String()
			if err != nil {
				return nil, "", errors.Trace(err)
			}
			for _, item := range similarItems {
				if !r.excludeSet.Contains(item.Id) {
					scores[item.Id] += item.Score
					categories[item.Id] = item.Categories
					digests.Add(digest)
				}
			}
		}
		// collect top scores
		filter := heap.NewTopKFilter[string, float64](r.config.CacheSize)
		for id, score := range scores {
			filter.Push(id, score)
		}
		elems := filter.PopAll()
		return lo.Map(elems, func(elem heap.Elem[string, float64], _ int) cache.Score {
			return cache.Score{
				Id:         elem.Value,
				Score:      elem.Weight,
				Categories: categories[elem.Value],
			}
		}), strings.Join(digests.ToSlice(), ""), nil
	}
}

func (r *Recommender) recommendUserToUser(name string) RecommenderFunc {
	return func(ctx context.Context) ([]cache.Score, string, error) {
		scores := make(map[string]float64)
		// load similar users
		similarUsers, err := r.cacheClient.SearchScores(ctx, cache.UserToUser, cache.Key(name, r.userId), nil, 0, r.config.CacheSize)
		if err != nil {
			return nil, "", errors.Trace(err)
		}
		// read digest
		digest, err := r.cacheClient.Get(ctx, cache.Key(cache.UserToUserDigest, name, r.userId)).String()
		if err != nil {
			return nil, "", errors.Trace(err)
		}
		// aggregate scores
		for _, user := range similarUsers {
			// load historical feedback
			feedbacks, err := r.dataClient.GetUserFeedback(ctx, user.Id, lo.ToPtr(time.Now()), r.config.DataSource.PositiveFeedbackTypes...)
			if err != nil {
				return nil, "", errors.Trace(err)
			}
			// add unseen items
			for _, feedback := range feedbacks {
				if !r.excludeSet.Contains(feedback.ItemId) {
					scores[feedback.ItemId] += user.Score
				}
			}
		}
		// collect top k
		filter := heap.NewTopKFilter[string, float64](r.config.CacheSize)
		for id, score := range scores {
			filter.Push(id, score)
		}
		elems := filter.PopAll()
		// filter by categories
		results := make([]cache.Score, 0, len(elems))
		ids := lo.Map(elems, func(elem heap.Elem[string, float64], _ int) string {
			return elem.Value
		})
		itemsMap, err := r.batchGetItemsMap(ctx, ids)
		if err != nil {
			return nil, "", errors.Trace(err)
		}
		expireBefore := r.expireBefore()
		for _, elem := range elems {
			if item, ok := itemsMap[elem.Value]; ok &&
				lo.Every(item.Categories, r.categories) &&
				availableItem(item, expireBefore) {
				results = append(results, cache.Score{
					Id:         item.ItemId,
					Score:      elem.Weight,
					Categories: item.Categories,
				})
			}
		}
		return results, digest, nil
	}
}

func (r *Recommender) recommendExternal(name string) RecommenderFunc {
	return func(ctx context.Context) ([]cache.Score, string, error) {
		var externalConfig config.ExternalConfig
		for _, extConfig := range r.config.External {
			if extConfig.Name == name {
				externalConfig = extConfig
				break
			}
		}

		if len(r.categories) > 0 {
			// external recommenders do not support categories
			return nil, externalConfig.Hash(), nil
		}

		external, err := NewExternal(externalConfig)
		if err != nil {
			return nil, "", errors.Trace(err)
		}
		defer external.Close()
		items, err := external.Pull(r.userId)
		if err != nil {
			return nil, "", errors.Trace(err)
		}
		scores := make([]cache.Score, 0, len(items))
		for _, itemId := range items {
			if !r.excludeSet.Contains(itemId) {
				scores = append(scores, cache.Score{
					Id: itemId,
				})
			}
		}
		scores, err = r.filterAvailableScores(ctx, scores)
		if err != nil {
			return nil, "", errors.Trace(err)
		}
		return scores, externalConfig.Hash(), nil
	}
}
