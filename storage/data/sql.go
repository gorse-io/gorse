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
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/juju/errors"
	_ "github.com/lib/pq"
	_ "github.com/mailru/go-clickhouse"
	"github.com/scylladb/go-set/strset"
	"github.com/zhenghaoz/gorse/base"
	"go.uber.org/zap"
	"strings"
	"time"
)

const bufSize = 1

type SQLDriver int

const (
	MySQL SQLDriver = iota
	Postgres
	ClickHouse
)

// SQLDatabase use MySQL as data storage.
type SQLDatabase struct {
	client *sql.DB
	driver SQLDriver
}

// Optimize is used by ClickHouse only.
func (d *SQLDatabase) Optimize() error {
	if d.driver == ClickHouse {
		for _, tableName := range []string{"users", "items", "feedback", "measurements"} {
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
		if _, err := d.client.Exec("CREATE TABLE IF NOT EXISTS items (" +
			"item_id varchar(256) NOT NULL," +
			"time_stamp timestamp NOT NULL," +
			"labels json NOT NULL," +
			"comment TEXT NOT NULL," +
			"PRIMARY KEY(item_id)" +
			")  ENGINE=InnoDB"); err != nil {
			return errors.Trace(err)
		}
		if _, err := d.client.Exec("CREATE TABLE IF NOT EXISTS users (" +
			"user_id varchar(256) NOT NULL," +
			"labels json NOT NULL," +
			"subscribe json NOT NULL," +
			"comment TEXT NOT NULL," +
			"PRIMARY KEY (user_id)" +
			")  ENGINE=InnoDB"); err != nil {
			return errors.Trace(err)
		}
		if _, err := d.client.Exec("CREATE TABLE IF NOT EXISTS feedback (" +
			"feedback_type varchar(256) NOT NULL," +
			"user_id varchar(256) NOT NULL," +
			"item_id varchar(256) NOT NULL," +
			"time_stamp timestamp NOT NULL," +
			"comment TEXT NOT NULL," +
			"PRIMARY KEY(feedback_type, user_id, item_id)," +
			"INDEX (user_id)," +
			"INDEX (item_id)" +
			")  ENGINE=InnoDB"); err != nil {
			return errors.Trace(err)
		}
		if _, err := d.client.Exec("CREATE TABLE IF NOT EXISTS measurements (" +
			"name varchar(256) NOT NULL," +
			"time_stamp timestamp NOT NULL," +
			"value double NOT NULL," +
			"comment TEXT NOT NULL," +
			"PRIMARY KEY(name, time_stamp)" +
			")  ENGINE=InnoDB"); err != nil {
			return errors.Trace(err)
		}
		// change settings
		_, err := d.client.Exec("SET SESSION sql_mode=\"" +
			"ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO," +
			"NO_ENGINE_SUBSTITUTION\"")
		return errors.Trace(err)
	case Postgres:
		// create tables
		if _, err := d.client.Exec("CREATE TABLE IF NOT EXISTS items (" +
			"item_id varchar(256) NOT NULL," +
			"time_stamp timestamptz NOT NULL DEFAULT '0001-01-01'," +
			"labels json NOT NULL DEFAULT '[]'," +
			"comment TEXT NOT NULL DEFAULT ''," +
			"PRIMARY KEY(item_id)" +
			")"); err != nil {
			return errors.Trace(err)
		}
		if _, err := d.client.Exec("CREATE TABLE IF NOT EXISTS users (" +
			"user_id varchar(256) NOT NULL," +
			"labels json NOT NULL DEFAULT '[]'," +
			"subscribe json NOT NULL DEFAULT '[]'," +
			"comment TEXT NOT NULL DEFAULT ''," +
			"PRIMARY KEY (user_id)" +
			")"); err != nil {
			return errors.Trace(err)
		}
		if _, err := d.client.Exec("CREATE TABLE IF NOT EXISTS feedback (" +
			"feedback_type varchar(256) NOT NULL," +
			"user_id varchar(256) NOT NULL," +
			"item_id varchar(256) NOT NULL," +
			"time_stamp timestamptz NOT NULL DEFAULT '0001-01-01'," +
			"comment TEXT NOT NULL DEFAULT ''," +
			"PRIMARY KEY(feedback_type, user_id, item_id)" +
			")"); err != nil {
			return errors.Trace(err)
		}
		if _, err := d.client.Exec("CREATE INDEX IF NOT EXISTS user_id_index ON feedback(user_id)"); err != nil {
			return errors.Trace(err)
		}
		if _, err := d.client.Exec("CREATE INDEX IF NOT EXISTS item_id_index ON feedback(item_id)"); err != nil {
			return errors.Trace(err)
		}
		if _, err := d.client.Exec("CREATE TABLE IF NOT EXISTS measurements (" +
			"name varchar(256) NOT NULL," +
			"time_stamp timestamptz NOT NULL," +
			"value double precision NOT NULL," +
			"comment TEXT NOT NULL," +
			"PRIMARY KEY(name, time_stamp)" +
			")"); err != nil {
			return errors.Trace(err)
		}
	case ClickHouse:
		// create tables
		if _, err := d.client.Exec("CREATE TABLE IF NOT EXISTS items (" +
			"item_id String," +
			"time_stamp Datetime," +
			"labels String DEFAULT '[]'," +
			"comment String," +
			"version DateTime" +
			") ENGINE = ReplacingMergeTree(version) ORDER BY item_id"); err != nil {
			return errors.Trace(err)
		}
		if _, err := d.client.Exec("CREATE TABLE IF NOT EXISTS users (" +
			"user_id String," +
			"labels String DEFAULT '[]'," +
			"subscribe String DEFAULT '[]'," +
			"comment String," +
			"version DateTime" +
			") ENGINE = ReplacingMergeTree(version) ORDER BY user_id"); err != nil {
			return errors.Trace(err)
		}
		if _, err := d.client.Exec("CREATE TABLE IF NOT EXISTS feedback (" +
			"feedback_type String," +
			"user_id String," +
			"item_id String," +
			"time_stamp Datetime," +
			"comment String," +
			"version DateTime" +
			") ENGINE = ReplacingMergeTree(version) ORDER BY (feedback_type, user_id, item_id)"); err != nil {
			return errors.Trace(err)
		}
		if _, err := d.client.Exec("CREATE TABLE IF NOT EXISTS measurements (" +
			"name String," +
			"time_stamp Datetime," +
			"value Float64," +
			"comment String" +
			") ENGINE = ReplacingMergeTree() ORDER BY (name, time_stamp)"); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// Close MySQL connection.
func (d *SQLDatabase) Close() error {
	return d.client.Close()
}

// InsertMeasurement insert a measurement into MySQL.
func (d *SQLDatabase) InsertMeasurement(measurement Measurement) error {
	var err error
	switch d.driver {
	case MySQL:
		_, err = d.client.Exec("INSERT INTO measurements(name, time_stamp, value, `comment`) VALUES (?, ?, ?, ?) "+
			"ON DUPLICATE KEY UPDATE value = VALUES(value), `comment` = VALUES(`comment`)",
			measurement.Name, measurement.Timestamp, measurement.Value, measurement.Comment)
	case Postgres:
		_, err = d.client.Exec("INSERT INTO measurements(name, time_stamp, value, comment) VALUES ($1, $2, $3, $4)  "+
			"ON CONFLICT (name, time_stamp) DO UPDATE SET value = EXCLUDED.value, comment = EXCLUDED.comment",
			measurement.Name, measurement.Timestamp, measurement.Value, measurement.Comment)
	case ClickHouse:
		_, err = d.client.Exec("INSERT INTO measurements(name, time_stamp, value, `comment`) VALUES (?, ?, ?, ?)",
			measurement.Name, measurement.Timestamp, measurement.Value, measurement.Comment)
	}
	return errors.Trace(err)
}

// GetMeasurements returns recent measurements from MySQL.
func (d *SQLDatabase) GetMeasurements(name string, n int) ([]Measurement, error) {
	measurements := make([]Measurement, 0)
	var result *sql.Rows
	var err error
	switch d.driver {
	case MySQL, ClickHouse:
		result, err = d.client.Query("SELECT name, time_stamp, value, `comment` FROM measurements WHERE name = ? ORDER BY time_stamp DESC LIMIT ?", name, n)
	case Postgres:
		result, err = d.client.Query("SELECT name, time_stamp, value, comment FROM measurements WHERE name = $1 ORDER BY time_stamp DESC LIMIT $2", name, n)
	}
	if err != nil {
		return measurements, errors.Trace(err)
	}
	defer result.Close()
	for result.Next() {
		var measurement Measurement
		if err = result.Scan(&measurement.Name, &measurement.Timestamp, &measurement.Value, &measurement.Comment); err != nil {
			return measurements, errors.Trace(err)
		}
		measurements = append(measurements, measurement)
	}
	return measurements, nil
}

// BatchInsertItems inserts a batch of items into MySQL.
func (d *SQLDatabase) BatchInsertItems(items []Item) error {
	builder := strings.Builder{}
	switch d.driver {
	case MySQL:
		builder.WriteString("INSERT INTO items(item_id, time_stamp, labels, `comment`) VALUES ")
	case Postgres:
		builder.WriteString("INSERT INTO items(item_id, time_stamp, labels, comment) VALUES ")
	case ClickHouse:
		builder.WriteString("INSERT INTO items(item_id, time_stamp, labels, comment, version) VALUES ")
	}
	var args []interface{}
	for i, item := range items {
		labels, err := json.Marshal(item.Labels)
		if err != nil {
			return errors.Trace(err)
		}
		switch d.driver {
		case MySQL:
			builder.WriteString("(?,?,?,?)")
		case Postgres:
			builder.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d)", len(args)+1, len(args)+2, len(args)+3, len(args)+4))
		case ClickHouse:
			builder.WriteString("(?,?,?,?,NOW())")
		}
		if i+1 < len(items) {
			builder.WriteString(",")
		}
		args = append(args, item.ItemId, item.Timestamp, string(labels), item.Comment)
	}
	switch d.driver {
	case MySQL:
		builder.WriteString(" ON DUPLICATE KEY " +
			"UPDATE time_stamp = VALUES(time_stamp), labels = VALUES(labels), `comment` = VALUES(`comment`)")
	case Postgres:
		builder.WriteString(" ON CONFLICT (item_id) " +
			"DO UPDATE SET time_stamp = EXCLUDED.time_stamp, labels = EXCLUDED.labels, comment = EXCLUDED.comment")
	}
	_, err := d.client.Exec(builder.String(), args...)
	return errors.Trace(err)
}

// DeleteItem deletes a item from MySQL.
func (d *SQLDatabase) DeleteItem(itemId string) error {
	txn, err := d.client.Begin()
	if err != nil {
		return errors.Trace(err)
	}
	switch d.driver {
	case MySQL:
		_, err = txn.Exec("DELETE FROM items WHERE item_id = ?", itemId)
	case Postgres:
		_, err = txn.Exec("DELETE FROM items WHERE item_id = $1", itemId)
	case ClickHouse:
		_, err = txn.Exec("ALTER TABLE items DELETE WHERE item_id = ?", itemId)
	}
	if err != nil {
		if err = txn.Rollback(); err != nil {
			return errors.Trace(err)
		}
		return errors.Trace(err)
	}
	switch d.driver {
	case MySQL:
		_, err = txn.Exec("DELETE FROM feedback WHERE item_id = ?", itemId)
	case Postgres:
		_, err = txn.Exec("DELETE FROM feedback WHERE item_id = $1", itemId)
	case ClickHouse:
		_, err = txn.Exec("ALTER TABLE feedback DELETE WHERE item_id = ?", itemId)
	}
	if err != nil {
		if err = txn.Rollback(); err != nil {
			return errors.Trace(err)
		}
		return errors.Trace(err)
	}
	return txn.Commit()
}

// GetItem get a item from MySQL.
func (d *SQLDatabase) GetItem(itemId string) (Item, error) {
	startTime := time.Now()
	var result *sql.Rows
	var err error
	switch d.driver {
	case MySQL, ClickHouse:
		result, err = d.client.Query("SELECT item_id, time_stamp, labels, `comment` FROM items WHERE item_id = ?", itemId)
	case Postgres:
		result, err = d.client.Query("SELECT item_id, time_stamp, labels, comment FROM items WHERE item_id = $1", itemId)
	}
	if err != nil {
		return Item{}, errors.Trace(err)
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
	switch d.driver {
	case MySQL, ClickHouse:
		if timeLimit == nil {
			result, err = d.client.Query("SELECT item_id, time_stamp, labels, `comment` FROM items "+
				"WHERE item_id >= ? ORDER BY item_id LIMIT ?", cursor, n+1)
		} else {
			result, err = d.client.Query("SELECT item_id, time_stamp, labels, `comment` FROM items "+
				"WHERE item_id >= ? AND time_stamp >= ? ORDER BY item_id LIMIT ?", cursor, *timeLimit, n+1)
		}
	case Postgres:
		if timeLimit == nil {
			result, err = d.client.Query("SELECT item_id, time_stamp, labels, comment FROM items "+
				"WHERE item_id >= $1 ORDER BY item_id LIMIT $2", cursor, n+1)
		} else {
			result, err = d.client.Query("SELECT item_id, time_stamp, labels, comment FROM items "+
				"WHERE item_id >= $1 AND time_stamp >= $2 ORDER BY item_id LIMIT $3", cursor, *timeLimit, n+1)
		}
	}
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	items := make([]Item, 0)
	defer result.Close()
	for result.Next() {
		var item Item
		var labels string
		if err = result.Scan(&item.ItemId, &item.Timestamp, &labels, &item.Comment); err != nil {
			return "", nil, errors.Trace(err)
		}
		if err = json.Unmarshal([]byte(labels), &item.Labels); err != nil {
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
		var result *sql.Rows
		var err error
		switch d.driver {
		case MySQL, ClickHouse:
			if timeLimit == nil {
				result, err = d.client.Query("SELECT item_id, time_stamp, labels, `comment` FROM items")
			} else {
				result, err = d.client.Query("SELECT item_id, time_stamp, labels, `comment` FROM items WHERE time_stamp >= ?", *timeLimit)
			}
		case Postgres:
			if timeLimit == nil {
				result, err = d.client.Query("SELECT item_id, time_stamp, labels, comment FROM items")
			} else {
				result, err = d.client.Query("SELECT item_id, time_stamp, labels, comment FROM items WHERE time_stamp >= $2", *timeLimit)
			}
		}
		if err != nil {
			errChan <- errors.Trace(err)
			return
		}
		// fetch result
		items := make([]Item, 0, batchSize)
		defer result.Close()
		for result.Next() {
			var item Item
			var labels string
			if err = result.Scan(&item.ItemId, &item.Timestamp, &labels, &item.Comment); err != nil {
				errChan <- errors.Trace(err)
				return
			}
			if err = json.Unmarshal([]byte(labels), &item.Labels); err != nil {
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
	startTime := time.Now()
	var result *sql.Rows
	var err error
	var builder strings.Builder
	switch d.driver {
	case MySQL, ClickHouse:
		builder.WriteString("SELECT user_id, item_id, feedback_type, time_stamp FROM feedback WHERE time_stamp <= NOW() AND item_id = ?")
	case Postgres:
		builder.WriteString("SELECT user_id, item_id, feedback_type, time_stamp FROM feedback WHERE time_stamp <= NOW() AND item_id = $1")
	}
	args := []interface{}{itemId}
	if len(feedbackTypes) > 0 {
		builder.WriteString(" AND feedback_type IN (")
		for i, feedbackType := range feedbackTypes {
			switch d.driver {
			case MySQL, ClickHouse:
				builder.WriteString("?")
			case Postgres:
				builder.WriteString(fmt.Sprintf("$%d", len(args)+1))
			}
			if i+1 < len(feedbackTypes) {
				builder.WriteString(",")
			}
			args = append(args, feedbackType)
		}
		builder.WriteString(")")
	}
	result, err = d.client.Query(builder.String(), args...)
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
	GetItemFeedbackLatency.Observe(time.Since(startTime).Seconds())
	return feedbacks, nil
}

// BatchInsertUsers inserts users into MySQL.
func (d *SQLDatabase) BatchInsertUsers(users []User) error {
	builder := strings.Builder{}
	switch d.driver {
	case MySQL:
		builder.WriteString("INSERT INTO users(user_id, labels, subscribe, `comment`) VALUES ")
	case Postgres:
		builder.WriteString("INSERT INTO users(user_id, labels, subscribe, comment) VALUES ")
	case ClickHouse:
		builder.WriteString("INSERT INTO users(user_id, labels, subscribe, comment, version) VALUES ")
	}
	var args []interface{}
	for i, user := range users {
		labels, err := json.Marshal(user.Labels)
		if err != nil {
			return errors.Trace(err)
		}
		subscribe, err := json.Marshal(user.Subscribe)
		if err != nil {
			return errors.Trace(err)
		}
		switch d.driver {
		case MySQL:
			builder.WriteString("(?,?,?,?)")
		case Postgres:
			builder.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d)", len(args)+1, len(args)+2, len(args)+3, len(args)+4))
		case ClickHouse:
			builder.WriteString("(?,?,?,?,NOW())")
		}
		if i+1 < len(users) {
			builder.WriteString(",")
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
	txn, err := d.client.Begin()
	if err != nil {
		return errors.Trace(err)
	}
	switch d.driver {
	case MySQL:
		_, err = txn.Exec("DELETE FROM users WHERE user_id = ?", userId)
	case Postgres:
		_, err = txn.Exec("DELETE FROM users WHERE user_id = $1", userId)
	case ClickHouse:
		_, err = txn.Exec("ALTER TABLE users DELETE WHERE user_id = ?", userId)
	}
	if err != nil {
		if err = txn.Rollback(); err != nil {
			return errors.Trace(err)
		}
		return errors.Trace(err)
	}
	switch d.driver {
	case MySQL:
		_, err = txn.Exec("DELETE FROM feedback WHERE user_id = ?", userId)
	case Postgres:
		_, err = txn.Exec("DELETE FROM feedback WHERE user_id = $1", userId)
	case ClickHouse:
		_, err = txn.Exec("ALTER TABLE feedback DELETE WHERE user_id = ?", userId)
	}
	if err != nil {
		if err = txn.Rollback(); err != nil {
			return errors.Trace(err)
		}
		return errors.Trace(err)
	}
	return txn.Commit()
}

// GetUser returns a user from MySQL.
func (d *SQLDatabase) GetUser(userId string) (User, error) {
	var result *sql.Rows
	var err error
	switch d.driver {
	case MySQL:
		result, err = d.client.Query("SELECT user_id, labels, subscribe, `comment` FROM users WHERE user_id = ?", userId)
	case Postgres:
		result, err = d.client.Query("SELECT user_id, labels, subscribe, comment FROM users WHERE user_id = $1", userId)
	case ClickHouse:
		result, err = d.client.Query("SELECT user_id, labels, subscribe, `comment` FROM users WHERE user_id = ?", userId)
	}
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
	return User{}, ErrUserNotExist
}

// GetUsers returns users from MySQL.
func (d *SQLDatabase) GetUsers(cursor string, n int) (string, []User, error) {
	var result *sql.Rows
	var err error
	switch d.driver {
	case MySQL:
		result, err = d.client.Query("SELECT user_id, labels, subscribe, `comment` FROM users "+
			"WHERE user_id >= ? ORDER BY user_id LIMIT ?", cursor, n+1)
	case Postgres:
		result, err = d.client.Query("SELECT user_id, labels, subscribe, comment FROM users "+
			"WHERE user_id >= $1 ORDER BY user_id LIMIT $2", cursor, n+1)
	case ClickHouse:
		result, err = d.client.Query("SELECT user_id, labels, subscribe, `comment` FROM users "+
			"WHERE user_id >= ? ORDER BY user_id LIMIT ?", cursor, n+1)
	}
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
		var result *sql.Rows
		var err error
		switch d.driver {
		case MySQL:
			result, err = d.client.Query("SELECT user_id, labels, subscribe, `comment` FROM users")
		case Postgres:
			result, err = d.client.Query("SELECT user_id, labels, subscribe, comment FROM users")
		case ClickHouse:
			result, err = d.client.Query("SELECT user_id, labels, subscribe, `comment` FROM users")
		}
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
	startTime := time.Now()
	var result *sql.Rows
	var err error
	var builder strings.Builder
	switch d.driver {
	case MySQL, ClickHouse:
		builder.WriteString("SELECT feedback_type, user_id, item_id, time_stamp, `comment` FROM feedback WHERE user_id = ?")
	case Postgres:
		builder.WriteString("SELECT feedback_type, user_id, item_id, time_stamp, comment FROM feedback WHERE user_id = $1")
	}
	if !withFuture {
		builder.WriteString(" AND time_stamp <= NOW() ")
	}
	args := []interface{}{userId}
	if len(feedbackTypes) > 0 {
		builder.WriteString(" AND feedback_type IN (")
		for i, feedbackType := range feedbackTypes {
			switch d.driver {
			case MySQL, ClickHouse:
				builder.WriteString("?")
			case Postgres:
				builder.WriteString(fmt.Sprintf("$%d", len(args)+1))
			}
			if i+1 < len(feedbackTypes) {
				builder.WriteString(",")
			}
			args = append(args, feedbackType)
		}
		builder.WriteString(")")
	}
	result, err = d.client.Query(builder.String(), args...)
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
	GetUserFeedbackLatency.Observe(time.Since(startTime).Seconds())
	return feedbacks, nil
}

// BatchInsertFeedback insert a batch feedback into MySQL.
// If insertUser set, new users will be insert to user table.
// If insertItem set, new items will be insert to item table.
func (d *SQLDatabase) BatchInsertFeedback(feedback []Feedback, insertUser, insertItem, overwrite bool) error {
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
			builder.WriteString("INSERT IGNORE users(user_id) VALUES ")
		case Postgres:
			builder.WriteString("INSERT INTO users(user_id) VALUES ")
		case ClickHouse:
			builder.WriteString("INSERT INTO users(user_id, version) VALUES ")
		}
		var args []interface{}
		for i, user := range userList {
			switch d.driver {
			case MySQL:
				builder.WriteString("(?)")
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
			var rs *sql.Rows
			var err error
			switch d.driver {
			case MySQL:
				rs, err = d.client.Query("SELECT user_id FROM users WHERE user_id = ?", user)
			case Postgres:
				rs, err = d.client.Query("SELECT user_id FROM users WHERE user_id = $1", user)
			}
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
			builder.WriteString("INSERT IGNORE items(item_id) VALUES ")
		case Postgres:
			builder.WriteString("INSERT INTO items(item_id) VALUES ")
		case ClickHouse:
			builder.WriteString("INSERT INTO items(item_id, version) VALUES ")
		}
		var args []interface{}
		for i, item := range itemList {
			switch d.driver {
			case MySQL:
				builder.WriteString("(?)")
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
			var rs *sql.Rows
			var err error
			switch d.driver {
			case MySQL:
				rs, err = d.client.Query("SELECT item_id FROM items WHERE item_id = ?", item)
			case Postgres:
				rs, err = d.client.Query("SELECT item_id FROM items WHERE item_id = $1", item)
			}
			if err != nil {
				return errors.Trace(err)
			} else if !rs.Next() {
				users.Remove(item)
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
	case ClickHouse:
		builder.WriteString("INSERT INTO feedback(feedback_type, user_id, item_id, time_stamp, `comment`, version) VALUES ")
	case Postgres:
		builder.WriteString("INSERT INTO feedback(feedback_type, user_id, item_id, time_stamp, comment) VALUES ")
	}
	var args []interface{}
	for i, f := range feedback {
		if users.Has(f.UserId) && items.Has(f.ItemId) {
			switch d.driver {
			case MySQL:
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
			if i+1 < len(feedback) {
				builder.WriteString(",")
			}
			args = append(args, f.FeedbackType, f.UserId, f.ItemId, f.Timestamp, f.Comment)
		}
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
	var result *sql.Rows
	var err error
	var builder strings.Builder
	switch d.driver {
	case MySQL, ClickHouse:
		builder.WriteString("SELECT feedback_type, user_id, item_id, time_stamp, `comment` FROM feedback WHERE time_stamp <= NOW() AND (feedback_type, user_id, item_id) >= (?,?,?)")
	case Postgres:
		builder.WriteString("SELECT feedback_type, user_id, item_id, time_stamp, comment FROM feedback WHERE time_stamp <= NOW() AND (feedback_type, user_id, item_id) >= ($1,$2,$3)")
	}
	args := []interface{}{cursorKey.FeedbackType, cursorKey.UserId, cursorKey.ItemId}
	if len(feedbackTypes) > 0 {
		builder.WriteString(" AND feedback_type IN (")
		for i, feedbackType := range feedbackTypes {
			switch d.driver {
			case MySQL, ClickHouse:
				builder.WriteString("?")
			case Postgres:
				builder.WriteString(fmt.Sprintf("$%d", len(args)+1))
			}
			if i+1 < len(feedbackTypes) {
				builder.WriteString(",")
			}
			args = append(args, feedbackType)
		}
		builder.WriteString(")")
	}
	if timeLimit != nil {
		switch d.driver {
		case MySQL, ClickHouse:
			builder.WriteString(" AND time_stamp >= ?")
		case Postgres:
			builder.WriteString(fmt.Sprintf(" AND time_stamp >= $%d", len(args)+1))
		}
		args = append(args, *timeLimit)
	}
	switch d.driver {
	case MySQL, ClickHouse:
		builder.WriteString(" ORDER BY feedback_type, user_id, item_id LIMIT ?")
	case Postgres:
		builder.WriteString(fmt.Sprintf(" ORDER BY feedback_type, user_id, item_id LIMIT $%d", len(args)+1))
	}
	args = append(args, n+1)
	result, err = d.client.Query(builder.String(), args...)
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
		var result *sql.Rows
		var err error
		var builder strings.Builder
		switch d.driver {
		case MySQL, ClickHouse:
			builder.WriteString("SELECT feedback_type, user_id, item_id, time_stamp, `comment` FROM feedback WHERE time_stamp <= NOW()")
		case Postgres:
			builder.WriteString("SELECT feedback_type, user_id, item_id, time_stamp, comment FROM feedback WHERE time_stamp <= NOW()")
		}
		var args []interface{}
		if len(feedbackTypes) > 0 {
			builder.WriteString(" AND feedback_type IN (")
			for i, feedbackType := range feedbackTypes {
				switch d.driver {
				case MySQL, ClickHouse:
					builder.WriteString("?")
				case Postgres:
					builder.WriteString(fmt.Sprintf("$%d", len(args)+1))
				}
				if i+1 < len(feedbackTypes) {
					builder.WriteString(",")
				}
				args = append(args, feedbackType)
			}
			builder.WriteString(")")
		}
		if timeLimit != nil {
			switch d.driver {
			case MySQL, ClickHouse:
				builder.WriteString(" AND time_stamp >= ?")
			case Postgres:
				builder.WriteString(fmt.Sprintf(" AND time_stamp >= $%d", len(args)+1))
			}
			args = append(args, *timeLimit)
		}
		result, err = d.client.Query(builder.String(), args...)
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
	startTime := time.Now()
	var result *sql.Rows
	var err error
	var builder strings.Builder
	switch d.driver {
	case MySQL, ClickHouse:
		builder.WriteString("SELECT feedback_type, user_id, item_id, time_stamp, `comment` FROM feedback WHERE user_id = ? AND item_id = ?")
	case Postgres:
		builder.WriteString("SELECT feedback_type, user_id, item_id, time_stamp, comment FROM feedback WHERE user_id = $1 AND item_id = $2")
	}
	args := []interface{}{userId, itemId}
	if len(feedbackTypes) > 0 {
		builder.WriteString(" AND feedback_type IN (")
		for i, feedbackType := range feedbackTypes {
			switch d.driver {
			case MySQL, ClickHouse:
				builder.WriteString("?")
			case Postgres:
				builder.WriteString(fmt.Sprintf("$%d", i+3))
			}
			if i+1 < len(feedbackTypes) {
				builder.WriteString(",")
			}
			args = append(args, feedbackType)
		}
		builder.WriteString(")")
	}
	result, err = d.client.Query(builder.String(), args...)
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
	GetUserItemFeedbackLatency.Observe(time.Since(startTime).Seconds())
	return feedbacks, nil
}

// DeleteUserItemFeedback deletes a feedback by user id and item id from MySQL.
func (d *SQLDatabase) DeleteUserItemFeedback(userId, itemId string, feedbackTypes ...string) (int, error) {
	var rs sql.Result
	var err error
	var builder strings.Builder
	switch d.driver {
	case MySQL:
		builder.WriteString("DELETE FROM feedback WHERE user_id = ? AND item_id = ?")
	case Postgres:
		builder.WriteString("DELETE FROM feedback WHERE user_id = $1 AND item_id = $2")
	case ClickHouse:
		builder.WriteString("ALTER TABLE feedback DELETE WHERE user_id = ? AND item_id = ?")
	}
	args := []interface{}{userId, itemId}
	if len(feedbackTypes) > 0 {
		builder.WriteString(" AND feedback_type IN (")
		for i, feedbackType := range feedbackTypes {
			switch d.driver {
			case MySQL, ClickHouse:
				builder.WriteString("?")
			case Postgres:
				builder.WriteString(fmt.Sprintf("$%d", len(args)+1))
			}
			if i+1 < len(feedbackTypes) {
				builder.WriteString(",")
			}
			args = append(args, feedbackType)
		}
		builder.WriteString(")")
	}
	rs, err = d.client.Exec(builder.String(), args...)
	if err != nil {
		return 0, errors.Trace(err)
	}
	deleteCount, err := rs.RowsAffected()
	if err != nil && d.driver != ClickHouse {
		return 0, errors.Trace(err)
	}
	return int(deleteCount), nil
}

// GetClickThroughRate computes the click-through-rate of a specified date.
func (d *SQLDatabase) GetClickThroughRate(date time.Time, positiveTypes, readTypes []string) (float64, error) {
	builder := strings.Builder{}
	// Get the average of click-through rates
	switch d.driver {
	case MySQL:
		builder.WriteString("SELECT IFNULL(AVG(user_ctr),0) FROM (")
	case ClickHouse:
		builder.WriteString("SELECT IF(isFinite(AVG(user_ctr)),AVG(user_ctr),0) FROM (")
	case Postgres:
		builder.WriteString("SELECT COALESCE(AVG(user_ctr),0) FROM (")
	}
	var args []interface{}
	// Get click-through rates
	switch d.driver {
	case MySQL:
		builder.WriteString("SELECT COUNT(positive_feedback.user_id) / COUNT(read_feedback.user_id) AS user_ctr FROM (")
	case ClickHouse:
		builder.WriteString("SELECT SUM(notEmpty(positive_feedback.user_id)) / SUM(notEmpty(read_feedback.user_id)) AS user_ctr FROM (")
	case Postgres:
		builder.WriteString("SELECT COUNT(positive_feedback.user_id) :: DOUBLE PRECISION / COUNT(read_feedback.user_id) :: DOUBLE PRECISION AS user_ctr FROM (")
	}
	// Get positive feedback
	switch d.driver {
	case MySQL, ClickHouse:
		builder.WriteString("SELECT DISTINCT user_id, item_id FROM feedback WHERE DATE(time_stamp) = DATE(?) AND feedback_type IN (")
	case Postgres:
		builder.WriteString(fmt.Sprintf("SELECT DISTINCT user_id, item_id FROM feedback WHERE DATE(time_stamp) = DATE($%d) AND feedback_type IN (", len(args)+1))
	}
	args = append(args, date)
	for i, positiveType := range positiveTypes {
		if i > 0 {
			builder.WriteString(",")
		}
		switch d.driver {
		case MySQL, ClickHouse:
			builder.WriteString("?")
		case Postgres:
			builder.WriteString(fmt.Sprintf("$%d", len(args)+1))
		}
		args = append(args, positiveType)
	}
	builder.WriteString(")) AS positive_feedback RIGHT JOIN (")
	// Get read feedback
	switch d.driver {
	case MySQL, ClickHouse:
		builder.WriteString("SELECT DISTINCT user_id, item_id FROM feedback WHERE DATE(time_stamp) = DATE(?) AND feedback_type IN (")
	case Postgres:
		builder.WriteString(fmt.Sprintf("SELECT DISTINCT user_id, item_id FROM feedback WHERE DATE(time_stamp) = DATE($%d) AND feedback_type IN (", len(args)+1))
	}
	args = append(args, date)
	for i, readType := range readTypes {
		if i > 0 {
			builder.WriteString(",")
		}
		switch d.driver {
		case MySQL, ClickHouse:
			builder.WriteString("?")
		case Postgres:
			builder.WriteString(fmt.Sprintf("$%d", len(args)+1))
		}
		args = append(args, readType)
	}
	builder.WriteString(")) AS read_feedback ON positive_feedback.user_id = read_feedback.user_id AND positive_feedback.item_id = read_feedback.item_id GROUP BY read_feedback.user_id ")
	// users must have at least one positive feedback
	switch d.driver {
	case MySQL:
		builder.WriteString("HAVING COUNT(positive_feedback.user_id) > 0) AS user_ctr")
	case ClickHouse:
		builder.WriteString("HAVING SUM(notEmpty(positive_feedback.user_id)) > 0) AS user_ctr")
	case Postgres:
		builder.WriteString("HAVING COUNT(positive_feedback.user_id) > 0) AS user_ctr")
	}
	base.Logger().Info("get click through rate from MySQL", zap.String("query", builder.String()))
	rs, err := d.client.Query(builder.String(), args...)
	if err != nil {
		return 0, errors.Trace(err)
	}
	defer rs.Close()
	if rs.Next() {
		var ctr float64
		if err = rs.Scan(&ctr); err != nil {
			return 0, errors.Trace(err)
		}
		return ctr, nil
	}
	return 0, nil
}
