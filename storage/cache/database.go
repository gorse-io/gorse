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
	"strings"
	"time"
)

const (
	IgnoreItems            = "ignore_items"            // ignored items for each user
	HiddenItems            = "hidden_items"            // hidden items
	ItemNeighbors          = "item_neighbors"          // neighbors of each item
	UserNeighbors          = "user_neighbors"          // neighbors of each user
	CollaborativeRecommend = "collaborative_recommend" // collaborative filtering recommendation for each user
	OfflineRecommend       = "offline_recommend"       // offline recommendation for each user
	// PopularItems is sorted set of popular items. The format of key:
	//   Global popular items      - latest_items
	//   Categorized popular items - latest_items/{category}
	PopularItems = "popular_items"
	// LatestItems is sorted set of the latest items. The format of key:
	//   Global latest items      - latest_items
	//   Categorized latest items - latest_items/{category}
	LatestItems = "latest_items"

	LastModifyItemTime          = "last_modify_item_time"           // the latest timestamp that a user related data was modified
	LastModifyUserTime          = "last_modify_user_time"           // the latest timestamp that an item related data was modified
	LastUpdateUserRecommendTime = "last_update_user_recommend_time" // the latest timestamp that a user's recommendation was updated
	LastUpdateUserNeighborsTime = "last_update_user_neighbors_time" // the latest timestamp that a user's neighbors item was updated
	LastUpdateItemNeighborsTime = "last_update_item_neighbors_time" // the latest timestamp that an item's neighbors was updated
	LastUpdateLatestItemsTime   = "last_update_latest_items_time"   // the latest timestamp that latest items were updated
	LastUpdatePopularItemsTime  = "last_update_popular_items_time"  // the latest timestamp that popular items were updated

	// GlobalMeta is global meta information
	GlobalMeta              = "global_meta"
	DataImported            = "data_imported"
	LastFitRankingModelTime = "last_fit_match_model_time"
	LastRankingModelVersion = "latest_match_model_version"
	// ItemCategories is the set of item categories.
	ItemCategories = "item_categories"
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

// Scores is slice of Scored.
type Scores []Scored

// Len is the number of elements in the collection.
func (s Scores) Len() int {
	return len(s)
}

// Less reports whether the element with index i
func (s Scores) Less(i, j int) bool {
	return s[i].Score > s[j].Score
}

// Swap swaps the elements with indexes i and j.
func (s Scores) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

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

// Database is the common interface for cache store.
type Database interface {
	Close() error
	SetScores(prefix, name string, items []Scored) error
	GetScores(prefix, name string, begin int, end int) ([]Scored, error)
	ClearScores(prefix, name string) error
	AppendScores(prefix, name string, items ...Scored) error
	SetCategoryScores(prefix, name, category string, items []Scored) error
	GetCategoryScores(prefix, name, category string, begin, end int) ([]Scored, error)
	GetString(prefix, name string) (string, error)
	SetString(prefix, name string, val string) error
	GetTime(prefix, name string) (time.Time, error)
	SetTime(prefix, name string, val time.Time) error
	GetInt(prefix, name string) (int, error)
	SetInt(prefix, name string, val int) error
	IncrInt(prefix, name string) error
	Delete(prefix, name string) error
	Exists(prefix string, names ...string) ([]int, error)

	GetSet(key string) ([]string, error)
	SetSet(key string, members ...string) error
	AddSet(key string, members ...string) error

	GetSort(key string, begin, end int) ([]Scored, error)
	SetSort(key string, scores []Scored) error
	IncrSort(key, member string) error
	RemSort(key, member string) error
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
