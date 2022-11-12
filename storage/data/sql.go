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
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/juju/errors"
	_ "github.com/lib/pq"
	_ "github.com/mailru/go-clickhouse/v2"
	"github.com/samber/lo"
	"github.com/scylladb/go-set/strset"
	_ "github.com/sijms/go-ora/v2"
	"github.com/zhenghaoz/gorse/base/json"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/storage"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	_ "modernc.org/sqlite"
	"time"
)

const bufSize = 1

type SQLDriver int

const (
	MySQL SQLDriver = iota
	Postgres
	ClickHouse
	SQLite
	Oracle
)

type SQLItem struct {
	ItemId     string    `gorm:"column:item_id;primaryKey"`
	IsHidden   bool      `gorm:"column:is_hidden"`
	Categories string    `gorm:"column:categories"`
	Timestamp  time.Time `gorm:"column:time_stamp"`
	Labels     string    `gorm:"column:labels"`
	Comment    string    `gorm:"column:comment"`
}

func NewSQLItem(item Item) (sqlItem SQLItem) {
	var buf []byte
	sqlItem.ItemId = item.ItemId
	sqlItem.IsHidden = item.IsHidden
	buf, _ = json.Marshal(item.Categories)
	sqlItem.Categories = string(buf)
	sqlItem.Timestamp = item.Timestamp
	buf, _ = json.Marshal(item.Labels)
	sqlItem.Labels = string(buf)
	sqlItem.Comment = item.Comment
	return
}

type SQLUser struct {
	UserId    string `gorm:"column:user_id;primaryKey"`
	Labels    string `gorm:"column:labels"`
	Subscribe string `gorm:"column:subscribe"`
	Comment   string `gorm:"column:comment"`
}

func NewSQLUser(user User) (sqlUser SQLUser) {
	var buf []byte
	sqlUser.UserId = user.UserId
	buf, _ = json.Marshal(user.Labels)
	sqlUser.Labels = string(buf)
	buf, _ = json.Marshal(user.Subscribe)
	sqlUser.Subscribe = string(buf)
	sqlUser.Comment = user.Comment
	return
}

type ClickHouseItem struct {
	SQLItem `gorm:"embedded"`
	Version time.Time `gorm:"column:version"`
}

func NewClickHouseItem(item Item) (clickHouseItem ClickHouseItem) {
	clickHouseItem.SQLItem = NewSQLItem(item)
	clickHouseItem.Timestamp = item.Timestamp.In(time.UTC)
	clickHouseItem.Version = time.Now().In(time.UTC)
	return
}

type ClickhouseUser struct {
	SQLUser `gorm:"embedded"`
	Version time.Time `gorm:"column:version"`
}

func NewClickhouseUser(user User) (clickhouseUser ClickhouseUser) {
	clickhouseUser.SQLUser = NewSQLUser(user)
	clickhouseUser.Version = time.Now().In(time.UTC)
	return
}

type ClickHouseFeedback struct {
	Feedback `gorm:"embedded"`
	Version  time.Time `gorm:"column:version"`
}

// SQLDatabase use MySQL as data storage.
type SQLDatabase struct {
	storage.TablePrefix
	gormDB *gorm.DB
	client *sql.DB
	driver SQLDriver
}

