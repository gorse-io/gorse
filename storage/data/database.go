// Copyright 2021 gorse Project Authors
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
	"github.com/go-redis/redis/v8"
	"github.com/juju/errors"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/storage"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"sort"
	"strings"
	"time"
)

var (
	ErrUserNotExist = errors.NotFoundf("user")
	ErrItemNotExist = errors.NotFoundf("item")
	ErrNoDatabase   = errors.NotAssignedf("database")
)

// Item stores meta data about item.
type Item struct {
	ItemId     string `gorm:"primaryKey"`
	IsHidden   bool
	Categories []string `gorm:"serializer:json"`
	Timestamp  time.Time
	Labels     []string `gorm:"serializer:json"`
	Comment    string
}

func (*Item) TableName() string {
	return "items"
}

// ItemPatch is the modification on an item.
type ItemPatch struct {
	IsHidden   *bool
	Categories []string
	Timestamp  *time.Time
	Labels     []string
	Comment    *string
}

// User stores meta data about user.
type User struct {
	UserId    string   `gorm:"primaryKey"`
	Labels    []string `gorm:"serializer:json"`
	Subscribe []string `gorm:"serializer:json"`
	Comment   string
}

func (*User) TableName() string {
	return "users"
}

// UserPatch is the modification on a user.
type UserPatch struct {
	Labels    []string
	Subscribe []string
	Comment   *string
}

// FeedbackKey identifies feedback.
type FeedbackKey struct {
	FeedbackType string
	UserId       string
	ItemId       string
}

func (*FeedbackKey) TableName() string {
	return "feedback"
}

// Feedback stores feedback.
type Feedback struct {
	FeedbackKey
	Timestamp time.Time
	Comment   string
}

// SortFeedbacks sorts feedback from latest to oldest.
func SortFeedbacks(feedback []Feedback) {
	sort.Sort(feedbackSorter(feedback))
}

type feedbackSorter []Feedback

func (sorter feedbackSorter) Len() int {
	return len(sorter)
}

func (sorter feedbackSorter) Less(i, j int) bool {
	return sorter[i].Timestamp.After(sorter[j].Timestamp)
}

func (sorter feedbackSorter) Swap(i, j int) {
	sorter[i], sorter[j] = sorter[j], sorter[i]
}

type Database interface {
	Init() error
	Close() error
	Optimize() error
	BatchInsertItems(items []Item) error
	BatchGetItems(itemIds []string) ([]Item, error)
	DeleteItem(itemId string) error
	GetItem(itemId string) (Item, error)
	ModifyItem(itemId string, patch ItemPatch) error
	GetItems(cursor string, n int, timeLimit *time.Time) (string, []Item, error)
	GetItemFeedback(itemId string, feedbackTypes ...string) ([]Feedback, error)
	BatchInsertUsers(users []User) error
	DeleteUser(userId string) error
	GetUser(userId string) (User, error)
	ModifyUser(userId string, patch UserPatch) error
	GetUsers(cursor string, n int) (string, []User, error)
	GetUserFeedback(userId string, withFuture bool, feedbackTypes ...string) ([]Feedback, error)
	GetUserItemFeedback(userId, itemId string, feedbackTypes ...string) ([]Feedback, error)
	DeleteUserItemFeedback(userId, itemId string, feedbackTypes ...string) (int, error)
	BatchInsertFeedback(feedback []Feedback, insertUser, insertItem, overwrite bool) error
	GetFeedback(cursor string, n int, timeLimit *time.Time, feedbackTypes ...string) (string, []Feedback, error)
	GetUserStream(batchSize int) (chan []User, chan error)
	GetItemStream(batchSize int, timeLimit *time.Time) (chan []Item, chan error)
	GetFeedbackStream(batchSize int, timeLimit *time.Time, feedbackTypes ...string) (chan []Feedback, chan error)
}

// Open a connection to a database.
func Open(path string) (Database, error) {
	var err error
	if strings.HasPrefix(path, storage.MySQLPrefix) {
		name := path[len(storage.MySQLPrefix):]
		// probe isolation variable name
		isolationVarName, err := storage.ProbeMySQLIsolationVariableName(name)
		if err != nil {
			return nil, errors.Trace(err)
		}
		// append parameters
		if name, err = storage.AppendMySQLParams(name, map[string]string{
			"sql_mode":       "'ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION'",
			isolationVarName: "'READ-UNCOMMITTED'",
			"parseTime":      "true",
		}); err != nil {
			return nil, errors.Trace(err)
		}
		// connect to database
		database := new(SQLDatabase)
		database.driver = MySQL
		if database.client, err = sql.Open("mysql", name); err != nil {
			return nil, errors.Trace(err)
		}
		database.gormDB, err = gorm.Open(mysql.New(mysql.Config{Conn: database.client}), gormConfig)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return database, nil
	} else if strings.HasPrefix(path, storage.PostgresPrefix) {
		database := new(SQLDatabase)
		database.driver = Postgres
		if database.client, err = sql.Open("postgres", path); err != nil {
			return nil, errors.Trace(err)
		}
		database.gormDB, err = gorm.Open(postgres.New(postgres.Config{Conn: database.client}), gormConfig)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return database, nil
	} else if strings.HasPrefix(path, storage.ClickhousePrefix) {
		uri := "http://" + path[len(storage.ClickhousePrefix):]
		database := new(SQLDatabase)
		database.driver = ClickHouse
		if database.client, err = sql.Open("clickhouse", uri); err != nil {
			return nil, errors.Trace(err)
		}
		database.gormDB, err = gorm.Open(clickhouse.New(clickhouse.Config{Conn: database.client}), gormConfig)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return database, nil
	} else if strings.HasPrefix(path, storage.MongoPrefix) || strings.HasPrefix(path, storage.MongoSrvPrefix) {
		// connect to database
		database := new(MongoDB)
		if database.client, err = mongo.Connect(context.Background(), options.Client().ApplyURI(path)); err != nil {
			return nil, errors.Trace(err)
		}
		// parse DSN and extract database name
		if cs, err := connstring.ParseAndValidate(path); err != nil {
			return nil, errors.Trace(err)
		} else {
			database.dbName = cs.Database
		}
		return database, nil
	} else if strings.HasPrefix(path, storage.SQLitePrefix) {
		name := path[len(storage.SQLitePrefix):]
		database := new(SQLDatabase)
		database.driver = SQLite
		if database.client, err = sql.Open("sqlite", name); err != nil {
			return nil, errors.Trace(err)
		}
		database.gormDB, err = gorm.Open(sqlite.Dialector{Conn: database.client}, gormConfig)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return database, nil
	} else if strings.HasPrefix(path, storage.RedisPrefix) {
		addr := path[len(storage.RedisPrefix):]
		database := new(Redis)
		database.client = redis.NewClient(&redis.Options{Addr: addr})
		log.Logger().Warn("redis is used for testing only")
		return database, nil
	}
	return nil, errors.Errorf("Unknown database: %s", path)
}
