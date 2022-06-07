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
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/juju/errors"
	_ "github.com/lib/pq"
	_ "github.com/mailru/go-clickhouse"
	"github.com/samber/lo"
	"github.com/scylladb/go-set/strset"
	"github.com/zhenghaoz/gorse/base/json"
	"github.com/zhenghaoz/gorse/base/log"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"moul.io/zapgorm2"
	"strings"
	"time"
)

const bufSize = 1

type SQLDriver int

const (
	MySQL SQLDriver = iota
	Postgres
	ClickHouse
	SQLite
)

var gormConfig = &gorm.Config{
	Logger:          zapgorm2.New(log.Logger()),
	CreateBatchSize: 1000,
	NamingStrategy: schema.NamingStrategy{
		SingularTable: true,
	},
}

// SQLDatabase use MySQL as data storage.
type SQLDatabase struct {
	gormDB *gorm.DB
	client *sql.DB
	driver SQLDriver
}

// Optimize is used by ClickHouse only.
func (d *SQLDatabase) Optimize() error {
	if d.driver == ClickHouse {
		for _, tableName := range []string{"users", "items", "feedback"} {
			_, err := d.client.Exec("OPTIMIZE TABLE " + tableName)
			if err != nil {
				return errors.Trace(err)
			}
		}
	}
	return nil
}

