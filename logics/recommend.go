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
	CollaborativeRecommender   = "collaborative"
)

type Recommender struct {
	config      config.RecommendConfig
	cacheClient cache.Database
	dataClient  data.Database

	online       bool
	userId       string
	userFeedback []data.Feedback
	categories   []string
	excludeSet   mapset.Set[string]
}

type RecommenderFunc func(ctx context.Context) ([]cache.Score, error)

func NewRecommender(config config.RecommendConfig, cacheClient cache.Database, dataClient data.Database, online bool, userId string, categories []string) (*Recommender, error) {
	// Load user feedback
	userFeedback, err := dataClient.GetUserFeedback(context.Background(), userId, lo.ToPtr(time.Now()))
	if err != nil {
		return nil, errors.Trace(err)
	}
	excludeSet := mapset.NewSet[string]()
	if !config.Replacement.EnableReplacement {
		for _, feedback := range userFeedback {
			excludeSet.Add(feedback.ItemId)
		}
	}
	return &Recommender{
		config:       config,
		cacheClient:  cacheClient,
		dataClient:   dataClient,
		userId:       userId,
		userFeedback: userFeedback,
		online:       online,
		categories:   categories,
		excludeSet:   excludeSet,
	}, nil
}

func (r *Recommender) ExcludeSet() mapset.Set[string] {
	return r.excludeSet
}

func (r *Recommender) Recommend(ctx context.Context, limit int) ([]cache.Score, error) {
	scores, err := r.cacheClient.SearchScores(ctx, cache.OfflineRecommend, r.userId, r.categories, 0, r.config.CacheSize)
	if err != nil {
		return nil, errors.Trace(err)
	}
	result := make([]cache.Score, 0, len(scores))
	for _, score := range scores {
		if !r.excludeSet.Contains(score.Id) {
			r.excludeSet.Add(score.Id)
			result = append(result, score)
		}
	}
	if len(result) >= limit && limit > 0 {
		return result[:limit], nil
	}
	return r.RecommendSequential(ctx, result, limit, r.config.Fallback.Recommenders...)
}

// RecommendSequential recommend items from multiple recommenders sequentially util reaching the limit.
// If limit <= 0, all recommendations are returned.
func (r *Recommender) RecommendSequential(ctx context.Context, result []cache.Score, limit int, names ...string) ([]cache.Score, error) {
	for _, name := range names {
		recommenderFunc, err := r.parse(name)
		if err != nil {
			return nil, errors.Trace(err)
		}
		scores, err := recommenderFunc(ctx)
		if err != nil {
			return nil, errors.Trace(err)
		}
		for _, score := range scores {
			r.excludeSet.Add(score.Id)
		}
		result = append(result, scores...)
		if limit > 0 && len(result) >= limit {
			return result[:limit], nil
		}
	}
	return result, nil
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
	} else {
		return nil, errors.Errorf("unknown recommender: %s", fullname)
	}
}

func (r *Recommender) recommendLatest(ctx context.Context) ([]cache.Score, error) {
	items, err := r.dataClient.GetLatestItems(ctx, r.config.CacheSize, r.categories)
	if err != nil {
		return nil, errors.Trace(err)
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
	return scores, nil
}

func (r *Recommender) recommendNonPersonalized(name string) RecommenderFunc {
	return func(ctx context.Context) ([]cache.Score, error) {
		var categories []string
		if len(r.categories) == 0 {
			categories = []string{""}
		} else {
			categories = r.categories
		}
		// fetch items from cache
		items, err := r.cacheClient.SearchScores(ctx, cache.NonPersonalized, name, categories, 0, r.config.CacheSize)
		if err != nil {
			return nil, errors.Trace(err)
		}
		// remove excluded items
		return lo.Filter(items, func(item cache.Score, index int) bool {
			return !r.excludeSet.Contains(item.Id)
		}), nil
	}
}

func (r *Recommender) recommendCollaborative(ctx context.Context) ([]cache.Score, error) {
	// fetch items from cache
	items, err := r.cacheClient.SearchScores(ctx, cache.CollaborativeFiltering, r.userId, r.categories, 0, r.config.CacheSize)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// remove excluded items
	return lo.Filter(items, func(item cache.Score, index int) bool {
		return !r.excludeSet.Contains(item.Id)
	}), nil
}

func (r *Recommender) recommendItemToItem(name string) RecommenderFunc {
	return func(ctx context.Context) ([]cache.Score, error) {
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
		for _, feedback := range userFeedback {
			similarItems, err := r.cacheClient.SearchScores(ctx, cache.ItemToItem, cache.Key(name, feedback.ItemId), r.categories, 0, r.config.CacheSize)
			if err != nil {
				return nil, errors.Trace(err)
			}
			for _, item := range similarItems {
				if !r.excludeSet.Contains(item.Id) {
					scores[item.Id] += item.Score
					categories[item.Id] = item.Categories
				}
			}
		}
		// collect top scores
		filter := heap.NewTopKFilter[string, float64](10)
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
		}), nil
	}
}

func (r *Recommender) recommendUserToUser(name string) RecommenderFunc {
	return func(ctx context.Context) ([]cache.Score, error) {
		scores := make(map[string]float64)
		// load similar users
		similarUsers, err := r.cacheClient.SearchScores(ctx, cache.UserToUser, cache.Key(name, r.userId), nil, 0, r.config.CacheSize)
		if err != nil {
			return nil, errors.Trace(err)
		}
		for _, user := range similarUsers {
			// load historical feedback
			feedbacks, err := r.dataClient.GetUserFeedback(ctx, user.Id, lo.ToPtr(time.Now()), r.config.DataSource.PositiveFeedbackTypes...)
			if err != nil {
				return nil, errors.Trace(err)
			}
			// add unseen items
			for _, feedback := range feedbacks {
				if !r.excludeSet.Contains(feedback.ItemId) {
					scores[feedback.ItemId] += user.Score
				}
			}
		}
		// collect top k
		filter := heap.NewTopKFilter[string, float64](10)
		for id, score := range scores {
			filter.Push(id, score)
		}
		elems := filter.PopAll()
		// filter by categories
		results := make([]cache.Score, 0, len(elems))
		ids := lo.Map(elems, func(elem heap.Elem[string, float64], _ int) string {
			return elem.Value
		})
		items, err := r.dataClient.BatchGetItems(ctx, ids)
		if err != nil {
			return nil, errors.Trace(err)
		}
		itemsMap := make(map[string]data.Item)
		for _, item := range items {
			itemsMap[item.ItemId] = item
		}
		for _, elem := range elems {
			if item, ok := itemsMap[elem.Value]; ok && lo.Every(item.Categories, r.categories) {
				results = append(results, cache.Score{
					Id:         item.ItemId,
					Score:      elem.Weight,
					Categories: item.Categories,
				})
			}
		}
		return results, nil
	}
}
