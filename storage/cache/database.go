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
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/juju/errors"
	"strings"
	"time"
)

const (
	IgnoreItems            = "ignore_items"
	ItemNeighbors          = "item_neighbors"
	UserNeighbors          = "user_neighbors"
	CollaborativeRecommend = "collaborative_recommend"
	CTRRecommend           = "ctr_recommend"
	PopularItems           = "popular_items"
	LatestItems            = "latest_items"

	LastModifyItemTime          = "last_modify_item_time"           // the latest timestamp that a user related data was modified
	LastModifyUserTime          = "last_modify_user_time"           // the latest timestamp that an item related data was modified
	LastUpdateUserRecommendTime = "last_update_user_recommend_time" // the latest timestamp that a user's recommendation was updated
	LastUpdateUserNeighborsTime = "last_update_user_neighbors_time" // the latest timestamp that a user's neighbors item was updated
	LastUpdateItemNeighborsTime = "last_update_item_neighbors_time" // the latest timestamp that an item's neighbors was updated

	// GlobalMeta is global meta information
	GlobalMeta              = "global_meta"
	DataImported            = "data_imported"
	LastFitRankingModelTime = "last_fit_match_model_time"
	LastRankingModelVersion = "latest_match_model_version"
)

var ErrObjectNotExist = fmt.Errorf("object not exists")
var ErrNoDatabase = fmt.Errorf("no database specified")

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

// Database is the common interface for cache store.
type Database interface {
	Close() error
	SetScores(prefix, name string, items []Scored) error
	GetScores(prefix, name string, begin int, end int) ([]Scored, error)
	ClearScores(prefix, name string) error
	AppendScores(prefix, name string, items ...Scored) error
	GetString(prefix, name string) (string, error)
	SetString(prefix, name string, val string) error
	GetTime(prefix, name string) (time.Time, error)
	SetTime(prefix, name string, val time.Time) error
	GetInt(prefix, name string) (int, error)
	SetInt(prefix, name string, val int) error
	IncrInt(prefix, name string) error
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