// Init tables and indices in MySQL.
func (d *SQLDatabase) Init() error {
	switch d.driver {
	case MySQL:
		// create tables
		type Items struct {
			ItemId     string    `gorm:"column:item_id;type:varchar(256) not null;primaryKey"`
			IsHidden   bool      `gorm:"column:is_hidden;type:bool;not null"`
			Categories []string  `gorm:"column:categories;type:json;not null"`
			Timestamp  time.Time `gorm:"column:time_stamp;type:datetime;not null"`
			Labels     []string  `gorm:"column:labels;type:json;not null"`
			Comment    string    `gorm:"column:comment;type:text;not null"`
		}
		type Users struct {
			UserId    string   `gorm:"column:user_id;type:varchar(256);not null;primaryKey"`
			Labels    []string `gorm:"column:labels;type:json;not null"`
			Subscribe []string `gorm:"column:subscribe;type:json;not null"`
			Comment   string   `gorm:"column:comment;type:text;not null"`
		}
		type Feedback struct {
			FeedbackType string    `gorm:"column:feedback_type;type:varchar(256);not null;primaryKey"`
			UserId       string    `gorm:"column:user_id;type:varchar(256);not null;primaryKey;index:user_id"`
			ItemId       string    `gorm:"column:item_id;type:varchar(256);not null;primaryKey;index:item_id"`
			Timestamp    time.Time `gorm:"column:time_stamp;type:datetime;not null"`
			Comment      string    `gorm:"column:comment;type:text;not null"`
		}
		err := d.gormDB.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(Users{}, Items{}, Feedback{})
		if err != nil {
			return errors.Trace(err)
		}
		// change settings
		if _, err := d.client.Exec("SET SESSION sql_mode=\"" +
			"ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO," +
			"NO_ENGINE_SUBSTITUTION\""); err != nil {
			return errors.Trace(err)
		}
		// disable lock
		if _, err := d.client.Exec("SET SESSION TRANSACTION ISOLATION LEVEL READ UNCOMMITTED"); err != nil {
			return errors.Trace(err)
		}
	case Postgres:
		// create tables
		type Items struct {
			ItemId     string    `gorm:"column:item_id;type:varchar(256);not null;primaryKey"`
			IsHidden   bool      `gorm:"column:is_hidden;type:bool;not null;default:false"`
			Categories []string  `gorm:"column:categories;type:json;not null;default:'[]'"`
			Timestamp  time.Time `gorm:"column:time_stamp;type:timestamptz;not null;default:'0001-01-01'::timestamptz"`
			Labels     []string  `gorm:"column:labels;type:json;not null;default:'[]'"`
			Comment    string    `gorm:"column:comment;type:text;not null;default:''"`
		}
		type Users struct {
			UserId    string   `gorm:"column:user_id;type:varchar(256) not null;primaryKey"`
			Labels    []string `gorm:"column:labels;type:json;not null;default:'[]'"`
			Subscribe []string `gorm:"column:subscribe;type:json;not null;default:'[]'"`
			Comment   string   `gorm:"column:comment;type:text;not null;default:''"`
		}
		type Feedback struct {
			FeedbackType string    `gorm:"column:feedback_type;type:varchar(256);not null;primaryKey"`
			UserId       string    `gorm:"column:user_id;type:varchar(256);not null;primaryKey;index:user_id_index"`
			ItemId       string    `gorm:"column:item_id;type:varchar(256);not null;primaryKey;index:item_id_index"`
			Timestamp    time.Time `gorm:"column:time_stamp;type:timestamptz;not null;default:'0001-01-01'::timestamptz"`
			Comment      string    `gorm:"column:comment;type:text;not null;default:''"`
		}
		err := d.gormDB.AutoMigrate(Users{}, Items{}, Feedback{})
		if err != nil {
			return errors.Trace(err)
		}
		// disable lock
		if _, err := d.client.Exec("SET SESSION CHARACTERISTICS AS TRANSACTION ISOLATION LEVEL READ UNCOMMITTED"); err != nil {
			return errors.Trace(err)
		}
	case SQLite:
		// create tables
		type Items struct {
			ItemId     string    `gorm:"column:item_id;type:varchar(256);not null;primaryKey"`
			IsHidden   bool      `gorm:"column:is_hidden;type:bool;not null;default:false"`
			Categories []string  `gorm:"column:categories;type:json;not null;default:'[]'"`
			Timestamp  time.Time `gorm:"column:time_stamp;type:datetime;not null;default:'0001-01-01'"`
			Labels     []string  `gorm:"column:labels;type:json;not null;default:'[]'"`
			Comment    string    `gorm:"column:comment;type:text;not null;default:''"`
		}
		type Users struct {
			UserId    string   `gorm:"column:user_id;type:varchar(256) not null;primaryKey"`
			Labels    []string `gorm:"column:labels;type:json;not null;default:'[]'"`
			Subscribe []string `gorm:"column:subscribe;type:json;not null;default:'[]'"`
			Comment   string   `gorm:"column:comment;type:text;not null;default:''"`
		}
		type Feedback struct {
			FeedbackType string    `gorm:"column:feedback_type;type:varchar(256);not null;primaryKey"`
			UserId       string    `gorm:"column:user_id;type:varchar(256);not null;primaryKey;index:user_id_index"`
			ItemId       string    `gorm:"column:item_id;type:varchar(256);not null;primaryKey;index:item_id_index"`
			Timestamp    time.Time `gorm:"column:time_stamp;type:datetime;not null;default:'0001-01-01'"`
			Comment      string    `gorm:"column:comment;type:text;not null;default:''"`
		}
		err := d.gormDB.AutoMigrate(Users{}, Items{}, Feedback{})
		if err != nil {
			return errors.Trace(err)
		}
	case ClickHouse:
		// create tables
		type Items struct {
			ItemId     string    `gorm:"column:item_id;type:String"`
			IsHidden   bool      `gorm:"column:is_hidden;type:Boolean default 0"`
			Categories []string  `gorm:"column:categories;type:String default '[]'"`
			Timestamp  time.Time `gorm:"column:time_stamp;type:Datetime"`
			Labels     []string  `gorm:"column:labels;type:String default '[]'"`
			Comment    string    `gorm:"column:comment;type:String"`
			Version    struct{}  `gorm:"column:version;type:DateTime"`
		}
		err := d.gormDB.Set("gorm:table_options", "ENGINE = ReplacingMergeTree(version) ORDER BY item_id").AutoMigrate(Items{})
		if err != nil {
			return errors.Trace(err)
		}
		type Users struct {
			UserId    string   `gorm:"column:user_id;type:String"`
			Labels    []string `gorm:"column:labels;type:String default '[]'"`
			Subscribe []string `gorm:"column:subscribe;type:String default '[]'"`
			Comment   string   `gorm:"column:comment;type:String"`
			Version   struct{} `gorm:"column:version;type:DateTime"`
		}
		err = d.gormDB.Set("gorm:table_options", "ENGINE = ReplacingMergeTree(version) ORDER BY user_id").AutoMigrate(Users{})
		if err != nil {
			return errors.Trace(err)
		}
		type Feedback struct {
			FeedbackType string    `gorm:"column:feedback_type;type:String"`
			UserId       string    `gorm:"column:user_id;type:String;index:user_index,type:bloom_filter(0.01),granularity:1"`
			ItemId       string    `gorm:"column:item_id;type:String;index:item_index,type:bloom_filter(0.01),granularity:1"`
			Timestamp    time.Time `gorm:"column:time_stamp;type:DateTime"`
			Comment      string    `gorm:"column:comment;type:String"`
			Version      struct{}  `gorm:"column:version;type:DateTime"`
		}
		err = d.gormDB.Set("gorm:table_options", "ENGINE = ReplacingMergeTree(version) ORDER BY (feedback_type, user_id, item_id)").AutoMigrate(Feedback{})
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// Close MySQL connection.
func (d *SQLDatabase) Close() error {
	return d.client.Close()
}

// BatchInsertItems inserts a batch of items into MySQL.
func (d *SQLDatabase) BatchInsertItems(items []Item) error {
	if len(items) == 0 {
		return nil
	}
	builder := strings.Builder{}
	switch d.driver {
	case MySQL:
		builder.WriteString("INSERT INTO items(item_id, is_hidden, categories, time_stamp, labels, `comment`) VALUES ")
	case Postgres:
		builder.WriteString("INSERT INTO items(item_id, is_hidden, categories, time_stamp, labels, comment) VALUES ")
	case ClickHouse:
		builder.WriteString("INSERT INTO items(item_id, is_hidden, categories, time_stamp, labels, comment, version) VALUES ")
	case SQLite:
		builder.WriteString("INSERT OR REPLACE INTO items(item_id, is_hidden, categories, time_stamp, labels, `comment`) VALUES ")
	}
	memo := strset.New()
	var args []interface{}
	for _, item := range items {
		// remove duplicate items
		if memo.Has(item.ItemId) {
			continue
		} else {
			memo.Add(item.ItemId)
		}
		if len(args) > 0 {
			builder.WriteString(",")
		}
		labels, err := json.Marshal(item.Labels)
		if err != nil {
			return errors.Trace(err)
		}
		categories, err := json.Marshal(item.Categories)
		if err != nil {
			return errors.Trace(err)
		}
		switch d.driver {
		case MySQL, SQLite:
			builder.WriteString("(?,?,?,?,?,?)")
		case Postgres:
			builder.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d)", len(args)+1, len(args)+2, len(args)+3, len(args)+4, len(args)+5, len(args)+6))
		case ClickHouse:
			builder.WriteString("(?,?,?,?,?,?,NOW())")
		}
		if d.driver == ClickHouse || d.driver == SQLite {
			args = append(args, item.ItemId, item.IsHidden, string(categories), item.Timestamp.In(time.UTC), string(labels), item.Comment)
		} else {
			args = append(args, item.ItemId, item.IsHidden, string(categories), item.Timestamp, string(labels), item.Comment)
		}
	}
	switch d.driver {
	case MySQL:
		builder.WriteString(" ON DUPLICATE KEY " +
			"UPDATE is_hidden = VALUES(is_hidden), categories = VALUES(categories), time_stamp = VALUES(time_stamp), labels = VALUES(labels), `comment` = VALUES(`comment`)")
	case Postgres:
		builder.WriteString(" ON CONFLICT (item_id) " +
			"DO UPDATE SET is_hidden = EXCLUDED.is_hidden, categories = EXCLUDED.categories, time_stamp = EXCLUDED.time_stamp, labels = EXCLUDED.labels, comment = EXCLUDED.comment")
	}
	_, err := d.client.Exec(builder.String(), args...)
	return errors.Trace(err)
}

func (d *SQLDatabase) BatchGetItems(itemIds []string) ([]Item, error) {
	if len(itemIds) == 0 {
		return nil, nil
	}
	result, err := d.gormDB.Table("items").Select("item_id, is_hidden, categories, time_stamp, labels, comment").Where("item_id IN ?", itemIds).Rows()
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer result.Close()
	var items []Item
	for result.Next() {
		var item Item
		var labels, categories string
		if err = result.Scan(&item.ItemId, &item.IsHidden, &categories, &item.Timestamp, &labels, &item.Comment); err != nil {
			return nil, errors.Trace(err)
		}
		if err = json.Unmarshal([]byte(labels), &item.Labels); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(categories), &item.Categories); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// DeleteItem deletes a item from MySQL.
func (d *SQLDatabase) DeleteItem(itemId string) error {
	if err := d.gormDB.Delete(&Item{ItemId: itemId}).Error; err != nil {
		return errors.Trace(err)
	}
	if err := d.gormDB.Delete(&Feedback{}, "item_id = ?", itemId).Error; err != nil {
		return errors.Trace(err)
	}
	return nil
}

// GetItem get a item from MySQL.
func (d *SQLDatabase) GetItem(itemId string) (Item, error) {
	var result *sql.Rows
	var err error
	result, err = d.gormDB.Table("items").Select("item_id, is_hidden, categories, time_stamp, labels, comment").Where("item_id = ?", itemId).Rows()
	if err != nil {
		return Item{}, errors.Trace(err)
	}
	defer result.Close()
	if result.Next() {
		var item Item
		var labels, categories string
		if err := result.Scan(&item.ItemId, &item.IsHidden, &categories, &item.Timestamp, &labels, &item.Comment); err != nil {
			return Item{}, errors.Trace(err)
		}
		if err := json.Unmarshal([]byte(labels), &item.Labels); err != nil {
			return Item{}, err
		}
		if err := json.Unmarshal([]byte(categories), &item.Categories); err != nil {
			return Item{}, err
		}
		return item, nil
	}
	return Item{}, errors.Annotate(ErrItemNotExist, itemId)
}

// ModifyItem modify an item in MySQL.
func (d *SQLDatabase) ModifyItem(itemId string, patch ItemPatch) error {
	// ignore empty patch
	if patch.Labels == nil && patch.Comment == nil && patch.Timestamp == nil {
		log.Logger().Debug("empty item patch")
		return nil
	}
	attributes := make(map[string]any)
	if patch.IsHidden != nil {
		attributes["is_hidden"] = *patch.IsHidden
	}
	if patch.Categories != nil {
		text, _ := json.Marshal(patch.Categories)
		attributes["categories"] = string(text)
	}
	if patch.Comment != nil {
		attributes["comment"] = *patch.Comment
	}
	if patch.Labels != nil {
		text, _ := json.Marshal(patch.Labels)
		attributes["labels"] = string(text)
	}
	if patch.Timestamp != nil {
		if d.driver == ClickHouse || d.driver == SQLite {
			attributes["time_stamp"] = patch.Timestamp.In(time.UTC)
		} else {
			attributes["time_stamp"] = patch.Timestamp
		}
	}
	err := d.gormDB.Model(&Item{ItemId: itemId}).Updates(attributes).Error
	return errors.Trace(err)
}

// GetItems returns items from MySQL.
func (d *SQLDatabase) GetItems(cursor string, n int, timeLimit *time.Time) (string, []Item, error) {
	tx := d.gormDB.Table("items").Select("item_id, is_hidden, categories, time_stamp, labels, comment").Where("item_id >= ?", cursor)
	if timeLimit != nil {
		tx.Where("time_stamp >= ?", *timeLimit)
	}
	result, err := tx.Order("item_id").Limit(n + 1).Rows()
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	items := make([]Item, 0)
	defer result.Close()
	for result.Next() {
		var item Item
		var labels, categories string
		if err = result.Scan(&item.ItemId, &item.IsHidden, &categories, &item.Timestamp, &labels, &item.Comment); err != nil {
			return "", nil, errors.Trace(err)
		}
		if err = json.Unmarshal([]byte(labels), &item.Labels); err != nil {
			return "", nil, errors.Trace(err)
		}
		if err = json.Unmarshal([]byte(categories), &item.Categories); err != nil {
			return "", nil, errors.Trace(err)
		}
		items = append(items, item)
	}
	if len(items) == n+1 {
		return items[len(items)-1].ItemId, items[:len(items)-1], nil
	}
	return "", items, nil
}

// GetItemStream reads items by stream.
func (d *SQLDatabase) GetItemStream(batchSize int, timeLimit *time.Time) (chan []Item, chan error) {
	itemChan := make(chan []Item, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(itemChan)
		defer close(errChan)
		// send query
		tx := d.gormDB.Table("items").Select("item_id, is_hidden, categories, time_stamp, labels, comment")
		if timeLimit != nil {
			tx.Where("time_stamp >= ?", *timeLimit)
		}
		result, err := tx.Rows()
		if err != nil {
			errChan <- errors.Trace(err)
			return
		}
		// fetch result
		items := make([]Item, 0, batchSize)
		defer result.Close()
		for result.Next() {
			var item Item
			var labels, categories string
			if err = result.Scan(&item.ItemId, &item.IsHidden, &categories, &item.Timestamp, &labels, &item.Comment); err != nil {
				errChan <- errors.Trace(err)
				return
			}
			if err = json.Unmarshal([]byte(labels), &item.Labels); err != nil {
				errChan <- errors.Trace(err)
				return
			}
			if err = json.Unmarshal([]byte(categories), &item.Categories); err != nil {
				errChan <- errors.Trace(err)
				return
			}
			items = append(items, item)
			if len(items) == batchSize {
				itemChan <- items
				items = make([]Item, 0, batchSize)
			}
		}
		if len(items) > 0 {
			itemChan <- items
		}
		errChan <- nil
	}()
	return itemChan, errChan
}

// GetItemFeedback returns feedback of a item from MySQL.
func (d *SQLDatabase) GetItemFeedback(itemId string, feedbackTypes ...string) ([]Feedback, error) {
	tx := d.gormDB.Table("feedback").Select("user_id, item_id, feedback_type, time_stamp")
	if d.driver == SQLite {
		tx.Where("time_stamp <= DATETIME() AND item_id = ?", itemId)
	} else {
		tx.Where("time_stamp <= NOW() AND item_id = ?", itemId)
	}
	if len(feedbackTypes) > 0 {
		tx.Where("feedback_type IN ?", feedbackTypes)
	}
	result, err := tx.Rows()
	if err != nil {
		return nil, errors.Trace(err)
	}
	feedbacks := make([]Feedback, 0)
	defer result.Close()
	for result.Next() {
		var feedback Feedback
		if err = result.Scan(&feedback.UserId, &feedback.ItemId, &feedback.FeedbackType, &feedback.Timestamp); err != nil {
			return nil, errors.Trace(err)
		}
		feedbacks = append(feedbacks, feedback)
	}
	return feedbacks, nil
}

// BatchInsertUsers inserts users into MySQL.
func (d *SQLDatabase) BatchInsertUsers(users []User) error {
	if len(users) == 0 {
		return nil
	}
	builder := strings.Builder{}
	switch d.driver {
	case MySQL:
		builder.WriteString("INSERT INTO users(user_id, labels, subscribe, `comment`) VALUES ")
	case Postgres:
		builder.WriteString("INSERT INTO users(user_id, labels, subscribe, comment) VALUES ")
	case ClickHouse:
		builder.WriteString("INSERT INTO users(user_id, labels, subscribe, comment, version) VALUES ")
	case SQLite:
		builder.WriteString("INSERT OR REPLACE INTO users(user_id, labels, subscribe, `comment`) VALUES ")
	}
	memo := strset.New()
	var args []interface{}
	for _, user := range users {
		// remove duplicate users
		if memo.Has(user.UserId) {
			continue
		} else {
			memo.Add(user.UserId)
		}
		if len(args) > 0 {
			builder.WriteString(",")
		}
		labels, err := json.Marshal(user.Labels)
		if err != nil {
			return errors.Trace(err)
		}
		subscribe, err := json.Marshal(user.Subscribe)
		if err != nil {
			return errors.Trace(err)
		}
		switch d.driver {
		case MySQL, SQLite:
			builder.WriteString("(?,?,?,?)")
		case Postgres:
			builder.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d)", len(args)+1, len(args)+2, len(args)+3, len(args)+4))
		case ClickHouse:
			builder.WriteString("(?,?,?,?,NOW())")
		}
		args = append(args, user.UserId, string(labels), string(subscribe), user.Comment)
	}
	switch d.driver {
	case MySQL:
		builder.WriteString(" ON DUPLICATE KEY " +
			"UPDATE labels = VALUES(labels), subscribe = VALUES(subscribe), `comment` = VALUES(`comment`)")
	case Postgres:
		builder.WriteString(" ON CONFLICT (user_id) " +
			"DO UPDATE SET labels = EXCLUDED.labels, subscribe = EXCLUDED.subscribe, comment = EXCLUDED.comment")
	}
	_, err := d.client.Exec(builder.String(), args...)
	return errors.Trace(err)
}

// DeleteUser deletes a user from MySQL.
func (d *SQLDatabase) DeleteUser(userId string) error {
	if err := d.gormDB.Delete(&User{UserId: userId}).Error; err != nil {
		return errors.Trace(err)
	}
	if err := d.gormDB.Delete(&Feedback{}, "user_id = ?", userId).Error; err != nil {
		return errors.Trace(err)
	}
	return nil
}

// GetUser returns a user from MySQL.
func (d *SQLDatabase) GetUser(userId string) (User, error) {
	var result *sql.Rows
	var err error
	result, err = d.gormDB.Table("users").Select("user_id, labels, subscribe, comment").Where("user_id = ?", userId).Rows()
	if err != nil {
		return User{}, errors.Trace(err)
	}
	defer result.Close()
	if result.Next() {
		var user User
		var labels string
		var subscribe string
		if err = result.Scan(&user.UserId, &labels, &subscribe, &user.Comment); err != nil {
			return User{}, errors.Trace(err)
		}
		if err = json.Unmarshal([]byte(labels), &user.Labels); err != nil {
			return User{}, errors.Trace(err)
		}
		if err = json.Unmarshal([]byte(subscribe), &user.Subscribe); err != nil {
			return User{}, errors.Trace(err)
		}
		return user, nil
	}
	return User{}, errors.Annotate(ErrUserNotExist, userId)
}

// ModifyUser modify a user in MySQL.
func (d *SQLDatabase) ModifyUser(userId string, patch UserPatch) error {
	// ignore empty patch
	if patch.Labels == nil && patch.Comment == nil {
		log.Logger().Debug("empty user patch")
		return nil
	}
	attributes := make(map[string]any)
	if patch.Comment != nil {
		attributes["comment"] = *patch.Comment
	}
	if patch.Labels != nil {
		text, _ := json.Marshal(patch.Labels)
		attributes["labels"] = string(text)
	}
	err := d.gormDB.Model(&User{UserId: userId}).Updates(attributes).Error
	return errors.Trace(err)
}

// GetUsers returns users from MySQL.
func (d *SQLDatabase) GetUsers(cursor string, n int) (string, []User, error) {
	result, err := d.gormDB.Table("users").Select("user_id, labels, subscribe, comment").Where("user_id >= ?", cursor).Order("user_id").Limit(n + 1).Rows()
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	users := make([]User, 0)
	defer result.Close()
	for result.Next() {
		var user User
		var labels string
		var subscribe string
		if err = result.Scan(&user.UserId, &labels, &subscribe, &user.Comment); err != nil {
			return "", nil, errors.Trace(err)
		}
		if err = json.Unmarshal([]byte(labels), &user.Labels); err != nil {
			return "", nil, errors.Trace(err)
		}
		if err = json.Unmarshal([]byte(subscribe), &user.Subscribe); err != nil {
			return "", nil, errors.Trace(err)
		}
		users = append(users, user)
	}
	if len(users) == n+1 {
		return users[len(users)-1].UserId, users[:len(users)-1], nil
	}
	return "", users, nil
}

// GetUserStream read users by stream.
func (d *SQLDatabase) GetUserStream(batchSize int) (chan []User, chan error) {
	userChan := make(chan []User, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(userChan)
		defer close(errChan)
		// send query
		result, err := d.gormDB.Table("users").Select("user_id, labels, subscribe, comment").Rows()
		if err != nil {
			errChan <- errors.Trace(err)
			return
		}
		// fetch result
		users := make([]User, 0, batchSize)
		defer result.Close()
		for result.Next() {
			var user User
			var labels string
			var subscribe string
			if err = result.Scan(&user.UserId, &labels, &subscribe, &user.Comment); err != nil {
				errChan <- errors.Trace(err)
				return
			}
			if err = json.Unmarshal([]byte(labels), &user.Labels); err != nil {
				errChan <- errors.Trace(err)
				return
			}
			if err = json.Unmarshal([]byte(subscribe), &user.Subscribe); err != nil {
				errChan <- errors.Trace(err)
				return
			}
			users = append(users, user)
			if len(users) == batchSize {
				userChan <- users
				users = make([]User, 0, batchSize)
			}
		}
		if len(users) > 0 {
			userChan <- users
		}
		errChan <- nil
	}()
	return userChan, errChan
}

// GetUserFeedback returns feedback of a user from MySQL.
func (d *SQLDatabase) GetUserFeedback(userId string, withFuture bool, feedbackTypes ...string) ([]Feedback, error) {
	tx := d.gormDB.Table("feedback").Select("feedback_type, user_id, item_id, time_stamp, comment").Where("user_id = ?", userId)
	if !withFuture {
		if d.driver == SQLite {
			tx.Where("time_stamp <= DATETIME()")
		} else {
			tx.Where("time_stamp <= NOW()")
		}
	}
	if len(feedbackTypes) > 0 {
		tx.Where("feedback_type IN ?", feedbackTypes)
	}
	result, err := tx.Rows()
	if err != nil {
		return nil, errors.Trace(err)
	}
	feedbacks := make([]Feedback, 0)
	defer result.Close()
	for result.Next() {
		var feedback Feedback
		if err = result.Scan(&feedback.FeedbackType, &feedback.UserId, &feedback.ItemId, &feedback.Timestamp, &feedback.Comment); err != nil {
			return nil, errors.Trace(err)
		}
		feedbacks = append(feedbacks, feedback)
	}
	return feedbacks, nil
}

// BatchInsertFeedback insert a batch feedback into MySQL.
// If insertUser set, new users will be inserted to user table.
// If insertItem set, new items will be inserted to item table.
func (d *SQLDatabase) BatchInsertFeedback(feedback []Feedback, insertUser, insertItem, overwrite bool) error {
	// skip empty list
	if len(feedback) == 0 {
		return nil
	}
	// collect users and items
	users := strset.New()
	items := strset.New()
	for _, v := range feedback {
		users.Add(v.UserId)
		items.Add(v.ItemId)
	}
	// insert users
	if insertUser {
		userList := users.List()
		builder := strings.Builder{}
		switch d.driver {
		case MySQL:
			builder.WriteString("INSERT IGNORE users(user_id, labels, subscribe) VALUES ")
		case Postgres:
			builder.WriteString("INSERT INTO users(user_id) VALUES ")
		case ClickHouse:
			builder.WriteString("INSERT INTO users(user_id, version) VALUES ")
		case SQLite:
			builder.WriteString("INSERT OR IGNORE INTO users(user_id, labels, subscribe) VALUES ")
		}
		var args []interface{}
		for i, user := range userList {
			switch d.driver {
			case MySQL, SQLite:
				builder.WriteString("(?, '[]', '[]')")
			case Postgres:
				builder.WriteString(fmt.Sprintf("($%d)", i+1))
			case ClickHouse:
				builder.WriteString("(?,'0000-00-00 00:00:00')")
			}
			if i+1 < len(userList) {
				builder.WriteString(",")
			}
			args = append(args, user)
		}
		if d.driver == Postgres {
			builder.WriteString(" ON CONFLICT (user_id) DO NOTHING")
		}
		if _, err := d.client.Exec(builder.String(), args...); err != nil {
			return errors.Trace(err)
		}
	} else {
		for _, user := range users.List() {
			rs, err := d.gormDB.Table("users").Select("user_id").Where("user_id = ?", user).Rows()
			if err != nil {
				return errors.Trace(err)
			} else if !rs.Next() {
				users.Remove(user)
			}
			if err = rs.Close(); err != nil {
				return errors.Trace(err)
			}
		}
	}
	// insert items
	if insertItem {
		itemList := items.List()
		builder := strings.Builder{}
		switch d.driver {
		case MySQL:
			builder.WriteString("INSERT IGNORE items(item_id, labels, categories) VALUES ")
		case Postgres:
			builder.WriteString("INSERT INTO items(item_id) VALUES ")
		case ClickHouse:
			builder.WriteString("INSERT INTO items(item_id, version) VALUES ")
		case SQLite:
			builder.WriteString("INSERT OR IGNORE INTO items(item_id, labels, categories) VALUES ")
		}
		var args []interface{}
		for i, item := range itemList {
			switch d.driver {
			case MySQL, SQLite:
				builder.WriteString("(?, '[]', '[]')")
			case Postgres:
				builder.WriteString(fmt.Sprintf("($%d)", i+1))
			case ClickHouse:
				builder.WriteString("(?,'0000-00-00 00:00:00')")
			}
			if i+1 < len(itemList) {
				builder.WriteString(",")
			}
			args = append(args, item)
		}
		if d.driver == Postgres {
			builder.WriteString(" ON CONFLICT (item_id) DO NOTHING")
		}
		if _, err := d.client.Exec(builder.String(), args...); err != nil {
			return errors.Trace(err)
		}
	} else {
		for _, item := range items.List() {
			rs, err := d.gormDB.Table("items").Select("item_id").Where("item_id = ?", item).Rows()
			if err != nil {
				return errors.Trace(err)
			} else if !rs.Next() {
				items.Remove(item)
			}
			if err = rs.Close(); err != nil {
				return errors.Trace(err)
			}
		}
	}
	// insert feedback
	builder := strings.Builder{}
	switch d.driver {
	case MySQL:
		if overwrite {
			builder.WriteString("INSERT INTO feedback(feedback_type, user_id, item_id, time_stamp, `comment`) VALUES ")
		} else {
			builder.WriteString("INSERT IGNORE INTO feedback(feedback_type, user_id, item_id, time_stamp, `comment`) VALUES ")
		}
	case SQLite:
		if overwrite {
			builder.WriteString("INSERT OR REPLACE INTO feedback(feedback_type, user_id, item_id, time_stamp, `comment`) VALUES ")
		} else {
			builder.WriteString("INSERT OR IGNORE INTO feedback(feedback_type, user_id, item_id, time_stamp, `comment`) VALUES ")
		}
	case ClickHouse:
		builder.WriteString("INSERT INTO feedback(feedback_type, user_id, item_id, time_stamp, `comment`, version) VALUES ")
	case Postgres:
		builder.WriteString("INSERT INTO feedback(feedback_type, user_id, item_id, time_stamp, comment) VALUES ")
	}
	var args []interface{}
	memo := make(map[lo.Tuple3[string, string, string]]struct{})
	for _, f := range feedback {
		if users.Has(f.UserId) && items.Has(f.ItemId) {
			// remove duplicate feedback
			if _, exist := memo[lo.Tuple3[string, string, string]{f.FeedbackType, f.UserId, f.ItemId}]; exist {
				continue
			} else {
				memo[lo.Tuple3[string, string, string]{f.FeedbackType, f.UserId, f.ItemId}] = struct{}{}
			}
			if len(args) > 0 {
				builder.WriteString(",")
			}
			switch d.driver {
			case MySQL, SQLite:
				builder.WriteString("(?,?,?,?,?)")
			case ClickHouse:
				if overwrite {
					builder.WriteString("(?,?,?,?,?,NOW())")
				} else {
					builder.WriteString("(?,?,?,?,?,0)")
				}
			case Postgres:
				builder.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d)",
					len(args)+1, len(args)+2, len(args)+3, len(args)+4, len(args)+5))
			}
			if d.driver == ClickHouse || d.driver == SQLite {
				args = append(args, f.FeedbackType, f.UserId, f.ItemId, f.Timestamp.In(time.UTC), f.Comment)
			} else {
				args = append(args, f.FeedbackType, f.UserId, f.ItemId, f.Timestamp, f.Comment)
			}
		}
	}
	if len(args) == 0 {
		return nil
	}
	if overwrite {
		switch d.driver {
		case MySQL:
			builder.WriteString(" ON DUPLICATE KEY UPDATE time_stamp = VALUES(time_stamp), `comment` = VALUES(`comment`)")
		case Postgres:
			builder.WriteString(" ON CONFLICT (feedback_type, user_id, item_id) DO UPDATE SET time_stamp = EXCLUDED.time_stamp, comment = EXCLUDED.comment")
		}
	} else if d.driver == Postgres {
		builder.WriteString(" ON CONFLICT (feedback_type, user_id, item_id) DO NOTHING")
	}
	_, err := d.client.Exec(builder.String(), args...)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// GetFeedback returns feedback from MySQL.
