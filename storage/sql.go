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
	"database/sql"
	"encoding/json"
	"fmt"
	"math/bits"
	"strconv"
)

const (
	recommendTable = "recommend"
	neighborsTable = "neighbors"
	latestTable    = "latest"
	popularTable   = "popular"
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
		"PRIMARY KEY (user_id)" +
		")"); err != nil {
		return err
	}
	if _, err := db.db.Exec("CREATE TABLE IF NOT EXISTS feedback (" +
		"user_id varchar(256) NOT NULL," +
		"item_id varchar(256) NOT NULL," +
		"PRIMARY KEY(user_id, item_id)" +
		")"); err != nil {
		return err
	}
	if _, err := db.db.Exec("CREATE TABLE IF NOT EXISTS ignored (" +
		"user_id varchar(256) NOT NULL," +
		"item_id varchar(256) NOT NULL," +
		"PRIMARY KEY(user_id, item_id)" +
		")"); err != nil {
		return err
	}
	if _, err := db.db.Exec("CREATE TABLE IF NOT EXISTS labels (" +
		"label varchar(256) NOT NULL," +
		"item_id varchar(256) NOT NULL," +
		"PRIMARY KEY(label, item_id)" +
		")"); err != nil {
		return err
	}
	if _, err := db.db.Exec("CREATE TABLE IF NOT EXISTS meta (" +
		"name varchar(256) NOT NULL," +
		"value varchar(256) NOT NULL," +
		"PRIMARY KEY(name)" +
		")"); err != nil {
		return err
	}
	tables := []string{recommendTable, neighborsTable, latestTable, popularTable}
	for _, tableName := range tables {
		if _, err := db.db.Exec("CREATE TABLE IF NOT EXISTS " + tableName + " (" +
			"label varchar(256) NOT NULL," +
			"pos int NOT NULL," +
			"item varchar(256) NOT NULL," +
			"PRIMARY KEY(label, pos)" +
			")"); err != nil {
			return err
		}
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
	for _, label := range item.Labels {
		if _, err = txn.Exec("INSERT labels(label, item_id) VALUES (?, ?)", label, item.ItemId); err != nil {
			txn.Rollback()
			return err
		}
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
	_, err = txn.Exec("DELETE FROM ignored WHERE item_id = ?", itemId)
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

func (db *SQLDatabase) GetItems(prefix string, cursor string, n int) (string, []Item, error) {
	result, err := db.db.Query("SELECT item_id, time_stamp, labels FROM items "+
		"WHERE item_id >= ? AND item_id LIKE CONCAT(?, '%') LIMIT ?", cursor, prefix, n+1)
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

func (db *SQLDatabase) GetItemFeedback(itemId string) ([]Feedback, error) {
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

func (db *SQLDatabase) GetLabelItems(label string) ([]Item, error) {
	result, err := db.db.Query("SELECT items.item_id, items.time_stamp, items.labels "+
		"FROM labels JOIN items ON items.item_id = labels.item_id WHERE label = ?", label)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0)
	for result.Next() {
		var item Item
		var labels string
		if err := result.Scan(&item.ItemId, &item.Timestamp, &labels); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(labels), &item.Labels); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (db *SQLDatabase) GetLabels(prefix string, cursor string, n int) (string, []string, error) {
	result, err := db.db.Query("SELECT DISTINCT label FROM labels "+
		"WHERE label >= ? AND label LIKE CONCAT(?, '%') LIMIT ?", cursor, prefix, n+1)
	if err != nil {
		return "", nil, err
	}
	labels := make([]string, 0)
	for result.Next() {
		var label string
		if err := result.Scan(&label); err != nil {
			return "", nil, err
		}
		labels = append(labels, label)
	}
	if len(labels) == n+1 {
		return labels[len(labels)-1], labels[:len(labels)-1], nil
	}
	return "", labels, nil
}

func (db *SQLDatabase) InsertUser(user User) error {
	_, err := db.db.Exec("INSERT users(user_id) VALUES (?)", user.UserId)
	return err
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
	_, err = txn.Exec("DELETE FROM ignored WHERE user_id = ?", userId)
	if err != nil {
		txn.Rollback()
		return err
	}
	return txn.Commit()
}

func (db *SQLDatabase) GetUser(userId string) (User, error) {
	result, err := db.db.Query("SELECT user_id FROM users WHERE user_id = ?", userId)
	if err != nil {
		return User{}, err
	}
	if result.Next() {
		var user User
		if err := result.Scan(&user.UserId); err != nil {
			return User{}, err
		}
		return user, nil
	}
	return User{}, fmt.Errorf("user not exists")
}

func (db *SQLDatabase) GetUsers(prefix string, cursor string, n int) (string, []User, error) {
	result, err := db.db.Query("SELECT user_id FROM users "+
		"WHERE user_id >= ? AND user_id LIKE CONCAT(?, '%') LIMIT ?", cursor, prefix, n+1)
	if err != nil {
		return "", nil, err
	}
	users := make([]User, 0)
	for result.Next() {
		var user User
		if err := result.Scan(&user.UserId); err != nil {
			return "", nil, err
		}
		users = append(users, user)
	}
	if len(users) == n+1 {
		return users[len(users)-1].UserId, users[:len(users)-1], nil
	}
	return "", users, nil
}

func (db *SQLDatabase) GetUserFeedback(userId string) ([]Feedback, error) {
	result, err := db.db.Query("SELECT user_id, item_id FROM feedback WHERE user_id = ?", userId)
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

func (db *SQLDatabase) InsertUserIgnore(userId string, items []string) error {
	for _, itemId := range items {
		_, err := db.db.Exec("INSERT IGNORE ignored(user_id, item_id) VALUES (?,?)", userId, itemId)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *SQLDatabase) GetUserIgnore(userId string) ([]string, error) {
	result, err := db.db.Query("SELECT item_id FROM ignored WHERE user_id = ?", userId)
	if err != nil {
		return nil, err
	}
	ignored := make([]string, 0)
	for result.Next() {
		var itemId string
		if err := result.Scan(&itemId); err != nil {
			return nil, err
		}
		ignored = append(ignored, itemId)
	}
	return ignored, nil
}

func (db *SQLDatabase) CountUserIgnore(userId string) (int, error) {
	result, err := db.db.Query("SELECT COUNT(item_id) FROM ignored WHERE user_id = ?", userId)
	if err != nil {
		return 0, err
	}
	if result.Next() {
		var count int
		if err := result.Scan(&count); err != nil {
			return 0, err
		}
		return count, nil
	}
	return 0, nil
}

func (db *SQLDatabase) InsertFeedback(feedback Feedback) error {
	txn, err := db.db.Begin()
	if err != nil {
		return err
	}
	_, err = txn.Exec("INSERT IGNORE feedback(user_id, item_id) VALUES (?,?)", feedback.UserId, feedback.ItemId)
	if err != nil {
		txn.Rollback()
		return err
	}
	_, err = txn.Exec("INSERT IGNORE users(user_id) VALUES (?)", feedback.UserId)
	if err != nil {
		txn.Rollback()
		return err
	}
	_, err = txn.Exec("INSERT IGNORE items(item_id) VALUES (?)", feedback.ItemId)
	if err != nil {
		txn.Rollback()
		return err
	}
	return txn.Commit()
}

func (db *SQLDatabase) BatchInsertFeedback(feedback []Feedback) error {
	for _, f := range feedback {
		if err := db.InsertFeedback(f); err != nil {
			return err
		}
	}
	return nil
}

func (db *SQLDatabase) GetFeedback(cursor string, n int) (string, []Feedback, error) {
	var cursorKey FeedbackKey
	if cursor != "" {
		if err := json.Unmarshal([]byte(cursor), &cursorKey); err != nil {
			return "", nil, err
		}
	}
	result, err := db.db.Query("SELECT user_id, item_id FROM feedback "+
		"WHERE user_id >= ? AND item_id >= ? LIMIT ?", cursorKey.UserId, cursorKey.ItemId, n+1)
	if err != nil {
		return "", nil, err
	}
	feedbacks := make([]Feedback, 0)
	for result.Next() {
		var feedback Feedback
		if err := result.Scan(&feedback.UserId, &feedback.ItemId); err != nil {
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

func (db *SQLDatabase) GetString(name string) (string, error) {
	result, err := db.db.Query("SELECT value FROM meta WHERE name = ?", name)
	if err != nil {
		return "", err
	}
	if result.Next() {
		var value string
		err = result.Scan(&value)
		return value, err
	}
	return "", fmt.Errorf("meta not exists")
}

func (db *SQLDatabase) SetString(name string, val string) error {
	_, err := db.db.Exec("INSERT meta(name, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = ?", name, val, val)
	return err
}

func (db *SQLDatabase) GetInt(name string) (int, error) {
	val, err := db.GetString(name)
	if err != nil {
		return -1, nil
	}
	buf, err := strconv.Atoi(val)
	if err != nil {
		return -1, err
	}
	return buf, err
}

func (db *SQLDatabase) SetInt(name string, val int) error {
	return db.SetString(name, strconv.Itoa(val))
}

func (db *SQLDatabase) SetNeighbors(itemId string, items []RecommendedItem) error {
	return db.setList(neighborsTable, itemId, items)
}

func (db *SQLDatabase) SetPop(label string, items []RecommendedItem) error {
	return db.setList(popularTable, label, items)
}

func (db *SQLDatabase) SetLatest(label string, items []RecommendedItem) error {
	return db.setList(latestTable, label, items)
}

func (db *SQLDatabase) SetRecommend(userId string, items []RecommendedItem) error {
	return db.setList(recommendTable, userId, items)
}

func (db *SQLDatabase) GetNeighbors(itemId string, n int, offset int) ([]RecommendedItem, error) {
	return db.getList(neighborsTable, itemId, n, offset)
}

func (db *SQLDatabase) GetPop(label string, n int, offset int) ([]RecommendedItem, error) {
	return db.getList(popularTable, label, n, offset)
}

func (db *SQLDatabase) GetLatest(label string, n int, offset int) ([]RecommendedItem, error) {
	return db.getList(latestTable, label, n, offset)
}

func (db *SQLDatabase) GetRecommend(userId string, n int, offset int) ([]RecommendedItem, error) {
	return db.getList(recommendTable, userId, n, offset)
}

func (db *SQLDatabase) setList(table, label string, items []RecommendedItem) error {
	txn, err := db.db.Begin()
	if err != nil {
		return err
	}
	_, err = db.db.Exec("DELETE FROM "+table+" WHERE label = ?", label)
	if err != nil {
		txn.Rollback()
		return err
	}
	for i, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			txn.Rollback()
			return err
		}
		_, err = db.db.Exec("INSERT "+table+"(label, pos, item) VALUES (?,?,?)", label, i, data)
		if err != nil {
			txn.Rollback()
			return err
		}
	}
	return txn.Commit()
}

func (db *SQLDatabase) getList(table, label string, n int, offset int) ([]RecommendedItem, error) {
	if n == 0 {
		n = (1<<bits.UintSize)/2 - 1
	}
	result, err := db.db.Query("SELECT item FROM "+table+
		" WHERE label = ? AND pos >= ? LIMIT ?", label, offset, n)
	if err != nil {
		return nil, err
	}
	items := make([]RecommendedItem, 0)
	for result.Next() {
		var data string
		if err = result.Scan(&data); err != nil {
			return nil, err
		}
		var item RecommendedItem
		if err = json.Unmarshal([]byte(data), &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
