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
	Collaborative              = "collaborative"
	LatestRecommender          = "latest"
	NonPersonalizedRecommender = "non-personalized/"
	ItemToItemRecommender      = "item-to-item/"
	UserToUserRecommender      = "user-to-user/"
)

type Recommender struct {
	config      config.Config
	cacheClient cache.Database
	dataClient  data.Database

	userId       string
	userFeedback []data.Feedback
	categories   []string
	excludeSet   mapset.Set[string]
}

type RecommenderFunc func(ctx context.Context) ([]cache.Score, error)

func NewRecommender(config config.Config,
	cacheClient cache.Database,
	dataClient data.Database,
	userId string, categories []string, exclude []string,
) *Recommender {
	return &Recommender{
		config:      config,
		cacheClient: cacheClient,
		dataClient:  dataClient,
		userId:      userId,
		categories:  categories,
		excludeSet:  mapset.NewSet(exclude...),
	}
}

func (r *Recommender) Parse(fullname string) (RecommenderFunc, error) {
	if fullname == Collaborative {
		return r.recommendCollaborative, nil
	} else if fullname == LatestRecommender {
		return r.recommendLatest, nil
	} else if strings.HasPrefix(fullname, NonPersonalizedRecommender) {
		name := strings.TrimPrefix(fullname, NonPersonalizedRecommender)
		return r.recommendNonPersonalized(name), nil
	} else {
		return nil, errors.Errorf("unknown recommender: %s", fullname)
	}
}

func (r *Recommender) recommendLatest(ctx context.Context) ([]cache.Score, error) {
	items, err := r.dataClient.GetLatestItems(ctx, r.config.Recommend.CacheSize, r.categories)
	if err != nil {
		return nil, errors.Trace(err)
	}
	scores := make([]cache.Score, 0, len(items))
	for _, item := range items {
		if !r.excludeSet.Contains(item.ItemId) {
			scores = append(scores, cache.Score{Id: item.ItemId, Score: float64(item.Timestamp.Unix())})
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
		items, err := r.cacheClient.SearchScores(ctx, cache.NonPersonalized, name, categories, 0, r.config.Recommend.CacheSize)
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
	items, err := r.cacheClient.SearchScores(ctx, cache.CollaborativeFiltering, r.userId, r.categories, 0, r.config.Recommend.CacheSize)
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
		userFeedback := make([]data.Feedback, 0, r.config.Recommend.CacheSize)
		for _, feedback := range r.userFeedback {
			if r.config.Recommend.CacheSize <= len(userFeedback) {
				break
			}
			if expression.MatchFeedbackTypeExpressions(r.config.Recommend.DataSource.PositiveFeedbackTypes, feedback.FeedbackType, feedback.Value) {
				userFeedback = append(userFeedback, feedback)
			}
		}
		// collect scores
		scores := make(map[string]float64)
		for _, feedback := range userFeedback {
			similarItems, err := r.cacheClient.SearchScores(ctx, cache.ItemToItem, cache.Key(name, feedback.ItemId), r.categories, 0, r.config.Recommend.CacheSize)
			if err != nil {
				return nil, errors.Trace(err)
			}
			for _, item := range similarItems {
				if !r.excludeSet.Contains(item.Id) {
					scores[item.Id] += item.Score
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
			return cache.Score{Id: elem.Value, Score: elem.Weight}
		}), nil
	}
}

func (r *Recommender) recommendUserToUser(name string) RecommenderFunc {
	return func(ctx context.Context) ([]cache.Score, error) {
		scores := make(map[string]float64)
		// load similar users
		similarUsers, err := r.cacheClient.SearchScores(ctx, cache.UserToUser, cache.Key(name, r.userId), nil, 0, r.config.Recommend.CacheSize)
		if err != nil {
			return nil, errors.Trace(err)
		}
		for _, user := range similarUsers {
			// load historical feedback
			feedbacks, err := r.dataClient.GetUserFeedback(ctx, user.Id, r.config.Now(), r.config.Recommend.DataSource.PositiveFeedbackTypes...)
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
		return lo.Map(filter.PopAll(), func(elem heap.Elem[string, float64], _ int) cache.Score {
			return cache.Score{Id: elem.Value, Score: elem.Weight}
		}), nil
	}
}
