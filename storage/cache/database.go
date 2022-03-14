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

package cache

import (
	"github.com/go-redis/redis/v8"
	"github.com/juju/errors"
	"sort"
	"strings"
	"time"
)

const (
	// IgnoreItems is sorted set of ignored items for each user
	//  Ignored items      - ignore_items/{user_id}
	IgnoreItems = "ignore_items"

	HiddenItems = "hidden_items" // hidden items

	// ItemNeighbors is sorted set of neighbors for each item.
	//  Global item neighbors      - item_neighbors/{item_id}
	//  Categorized item neighbors - item_neighbors/{item_id}/{category}
	ItemNeighbors = "item_neighbors"

	// UserNeighbors is sorted set of neighbors for each user.
	//  User neighbors      - user_neighbors/{user_id}
	UserNeighbors = "user_neighbors"

	CollaborativeRecommend = "collaborative_recommend" // collaborative filtering recommendation for each user
	OfflineRecommend       = "offline_recommend"       // offline recommendation for each user

	// PopularItems is sorted set of popular items. The format of key:
	//  Global popular items      - latest_items
	//  Categorized popular items - latest_items/{category}
	PopularItems = "popular_items"

	// LatestItems is sorted set of the latest items. The format of key:
	//  Global latest items      - latest_items
	//  Categorized the latest items - latest_items/{category}
	LatestItems = "latest_items"

	// ItemCategories is the set of item categories. The format of key:
	//	Global item categories - item_categories
	//	Categories of an item  - item_categories/{item_id}
	ItemCategories = "item_categories"

	LastModifyItemTime          = "last_modify_item_time"           // the latest timestamp that a user related data was modified
	LastModifyUserTime          = "last_modify_user_time"           // the latest timestamp that an item related data was modified
	LastUpdateUserRecommendTime = "last_update_user_recommend_time" // the latest timestamp that a user's recommendation was updated
	LastUpdateUserNeighborsTime = "last_update_user_neighbors_time" // the latest timestamp that a user's neighbors item was updated
	LastUpdateItemNeighborsTime = "last_update_item_neighbors_time" // the latest timestamp that an item's neighbors was updated

	// GlobalMeta is global meta information
	GlobalMeta                 = "global_meta"
	DataImported               = "data_imported"
	NumUsers                   = "num_users"
	NumItems                   = "num_items"
	NumUserLabels              = "num_user_labels"
	NumItemLabels              = "num_item_labels"
	NumTotalPosFeedbacks       = "num_total_pos_feedbacks"
	NumValidPosFeedbacks       = "num_valid_pos_feedbacks"
	NumValidNegFeedbacks       = "num_valid_neg_feedbacks"
	LastFitMatchingModelTime   = "last_fit_matching_model_time"
	LastFitRankingModelTime    = "last_fit_ranking_model_time"
	LastUpdateLatestItemsTime  = "last_update_latest_items_time"  // the latest timestamp that latest items were updated
	LastUpdatePopularItemsTime = "last_update_popular_items_time" // the latest timestamp that popular items were updated
	UserNeighborIndexRecall    = "user_neighbor_index_recall"
	ItemNeighborIndexRecall    = "item_neighbor_index_recall"
	MatchingIndexRecall        = "matching_index_recall"
)

var (
	ErrObjectNotExist = errors.NotFoundf("object")
	ErrNoDatabase     = errors.NotAssignedf("database")
)

// Scored associate a id with a score.
type Scored struct {
	Id    string
	Score float32
}

// CreateScoredItems from items and scores.
func CreateScoredItems(itemIds []string, scores []float32) []Scored {
	if len(itemIds) != len(scores) {
		panic("the length of itemIds and scores should be equal")
	}
	items := make([]Scored, len(itemIds))
	for i := range items {
		items[i].Id = itemIds[i]
		items[i].Score = scores[i]
	}
	return items
}

// RemoveScores resolve items for a slice of ScoredItems.
func RemoveScores(items []Scored) []string {
	ids := make([]string, len(items))
	for i := range ids {
		ids[i] = items[i].Id
	}
	return ids
}

// GetScores resolve scores for a slice of Scored.
func GetScores(s []Scored) []float32 {
	scores := make([]float32, len(s))
	for i := range s {
		scores[i] = s[i].Score
	}
	return scores
}

// SortScores sorts scores from high score to low score.
func SortScores(scores []Scored) {
	sort.Sort(scoresSorter(scores))
}

type scoresSorter []Scored

// Len is the number of elements in the collection.
func (s scoresSorter) Len() int {
	return len(s)
}

// Less reports whether the element with index i
func (s scoresSorter) Less(i, j int) bool {
	return s[i].Score > s[j].Score
}

// Swap swaps the elements with indexes i and j.
func (s scoresSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Key creates key for cache. Empty field will be ignored.
func Key(keys ...string) string {
	if len(keys) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(keys[0])
	for _, key := range keys[1:] {
		if key != "" {
			builder.WriteRune('/')
			builder.WriteString(key)
		}
	}
	return builder.String()
}

func BatchKey(prefix string, keys ...string) []string {
	for i, key := range keys {
		keys[i] = Key(prefix, key)
	}
	return keys
}

// Database is the common interface for cache store.
type Database interface {
	Close() error
	SetScores(name string, items []Scored) error
	GetScores(name string, begin int, end int) ([]Scored, error)
	ClearScores(name string) error
	AppendScores(name string, items ...Scored) error
	SetCategoryScores(name, category string, items []Scored) error
	GetCategoryScores(name, category string, begin, end int) ([]Scored, error)
	GetString(name string) (string, error)
	SetString(name string, val string) error
	GetTime(name string) (time.Time, error)
	SetTime(name string, val time.Time) error
	GetInt(name string) (int, error)
	SetInt(name string, val int) error
	IncrInt(name string) error
	Delete(name string) error
	Exists(names ...string) ([]int, error)

	GetSet(key string) ([]string, error)
	SetSet(key string, members ...string) error
	AddSet(key string, members ...string) error
	RemSet(key string, members ...string) error

	GetSortedScore(key, member string) (float32, error)
	GetSorted(key string, begin, end int) ([]Scored, error)
	GetSortedByScore(key string, begin, end float32) ([]Scored, error)
	AddSorted(key string, scores []Scored) error
	SetSorted(key string, scores []Scored) error
	IncrSorted(key, member string) error
	RemSorted(key, member string) error
}

const redisPrefix = "redis://"

// Open a connection to a database.
func Open(path string) (Database, error) {
	if strings.HasPrefix(path, redisPrefix) {
		opt, err := redis.ParseURL(path)
		if err != nil {
			return nil, err
		}
		database := new(Redis)
		database.client = redis.NewClient(opt)
		return database, nil
	}
	return nil, errors.Errorf("Unknown database: %s", path)
}
