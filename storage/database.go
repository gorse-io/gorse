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
package storage

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"strings"
	"time"
)

// Item stores meta data about item.
type Item struct {
	ItemId    string
	Timestamp time.Time
	Labels    []string
}

// User stores meta data about user.
type User struct {
	UserId string
}

// Feedback stores feedback.
type Feedback struct {
	UserId string
	ItemId string
	Rating float64
}

// RecommendedItem is the structure for a recommended item.
type RecommendedItem struct {
	ItemId string
	Score  float64 // score
}

type Database interface {
	Close() error
	// items
	InsertItem(item Item) error
	BatchInsertItem(items []Item) error
	DeleteItem(itemId string) error
	GetItem(itemId string) (Item, error)
	GetItems(n int, offset int) ([]Item, error)
	GetItemFeedback(itemId string) ([]Feedback, error)
	// label
	GetLabelItems(label string) ([]Item, error)
	GetLabels() ([]string, error)
	// users
	InsertUser(user User) error
	DeleteUser(userId string) error
	GetUser(userId string) (User, error)
	GetUsers() ([]User, error)
	GetUserFeedback(userId string) ([]Feedback, error)
	InsertUserIgnore(userId string, items []string) error
	GetUserIgnore(userId string) ([]string, error)
	CountUserIgnore(userId string) (int, error)
	// feedback
	InsertFeedback(feedback Feedback) error
	BatchInsertFeedback(feedback []Feedback) error
	GetFeedback() ([]Feedback, error)
	// metadata
	GetString(name string) (string, error)
	SetString(name string, val string) error
	GetInt(name string) (int, error)
	SetInt(name string, val int) error
	// recommendation
	SetNeighbors(itemId string, items []RecommendedItem) error
	SetPop(label string, items []RecommendedItem) error
	SetLatest(label string, items []RecommendedItem) error
	SetRecommend(userId string, items []RecommendedItem) error
	GetNeighbors(itemId string, n int, offset int) ([]RecommendedItem, error)
	GetPop(label string, n int, offset int) ([]RecommendedItem, error)
	GetLatest(label string, n int, offset int) ([]RecommendedItem, error)
	GetRecommend(userId string, n int, offset int) ([]RecommendedItem, error)
}

const bagderPrefix = "badger://"
const redisPrefix = "redis://"

// Open a connection to a database.
func Open(path string) (Database, error) {
	var err error
	if strings.HasPrefix(path, bagderPrefix) {
		dataFolder := path[len(bagderPrefix):]
		database := new(Badger)
		if database.db, err = badger.Open(badger.DefaultOptions(dataFolder)); err != nil {
			return nil, err
		}
		return database, nil
	} else if strings.HasPrefix(path, redisPrefix) {
		addr := path[len(redisPrefix):]
		database := new(Redis)
		if database.client = redis.NewClient(&redis.Options{Addr: addr}); err != nil {
			return nil, err
		}
		return database, nil
	}
	return nil, errors.Errorf("Unknown database: %s", path)
}
