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
package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"strings"
	"time"
)

const (
	ErrUserNotExist = "user not exist"
	ErrItemNotExist = "item not exist"
)

// Item stores meta data about item.
type Item struct {
	ItemId    string `bson:"_id"`
	Timestamp time.Time
	Labels    []string
}

// User stores meta data about user.
type User struct {
	UserId string `bson:"_id"`
	Labels []string
}

// FeedbackKey identifies feedback.
type FeedbackKey struct {
	FeedbackType string
	UserId       string
	ItemId       string
}

func FeedbackKeyFromString(s string) (*FeedbackKey, error) {
	var feedbackKey FeedbackKey
	err := json.Unmarshal([]byte(s), &feedbackKey)
	return &feedbackKey, err
}

func (k *FeedbackKey) ToString() (string, error) {
	b, err := json.Marshal(k)
	return string(b), err
}

// Feedback stores feedback.
type Feedback struct {
	FeedbackKey `bson:"_id"`
	Timestamp   time.Time
}

type Database interface {
	Init() error
	Close() error
	// items
	InsertItem(item Item) error
	BatchInsertItem(items []Item) error
	DeleteItem(itemId string) error
	GetItem(itemId string) (Item, error)
	GetItems(cursor string, n int) (string, []Item, error)
	GetItemFeedback(feedbackType, itemId string) ([]Feedback, error)
	// users
	InsertUser(user User) error
	DeleteUser(userId string) error
	GetUser(userId string) (User, error)
	GetUsers(cursor string, n int) (string, []User, error)
	GetUserFeedback(feedbackType, userId string) ([]Feedback, error)
	// feedback
	InsertFeedback(feedback Feedback, insertUser, insertItem bool) error
	BatchInsertFeedback(feedback []Feedback, insertUser, insertItem bool) error
	GetFeedback(feedbackType, cursor string, n int) (string, []Feedback, error)
}

const mySQLPrefix = "mysql://"
const mongoPredix = "mongodb://"
const redisPrefix = "redis://"

// Open a connection to a database.
func Open(path string) (Database, error) {
	var err error
	if strings.HasPrefix(path, mySQLPrefix) {
		name := path[len(mySQLPrefix):]
		database := new(SQLDatabase)
		if database.db, err = sql.Open("mysql", name); err != nil {
			return nil, err
		}
		return database, nil
	} else if strings.HasPrefix(path, mongoPredix) {
		database := new(MongoDB)
		if database.client, err = mongo.Connect(context.Background(), options.Client().ApplyURI(path)); err != nil {
			return nil, err
		}
		return database, nil
	} else if strings.HasPrefix(path, redisPrefix) {
		addr := path[len(redisPrefix):]
		database := new(Redis)
		database.client = redis.NewClient(&redis.Options{Addr: addr})
		log.Warn("redis is used for testing only")
		return database, nil
	}
	return nil, errors.Errorf("Unknown database: %s", path)
}
