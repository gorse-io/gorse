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

import "database/sql"

type SQLDatabase struct {
	db *sql.DB
}

func (db *SQLDatabase) Init() error {
	if _, err := db.db.Exec("CREATE TABLE items (" +
		"item_id VARCHAR NOT NULL," +
		"time_stamp TIMESTAMP NOT NULL," +
		"PRIMARY KEY(item_id)" +
		")"); err != nil {
		return err
	}
	if _, err := db.db.Exec("CREATE TABLE users (" +
		"user_id VARCHAR NOT NULL," +
		"PRIMARY KEY(user_id)" +
		")"); err != nil {
		return err
	}
	if _, err := db.db.Exec("CREATE TABLE feedback (" +
		"user_id VARCHAR NOT NULL," +
		"item_id VARCHAR NOT NULL," +
		"PRIMARY KEY(user_id, item_id)" +
		")"); err != nil {
		return err
	}
	if _, err := db.db.Exec("CREATE TABLE label (" +
		"label VARCHAR NOT NULL," +
		"item_id VARCHAR NOT NULL," +
		"PRIMARY KEY(label, item_id)" +
		")"); err != nil {
		return err
	}
	if _, err := db.db.Exec("CREATE TABLE recommend (" +
		"user_id VARCHAR NOT NULL," +
		"pos_id INT NOT NULL," +
		"item_id VARCHAR NOT NULL," +
		"PRIMARY KEY(user_id, pos_id)" +
		")"); err != nil {
		return err
	}
	if _, err := db.db.Exec("CREATE TABLE neighbors (" +
		"item_id VARCHAR NOT NULL," +
		"pos_id INT NOT NULL," +
		"item_id VARCHAR NOT NULL," +
		"PRIMARY KEY(item_id, pos_id)" +
		")"); err != nil {
		return err
	}
	if _, err := db.db.Exec("CREATE TABLE latest (" +
		"label VARCHAR NOT NULL," +
		"pos_id INT NOT NULL," +
		"item_id VARCHAR NOT NULL," +
		"PRIMARY KEY(label, pos_id)" +
		")"); err != nil {
		return err
	}
	if _, err := db.db.Exec("CREATE TABLE popular (" +
		"label VARCHAR NOT NULL," +
		"pos_id INT NOT NULL," +
		"item_id VARCHAR NOT NULL," +
		"PRIMARY KEY(label, pos_id)" +
		")"); err != nil {
		return err
	}
	return nil
}

func (db *SQLDatabase) Close() error {
	return db.db.Close()
}

func (db *SQLDatabase) InsertItem(item Item) error {
	panic("not implemented")
}

func (db *SQLDatabase) BatchInsertItem(items []Item) error {
	panic("not implemented")
}

func (db *SQLDatabase) DeleteItem(itemId string) error {
	panic("not implemented")
}

func (db *SQLDatabase) GetItem(itemId string) (Item, error) {
	panic("not implemented")
}

func (db *SQLDatabase) GetItems(n int, offset int) ([]Item, error) {
	panic("not implemented")
}

func (db *SQLDatabase) GetItemFeedback(itemId string) ([]Feedback, error) {
	panic("not implemented")
}

func (db *SQLDatabase) GetLabelItems(label string) ([]Item, error) {
	panic("not implemented")
}

func (db *SQLDatabase) GetLabels() ([]string, error) {
	panic("not implemented")
}

func (db *SQLDatabase) InsertUser(user User) error {
	panic("not implemented")
}

func (db *SQLDatabase) DeleteUser(userId string) error {
	panic("not implemented")
}

func (db *SQLDatabase) GetUser(userId string) (User, error) {
	panic("not implemented")
}

func (db *SQLDatabase) GetUsers() ([]User, error) {
	panic("not implemented")
}

func (db *SQLDatabase) GetUserFeedback(userId string) ([]Feedback, error) {
	panic("not implemented")
}
func (db *SQLDatabase) InsertUserIgnore(userId string, items []string) error {
	panic("not implemented")
}

func (db *SQLDatabase) GetUserIgnore(userId string) ([]string, error) {
	panic("not implemented")
}

func (db *SQLDatabase) CountUserIgnore(userId string) (int, error) {
	panic("not implemented")
}

func (db *SQLDatabase) InsertFeedback(feedback Feedback) error {
	panic("not implemented")
}

func (db *SQLDatabase) BatchInsertFeedback(feedback []Feedback) error {
	panic("not implemented")
}

func (db *SQLDatabase) GetFeedback() ([]Feedback, error) {
	panic("not implemented")
}

func (db *SQLDatabase) GetString(name string) (string, error) {
	panic("not implemented")
}

func (db *SQLDatabase) SetString(name string, val string) error {
	panic("not implemented")
}

func (db *SQLDatabase) GetInt(name string) (int, error) {
	panic("not implemented")
}

func (db *SQLDatabase) SetInt(name string, val int) error {
	panic("not implemented")
}

func (db *SQLDatabase) SetNeighbors(itemId string, items []RecommendedItem) error {
	panic("not implemented")
}

func (db *SQLDatabase) SetPop(label string, items []RecommendedItem) error {
	panic("not implemented")
}

func (db *SQLDatabase) SetLatest(label string, items []RecommendedItem) error {
	panic("not implemented")
}

func (db *SQLDatabase) SetRecommend(userId string, items []RecommendedItem) error {
	panic("not implemented")
}

func (db *SQLDatabase) GetNeighbors(itemId string, n int, offset int) ([]RecommendedItem, error) {
	panic("not implemented")
}

func (db *SQLDatabase) GetPop(label string, n int, offset int) ([]RecommendedItem, error) {
	panic("not implemented")
}

func (db *SQLDatabase) GetLatest(label string, n int, offset int) ([]RecommendedItem, error) {
	panic("not implemented")
}

func (db *SQLDatabase) GetRecommend(userId string, n int, offset int) ([]RecommendedItem, error) {
	panic("not implemented")
}
