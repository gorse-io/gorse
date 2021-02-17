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
	"strconv"
	"strings"
)

const (
	// Source data
	prefixLabelIndex = "index/label/" // prefix for label index
	prefixItemIndex  = "index/item/"  // prefix for item index
	prefixUserIndex  = "index/user/"  // prefix for user index
	prefixItem       = "item/"        // prefix for items
	prefixUser       = "user/"        // prefix for users
	prefixFeedback   = "feedback/"    // prefix for feedback
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
	// write index
	for _, label := range item.Labels {
		// inset label index
		if err = redis.client.SAdd(ctx, prefixLabelIndex+label, item.ItemId).Err(); err != nil {
			return err
		}
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

func (redis *Redis) DeleteItem(itemId string) error {
	var ctx = context.Background()
	// remove user
	if err := redis.client.Del(ctx, prefixItem+itemId).Err(); err != nil {
		return err
	}
	// remove feedback
	cursor := uint64(0)
	var err error
	var keys []string
	for {
		keys, cursor, err = redis.client.Scan(ctx, cursor, prefixItemIndex+itemId+"*", 0).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			_, tp := parseItemIndexKey(key)
			// remove feedbacks
			userIds, err := redis.client.SMembers(ctx, createItemIndexKey(tp, itemId)).Result()
			if err != nil {
				return err
			}
			for _, userId := range userIds {
				if err = redis.client.Del(ctx, createFeedbackKey(FeedbackKey{tp, userId, itemId})).Err(); err != nil {
					return err
				}
			}
			// remove index
			if err = redis.client.Del(ctx, createItemIndexKey(tp, itemId)).Err(); err != nil {
				return err
			}
		}
		if cursor == 0 {
			break
		}
	}
	return nil
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

func (redis *Redis) GetItemFeedback(feedbackType, itemId string) ([]Feedback, error) {
	var ctx = context.Background()
	feedback := make([]Feedback, 0)
	userIds, err := redis.client.SMembers(ctx, createItemIndexKey(feedbackType, itemId)).Result()
	if err != nil {
		return nil, err
	}

	for _, userId := range userIds {
		val, err := redis.getFeedback(feedbackType, userId, itemId)
		if err != nil {
			return nil, err
		}
		feedback = append(feedback, val)
	}
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
	cursor := uint64(0)
	var err error
	var keys []string
	for {
		keys, cursor, err = redis.client.Scan(ctx, cursor, prefixUserIndex+userId+"*", 0).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			_, tp := parseUserIndexKey(key)
			// remove feedbacks
			itemIds, err := redis.client.SMembers(ctx, createUserIndexKey(tp, userId)).Result()
			if err != nil {
				return err
			}
			for _, itemId := range itemIds {
				if err = redis.client.Del(ctx, createFeedbackKey(FeedbackKey{tp, userId, itemId})).Err(); err != nil {
					return err
				}
			}
			// remove index
			if err = redis.client.Del(ctx, createUserIndexKey(tp, userId)).Err(); err != nil {
				return err
			}
		}
		if cursor == 0 {
			break
		}
	}
	return nil
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

func (redis *Redis) GetUserFeedback(feedbackType, userId string) ([]Feedback, error) {
	var ctx = context.Background()
	feedback := make([]Feedback, 0)

	// get itemId list by userId
	itemIds, err := redis.client.SMembers(ctx, createUserIndexKey(feedbackType, userId)).Result()
	if err != nil {
		return nil, err
	}
	// get feedback by itemId and userId
	for _, itemId := range itemIds {
		val, err := redis.getFeedback(feedbackType, userId, itemId)
		if err != nil {
			return nil, err
		}
		feedback = append(feedback, val)
	}
	return feedback, err
}

func (redis *Redis) getFeedback(tp, userId, itemId string) (Feedback, error) {
	var ctx = context.Background()
	feedbackKey := FeedbackKey{FeedbackType: tp, UserId: userId, ItemId: itemId}
	// get feedback by feedbackKey
	val, err := redis.client.Get(ctx, createFeedbackKey(feedbackKey)).Result()
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

func createUserIndexKey(tp, userId string) string {
	return prefixUserIndex + userId + "/" + tp
}

func createItemIndexKey(tp, itemId string) string {
	return prefixItemIndex + itemId + "/" + tp
}

func parseUserIndexKey(key string) (userId, tp string) {
	fields := strings.Split(key, "/")
	return fields[1], fields[3]
}

func parseItemIndexKey(key string) (itemId, tp string) {
	fields := strings.Split(key, "/")
	return fields[1], fields[3]
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
	// insert user index
	if err = redis.client.SAdd(ctx, createUserIndexKey(feedback.FeedbackType, feedback.UserId), feedback.ItemId).Err(); err != nil {
		return err
	}
	// insert item index
	if err = redis.client.SAdd(ctx, createItemIndexKey(feedback.FeedbackType, feedback.ItemId), feedback.UserId).Err(); err != nil {
		return err
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

func (redis *Redis) GetFeedback(feedbackType, cursor string, n int) (string, []Feedback, error) {
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
	keys, cursorNum, err = redis.client.Scan(ctx, cursorNum, prefixFeedback+feedbackType+"*", int64(n)).Result()
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