func (d *SQLDatabase) GetFeedback(cursor string, n int, timeLimit *time.Time, feedbackTypes ...string) (string, []Feedback, error) {
	var cursorKey FeedbackKey
	if cursor != "" {
		if err := json.Unmarshal([]byte(cursor), &cursorKey); err != nil {
			return "", nil, err
		}
	}
	tx := d.gormDB.Table("feedback").Select("feedback_type, user_id, item_id, time_stamp, comment")
	if d.driver == SQLite {
		tx.Where("time_stamp <= DATETIME() AND (feedback_type, user_id, item_id) >= (?,?,?)", cursorKey.FeedbackType, cursorKey.UserId, cursorKey.ItemId)
	} else {
		tx.Where("time_stamp <= NOW() AND (feedback_type, user_id, item_id) >= (?,?,?)", cursorKey.FeedbackType, cursorKey.UserId, cursorKey.ItemId)
	}
	if len(feedbackTypes) > 0 {
		tx.Where("feedback_type IN ?", feedbackTypes)
	}
	if timeLimit != nil {
		tx.Where("time_stamp >= ?", *timeLimit)
	}
	tx.Order("feedback_type, user_id, item_id").Limit(n + 1)
	result, err := tx.Rows()
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	feedbacks := make([]Feedback, 0)
	defer result.Close()
	for result.Next() {
		var feedback Feedback
		if err = result.Scan(&feedback.FeedbackType, &feedback.UserId, &feedback.ItemId, &feedback.Timestamp, &feedback.Comment); err != nil {
			return "", nil, errors.Trace(err)
		}
		feedbacks = append(feedbacks, feedback)
	}
	if len(feedbacks) == n+1 {
		nextCursorKey := feedbacks[len(feedbacks)-1].FeedbackKey
		nextCursor, err := json.Marshal(nextCursorKey)
		if err != nil {
			return "", nil, errors.Trace(err)
		}
		return string(nextCursor), feedbacks[:len(feedbacks)-1], nil
	}
	return "", feedbacks, nil
}

