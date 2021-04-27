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
	"github.com/zhenghaoz/gorse/base"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"strings"
	"time"
)

const (
	ErrUserNotExist = "user not exist"
	ErrItemNotExist = "item not exist"
)

// Item stores meta data about item.
type Item struct {
	ItemId    string
	Timestamp time.Time
	Labels    []string
	Comment   string
}

// User stores meta data about user.
type User struct {
	UserId    string
	Labels    []string
	Subscribe []string
	Comment   string
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
	FeedbackKey
	Timestamp time.Time
	Comment   string
}

type Measurement struct {
	Name      string
	Timestamp time.Time
	Value     float32
	Comment   string
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
	GetItemFeedback(itemId string, feedbackType *string) ([]Feedback, error)
	// users
	InsertUser(user User) error
	DeleteUser(userId string) error
	GetUser(userId string) (User, error)
	GetUsers(cursor string, n int) (string, []User, error)
	GetUserFeedback(userId string, feedbackType *string) ([]Feedback, error)
	// feedback
	GetUserItemFeedback(userId, itemId string, feedbackType *string) ([]Feedback, error)
	DeleteUserItemFeedback(userId, itemId string, feedbackType *string) (int, error)
	InsertFeedback(feedback Feedback, insertUser, insertItem bool) error
	BatchInsertFeedback(feedback []Feedback, insertUser, insertItem bool) error
	GetFeedback(cursor string, n int, feedbackType *string) (string, []Feedback, error)
	// measurement
	InsertMeasurement(measurement Measurement) error
	GetMeasurements(name string, n int) ([]Measurement, error)
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
		// connect to database
		database := new(MongoDB)
		if database.client, err = mongo.Connect(context.Background(), options.Client().ApplyURI(path)); err != nil {
			return nil, err
		}
		// parse DSN and extract database name
		if cs, err := connstring.ParseAndValidate(path); err != nil {
			return nil, err
		} else {
			database.dbName = cs.Database
		}
		return database, nil
	} else if strings.HasPrefix(path, redisPrefix) {
		addr := path[len(redisPrefix):]
		database := new(Redis)
		database.client = redis.NewClient(&redis.Options{Addr: addr})
		base.Logger().Warn("redis is used for testing only")
		return database, nil
	}
	return nil, errors.Errorf("Unknown database: %s", path)
}
