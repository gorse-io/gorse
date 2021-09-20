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
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"github.com/juju/errors"
	"github.com/scylladb/go-set/strset"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	prefixItem     = "item/"     // prefix for items
	prefixUser     = "user/"     // prefix for users
	prefixFeedback = "feedback/" // prefix for feedback
	prefixMeasure  = "measure/"  // prefix for measurements
)

// Redis use Redis as data storage, but used for test only.
type Redis struct {
	client *redis.Client
}

// Optimize is used by ClickHouse only.
func (r *Redis) Optimize() error {
	return nil
}

// Init does nothing.
func (r *Redis) Init() error {
	return nil
}

// Close Redis connection.
func (r *Redis) Close() error {
	return r.client.Close()
}

// InsertMeasurement insert a measurement into Redis.
func (r *Redis) InsertMeasurement(measurement Measurement) error {
	var ctx = context.Background()
	data, err := json.Marshal(measurement)
	if err != nil {
		return errors.Trace(err)
	}
	if err = r.client.Set(ctx, prefixMeasure+measurement.Name+"/"+measurement.Timestamp.String(),
		data, 0).Err(); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// GetMeasurements returns recent measurements from Redis.
func (r *Redis) GetMeasurements(name string, n int) ([]Measurement, error) {
	var ctx = context.Background()
	measurements := make([]Measurement, 0)
	var err error
	var cursor uint64
	var keys []string
	for {
		keys, cursor, err = r.client.Scan(ctx, cursor, prefixMeasure+name+"/*", 0).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			data, err := r.client.Get(ctx, key).Result()
			if err != nil {
				return measurements, err
			}
			var measurement Measurement
			err = json.Unmarshal([]byte(data), &measurement)
			if err != nil {
				return measurements, err
			}
			measurements = append(measurements, measurement)
		}
		if cursor == 0 {
			break
		}
	}
	// sort measurements by timestamp
	s := &sortMeasurements{measurements: measurements}
	sort.Sort(s)
	if len(measurements) > n {
		measurements = measurements[:n]
	}
	return measurements, nil
}

type sortMeasurements struct {
	measurements []Measurement
}

func (s *sortMeasurements) Len() int {
	return len(s.measurements)
}

func (s *sortMeasurements) Less(i, j int) bool {
	return s.measurements[i].Timestamp.Unix() > s.measurements[j].Timestamp.Unix()
}

func (s *sortMeasurements) Swap(i, j int) {
	s.measurements[i], s.measurements[j] = s.measurements[j], s.measurements[i]
}

