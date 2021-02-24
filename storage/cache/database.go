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
	"github.com/pkg/errors"
	"strings"
)

const (
	PopularItems = "popular_items"
	LatestItems  = "latest_items"
	SimilarItems = "similar_items"
	MatchedItems = "matched_items"

	GlobalMeta              = "global_meta"
	LastUpdatePopularTime   = "last_update_popular_time"
	LastUpdateLatestTime    = "last_update_latest_time"
	LastUpdateSimilarTime   = "last_update_similar_time"
	LastRenewMatchModelTime = "last_renew_match_model_time"
	LatestMatchModelTerm    = "latest_match_model_term"
	LatestDatasetHash       = "latest_dataset_hash"
)

type Database interface {
	Close() error
	SetList(prefix, name string, items []string) error
	GetList(prefix, name string, n int, offset int) ([]string, error)
	GetString(prefix, name string) (string, error)
	SetString(prefix, name string, val string) error
	GetInt(prefix, name string) (int, error)
	SetInt(prefix, name string, val int) error
}

const redisPrefix = "redis://"

// Open a connection to a database.
func Open(path string) (Database, error) {
	if strings.HasPrefix(path, redisPrefix) {
		addr := path[len(redisPrefix):]
		database := new(Redis)
		database.client = redis.NewClient(&redis.Options{Addr: addr})
		return database, nil
	}
	return nil, errors.Errorf("Unknown database: %s", path)
}
