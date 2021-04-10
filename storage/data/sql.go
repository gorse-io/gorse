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
	"database/sql"
	"encoding/json"
	"errors"
	_ "github.com/go-sql-driver/mysql"
	"github.com/zhenghaoz/gorse/base"
	"go.uber.org/zap"
	"time"
)

type SQLDatabase struct {
	db *sql.DB
}

func (d *SQLDatabase) Init() error {
	// create tables
	if _, err := d.db.Exec("CREATE TABLE IF NOT EXISTS items (" +
		"item_id varchar(256) NOT NULL," +
		"time_stamp timestamp NOT NULL," +
		"labels json NOT NULL," +
		"comment TEXT NOT NULL," +
		"PRIMARY KEY(item_id)" +
		")"); err != nil {
		return err
	}
	if _, err := d.db.Exec("CREATE TABLE IF NOT EXISTS users (" +
		"user_id varchar(256) NOT NULL," +
		"labels json NOT NULL," +
		"subscribe json NOT NULL," +
		"comment TEXT NOT NULL," +
		"PRIMARY KEY (user_id)" +
		")"); err != nil {
		return err
	}
	if _, err := d.db.Exec("CREATE TABLE IF NOT EXISTS feedback (" +
		"feedback_type varchar(256) NOT NULL," +
		"user_id varchar(256) NOT NULL," +
		"item_id varchar(256) NOT NULL," +
		"time_stamp timestamp NOT NULL," +
		"comment TEXT NOT NULL," +
		"PRIMARY KEY(feedback_type, user_id, item_id)" +
		")"); err != nil {
		return err
	}
	// change settings
	_, err := d.db.Exec("SET SESSION sql_mode=\"" +
		"ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO," +
		"NO_ENGINE_SUBSTITUTION\"")
	return err
}

func (d *SQLDatabase) Close() error {
	return d.db.Close()
}

func (d *SQLDatabase) InsertItem(item Item) error {
	startTime := time.Now()
	labels, err := json.Marshal(item.Labels)
	if err != nil {
		return err
	}
	_, err = d.db.Exec("INSERT items(item_id, time_stamp, labels, `comment`) VALUES (?, ?, ?, ?) "+
		"ON DUPLICATE KEY UPDATE time_stamp = ?, labels = ?, `comment` = ?",
		item.ItemId, item.Timestamp, labels, item.Comment, item.Timestamp, labels, item.Comment)
	InsertItemLatency.Observe(time.Since(startTime).Seconds())
	return err
}

func (d *SQLDatabase) BatchInsertItem(items []Item) error {
	for _, item := range items {
		if err := d.InsertItem(item); err != nil {
			return err
		}
	}
	return nil
}

func (d *SQLDatabase) DeleteItem(itemId string) error {
	txn, err := d.db.Begin()
	if err != nil {
		return err
	}
	_, err = txn.Exec("DELETE FROM items WHERE item_id = ?", itemId)
	if err != nil {
		txn.Rollback()
		return err
	}
	_, err = txn.Exec("DELETE FROM feedback WHERE item_id = ?", itemId)
	if err != nil {
		txn.Rollback()
		return err
	}
	return txn.Commit()
}

func (d *SQLDatabase) GetItem(itemId string) (Item, error) {
	startTime := time.Now()
	result, err := d.db.Query("SELECT item_id, time_stamp, labels, `comment` FROM items WHERE item_id = ?", itemId)
	if err != nil {
		return Item{}, err
	}
	defer result.Close()
	if result.Next() {
		var item Item
		var labels string
		if err := result.Scan(&item.ItemId, &item.Timestamp, &labels, &item.Comment); err != nil {
			return Item{}, err
		}
		if err := json.Unmarshal([]byte(labels), &item.Labels); err != nil {
			return Item{}, err
		}
		GetItemLatency.Observe(time.Since(startTime).Seconds())
		return item, nil
	}
	return Item{}, errors.New(ErrItemNotExist)
}

func (d *SQLDatabase) GetItems(cursor string, n int) (string, []Item, error) {
	result, err := d.db.Query("SELECT item_id, time_stamp, labels, `comment` FROM items "+
		"WHERE item_id >= ? ORDER BY item_id LIMIT ?", cursor, n+1)
	if err != nil {
		return "", nil, err
	}
	items := make([]Item, 0)
	defer result.Close()
	for result.Next() {
		var item Item
		var labels string
		if err := result.Scan(&item.ItemId, &item.Timestamp, &labels, &item.Comment); err != nil {
			return "", nil, err
		}
		if err := json.Unmarshal([]byte(labels), &item.Labels); err != nil {
			return "", nil, err
		}
		items = append(items, item)
	}
	if len(items) == n+1 {
		return items[len(items)-1].ItemId, items[:len(items)-1], nil
	}
	return "", items, nil
}