// Optimize is used by ClickHouse only.
func (d *SQLDatabase) Optimize() error {
	if d.driver == ClickHouse {
		for _, tableName := range []string{d.UsersTable(), d.ItemsTable(), d.FeedbackTable()} {
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
	case Postgres:
		// create tables
		type Items struct {
			ItemId     string    `gorm:"column:item_id;type:varchar(256);not null;primaryKey"`
			IsHidden   bool      `gorm:"column:is_hidden;type:bool;not null;default:false"`
			Categories string    `gorm:"column:categories;type:json;not null;default:'[]'"`
			Timestamp  time.Time `gorm:"column:time_stamp;type:timestamptz;not null"`
			Labels     string    `gorm:"column:labels;type:json;not null;default:'[]'"`
			Comment    string    `gorm:"column:comment;type:text;not null;default:''"`
		}
		type Users struct {
			UserId    string `gorm:"column:user_id;type:varchar(256) not null;primaryKey"`
			Labels    string `gorm:"column:labels;type:json;not null;default:'[]'"`
			Subscribe string `gorm:"column:subscribe;type:json;not null;default:'[]'"`
			Comment   string `gorm:"column:comment;type:text;not null;default:''"`
		}
		type Feedback struct {
			FeedbackType string    `gorm:"column:feedback_type;type:varchar(256);not null;primaryKey"`
			UserId       string    `gorm:"column:user_id;type:varchar(256);not null;primaryKey;index:user_id_index"`
			ItemId       string    `gorm:"column:item_id;type:varchar(256);not null;primaryKey;index:item_id_index"`
			Timestamp    time.Time `gorm:"column:time_stamp;type:timestamptz;not null"`
			Comment      string    `gorm:"column:comment;type:text;not null;default:''"`
		}
		err := d.gormDB.AutoMigrate(Users{}, Items{}, Feedback{})
		if err != nil {
			return errors.Trace(err)
		}
	case SQLite:
		// create tables
		type Items struct {
			ItemId     string `gorm:"column:item_id;type:varchar(256);not null;primaryKey"`
			IsHidden   bool   `gorm:"column:is_hidden;type:bool;not null;default:false"`
			Categories string `gorm:"column:categories;type:json;not null;default:'[]'"`
			Timestamp  string `gorm:"column:time_stamp;type:datetime;not null;default:'0001-01-01'"`
			Labels     string `gorm:"column:labels;type:json;not null;default:'[]'"`
			Comment    string `gorm:"column:comment;type:text;not null;default:''"`
		}
		type Users struct {
			UserId    string `gorm:"column:user_id;type:varchar(256) not null;primaryKey"`
			Labels    string `gorm:"column:labels;type:json;not null;default:'[]'"`
			Subscribe string `gorm:"column:subscribe;type:json;not null;default:'[]'"`
			Comment   string `gorm:"column:comment;type:text;not null;default:''"`
		}
		type Feedback struct {
			FeedbackType string `gorm:"column:feedback_type;type:varchar(256);not null;primaryKey"`
			UserId       string `gorm:"column:user_id;type:varchar(256);not null;primaryKey;index:user_id_index"`
			ItemId       string `gorm:"column:item_id;type:varchar(256);not null;primaryKey;index:item_id_index"`
			Timestamp    string `gorm:"column:time_stamp;type:datetime;not null;default:'0001-01-01'"`
			Comment      string `gorm:"column:comment;type:text;not null;default:''"`
		}
		err := d.gormDB.AutoMigrate(Users{}, Items{}, Feedback{})
		if err != nil {
			return errors.Trace(err)
		}
	case Oracle:
		// create tables
		type Items struct {
			ItemId     string    `gorm:"column:ITEM_ID;type:varchar2(256);not null;primaryKey"`
			IsHidden   bool      `gorm:"column:IS_HIDDEN;type:bool;not null"`
			Categories []string  `gorm:"column:CATEGORIES;type:varchar2(4000);not null"`
			Timestamp  time.Time `gorm:"column:TIME_STAMP;type:TIMESTAMP;not null"`
			Labels     []string  `gorm:"column:LABELS;type:varchar2(4000);not null"`
			Comment    string    `gorm:"column:\"COMMENT\";type:varchar2(4000)"`
		}
		type Users struct {
			UserId    string   `gorm:"column:USER_ID;type:varchar2(256);not null;primaryKey"`
			Labels    []string `gorm:"column:LABELS;type:varchar2(4000);not null"`
			Subscribe []string `gorm:"column:SUBSCRIBE;type:varchar2(4000);not null"`
			Comment   string   `gorm:"column:\"COMMENT\";type:varchar2(4000)"`
		}
		type Feedback struct {
			FeedbackType string    `gorm:"column:FEEDBACK_TYPE;type:varchar2(256);not null;primaryKey"`
			UserId       string    `gorm:"column:USER_ID;type:varchar2(256);not null;primaryKey;index:user_id_index"`
			ItemId       string    `gorm:"column:ITEM_ID;type:varchar2(256);not null;primaryKey;index:item_id_index"`
			Timestamp    time.Time `gorm:"column:TIME_STAMP;type:TIMESTAMP;not null"`
			Comment      string    `gorm:"column:\"COMMENT\";type:varchar2(4000)"`
		}
		err := d.gormDB.AutoMigrate(Users{}, Items{}, Feedback{})
		if err != nil {
			return errors.Trace(err)
		}
	case ClickHouse:
		// create tables
		type Items struct {
			ItemId     string    `gorm:"column:item_id;type:String"`
			IsHidden   int       `gorm:"column:is_hidden;type:Boolean;default:0"`
			Categories string    `gorm:"column:categories;type:String;default:'[]'"`
			Timestamp  time.Time `gorm:"column:time_stamp;type:Datetime"`
			Labels     string    `gorm:"column:labels;type:String;default:'[]'"`
			Comment    string    `gorm:"column:comment;type:String"`
			Version    struct{}  `gorm:"column:version;type:DateTime"`
		}
		err := d.gormDB.Set("gorm:table_options", "ENGINE = ReplacingMergeTree(version) ORDER BY item_id").AutoMigrate(Items{})
		if err != nil {
			return errors.Trace(err)
		}
		type Users struct {
			UserId    string   `gorm:"column:user_id;type:String"`
			Labels    string   `gorm:"column:labels;type:String;default:'[]'"`
			Subscribe string   `gorm:"column:subscribe;type:String;default:'[]'"`
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

func (d *SQLDatabase) Purge() error {
	tables := []string{d.ItemsTable(), d.FeedbackTable(), d.UsersTable()}
	if d.driver == ClickHouse {
		for _, tableName := range tables {
			err := d.gormDB.Exec(fmt.Sprintf("alter table %s delete where 1=1", tableName)).Error
			if err != nil {
				return errors.Trace(err)
			}
		}
	} else {
		for _, tableName := range tables {
			err := d.gormDB.Exec(fmt.Sprintf("DELETE FROM %s", tableName)).Error
			if err != nil {
				return errors.Trace(err)
			}
		}
	}
	return nil
}

// BatchInsertItems inserts a batch of items into MySQL.
func (d *SQLDatabase) BatchInsertItems(ctx context.Context, items []Item) error {
	if len(items) == 0 {
		return nil
	}
	if d.driver == ClickHouse {
		rows := make([]ClickHouseItem, 0, len(items))
		memo := strset.New()
		for _, item := range items {
			if !memo.Has(item.ItemId) {
				memo.Add(item.ItemId)
				rows = append(rows, NewClickHouseItem(item))
			}
		}
		err := d.gormDB.Create(rows).Error
		return errors.Trace(err)
	} else {
		rows := make([]SQLItem, 0, len(items))
		memo := strset.New()
		for _, item := range items {
			if !memo.Has(item.ItemId) {
				memo.Add(item.ItemId)
				row := NewSQLItem(item)
				if d.driver == SQLite || d.driver == Oracle {
					row.Timestamp = row.Timestamp.In(time.UTC)
				}
				rows = append(rows, row)
			}
		}
		err := d.gormDB.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "item_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"is_hidden", "categories", "time_stamp", "labels", "comment"}),
		}).Create(rows).Error
		return errors.Trace(err)
	}
}

func (d *SQLDatabase) BatchGetItems(ctx context.Context, itemIds []string) ([]Item, error) {
	if len(itemIds) == 0 {
		return nil, nil
	}
	result, err := d.gormDB.WithContext(ctx).Table(d.ItemsTable()).
		Select("item_id, is_hidden, categories, time_stamp, labels, comment").
		Where("item_id IN ?", itemIds).Rows()
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
func (d *SQLDatabase) DeleteItem(ctx context.Context, itemId string) error {
	if err := d.gormDB.WithContext(ctx).Delete(&SQLItem{ItemId: itemId}).Error; err != nil {
		return errors.Trace(err)
	}
	if err := d.gormDB.WithContext(ctx).Delete(&Feedback{}, "item_id = ?", itemId).Error; err != nil {
		return errors.Trace(err)
	}
	return nil
}

// GetItem get a item from MySQL.
func (d *SQLDatabase) GetItem(ctx context.Context, itemId string) (Item, error) {
	var result *sql.Rows
	var err error
	result, err = d.gormDB.WithContext(ctx).Table(d.ItemsTable()).Select("item_id, is_hidden, categories, time_stamp, labels, comment").Where("item_id = ?", itemId).Rows()
	if err != nil {
		return Item{}, errors.Trace(err)
	}
	defer result.Close()
	if result.Next() {
		var item Item
		var labels, categories string
		var comment sql.NullString
		if err := result.Scan(&item.ItemId, &item.IsHidden, &categories, &item.Timestamp, &labels, &comment); err != nil {
			return Item{}, errors.Trace(err)
		}
		if err := json.Unmarshal([]byte(labels), &item.Labels); err != nil {
			return Item{}, err
		}
		if err := json.Unmarshal([]byte(categories), &item.Categories); err != nil {
			return Item{}, err
		}
		item.Comment = comment.String
		return item, nil
	}
	return Item{}, errors.Annotate(ErrItemNotExist, itemId)
}

// ModifyItem modify an item in MySQL.
func (d *SQLDatabase) ModifyItem(ctx context.Context, itemId string, patch ItemPatch) error {
	// ignore empty patch
	if patch.IsHidden == nil && patch.Categories == nil && patch.Labels == nil && patch.Comment == nil && patch.Timestamp == nil {
		log.Logger().Debug("empty item patch")
		return nil
	}
	attributes := make(map[string]any)
	if patch.IsHidden != nil {
		if *patch.IsHidden {
			attributes["is_hidden"] = 1
		} else {
			attributes["is_hidden"] = 0
		}
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
		switch d.driver {
		case ClickHouse, SQLite, Oracle:
			attributes["time_stamp"] = patch.Timestamp.In(time.UTC)
		default:
			attributes["time_stamp"] = patch.Timestamp
		}
	}
	err := d.gormDB.WithContext(ctx).Model(&SQLItem{ItemId: itemId}).Updates(attributes).Error
	return errors.Trace(err)
}

// GetItems returns items from MySQL.
func (d *SQLDatabase) GetItems(ctx context.Context, cursor string, n int, timeLimit *time.Time) (string, []Item, error) {
	tx := d.gormDB.WithContext(ctx).Table(d.ItemsTable()).Select("item_id, is_hidden, categories, time_stamp, labels, comment")
	if cursor != "" {
		tx.Where("item_id >= ?", cursor)
	}
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
		var comment sql.NullString
		if err = result.Scan(&item.ItemId, &item.IsHidden, &categories, &item.Timestamp, &labels, &comment); err != nil {
			return "", nil, errors.Trace(err)
		}
		if err = json.Unmarshal([]byte(labels), &item.Labels); err != nil {
			return "", nil, errors.Trace(err)
		}
		if err = json.Unmarshal([]byte(categories), &item.Categories); err != nil {
			return "", nil, errors.Trace(err)
		}
		item.Comment = comment.String
		items = append(items, item)
	}
	if len(items) == n+1 {
		return items[len(items)-1].ItemId, items[:len(items)-1], nil
	}
	return "", items, nil
}

// GetItemStream reads items by stream.
func (d *SQLDatabase) GetItemStream(ctx context.Context, batchSize int, timeLimit *time.Time) (chan []Item, chan error) {
	itemChan := make(chan []Item, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(itemChan)
		defer close(errChan)
		// send query
		tx := d.gormDB.WithContext(ctx).Table(d.ItemsTable()).Select("item_id, is_hidden, categories, time_stamp, labels, comment")
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
func (d *SQLDatabase) GetItemFeedback(ctx context.Context, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	tx := d.gormDB.WithContext(ctx).Table(d.FeedbackTable()).Select("user_id, item_id, feedback_type, time_stamp")
	switch d.driver {
	case SQLite:
		tx.Where("time_stamp <= DATETIME() AND item_id = ?", itemId)
	case Oracle:
		tx.Where("time_stamp <= SYS_EXTRACT_UTC(SYSTIMESTAMP) AND item_id = ?", itemId)
	default:
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
func (d *SQLDatabase) BatchInsertUsers(ctx context.Context, users []User) error {
	if len(users) == 0 {
		return nil
	}
	if d.driver == ClickHouse {
		rows := make([]ClickhouseUser, 0, len(users))
		memo := strset.New()
		for _, user := range users {
			if !memo.Has(user.UserId) {
				memo.Add(user.UserId)
				rows = append(rows, NewClickhouseUser(user))
			}
		}
		err := d.gormDB.Create(rows).Error
		return errors.Trace(err)
	} else {
		rows := make([]SQLUser, 0, len(users))
		memo := strset.New()
		for _, user := range users {
			if !memo.Has(user.UserId) {
				memo.Add(user.UserId)
				rows = append(rows, NewSQLUser(user))
			}
		}
		err := d.gormDB.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"labels", "subscribe", "comment"}),
		}).Create(rows).Error
		return errors.Trace(err)
	}
}

// DeleteUser deletes a user from MySQL.
func (d *SQLDatabase) DeleteUser(ctx context.Context, userId string) error {
	if err := d.gormDB.WithContext(ctx).Delete(&SQLUser{UserId: userId}).Error; err != nil {
		return errors.Trace(err)
	}
	if err := d.gormDB.WithContext(ctx).Delete(&Feedback{}, "user_id = ?", userId).Error; err != nil {
		return errors.Trace(err)
	}
	return nil
}

// GetUser returns a user from MySQL.
func (d *SQLDatabase) GetUser(ctx context.Context, userId string) (User, error) {
	var result *sql.Rows
	var err error
	result, err = d.gormDB.WithContext(ctx).Table(d.UsersTable()).
		Select("user_id, labels, subscribe, comment").
		Where("user_id = ?", userId).Rows()
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
func (d *SQLDatabase) ModifyUser(ctx context.Context, userId string, patch UserPatch) error {
	// ignore empty patch
	if patch.Labels == nil && patch.Subscribe == nil && patch.Comment == nil {
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
	if patch.Subscribe != nil {
		text, _ := json.Marshal(patch.Subscribe)
		attributes["subscribe"] = string(text)
	}
	err := d.gormDB.WithContext(ctx).Model(&SQLUser{UserId: userId}).Updates(attributes).Error
	return errors.Trace(err)
}

// GetUsers returns users from MySQL.
func (d *SQLDatabase) GetUsers(ctx context.Context, cursor string, n int) (string, []User, error) {
	tx := d.gormDB.WithContext(ctx).Table(d.UsersTable()).Select("user_id, labels, subscribe, comment")
	if cursor != "" {
		tx.Where("user_id >= ?", cursor)
	}
	result, err := tx.Order("user_id").Limit(n + 1).Rows()
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	users := make([]User, 0)
	defer result.Close()
	for result.Next() {
		var user User
		var labels, subscribe string
		var comment sql.NullString
		if err = result.Scan(&user.UserId, &labels, &subscribe, &comment); err != nil {
			return "", nil, errors.Trace(err)
		}
		if err = json.Unmarshal([]byte(labels), &user.Labels); err != nil {
			return "", nil, errors.Trace(err)
		}
		if err = json.Unmarshal([]byte(subscribe), &user.Subscribe); err != nil {
			return "", nil, errors.Trace(err)
		}
		user.Comment = comment.String
		users = append(users, user)
	}
	if len(users) == n+1 {
		return users[len(users)-1].UserId, users[:len(users)-1], nil
	}
	return "", users, nil
}

// GetUserStream read users by stream.
func (d *SQLDatabase) GetUserStream(ctx context.Context, batchSize int) (chan []User, chan error) {
	userChan := make(chan []User, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(userChan)
		defer close(errChan)
		// send query
		result, err := d.gormDB.WithContext(ctx).Table(d.UsersTable()).Select("user_id, labels, subscribe, comment").Rows()
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
func (d *SQLDatabase) GetUserFeedback(ctx context.Context, userId string, endTime *time.Time, feedbackTypes ...string) ([]Feedback, error) {
	tx := d.gormDB.WithContext(ctx).Table(d.FeedbackTable()).
		Select("feedback_type, user_id, item_id, time_stamp, comment").
		Where("user_id = ?", userId)
	if endTime != nil {
		tx.Where("time_stamp <= ?", d.convertTimeZone(endTime))
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
		var comment sql.NullString
		if err = result.Scan(&feedback.FeedbackType, &feedback.UserId, &feedback.ItemId, &feedback.Timestamp, &comment); err != nil {
			return nil, errors.Trace(err)
		}
		feedback.Comment = comment.String
		feedbacks = append(feedbacks, feedback)
	}
	return feedbacks, nil
}

// BatchInsertFeedback insert a batch feedback into MySQL.
// If insertUser set, new users will be inserted to user table.
// If insertItem set, new items will be inserted to item table.
func (d *SQLDatabase) BatchInsertFeedback(ctx context.Context, feedback []Feedback, insertUser, insertItem, overwrite bool) error {
	tx := d.gormDB.WithContext(ctx)
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
		if d.driver == ClickHouse {
			err := tx.Create(lo.Map(userList, func(userId string, _ int) ClickhouseUser {
				return ClickhouseUser{
					SQLUser: SQLUser{
						UserId:    userId,
						Labels:    "[]",
						Subscribe: "[]",
					},
				}
			})).Error
			if err != nil {
				return errors.Trace(err)
			}
		} else {
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "user_id"}},
				DoNothing: true,
			}).Create(lo.Map(userList, func(userId string, _ int) SQLUser {
				return SQLUser{
					UserId:    userId,
					Labels:    "[]",
					Subscribe: "[]",
				}
			})).Error
			if err != nil {
				return errors.Trace(err)
			}
		}
	} else {
		for _, user := range users.List() {
			rs, err := tx.Table(d.UsersTable()).Select("user_id").Where("user_id = ?", user).Rows()
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
		if d.driver == ClickHouse {
			err := tx.Create(lo.Map(itemList, func(itemId string, _ int) ClickHouseItem {
				return ClickHouseItem{
					SQLItem: SQLItem{
						ItemId:     itemId,
						Labels:     "[]",
						Categories: "[]",
					},
				}
			})).Error
			if err != nil {
				return errors.Trace(err)
			}
		} else {
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "item_id"}},
				DoNothing: true,
			}).Create(lo.Map(itemList, func(itemId string, _ int) SQLItem {
				return SQLItem{
					ItemId:     itemId,
					Labels:     "[]",
					Categories: "[]",
				}
			})).Error
			if err != nil {
				return errors.Trace(err)
			}
		}
	} else {
		for _, item := range items.List() {
			rs, err := tx.Table(d.ItemsTable()).Select("item_id").Where("item_id = ?", item).Rows()
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
	if d.driver == ClickHouse {
		rows := make([]ClickHouseFeedback, 0, len(feedback))
		memo := make(map[lo.Tuple3[string, string, string]]struct{})
		for _, f := range feedback {
			if users.Has(f.UserId) && items.Has(f.ItemId) {
				if _, exist := memo[lo.Tuple3[string, string, string]{f.FeedbackType, f.UserId, f.ItemId}]; !exist {
					memo[lo.Tuple3[string, string, string]{f.FeedbackType, f.UserId, f.ItemId}] = struct{}{}
					f.Timestamp = f.Timestamp.In(time.UTC)
					rows = append(rows, ClickHouseFeedback{
						Feedback: f,
						Version:  lo.If(overwrite, time.Now().In(time.UTC)).Else(time.Time{}),
					})
				}
			}
		}
		if len(rows) == 0 {
			return nil
		}
		err := tx.Create(rows).Error
		return errors.Trace(err)
	} else {
		rows := make([]Feedback, 0, len(feedback))
		memo := make(map[lo.Tuple3[string, string, string]]struct{})
		for _, f := range feedback {
			if users.Has(f.UserId) && items.Has(f.ItemId) {
				if _, exist := memo[lo.Tuple3[string, string, string]{f.FeedbackType, f.UserId, f.ItemId}]; !exist {
					memo[lo.Tuple3[string, string, string]{f.FeedbackType, f.UserId, f.ItemId}] = struct{}{}
					if d.driver == SQLite || d.driver == Oracle {
						f.Timestamp = f.Timestamp.In(time.UTC)
					}
					rows = append(rows, f)
				}
			}
		}
		if len(rows) == 0 {
			return nil
		}
		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "feedback_type"}, {Name: "user_id"}, {Name: "item_id"}},
			DoNothing: !overwrite,
			DoUpdates: lo.If(overwrite, clause.AssignmentColumns([]string{"time_stamp", "comment"})).Else(nil),
		}).Create(rows).Error
		return errors.Trace(err)
	}
}

