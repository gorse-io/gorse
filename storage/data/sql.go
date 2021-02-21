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
		"PRIMARY KEY(item_id)" +
		")"); err != nil {
		return err
	}
	if _, err := d.db.Exec("CREATE TABLE IF NOT EXISTS users (" +
		"user_id varchar(256) NOT NULL," +
		"labels json NOT NULL," +
		"PRIMARY KEY (user_id)" +
		")"); err != nil {
		return err
	}
	if _, err := d.db.Exec("CREATE TABLE IF NOT EXISTS feedback (" +
		"feedback_type varchar(256) NOT NULL," +
		"user_id varchar(256) NOT NULL," +
		"item_id varchar(256) NOT NULL," +
		"time_stamp timestamp NOT NULL," +
		"PRIMARY KEY(feedback_type, user_id, item_id)" +
		")"); err != nil {
		return err
	}
	// change settings
	_, err := d.db.Exec("SET GLOBAL sql_mode=\"" +
		"ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO," +
		"NO_ENGINE_SUBSTITUTION\"")
	return err
}

func (d *SQLDatabase) Close() error {
	return d.db.Close()
}

func (d *SQLDatabase) InsertItem(item Item) error {
	labels, err := json.Marshal(item.Labels)
	if err != nil {
		return err
	}
	_, err = d.db.Exec("INSERT IGNORE items(item_id, time_stamp, labels) VALUES (?, ?, ?)",
		item.ItemId, item.Timestamp, labels)
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
	result, err := d.db.Query("SELECT item_id, time_stamp, labels FROM items WHERE item_id = ?", itemId)
	if err != nil {
		return Item{}, err
	}
	defer result.Close()
	if result.Next() {
		var item Item
		var labels string
		if err := result.Scan(&item.ItemId, &item.Timestamp, &labels); err != nil {
			return Item{}, err
		}
		if err := json.Unmarshal([]byte(labels), &item.Labels); err != nil {
			return Item{}, err
		}
		return item, nil
	}
	return Item{}, errors.New(ErrItemNotExist)
}

func (d *SQLDatabase) GetItems(cursor string, n int) (string, []Item, error) {
	result, err := d.db.Query("SELECT item_id, time_stamp, labels FROM items "+
		"WHERE item_id >= ? LIMIT ?", cursor, n+1)
	if err != nil {
		return "", nil, err
	}
	items := make([]Item, 0)
	defer result.Close()
	for result.Next() {
		var item Item
		var labels string
		if err := result.Scan(&item.ItemId, &item.Timestamp, &labels); err != nil {
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

func (d *SQLDatabase) GetItemFeedback(feedbackType, itemId string) ([]Feedback, error) {
	result, err := d.db.Query("SELECT user_id, item_id FROM feedback WHERE item_id = ?", itemId)
	if err != nil {
		return nil, err
	}
	feedbacks := make([]Feedback, 0)
	for result.Next() {
		var feedback Feedback
		if err := result.Scan(&feedback.UserId, &feedback.ItemId); err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	return feedbacks, nil
}

func (d *SQLDatabase) InsertUser(user User) error {
	labels, err := json.Marshal(user.Labels)
	if err != nil {
		return err
	}
	_, err = d.db.Exec("INSERT users(user_id, labels) VALUES (?, ?)", user.UserId, labels)
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
	result, err := d.db.Query("SELECT user_id, labels FROM users WHERE user_id = ?", userId)
	if err != nil {
		return User{}, err
	}
	defer result.Close()
	if result.Next() {
		var user User
		var labels string
		if err := result.Scan(&user.UserId, &labels); err != nil {
			return User{}, err
		}
		if err := json.Unmarshal([]byte(labels), &user.Labels); err != nil {
			return User{}, err
		}
		return user, nil
	}
	return User{}, errors.New(ErrUserNotExist)
}

func (d *SQLDatabase) GetUsers(cursor string, n int) (string, []User, error) {
	result, err := d.db.Query("SELECT user_id, labels FROM users "+
		"WHERE user_id >= ? LIMIT ?", cursor, n+1)
	if err != nil {
		return "", nil, err
	}
	users := make([]User, 0)
	defer result.Close()
	for result.Next() {
		var user User
		var labels string
		if err := result.Scan(&user.UserId, &labels); err != nil {
			return "", nil, err
		}
		if err := json.Unmarshal([]byte(labels), &user.Labels); err != nil {
			return "", nil, err
		}
		users = append(users, user)
	}
	if len(users) == n+1 {
		return users[len(users)-1].UserId, users[:len(users)-1], nil
	}
	return "", users, nil
}

func (d *SQLDatabase) GetUserFeedback(feedbackType, userId string) ([]Feedback, error) {
	result, err := d.db.Query("SELECT feedback_type, user_id, item_id, time_stamp FROM feedback WHERE user_id = ?", userId)
	if err != nil {
		return nil, err
	}
	feedbacks := make([]Feedback, 0)
	for result.Next() {
		var feedback Feedback
		if err := result.Scan(&feedback.FeedbackType, &feedback.UserId, &feedback.ItemId, &feedback.Timestamp); err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	return feedbacks, nil
}

func (d *SQLDatabase) InsertFeedback(feedback Feedback, insertUser, insertItem bool) error {
	// insert users
	if insertUser {
		_, err := d.db.Exec("INSERT IGNORE users(user_id) VALUES (?)", feedback.UserId)
		if err != nil {
			return err
		}
	} else {
		if _, err := d.GetUser(feedback.UserId); err != nil {
			if err.Error() == ErrUserNotExist {
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
				return nil
			}
			return err
		}
	}
	// insert feedback
	_, err := d.db.Exec("INSERT IGNORE feedback(feedback_type, user_id, item_id, time_stamp) VALUES (?,?,?,?)",
		feedback.FeedbackType, feedback.UserId, feedback.ItemId, feedback.Timestamp)
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

func (d *SQLDatabase) GetFeedback(feedbackType, cursor string, n int) (string, []Feedback, error) {
	var cursorKey FeedbackKey
	if cursor != "" {
		if err := json.Unmarshal([]byte(cursor), &cursorKey); err != nil {
			return "", nil, err
		}
	}
	result, err := d.db.Query("SELECT feedback_type, user_id, item_id, time_stamp FROM feedback "+
		"WHERE feedback_type = ? AND user_id >= ? AND item_id >= ? LIMIT ?", feedbackType, cursorKey.UserId, cursorKey.ItemId, n+1)
	if err != nil {
		return "", nil, err
	}
	feedbacks := make([]Feedback, 0)
	for result.Next() {
		var feedback Feedback
		if err := result.Scan(&feedback.FeedbackType, &feedback.UserId, &feedback.ItemId, &feedback.Timestamp); err != nil {
			return "", nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	if len(feedbacks) == n+1 {
		nextCursorKey := feedbacks[len(feedbacks)-1]
		nextCursor, err := json.Marshal(nextCursorKey)
		if err != nil {
			return "", nil, err
		}
		return string(nextCursor), feedbacks[:len(feedbacks)-1], nil
	}
	return "", feedbacks, nil
}