func (d *SQLDatabase) GetItemFeedback(itemId string, feedbackType *string) ([]Feedback, error) {
	startTime := time.Now()
	var result *sql.Rows
	var err error
	if feedbackType != nil {
		result, err = d.db.Query("SELECT user_id, item_id, feedback_type FROM feedback "+
			"WHERE item_id = ? AND feedback_type = ?",
			itemId, *feedbackType)
	} else {
		result, err = d.db.Query("SELECT user_id, item_id, feedback_type FROM feedback WHERE item_id = ?", itemId)
	}
	if err != nil {
		return nil, err
	}
	feedbacks := make([]Feedback, 0)
	for result.Next() {
		var feedback Feedback
		if err := result.Scan(&feedback.UserId, &feedback.ItemId, &feedback.FeedbackType); err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	GetItemFeedbackLatency.Observe(time.Since(startTime).Seconds())
	return feedbacks, nil
}

func (d *SQLDatabase) InsertUser(user User) error {
	labels, err := json.Marshal(user.Labels)
	if err != nil {
		return err
	}
	subscribe, err := json.Marshal(user.Subscribe)
	if err != nil {
		return err
	}
	_, err = d.db.Exec("INSERT users(user_id, labels, subscribe, `comment`) VALUES (?, ?, ?, ?) "+
		"ON DUPLICATE KEY UPDATE labels = ?, subscribe = ?, `comment` = ?",
		user.UserId, labels, subscribe, user.Comment, labels, subscribe, user.Comment)
	return err
}

func (d *SQLDatabase) DeleteUser(userId string) error {
	txn, err := d.db.Begin()
	if err != nil {
		return err
	}
	_, err = txn.Exec("DELETE FROM users WHERE user_id = ?", userId)
	if err != nil {
		txn.Rollback()
		return err
	}
	_, err = txn.Exec("DELETE FROM feedback WHERE user_id = ?", userId)
	if err != nil {
		txn.Rollback()
		return err
	}
	return txn.Commit()
}

func (d *SQLDatabase) GetUser(userId string) (User, error) {
	result, err := d.db.Query("SELECT user_id, labels, subscribe, `comment` FROM users WHERE user_id = ?", userId)
	if err != nil {
		return User{}, err
	}
	defer result.Close()
	if result.Next() {
		var user User
		var labels string
		var subscribe string
		if err := result.Scan(&user.UserId, &labels, &subscribe, &user.Comment); err != nil {
			return User{}, err
		}
		if err := json.Unmarshal([]byte(labels), &user.Labels); err != nil {
			return User{}, err
		}
		if err = json.Unmarshal([]byte(subscribe), &user.Subscribe); err != nil {
			return User{}, err
		}
		return user, nil
	}
	return User{}, errors.New(ErrUserNotExist)
}

func (d *SQLDatabase) GetUsers(cursor string, n int) (string, []User, error) {
	result, err := d.db.Query("SELECT user_id, labels, subscribe, `comment` FROM users "+
		"WHERE user_id >= ? ORDER BY user_id LIMIT ?", cursor, n+1)
	if err != nil {
		return "", nil, err
	}
	users := make([]User, 0)
	defer result.Close()
	for result.Next() {
		var user User
		var labels string
		var subscribe string
		if err := result.Scan(&user.UserId, &labels, &subscribe, &user.Comment); err != nil {
			return "", nil, err
		}
		if err := json.Unmarshal([]byte(labels), &user.Labels); err != nil {
			return "", nil, err
		}
		if err := json.Unmarshal([]byte(subscribe), &user.Subscribe); err != nil {
			return "", nil, err
		}
		users = append(users, user)
	}
	if len(users) == n+1 {
		return users[len(users)-1].UserId, users[:len(users)-1], nil
	}
	return "", users, nil
}

func (d *SQLDatabase) GetUserFeedback(userId string, feedbackType *string) ([]Feedback, error) {
	startTime := time.Now()
	var result *sql.Rows
	var err error
	if feedbackType != nil {
		result, err = d.db.Query("SELECT feedback_type, user_id, item_id, time_stamp, `comment` "+
			"FROM feedback WHERE user_id = ? AND feedback_type = ?", userId, *feedbackType)
	} else {
		result, err = d.db.Query("SELECT feedback_type, user_id, item_id, time_stamp, `comment` "+
			"FROM feedback WHERE user_id = ?", userId)
	}
	if err != nil {
		return nil, err
	}
	feedbacks := make([]Feedback, 0)
	for result.Next() {
		var feedback Feedback
		if err := result.Scan(&feedback.FeedbackType, &feedback.UserId, &feedback.ItemId, &feedback.Timestamp, &feedback.Comment); err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	GetUserFeedbackLatency.Observe(time.Since(startTime).Seconds())
	return feedbacks, nil
}

func (d *SQLDatabase) InsertFeedback(feedback Feedback, insertUser, insertItem bool) error {
	startTime := time.Now()
	// insert users
	if insertUser {
		_, err := d.db.Exec("INSERT IGNORE users(user_id) VALUES (?)", feedback.UserId)
		if err != nil {
			return err
		}
	} else {
		if _, err := d.GetUser(feedback.UserId); err != nil {
			if err.Error() == ErrUserNotExist {
				base.Logger().Warn("user doesn't exist", zap.String("user_id", feedback.UserId))
				return nil
			}
			return err
		}
	}
	// insert items
	if insertItem {
		_, err := d.db.Exec("INSERT IGNORE items(item_id) VALUES (?)", feedback.ItemId)
		if err != nil {
			return err
		}
	} else {
		if _, err := d.GetItem(feedback.ItemId); err != nil {
			if err.Error() == ErrItemNotExist {
				base.Logger().Warn("item doesn't exist", zap.String("item_id", feedback.ItemId))
				return nil
			}
			return err
		}
	}
	// insert feedback
	_, err := d.db.Exec("INSERT feedback(feedback_type, user_id, item_id, time_stamp, `comment`) VALUES (?,?,?,?,?) "+
		"ON DUPLICATE KEY UPDATE time_stamp = ?, `comment` = ?",
		feedback.FeedbackType, feedback.UserId, feedback.ItemId, feedback.Timestamp, feedback.Comment, feedback.Timestamp, feedback.Comment)
	InsertFeedbackLatency.Observe(time.Since(startTime).Seconds())
	return err
}

func (d *SQLDatabase) BatchInsertFeedback(feedback []Feedback, insertUser, insertItem bool) error {
	for _, f := range feedback {
		if err := d.InsertFeedback(f, insertUser, insertItem); err != nil {
			return err
		}
	}
	return nil
}

func (d *SQLDatabase) GetFeedback(cursor string, n int, feedbackType *string) (string, []Feedback, error) {
	var cursorKey FeedbackKey
	if cursor != "" {
		if err := json.Unmarshal([]byte(cursor), &cursorKey); err != nil {
			return "", nil, err
		}
	}
	var result *sql.Rows
	var err error
	if feedbackType != nil {
		result, err = d.db.Query("SELECT feedback_type, user_id, item_id, time_stamp, `comment` FROM feedback "+
			"WHERE feedback_type = ? AND user_id >= ? AND item_id >= ? ORDER BY feedback_type, user_id, item_id LIMIT ?",
			*feedbackType, cursorKey.UserId, cursorKey.ItemId, n+1)
	} else {
		result, err = d.db.Query("SELECT feedback_type, user_id, item_id, time_stamp, `comment` FROM feedback "+
			"WHERE feedback_type >= ? AND user_id >= ? AND item_id >= ? ORDER BY feedback_type, user_id, item_id LIMIT ?",
			cursorKey.FeedbackType, cursorKey.UserId, cursorKey.ItemId, n+1)
	}
	if err != nil {
		return "", nil, err
	}
	feedbacks := make([]Feedback, 0)
	for result.Next() {
		var feedback Feedback
		if err := result.Scan(&feedback.FeedbackType, &feedback.UserId, &feedback.ItemId, &feedback.Timestamp, &feedback.Comment); err != nil {
			return "", nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	if len(feedbacks) == n+1 {
		nextCursorKey := feedbacks[len(feedbacks)-1].FeedbackKey
		nextCursor, err := json.Marshal(nextCursorKey)
		if err != nil {
			return "", nil, err
		}
		return string(nextCursor), feedbacks[:len(feedbacks)-1], nil
	}
	return "", feedbacks, nil
}

func (d *SQLDatabase) GetUserItemFeedback(userId, itemId string, feedbackType *string) ([]Feedback, error) {
	startTime := time.Now()
	var result *sql.Rows
	var err error
	if feedbackType != nil {
		result, err = d.db.Query("SELECT feedback_type, user_id, item_id, time_stamp, `comment` FROM feedback "+
			"WHERE feedback_type = ? AND user_id = ? AND item_id = ?", *feedbackType, userId, itemId)
	} else {
		result, err = d.db.Query("SELECT feedback_type, user_id, item_id, time_stamp, `comment` FROM feedback "+
			"WHERE user_id = ? AND item_id = ?", userId, itemId)
	}
	if err != nil {
		return nil, err
	}
	feedbacks := make([]Feedback, 0)
	for result.Next() {
		var feedback Feedback
		if err = result.Scan(&feedback.FeedbackType, &feedback.UserId, &feedback.ItemId, &feedback.Timestamp, &feedback.Comment); err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	GetUserItemFeedbackLatency.Observe(time.Since(startTime).Seconds())
	return feedbacks, nil
}

func (d *SQLDatabase) DeleteUserItemFeedback(userId, itemId string, feedbackType *string) (int, error) {
	var rs sql.Result
	var err error
	if feedbackType != nil {
		rs, err = d.db.Exec("DELETE FROM feedback WHERE feedback_type = ? AND user_id = ? AND item_id >= ?",
			*feedbackType, userId, itemId)
	} else {
		rs, err = d.db.Exec("DELETE FROM feedback WHERE user_id = ? AND item_id = ?", userId, itemId)
	}
	if err != nil {
		return 0, err
	}
	deleteCount, err := rs.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(deleteCount), nil
}