// GetFeedback returns feedback from MySQL.
func (d *SQLDatabase) GetFeedback(ctx context.Context, cursor string, n int, beginTime, endTime *time.Time, feedbackTypes ...string) (string, []Feedback, error) {
	tx := d.gormDB.WithContext(ctx).Table(d.FeedbackTable()).Select("feedback_type, user_id, item_id, time_stamp, comment")
	if cursor != "" {
		var cursorKey FeedbackKey
		if err := json.Unmarshal([]byte(cursor), &cursorKey); err != nil {
			return "", nil, err
		}
		if d.driver == Oracle {
			tx.Where("feedback_type > ? OR feedback_type = ? AND user_id > ? OR feedback_type = ? AND user_id = ? AND item_id >= ?",
				cursorKey.FeedbackType,
				cursorKey.FeedbackType, cursorKey.UserId,
				cursorKey.FeedbackType, cursorKey.UserId, cursorKey.ItemId)
		} else {
			tx.Where("(feedback_type, user_id, item_id) >= (?,?,?)", cursorKey.FeedbackType, cursorKey.UserId, cursorKey.ItemId)
		}
	}
	if len(feedbackTypes) > 0 {
		tx.Where("feedback_type IN ?", feedbackTypes)
	}
	if beginTime != nil {
		tx.Where("time_stamp >= ?", d.convertTimeZone(beginTime))
	}
	if endTime != nil {
		tx.Where("time_stamp <= ?", d.convertTimeZone(endTime))
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
		var comment sql.NullString
		if err = result.Scan(&feedback.FeedbackType, &feedback.UserId, &feedback.ItemId, &feedback.Timestamp, &comment); err != nil {
			return "", nil, errors.Trace(err)
		}
		feedback.Comment = comment.String
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
func (d *SQLDatabase) GetFeedbackStream(ctx context.Context, batchSize int, beginTime, endTime *time.Time, feedbackTypes ...string) (chan []Feedback, chan error) {
	feedbackChan := make(chan []Feedback, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(feedbackChan)
		defer close(errChan)
		// send query
		tx := d.gormDB.WithContext(ctx).Table(d.FeedbackTable()).Select("feedback_type, user_id, item_id, time_stamp, comment")
		if len(feedbackTypes) > 0 {
			tx.Where("feedback_type IN ?", feedbackTypes)
		}
		if beginTime != nil {
			tx.Where("time_stamp >= ?", d.convertTimeZone(beginTime))
		}
		if endTime != nil {
			tx.Where("time_stamp <= ?", d.convertTimeZone(endTime))
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
			var comment sql.NullString
			if err = result.Scan(&feedback.FeedbackType, &feedback.UserId, &feedback.ItemId, &feedback.Timestamp, &comment); err != nil {
				errChan <- errors.Trace(err)
				return
			}
			feedback.Comment = comment.String
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
func (d *SQLDatabase) GetUserItemFeedback(ctx context.Context, userId, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	tx := d.gormDB.WithContext(ctx).Table(d.FeedbackTable()).
		Select("feedback_type, user_id, item_id, time_stamp, comment").
		Where("user_id = ? AND item_id = ?", userId, itemId)
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
		var comment sql.NullString
		if err = result.Scan(&feedback.FeedbackType, &feedback.UserId, &feedback.ItemId, &feedback.Timestamp, &comment); err != nil {
			return nil, errors.Trace(err)
		}
		feedback.Comment = comment.String
		feedbacks = append(feedbacks, feedback)
	}
	return feedbacks, nil
}

// DeleteUserItemFeedback deletes a feedback by user id and item id from MySQL.
func (d *SQLDatabase) DeleteUserItemFeedback(ctx context.Context, userId, itemId string, feedbackTypes ...string) (int, error) {
	tx := d.gormDB.WithContext(ctx).Where("user_id = ? AND item_id = ?", userId, itemId)
	if len(feedbackTypes) > 0 {
		tx.Where("feedback_type IN ?", feedbackTypes)
	}
	tx.Delete(&Feedback{})
	if tx.Error != nil {
		return 0, errors.Trace(tx.Error)
	}
	if tx.Error != nil && d.driver != ClickHouse {
		return 0, errors.Trace(tx.Error)
	}
	return int(tx.RowsAffected), nil
}

func (d *SQLDatabase) convertTimeZone(timestamp *time.Time) time.Time {
	switch d.driver {
	case ClickHouse, SQLite, Oracle:
		return timestamp.In(time.UTC)
	default:
		return *timestamp
	}
}
