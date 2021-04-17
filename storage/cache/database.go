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
	PopularItems       = "popular_items"
	LatestItems        = "latest_items"
	SimilarItems       = "similar_items"
	CollaborativeItems = "collaborative_items"
	SubscribeItems     = "subscribe_items"

	GlobalMeta                  = "global_meta"
	CollectPopularTime          = "last_update_popular_time"
	CollectLatestTime           = "last_update_latest_time"
	CollectSimilarTime          = "last_update_similar_time"
	FitMatrixFactorizationTime  = "last_fit_match_model_time"
	FitFactorizationMachineTime = "last_fit_rank_model_time"
	CollaborativeRecommendTime  = "offline_recommend_time"
	MatrixFactorizationVersion  = "latest_match_model_version"
	FactorizationMachineVersion = "latest_rank_model_version"
)

type Database interface {
	Close() error
	SetList(prefix, name string, items []string) error
	GetList(prefix, name string, begin int, end int) ([]string, error)
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
