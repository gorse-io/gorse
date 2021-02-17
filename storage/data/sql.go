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
	"fmt"
	_ "github.com/go-sql-driver/mysql"
)

type SQLDatabase struct {
	db *sql.DB
}

func (db *SQLDatabase) Init() error {
	// create tables
	if _, err := db.db.Exec("CREATE TABLE IF NOT EXISTS items (" +
		"item_id varchar(256) NOT NULL," +
		"time_stamp timestamp NOT NULL," +
		"labels json NOT NULL," +
		"PRIMARY KEY(item_id)" +
		")"); err != nil {
		return err
	}
	if _, err := db.db.Exec("CREATE TABLE IF NOT EXISTS users (" +
		"user_id varchar(256) NOT NULL," +
		"labels json NOT NULL," +
		"PRIMARY KEY (user_id)" +
		")"); err != nil {
		return err
	}
	if _, err := db.db.Exec("CREATE TABLE IF NOT EXISTS feedback (" +
		"feedback_type varchar(256) NOT NULL," +
		"user_id varchar(256) NOT NULL," +
		"item_id varchar(256) NOT NULL," +
		"time_stamp timestamp NOT NULL," +
		"PRIMARY KEY(feedback_type, user_id, item_id)" +
		")"); err != nil {
		return err
	}
	// change settings
	_, err := db.db.Exec("SET GLOBAL sql_mode=\"" +
		"ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO," +
		"NO_ENGINE_SUBSTITUTION\"")
	return err
}

func (db *SQLDatabase) Close() error {
	return db.db.Close()
}

func (db *SQLDatabase) InsertItem(item Item) error {
	labels, err := json.Marshal(item.Labels)
	if err != nil {
		return err
	}
	txn, err := db.db.Begin()
	if err != nil {
		return err
	}
	_, err = txn.Exec("INSERT items(item_id, time_stamp, labels) VALUES (?, ?, ?)",
		item.ItemId, item.Timestamp, labels)
	if err != nil {
		txn.Rollback()
		return err
	}
	return txn.Commit()
}

func (db *SQLDatabase) BatchInsertItem(items []Item) error {
	for _, item := range items {
		if err := db.InsertItem(item); err != nil {
			return err
		}
	}
	return nil
}

func (db *SQLDatabase) DeleteItem(itemId string) error {
	txn, err := db.db.Begin()
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

func (db *SQLDatabase) GetItem(itemId string) (Item, error) {
	result, err := db.db.Query("SELECT item_id, time_stamp, labels FROM items WHERE item_id = ?", itemId)
	if err != nil {
		return Item{}, err
	}
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
	return Item{}, fmt.Errorf("user not exists")
}

func (db *SQLDatabase) GetItems(cursor string, n int) (string, []Item, error) {
	result, err := db.db.Query("SELECT item_id, time_stamp, labels FROM items "+
		"WHERE item_id >= ? LIMIT ?", cursor, n+1)
	if err != nil {
		return "", nil, err
	}
	items := make([]Item, 0)
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

func (db *SQLDatabase) GetItemFeedback(feedbackType, itemId string) ([]Feedback, error) {
	result, err := db.db.Query("SELECT user_id, item_id FROM feedback WHERE item_id = ?", itemId)
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

func (db *SQLDatabase) InsertUser(user User) error {
	labels, err := json.Marshal(user.Labels)
	if err != nil {
		return err
	}
	txn, err := db.db.Begin()
	if err != nil {
		return err
	}
	_, err = txn.Exec("INSERT users(user_id, labels) VALUES (?, ?)", user.UserId, labels)
	if err != nil {
		txn.Rollback()
		return err
	}
	return txn.Commit()
}

func (db *SQLDatabase) DeleteUser(userId string) error {
	txn, err := db.db.Begin()
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

func (db *SQLDatabase) GetUser(userId string) (User, error) {
	result, err := db.db.Query("SELECT user_id, labels FROM users WHERE user_id = ?", userId)
	if err != nil {
		return User{}, err
	}
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
	return User{}, fmt.Errorf("user not exists")
}

func (db *SQLDatabase) GetUsers(cursor string, n int) (string, []User, error) {
	result, err := db.db.Query("SELECT user_id, labels FROM users "+
		"WHERE user_id >= ? LIMIT ?", cursor, n+1)
	if err != nil {
		return "", nil, err
	}
	users := make([]User, 0)
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

func (db *SQLDatabase) GetUserFeedback(feedbackType, userId string) ([]Feedback, error) {
	result, err := db.db.Query("SELECT feedback_type, user_id, item_id, time_stamp FROM feedback WHERE user_id = ?", userId)
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

func (db *SQLDatabase) InsertFeedback(feedback Feedback, insertUser, insertItem bool) error {
	txn, err := db.db.Begin()
	if err != nil {
		return err
	}
	_, err = txn.Exec("INSERT IGNORE feedback(feedback_type, user_id, item_id, time_stamp) VALUES (?,?,?,?)",
		feedback.FeedbackType, feedback.UserId, feedback.ItemId, feedback.Timestamp)
	if err != nil {
		txn.Rollback()
		return err
	}
	// insert users
	if insertUser {
		_, err = txn.Exec("INSERT IGNORE users(user_id) VALUES (?)", feedback.UserId)
		if err != nil {
			txn.Rollback()
			return err
		}
	}
	// insert items
	if insertItem {
		_, err = txn.Exec("INSERT IGNORE items(item_id) VALUES (?)", feedback.ItemId)
		if err != nil {
			txn.Rollback()
			return err
		}
	}
	return txn.Commit()
}

func (db *SQLDatabase) BatchInsertFeedback(feedback []Feedback, insertUser, insertItem bool) error {
	for _, f := range feedback {
		if err := db.InsertFeedback(f, insertUser, insertItem); err != nil {
			return err
		}
	}
	return nil
}

func (db *SQLDatabase) GetFeedback(feedbackType, cursor string, n int) (string, []Feedback, error) {
	var cursorKey FeedbackKey
	if cursor != "" {
		if err := json.Unmarshal([]byte(cursor), &cursorKey); err != nil {
			return "", nil, err
		}
	}
	result, err := db.db.Query("SELECT feedback_type, user_id, item_id, time_stamp FROM feedback "+
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