// GetFeedbackStream reads feedback by stream.
func (d *SQLDatabase) GetFeedbackStream(batchSize int, timeLimit *time.Time, feedbackTypes ...string) (chan []Feedback, chan error) {
	feedbackChan := make(chan []Feedback, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(feedbackChan)
		defer close(errChan)
		// send query
		tx := d.gormDB.Table("feedback").Select("feedback_type, user_id, item_id, time_stamp, comment")
		if d.driver == SQLite {
			tx.Where("time_stamp <= DATETIME()")
		} else {
			tx.Where("time_stamp <= NOW()")
		}
		if len(feedbackTypes) > 0 {
			tx.Where("feedback_type IN ?", feedbackTypes)
		}
		if timeLimit != nil {
			tx.Where("time_stamp >= ?", *timeLimit)
		}
		result, err := tx.Rows()
		if err != nil {
			errChan <- errors.Trace(err)
			return
		}
		// fetch result
		feedbacks := make([]Feedback, 0, batchSize)
		defer result.Close()
		for result.Next() {
			var feedback Feedback
			if err = result.Scan(&feedback.FeedbackType, &feedback.UserId, &feedback.ItemId, &feedback.Timestamp, &feedback.Comment); err != nil {
				errChan <- errors.Trace(err)
				return
			}
			feedbacks = append(feedbacks, feedback)
			if len(feedbacks) == batchSize {
				feedbackChan <- feedbacks
				feedbacks = make([]Feedback, 0, batchSize)
			}
		}
		if len(feedbacks) > 0 {
			feedbackChan <- feedbacks
		}
		errChan <- nil
	}()
	return feedbackChan, errChan
}