// insertItem inserts an item into Redis.
func (r *Redis) insertItem(item Item) error {
	var ctx = context.Background()
	// write item
	data, err := json.Marshal(item)
	if err != nil {
		return errors.Trace(err)
	}
	if err = r.client.Set(ctx, prefixItem+item.ItemId, data, 0).Err(); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// BatchInsertItems inserts a batch of items into Redis.
func (r *Redis) BatchInsertItems(items []Item) error {
	for _, item := range items {
		if err := r.insertItem(item); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (r *Redis) ForFeedback(ctx context.Context, action func(key, thisFeedbackType, thisUserId, thisItemId string) error) error {
	cursor := uint64(0)
	var err error
	var keys []string
	for {
		keys, cursor, err = r.client.Scan(ctx, cursor, prefixFeedback+"*", 0).Result()
		if err != nil {
			return errors.Trace(err)
		}
		for _, key := range keys {
			thisFeedbackType, thisUserId, thisItemId := parseFeedbackKey(key)
			if err = action(key, thisFeedbackType, thisUserId, thisItemId); err != nil {
				return errors.Trace(err)
			}
		}
		if cursor == 0 {
			break
		}
	}
	return nil
}

// DeleteItem deletes a item from Redis.
func (r *Redis) DeleteItem(itemId string) error {
	var ctx = context.Background()
	// remove user
	if err := r.client.Del(ctx, prefixItem+itemId).Err(); err != nil {
		return errors.Trace(err)
	}
	// remove feedback
	return r.ForFeedback(ctx, func(key, _, _, thisItemId string) error {
		if thisItemId == itemId {
			// remove feedbacks
			return r.client.Del(ctx, key).Err()
		}
		return nil
	})
}

// GetItem get a item from Redis.
func (r *Redis) GetItem(itemId string) (Item, error) {
	var ctx = context.Background()
	data, err := r.client.Get(ctx, prefixItem+itemId).Result()
	if err != nil {
		if err == redis.Nil {
			return Item{}, ErrItemNotExist
		}
		return Item{}, err
	}
	var item Item
	err = json.Unmarshal([]byte(data), &item)
	return item, err
}

// GetItems returns items from Redis.
func (r *Redis) GetItems(cursor string, n int, timeLimit *time.Time) (string, []Item, error) {
	var ctx = context.Background()
	var err error
	cursorNum := uint64(0)
	if len(cursor) > 0 {
		cursorNum, err = strconv.ParseUint(cursor, 10, 64)
		if err != nil {
			return "", nil, err
		}
	}
	items := make([]Item, 0)
	// scan * from zero util cursor is zero
	var keys []string
	keys, cursorNum, err = r.client.Scan(ctx, cursorNum, prefixItem+"*", int64(n)).Result()
	if err != nil {
		return "", nil, err
	}
	for _, key := range keys {
		data, err := r.client.Get(ctx, key).Result()
		if err != nil {
			return "", nil, err
		}
		var item Item
		err = json.Unmarshal([]byte(data), &item)
		if err != nil {
			return "", nil, err
		}
		// compare timestamp
		if timeLimit != nil && item.Timestamp.Unix() < timeLimit.Unix() {
			continue
		}
		items = append(items, item)
	}
	if cursorNum == 0 {
		cursor = ""
	} else {
		cursor = strconv.Itoa(int(cursorNum))
	}
	return cursor, items, nil
}

// GetItemStream read items from Redis by stream.
func (r *Redis) GetItemStream(batchSize int, timeLimit *time.Time) (chan []Item, chan error) {
	itemChan := make(chan []Item, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(itemChan)
		defer close(errChan)
		ctx := context.Background()
		var cursor uint64
		var keys []string
		var err error
		items := make([]Item, 0, batchSize)
		for {
			keys, cursor, err = r.client.Scan(ctx, cursor, prefixItem+"*", int64(batchSize)).Result()
			if err != nil {
				errChan <- errors.Trace(err)
				return
			}
			for _, key := range keys {
				data, err := r.client.Get(ctx, key).Result()
				if err != nil {
					errChan <- errors.Trace(err)
					return
				}
				var item Item
				err = json.Unmarshal([]byte(data), &item)
				if err != nil {
					errChan <- errors.Trace(err)
					return
				}
				// compare timestamp
				if timeLimit != nil && item.Timestamp.Unix() < timeLimit.Unix() {
					continue
				}
				items = append(items, item)
				if len(items) == batchSize {
					itemChan <- items
					items = make([]Item, 0, batchSize)
				}
			}
			if cursor == 0 {
				break
			}
		}
		if len(items) > 0 {
			itemChan <- items
		}
		errChan <- nil
	}()
	return itemChan, errChan
}

// GetItemFeedback returns feedback of an item from Redis.
func (r *Redis) GetItemFeedback(itemId string, feedbackTypes ...string) ([]Feedback, error) {
	var ctx = context.Background()
	feedback := make([]Feedback, 0)
	feedbackTypeSet := strset.New(feedbackTypes...)
	err := r.ForFeedback(ctx, func(key, thisFeedbackType, _, thisItemId string) error {
		if itemId == thisItemId && (feedbackTypeSet.IsEmpty() || feedbackTypeSet.Has(thisFeedbackType)) {
			val, err := r.getFeedbackInternal(key)
			if err != nil {
				return errors.Trace(err)
			}
			feedback = append(feedback, val)
		}
		return nil
	})
	return feedback, err
}

// insertUser inserts a user into Redis.
func (r *Redis) insertUser(user User) error {
	var ctx = context.Background()
	data, err := json.Marshal(user)
	if err != nil {
		return errors.Trace(err)
	}
	return r.client.Set(ctx, prefixUser+user.UserId, data, 0).Err()
}

// BatchInsertUsers inserts a batch pf user into Redis.
func (r *Redis) BatchInsertUsers(users []User) error {
	for _, user := range users {
		if err := r.insertUser(user); err != nil {
			return err
		}
	}
	return nil
}

// DeleteUser deletes a user from Redis.
func (r *Redis) DeleteUser(userId string) error {
	var ctx = context.Background()
	// remove user
	if err := r.client.Del(ctx, prefixUser+userId).Err(); err != nil {
		return errors.Trace(err)
	}
	// remove feedback
	return r.ForFeedback(ctx, func(key, thisFeedbackType, thisUserId, thisItemId string) error {
		if thisUserId == userId {
			return r.client.Del(ctx, key).Err()
		}
		return nil
	})
}

// GetUser returns a user from Redis.
func (r *Redis) GetUser(userId string) (User, error) {
	var ctx = context.Background()
	val, err := r.client.Get(ctx, prefixUser+userId).Result()
	if err != nil {
		if err == redis.Nil {
			return User{}, ErrUserNotExist
		}
		return User{}, err
	}
	var user User
	err = json.Unmarshal([]byte(val), &user)
	if err != nil {
		return User{}, err
	}
	return user, err
}

// GetUsers returns users from Redis.
func (r *Redis) GetUsers(cursor string, n int) (string, []User, error) {
	var ctx = context.Background()
	var err error
	cursorNum := uint64(0)
	if len(cursor) > 0 {
		cursorNum, err = strconv.ParseUint(cursor, 10, 64)
		if err != nil {
			return "", nil, err
		}
	}
	users := make([]User, 0)
	var keys []string
	keys, cursorNum, err = r.client.Scan(ctx, cursorNum, prefixUser+"*", int64(n)).Result()
	if err != nil {
		return "", nil, err
	}
	for _, key := range keys {
		var user User
		val, err := r.client.Get(ctx, key).Result()
		if err != nil {
			return "", nil, err
		}
		err = json.Unmarshal([]byte(val), &user)
		if err != nil {
			return "", nil, err
		}
		users = append(users, user)
	}
	if cursorNum == 0 {
		cursor = ""
	} else {
		cursor = strconv.Itoa(int(cursorNum))
	}
	return cursor, users, nil
}

// GetUserStream read users from Redis by stream.
func (r *Redis) GetUserStream(batchSize int) (chan []User, chan error) {
	userChan := make(chan []User, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(userChan)
		defer close(errChan)
		ctx := context.Background()
		var cursor uint64
		var keys []string
		var err error
		users := make([]User, 0, batchSize)
		for {
			keys, cursor, err = r.client.Scan(ctx, cursor, prefixUser+"*", int64(batchSize)).Result()
			if err != nil {
				errChan <- errors.Trace(err)
				return
			}
			for _, key := range keys {
				var user User
				val, err := r.client.Get(ctx, key).Result()
				if err != nil {
					errChan <- errors.Trace(err)
					return
				}
				err = json.Unmarshal([]byte(val), &user)
				if err != nil {
					errChan <- errors.Trace(err)
					return
				}
				users = append(users, user)
				if len(users) == batchSize {
					userChan <- users
					users = make([]User, 0, batchSize)
				}
			}
			if cursor == 0 {
				break
			}
		}
		if len(users) > 0 {
			userChan <- users
		}
		errChan <- nil
	}()
	return userChan, errChan
}

// GetUserFeedback returns feedback of a user from Redis.
func (r *Redis) GetUserFeedback(userId string, feedbackTypes ...string) ([]Feedback, error) {
	var ctx = context.Background()
	feedback := make([]Feedback, 0)
	feedbackTypeSet := strset.New(feedbackTypes...)
	// get itemId list by userId
	err := r.ForFeedback(ctx, func(key, thisFeedbackType, thisUserId, thisItemId string) error {
		if thisUserId == userId && (feedbackTypeSet.IsEmpty() || feedbackTypeSet.Has(thisFeedbackType)) {
			val, err := r.getFeedbackInternal(key)
			if err != nil {
				return errors.Trace(err)
			}
			feedback = append(feedback, val)
		}
		return nil
	})
	return feedback, err
}

func (r *Redis) getFeedbackInternal(key string) (Feedback, error) {
	var ctx = context.Background()
	// get feedback by feedbackKey
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return Feedback{}, err
	}
	var feedback Feedback
	err = json.Unmarshal([]byte(val), &feedback)
	if err != nil {
		return Feedback{}, err
	}
	return feedback, err
}

func createFeedbackKey(key FeedbackKey) string {
	return prefixFeedback + key.FeedbackType + "/" + key.UserId + "/" + key.ItemId
}

func parseFeedbackKey(key string) (feedbackType, userId, itemId string) {
	fields := strings.Split(key, "/")
	return fields[1], fields[2], fields[3]
}

// insertFeedback insert a feedback into Redis.
// If insertUser set, a new user will be insert to user table.
// If insertItem set, a new item will be insert to item table.
func (r *Redis) insertFeedback(feedback Feedback, insertUser, insertItem bool) error {
	var ctx = context.Background()
	val, err := json.Marshal(feedback)
	if err != nil {
		return errors.Trace(err)
	}
	// insert feedback
	err = r.client.Set(ctx, createFeedbackKey(feedback.FeedbackKey), val, 0).Err()
	if err != nil {
		return errors.Trace(err)
	}
	// insert user
	if insertUser {
		if exist, err := r.client.Exists(ctx, prefixUser+feedback.UserId).Result(); err != nil {
			return errors.Trace(err)
		} else if exist == 0 {
			user := User{UserId: feedback.UserId}
			data, err := json.Marshal(user)
			if err != nil {
				return errors.Trace(err)
			}
			if err = r.client.Set(ctx, prefixUser+feedback.UserId, data, 0).Err(); err != nil {
				return errors.Trace(err)
			}
		}
	}
	// Insert item
	if insertItem {
		if exist, err := r.client.Exists(ctx, prefixItem+feedback.ItemId).Result(); err != nil {
			return errors.Trace(err)
		} else if exist == 0 {
			item := Item{ItemId: feedback.ItemId}
			data, err := json.Marshal(item)
			if err != nil {
				return errors.Trace(err)
			}
			if err = r.client.Set(ctx, prefixItem+feedback.ItemId, data, 0).Err(); err != nil {
				return errors.Trace(err)
			}
		}
	}
	return nil
}

// BatchInsertFeedback insert a batch feedback into Redis.
// If insertUser set, new users will be insert to user table.
// If insertItem set, new items will be insert to item table.
func (r *Redis) BatchInsertFeedback(feedback []Feedback, insertUser, insertItem bool) error {
	for _, temp := range feedback {
		if err := r.insertFeedback(temp, insertUser, insertItem); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// GetFeedback returns feedback from Redis.
func (r *Redis) GetFeedback(_ string, _ int, timeLimit *time.Time, feedbackTypes ...string) (string, []Feedback, error) {
	var ctx = context.Background()
	feedback := make([]Feedback, 0)
	feedbackTypeSet := strset.New(feedbackTypes...)
	err := r.ForFeedback(ctx, func(key, thisFeedbackType, thisUserId, thisItemId string) error {
		if feedbackTypeSet.IsEmpty() || feedbackTypeSet.Has(thisFeedbackType) {
			val, err := r.getFeedbackInternal(key)
			if err != nil {
				return errors.Trace(err)
			}
			if timeLimit != nil && val.Timestamp.Unix() < timeLimit.Unix() {
				return nil
			}
			feedback = append(feedback, val)
		}
		return nil
	})
	return "", feedback, err
}

// GetFeedbackStream reads feedback by stream.
func (r *Redis) GetFeedbackStream(batchSize int, timeLimit *time.Time, feedbackTypes ...string) (chan []Feedback, chan error) {
	feedbackChan := make(chan []Feedback, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(feedbackChan)
		defer close(errChan)
		feedbackTypeSet := strset.New(feedbackTypes...)
		ctx := context.Background()
		feedback := make([]Feedback, 0, batchSize)
		err := r.ForFeedback(ctx, func(key, thisFeedbackType, thisUserId, thisItemId string) error {
			if feedbackTypeSet.IsEmpty() || feedbackTypeSet.Has(thisFeedbackType) {
				val, err := r.getFeedbackInternal(key)
				if err != nil {
					return errors.Trace(err)
				}
				if timeLimit != nil && val.Timestamp.Unix() < timeLimit.Unix() {
					return nil
				}
				feedback = append(feedback, val)
				if len(feedback) == batchSize {
					feedbackChan <- feedback
					feedback = make([]Feedback, 0, batchSize)
				}
			}
			return nil
		})
		if len(feedback) > 0 {
			feedbackChan <- feedback
		}
		errChan <- errors.Trace(err)
	}()
	return feedbackChan, errChan
}

// GetUserItemFeedback gets a feedback by user id and item id from Redis.
func (r *Redis) GetUserItemFeedback(userId, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	var ctx = context.Background()
	feedback := make([]Feedback, 0)
	feedbackTypeSet := strset.New(feedbackTypes...)
	err := r.ForFeedback(ctx, func(key, thisFeedbackType, thisUserId, thisItemId string) error {
		if thisUserId == userId && thisItemId == itemId && (feedbackTypeSet.IsEmpty() || feedbackTypeSet.Has(thisFeedbackType)) {
			val, err := r.getFeedbackInternal(key)
			if err != nil {
				return errors.Trace(err)
			}
			feedback = append(feedback, val)
		}
		return nil
	})
	return feedback, err
}

// DeleteUserItemFeedback deletes a feedback by user id and item id from Redis.
func (r *Redis) DeleteUserItemFeedback(userId, itemId string, feedbackTypes ...string) (int, error) {
	var ctx = context.Background()
	feedbackTypeSet := strset.New(feedbackTypes...)
	deleteCount := 0
	err := r.ForFeedback(ctx, func(key, thisFeedbackType, thisUserId, thisItemId string) error {
		if thisUserId == userId && thisItemId == itemId && (feedbackTypeSet.IsEmpty() || feedbackTypeSet.Has(thisFeedbackType)) {
			r.client.Del(ctx, key)
			deleteCount++
		}
		return nil
	})
	return deleteCount, err
}

// GetClickThroughRate method of Redis returns ErrUnsupported.
func (r *Redis) GetClickThroughRate(_ time.Time, _ []string, _ string) (float64, error) {
	return 0, ErrUnsupported
}
