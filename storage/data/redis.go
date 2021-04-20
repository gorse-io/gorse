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
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"sort"
	"strconv"
	"strings"
)

const (
	prefixItem     = "item/"     // prefix for items
	prefixUser     = "user/"     // prefix for users
	prefixFeedback = "feedback/" // prefix for feedback
	prefixMeasure  = "measure/"  // prefix for measurements
)

type Redis struct {
	client *redis.Client
}

func (redis *Redis) Init() error {
	return nil
}

func (redis *Redis) Close() error {
	return redis.client.Close()
}

func (redis *Redis) InsertMeasurement(measurement Measurement) error {
	var ctx = context.Background()
	data, err := json.Marshal(measurement)
	if err != nil {
		return err
	}
	if err = redis.client.Set(ctx, prefixMeasure+measurement.Name+"/"+measurement.Timestamp.String(),
		data, 0).Err(); err != nil {
		return err
	}
	return nil
}

func (redis *Redis) GetMeasurements(name string, n int) ([]Measurement, error) {
	var ctx = context.Background()
	measurements := make([]Measurement, 0)
	var err error
	var cursor uint64
	var keys []string
	for {
		keys, cursor, err = redis.client.Scan(ctx, cursor, prefixMeasure+name+"/*", 0).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			data, err := redis.client.Get(ctx, key).Result()
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

func (redis *Redis) InsertItem(item Item) error {
	var ctx = context.Background()
	// write item
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	if err = redis.client.Set(ctx, prefixItem+item.ItemId, data, 0).Err(); err != nil {
		return err
	}
	return nil
}

func (redis *Redis) BatchInsertItem(items []Item) error {
	for _, item := range items {
		if err := redis.InsertItem(item); err != nil {
			return err
		}
	}
	return nil
}

func (redis *Redis) ForFeedback(ctx context.Context, action func(key, thisFeedbackType, thisUserId, thisItemId string) error) error {
	cursor := uint64(0)
	var err error
	var keys []string
	for {
		keys, cursor, err = redis.client.Scan(ctx, cursor, prefixFeedback+"*", 0).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			thisFeedbackType, thisUserId, thisItemId := parseFeedbackKey(key)
			if err = action(key, thisFeedbackType, thisUserId, thisItemId); err != nil {
				return err
			}
		}
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (redis *Redis) DeleteItem(itemId string) error {
	var ctx = context.Background()
	// remove user
	if err := redis.client.Del(ctx, prefixItem+itemId).Err(); err != nil {
		return err
	}
	// remove feedback
	return redis.ForFeedback(ctx, func(key, _, _, thisItemId string) error {
		if thisItemId == itemId {
			// remove feedbacks
			return redis.client.Del(ctx, key).Err()
		}
		return nil
	})
}

func (redis *Redis) GetItem(itemId string) (Item, error) {
	var ctx = context.Background()
	data, err := redis.client.Get(ctx, prefixItem+itemId).Result()
	if err != nil {
		return Item{}, err
	}
	var item Item
	err = json.Unmarshal([]byte(data), &item)
	return item, err
}

func (redis *Redis) GetItems(cursor string, n int) (string, []Item, error) {
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
	keys, cursorNum, err = redis.client.Scan(ctx, cursorNum, prefixItem+"*", int64(n)).Result()
	if err != nil {
		return "", nil, err
	}
	for _, key := range keys {
		data, err := redis.client.Get(ctx, key).Result()
		if err != nil {
			return "", nil, err
		}
		var item Item
		err = json.Unmarshal([]byte(data), &item)
		if err != nil {
			return "", nil, err
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

func (redis *Redis) GetItemFeedback(itemId string, feedbackType *string) ([]Feedback, error) {
	var ctx = context.Background()
	feedback := make([]Feedback, 0)
	err := redis.ForFeedback(ctx, func(key, thisFeedbackType, _, thisItemId string) error {
		if itemId == thisItemId && (feedbackType == nil || *feedbackType == thisFeedbackType) {
			val, err := redis.getFeedback(key)
			if err != nil {
				return err
			}
			feedback = append(feedback, val)
		}
		return nil
	})
	return feedback, err
}

func (redis *Redis) InsertUser(user User) error {
	var ctx = context.Background()
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return redis.client.Set(ctx, prefixUser+user.UserId, data, 0).Err()
}

func (redis *Redis) DeleteUser(userId string) error {
	var ctx = context.Background()
	// remove user
	if err := redis.client.Del(ctx, prefixUser+userId).Err(); err != nil {
		return err
	}
	// remove feedback
	return redis.ForFeedback(ctx, func(key, thisFeedbackType, thisUserId, thisItemId string) error {
		if thisUserId == userId {
			return redis.client.Del(ctx, key).Err()
		}
		return nil
	})
}

func (redis *Redis) GetUser(userId string) (User, error) {
	var ctx = context.Background()
	val, err := redis.client.Get(ctx, prefixUser+userId).Result()
	if err != nil {
		return User{}, err
	}
	var user User
	err = json.Unmarshal([]byte(val), &user)
	if err != nil {
		return User{}, err
	}
	return user, err
}

func (redis *Redis) GetUsers(cursor string, n int) (string, []User, error) {
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
	keys, cursorNum, err = redis.client.Scan(ctx, cursorNum, prefixUser+"*", int64(n)).Result()
	if err != nil {
		return "", nil, err
	}
	for _, key := range keys {
		var user User
		val, err := redis.client.Get(ctx, key).Result()
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

func (redis *Redis) GetUserFeedback(userId string, feedbackType *string) ([]Feedback, error) {
	var ctx = context.Background()
	feedback := make([]Feedback, 0)

	// get itemId list by userId
	err := redis.ForFeedback(ctx, func(key, thisFeedbackType, thisUserId, thisItemId string) error {
		if thisUserId == userId && (feedbackType == nil || *feedbackType == thisFeedbackType) {
			val, err := redis.getFeedback(key)
			if err != nil {
				return err
			}
			feedback = append(feedback, val)
		}
		return nil
	})
	return feedback, err
}

func (redis *Redis) getFeedback(key string) (Feedback, error) {
	var ctx = context.Background()
	// get feedback by feedbackKey
	val, err := redis.client.Get(ctx, key).Result()
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

func (redis *Redis) InsertFeedback(feedback Feedback, insertUser, insertItem bool) error {
	var ctx = context.Background()
	val, err := json.Marshal(feedback)
	if err != nil {
		return err
	}
	// insert feedback
	err = redis.client.Set(ctx, createFeedbackKey(feedback.FeedbackKey), val, 0).Err()
	if err != nil {
		return err
	}
	// insert user
	if insertUser {
		if exist, err := redis.client.Exists(ctx, prefixUser+feedback.UserId).Result(); err != nil {
			return err
		} else if exist == 0 {
			user := User{UserId: feedback.UserId}
			data, err := json.Marshal(user)
			if err != nil {
				return err
			}
			if err = redis.client.Set(ctx, prefixUser+feedback.UserId, data, 0).Err(); err != nil {
				return err
			}
		}
	}
	// Insert item
	if insertItem {
		if exist, err := redis.client.Exists(ctx, prefixItem+feedback.ItemId).Result(); err != nil {
			return err
		} else if exist == 0 {
			item := Item{ItemId: feedback.ItemId}
			data, err := json.Marshal(item)
			if err != nil {
				return err
			}
			if err = redis.client.Set(ctx, prefixItem+feedback.ItemId, data, 0).Err(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (redis *Redis) BatchInsertFeedback(feedback []Feedback, insertUser, insertItem bool) error {
	for _, temp := range feedback {
		if err := redis.InsertFeedback(temp, insertUser, insertItem); err != nil {
			return err
		}
	}
	return nil
}

func (redis *Redis) GetFeedback(cursor string, n int, feedbackType *string) (string, []Feedback, error) {
	var ctx = context.Background()
	var err error
	cursorNum := uint64(0)
	if len(cursor) > 0 {
		cursorNum, err = strconv.ParseUint(cursor, 10, 64)
		if err != nil {
			return "", nil, err
		}
	}
	feedback := make([]Feedback, 0)
	var keys []string
	if feedbackType != nil {
		keys, cursorNum, err = redis.client.Scan(ctx, cursorNum, prefixFeedback+*feedbackType+"*", int64(n)).Result()
	} else {
		keys, cursorNum, err = redis.client.Scan(ctx, cursorNum, prefixFeedback+"*", int64(n)).Result()
	}
	if err != nil {
		return "", nil, err
	}
	for _, key := range keys {
		val, err := redis.client.Get(ctx, key).Result()
		if err != nil {
			return "", nil, err
		}
		var data Feedback
		err = json.Unmarshal([]byte(val), &data)
		if err != nil {
			return "", nil, err
		}
		feedback = append(feedback, data)
	}
	if cursorNum == 0 {
		cursor = ""
	} else {
		cursor = strconv.Itoa(int(cursorNum))
	}
	return cursor, feedback, nil
}

func (redis *Redis) GetUserItemFeedback(userId, itemId string, feedbackType *string) ([]Feedback, error) {
	var ctx = context.Background()
	feedback := make([]Feedback, 0)
	err := redis.ForFeedback(ctx, func(key, thisFeedbackType, thisUserId, thisItemId string) error {
		if thisUserId == userId && thisItemId == itemId && (feedbackType == nil || *feedbackType == thisFeedbackType) {
			val, err := redis.getFeedback(key)
			if err != nil {
				return err
			}
			feedback = append(feedback, val)
		}
		return nil
	})
	return feedback, err
}

func (redis *Redis) DeleteUserItemFeedback(userId, itemId string, feedbackType *string) (int, error) {
	var ctx = context.Background()
	deleteCount := 0
	err := redis.ForFeedback(ctx, func(key, thisFeedbackType, thisUserId, thisItemId string) error {
		if thisUserId == userId && thisItemId == itemId && (feedbackType == nil || *feedbackType == thisFeedbackType) {
			redis.client.Del(ctx, key)
			deleteCount++
		}
		return nil
	})
	return deleteCount, err
}
