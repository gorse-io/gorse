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
	"encoding/json"
	_ "github.com/go-sql-driver/mysql"
	"github.com/scylladb/go-set/strset"
	"github.com/zhenghaoz/gorse/base"
	"go.uber.org/zap"
	"strings"
	"time"
)

// SQLDatabase use MySQL as data storage.
type SQLDatabase struct {
	db *sql.DB
}

// Init tables and indices in MySQL.
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
	if _, err := d.db.Exec("CREATE TABLE IF NOT EXISTS measurements (" +
		"name varchar(256) NOT NULL," +
		"time_stamp timestamp NOT NULL," +
		"value double NOT NULL," +
		"comment TEXT NOT NULL," +
		"PRIMARY KEY(name, time_stamp)" +
		")"); err != nil {
		return err
	}
	// create index
	if exist, err := d.checkIfIndexExists("feedback", "user_id"); err != nil {
		return err
	} else if !exist {
		if _, err = d.db.Exec("CREATE INDEX user_id ON feedback(user_id)"); err != nil {
			return err
		}
	}
	// change settings
	_, err := d.db.Exec("SET SESSION sql_mode=\"" +
		"ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO," +
		"NO_ENGINE_SUBSTITUTION\"")
	return err
}

func (d *SQLDatabase) checkIfIndexExists(table, index string) (bool, error) {
	r, err := d.db.Query("SHOW INDEX IN "+table+" WHERE Key_name = ?", index)
	if err != nil {
		return false, err
	}
	return r.Next(), nil
}

// Close MySQL connection.
func (d *SQLDatabase) Close() error {
	return d.db.Close()
}

// InsertMeasurement insert a measurement into MySQL.
func (d *SQLDatabase) InsertMeasurement(measurement Measurement) error {
	_, err := d.db.Exec("INSERT measurements(name, time_stamp, value, `comment`) VALUES (?, ?, ?, ?)",
		measurement.Name, measurement.Timestamp, measurement.Value, measurement.Comment)
	return err
}

// GetMeasurements returns recent measurements from MySQL.
func (d *SQLDatabase) GetMeasurements(name string, n int) ([]Measurement, error) {
	measurements := make([]Measurement, 0)
	result, err := d.db.Query("SELECT name, time_stamp, value, `comment` FROM measurements WHERE name = ? ORDER BY time_stamp DESC LIMIT ?", name, n)
	if err != nil {
		return measurements, err
	}
	defer result.Close()
	for result.Next() {
		var measurement Measurement
		if err = result.Scan(&measurement.Name, &measurement.Timestamp, &measurement.Value, &measurement.Comment); err != nil {
			return measurements, err
		}
		measurements = append(measurements, measurement)
	}
	return measurements, nil
}

// InsertItem inserts a item into MySQL.
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