// GetUserItemFeedback gets a feedback by user id and item id from MySQL.
func (d *SQLDatabase) GetUserItemFeedback(userId, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	tx := d.gormDB.Table("feedback").Select("feedback_type, user_id, item_id, time_stamp, comment").Where("user_id = ? AND item_id = ?", userId, itemId)
	if len(feedbackTypes) > 0 {
		tx.Where("feedback_type IN ?", feedbackTypes)
	}
	result, err := tx.Rows()
	if err != nil {
		return nil, errors.Trace(err)
	}
	feedbacks := make([]Feedback, 0)
	defer result.Close()
	for result.Next() {
		var feedback Feedback
		if err = result.Scan(&feedback.FeedbackType, &feedback.UserId, &feedback.ItemId, &feedback.Timestamp, &feedback.Comment); err != nil {
			return nil, errors.Trace(err)
		}
		feedbacks = append(feedbacks, feedback)
	}
	return feedbacks, nil
}

// DeleteUserItemFeedback deletes a feedback by user id and item id from MySQL.
func (d *SQLDatabase) DeleteUserItemFeedback(userId, itemId string, feedbackTypes ...string) (int, error) {
	tx := d.gormDB.Where("user_id = ? AND item_id = ?", userId, itemId)
	if len(feedbackTypes) > 0 {
		tx.Where("feedback_type IN ?", feedbackTypes)
	}
	tx.Delete(&FeedbackKey{})
	if tx.Error != nil {
		return 0, errors.Trace(tx.Error)
	}
	if tx.Error != nil && d.driver != ClickHouse {
		return 0, errors.Trace(tx.Error)
	}
	return int(tx.RowsAffected), nil
}