// BatchInsertItem inserts a batch of items into MySQL.
func (d *SQLDatabase) BatchInsertItem(items []Item) error {
	const batchSize = 10000
	for i := 0; i < len(items); i += batchSize {
		batchItems := items[i:base.Min(i+batchSize, len(items))]
		// build query
		builder := strings.Builder{}
		builder.WriteString("INSERT items(item_id, time_stamp, labels, `comment`) VALUES ")
		var args []interface{}
		for i, item := range batchItems {
			labels, err := json.Marshal(item.Labels)
			if err != nil {
				return err
			}
			builder.WriteString("(?,?,?,?)")
			if i+1 < len(batchItems) {
				builder.WriteString(",")
			}
			args = append(args, item.ItemId, item.Timestamp, labels, item.Comment)
		}
		builder.WriteString(" AS new ON DUPLICATE KEY UPDATE time_stamp = new.time_stamp, labels = new.labels, `comment` = new.comment")
		_, err := d.db.Exec(builder.String(), args...)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteItem deletes a item from MySQL.
func (d *SQLDatabase) DeleteItem(itemId string) error {
	txn, err := d.db.Begin()
	if err != nil {
		return err
	}
	_, err = txn.Exec("DELETE FROM items WHERE item_id = ?", itemId)
	if err != nil {
		if err = txn.Rollback(); err != nil {
			return err
		}
		return err
	}
	_, err = txn.Exec("DELETE FROM feedback WHERE item_id = ?", itemId)
	if err != nil {
		if err = txn.Rollback(); err != nil {
			return err
		}
		return err
	}
	return txn.Commit()
}

// GetItem get a item from MySQL.
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
	return Item{}, ErrItemNotExist
}

// GetItems returns items from MySQL.
func (d *SQLDatabase) GetItems(cursor string, n int, timeLimit *time.Time) (string, []Item, error) {
	var result *sql.Rows
	var err error
	if timeLimit == nil {
		result, err = d.db.Query("SELECT item_id, time_stamp, labels, `comment` FROM items "+
			"WHERE item_id >= ? ORDER BY item_id LIMIT ?", cursor, n+1)
	} else {
		result, err = d.db.Query("SELECT item_id, time_stamp, labels, `comment` FROM items "+
			"WHERE item_id >= ? AND time_stamp >= ? ORDER BY item_id LIMIT ?", cursor, *timeLimit, n+1)
	}
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

// GetItemFeedback returns feedback of a item from MySQL.
func (d *SQLDatabase) GetItemFeedback(itemId string, feedbackTypes ...string) ([]Feedback, error) {
	startTime := time.Now()
	var result *sql.Rows
	var err error
	var builder strings.Builder
	builder.WriteString("SELECT user_id, item_id, feedback_type FROM feedback WHERE item_id = ?")
	args := []interface{}{itemId}
	if len(feedbackTypes) > 0 {
		builder.WriteString(" AND feedback_type IN (")
		for i, feedbackType := range feedbackTypes {
			builder.WriteString("?")
			if i+1 < len(feedbackTypes) {
				builder.WriteString(",")
			}
			args = append(args, feedbackType)
		}
		builder.WriteString(")")
	}
	result, err = d.db.Query(builder.String(), args...)
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

// InsertUser inserts a user into MySQL.
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

// DeleteUser deletes a user from MySQL.
func (d *SQLDatabase) DeleteUser(userId string) error {
	txn, err := d.db.Begin()
	if err != nil {
		return err
	}
	_, err = txn.Exec("DELETE FROM users WHERE user_id = ?", userId)
	if err != nil {
		if err = txn.Rollback(); err != nil {
			return err
		}
		return err
	}
	_, err = txn.Exec("DELETE FROM feedback WHERE user_id = ?", userId)
	if err != nil {
		if err = txn.Rollback(); err != nil {
			return err
		}
		return err
	}
	return txn.Commit()
}

// GetUser returns a user from MySQL.
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
	return User{}, ErrUserNotExist
}

// GetUsers returns users from MySQL.
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

// GetUserFeedback returns feedback of a user from MySQL.
func (d *SQLDatabase) GetUserFeedback(userId string, feedbackTypes ...string) ([]Feedback, error) {
	startTime := time.Now()
	var result *sql.Rows
	var err error
	var builder strings.Builder
	builder.WriteString("SELECT feedback_type, user_id, item_id, time_stamp, `comment` FROM feedback WHERE user_id = ?")
	args := []interface{}{userId}
	if len(feedbackTypes) > 0 {
		builder.WriteString(" AND feedback_type IN (")
		for i, feedbackType := range feedbackTypes {
			builder.WriteString("?")
			if i+1 < len(feedbackTypes) {
				builder.WriteString(",")
			}
			args = append(args, feedbackType)
		}
		builder.WriteString(")")
	}
	result, err = d.db.Query(builder.String(), args...)
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

// InsertFeedback insert a feedback into MySQL.
// If insertUser set, a new user will be insert to user table.
// If insertItem set, a new item will be insert to item table.
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
			if err == ErrUserNotExist {
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
			if err == ErrItemNotExist {
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

// BatchInsertFeedback insert a batch feedback into MySQL.
// If insertUser set, new users will be insert to user table.
// If insertItem set, new items will be insert to item table.
func (d *SQLDatabase) BatchInsertFeedback(feedback []Feedback, insertUser, insertItem bool) error {
	const batchSize = 10000
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
		for i := 0; i < len(userList); i += batchSize {
			batchUsers := userList[i:base.Min(i+batchSize, len(userList))]
			builder := strings.Builder{}
			builder.WriteString("INSERT IGNORE users(user_id) VALUES ")
			var args []interface{}
			for i, user := range batchUsers {
				builder.WriteString("(?)")
				if i+1 < len(batchUsers) {
					builder.WriteString(",")
				}
				args = append(args, user)
			}
			if _, err := d.db.Exec(builder.String(), args...); err != nil {
				return err
			}
		}
	} else {
		for _, user := range users.List() {
			rs, err := d.db.Query("SELECT user_id FROM users WHERE user_id = ?", user)
			if err != nil {
				return err
			} else if !rs.Next() {
				users.Remove(user)
			}
			rs.Close()
		}
	}
	// insert items
	if insertItem {
		itemList := items.List()
		for i := 0; i < len(itemList); i += batchSize {
			batchItems := itemList[i:base.Min(i+batchSize, len(itemList))]
			builder := strings.Builder{}
			builder.WriteString("INSERT IGNORE items(item_id) VALUES ")
			var args []interface{}
			for i, item := range batchItems {
				builder.WriteString("(?)")
				if i+1 < len(batchItems) {
					builder.WriteString(",")
				}
				args = append(args, item)
			}
			if _, err := d.db.Exec(builder.String(), args...); err != nil {
				return err
			}
		}
	} else {
		for _, item := range items.List() {
			rs, err := d.db.Query("SELECT item_id FROM items WHERE item_id = ?", item)
			if err != nil {
				return err
			} else if !rs.Next() {
				users.Remove(item)
			}
			rs.Close()
		}
	}
	// insert feedback
	for i := 0; i < len(feedback); i += batchSize {
		batchFeedback := feedback[i:base.Min(i+batchSize, len(feedback))]
		builder := strings.Builder{}
		builder.WriteString("INSERT feedback(feedback_type, user_id, item_id, time_stamp, `comment`) VALUES ")
		var args []interface{}
		for i, f := range batchFeedback {
			if users.Has(f.UserId) && items.Has(f.ItemId) {
				builder.WriteString("(?,?,?,?,?)")
				if i+1 < len(batchFeedback) {
					builder.WriteString(",")
				}
				args = append(args, f.FeedbackType, f.UserId, f.ItemId, f.Timestamp, f.Comment)
			}
		}
		builder.WriteString(" AS new ON DUPLICATE KEY UPDATE time_stamp = new.time_stamp, `comment` = new.`comment`")
		_, err := d.db.Exec(builder.String(), args...)
		if err != nil {
			return err
		}
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
	var result *sql.Rows
	var err error
	var builder strings.Builder
	builder.WriteString("SELECT feedback_type, user_id, item_id, time_stamp, `comment` FROM feedback WHERE feedback_type >= ? AND user_id >= ? AND item_id >= ?")
	args := []interface{}{cursorKey.FeedbackType, cursorKey.UserId, cursorKey.ItemId}
	if len(feedbackTypes) > 0 {
		builder.WriteString(" AND feedback_type IN (")
		for i, feedbackType := range feedbackTypes {
			builder.WriteString("?")
			if i+1 < len(feedbackTypes) {
				builder.WriteString(",")
			}
			args = append(args, feedbackType)
		}
		builder.WriteString(")")
	}
	if timeLimit != nil {
		builder.WriteString(" AND time_stamp >= ?")
		args = append(args, *timeLimit)
	}
	builder.WriteString(" ORDER BY feedback_type, user_id, item_id LIMIT ?")
	args = append(args, n+1)
	result, err = d.db.Query(builder.String(), args...)
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

// GetUserItemFeedback gets a feedback by user id and item id from MySQL.
func (d *SQLDatabase) GetUserItemFeedback(userId, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	startTime := time.Now()
	var result *sql.Rows
	var err error
	var builder strings.Builder
	builder.WriteString("SELECT feedback_type, user_id, item_id, time_stamp, `comment` FROM feedback WHERE user_id = ? AND item_id = ?")
	args := []interface{}{userId, itemId}
	if len(feedbackTypes) > 0 {
		builder.WriteString(" AND feedback_type IN (")
		for i, feedbackType := range feedbackTypes {
			builder.WriteString("?")
			if i+1 < len(feedbackTypes) {
				builder.WriteString(",")
			}
			args = append(args, feedbackType)
		}
		builder.WriteString(")")
	}
	result, err = d.db.Query(builder.String(), args...)
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

// DeleteUserItemFeedback deletes a feedback by user id and item id from MySQL.
func (d *SQLDatabase) DeleteUserItemFeedback(userId, itemId string, feedbackTypes ...string) (int, error) {
	var rs sql.Result
	var err error
	var builder strings.Builder
	builder.WriteString("DELETE FROM feedback WHERE user_id = ? AND item_id = ?")
	args := []interface{}{userId, itemId}
	if len(feedbackTypes) > 0 {
		builder.WriteString(" AND feedback_type IN (")
		for i, feedbackType := range feedbackTypes {
			builder.WriteString("?")
			if i+1 < len(feedbackTypes) {
				builder.WriteString(",")
			}
			args = append(args, feedbackType)
		}
		builder.WriteString(")")
	}
	rs, err = d.db.Exec(builder.String(), args...)
	if err != nil {
		return 0, err
	}
	deleteCount, err := rs.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(deleteCount), nil
}

// GetClickThroughRate computes the click-through-rate of a specified date.
func (d *SQLDatabase) GetClickThroughRate(date time.Time, positiveTypes []string, readType string) (float64, error) {
	builder := strings.Builder{}
	var args []interface{}
	builder.WriteString("SELECT positive_count.positive_count / read_count.read_count FROM (")
	builder.WriteString("SELECT user_id, COUNT(*) AS positive_count FROM (")
	builder.WriteString("SELECT DISTINCT user_id, item_id FROM feedback WHERE DATE(time_stamp) = DATE(?) AND feedback_type IN (")
	args = append(args, date)
	for i, positiveType := range positiveTypes {
		if i > 0 {
			builder.WriteString(",")
		}
		builder.WriteString("?")
		args = append(args, positiveType)
	}
	builder.WriteString(")) AS positive_feedback GROUP BY user_id) AS positive_count ")
	builder.WriteString("JOIN (")
	builder.WriteString("SELECT user_id, COUNT(*) AS read_count FROM (")
	builder.WriteString("SELECT DISTINCT user_id, item_id FROM feedback WHERE DATE(time_stamp) = DATE(?) AND feedback_type IN (?")
	args = append(args, date, readType)
	for _, positiveType := range positiveTypes {
		builder.WriteString(",?")
		args = append(args, positiveType)
	}
	builder.WriteString(")) AS read_feedback GROUP BY user_id) AS read_count ")
	builder.WriteString("ON positive_count.user_id = read_count.user_id")
	base.Logger().Info("get click through rate from MySQL", zap.String("query", builder.String()))
	rs, err := d.db.Query(builder.String(), args...)
	if err != nil {
		return 0, err
	}
	var sum, count float64
	for rs.Next() {
		var temp float64
		if err = rs.Scan(&temp); err != nil {
			return 0, err
		}
		sum += temp
		count++
	}
	if count > 0 {
		sum /= count
	}
	return sum, err
}

// CountActiveUsers returns the number active users starting from a specified date.
func (d *SQLDatabase) CountActiveUsers(date time.Time) (int, error) {
	rs, err := d.db.Query("SELECT COUNT(DISTINCT user_id) FROM feedback WHERE DATE(time_stamp) >= DATE(?)", date)
	if err != nil {
		return 0, err
	}
	var count int
	if rs.Next() {
		if err = rs.Scan(&count); err != nil {
			return 0, err
		}
	}
	return count, nil
}
