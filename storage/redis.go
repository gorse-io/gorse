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
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"math/bits"
	"strconv"
)

const count = 10

type Redis struct {
	client *redis.Client
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
		// insert label
		if err = redis.client.Set(ctx, prefixLabel+label, label, 0).Err(); err != nil {
			return err
		}
		// inset label index
		if err = redis.client.RPush(ctx, prefixLabelIndex+label, item.ItemId).Err(); err != nil {
			return err
		}
	}
	return nil
}

func (redis *Redis) BatchInsertItem(items []Item) error {
	for _, item := range items {
		redis.InsertItem(item)
	}
	return nil
}

func (redis *Redis) DeleteItem(itemId string) error {
	var ctx = context.Background()
	if err := redis.client.Del(ctx, prefixItem+itemId).Err(); err != nil {
		return err
	}

	return redis.client.Del(ctx, prefixItemIndex+itemId).Err()
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

func (redis *Redis) GetItems(n int, offset int) ([]Item, error) {
	var ctx = context.Background()
	if n == 0 {
		n = (1<<bits.UintSize)/2 - 1
	}
	pos := 0
	items := make([]Item, 0)
	cursor := uint64(0)

	for {
		// scan * from zero util cursor is zero
		keys, cursor, err := redis.client.Scan(ctx, cursor, prefixItem+"*", count).Result()
		if err != nil {
			return nil, err
		}
		if pos+len(keys) < offset && cursor != uint64(0) {
			pos += len(keys)
			continue
		}
		for _, key := range keys {
			if len(items) < n && pos >= offset {
				data, err := redis.client.Get(ctx, key).Result()
				if err != nil {
					return nil, err
				}
				var item Item
				err = json.Unmarshal([]byte(data), &item)
				if err != nil {
					return nil, err
				}
				items = append(items, item)
			}
			pos++
		}
		if cursor == uint64(0) {
			return items, nil
		}
	}
}

func (redis *Redis) GetItemFeedback(itemId string) ([]Feedback, error) {
	var ctx = context.Background()
	feedback := make([]Feedback, 0)
	userIds, err := redis.client.LRange(ctx, prefixItemIndex+itemId, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	for _, userId := range userIds {
		val, err := redis.getFeedback(userId, itemId)
		if err != nil {
			return nil, err
		}
		feedback = append(feedback, val)
	}
	return feedback, err
}

// GetLabelItems list items by label
func (redis *Redis) GetLabelItems(label string) ([]Item, error) {
	var ctx = context.Background()
	items := make([]Item, 0)

	itemIds, err := redis.client.LRange(ctx, prefixLabelIndex+label, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	for _, itemId := range itemIds {
		item, err := redis.GetItem(itemId)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, err
}

func (redis *Redis) GetLabels() ([]string, error) {
	var ctx = context.Background()
	labels := make([]string, 0)
	cursor := uint64(0)
	for {
		keys, cursor, err := redis.client.Scan(ctx, cursor, prefixLabel+"*", count).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			val, err := redis.client.Get(ctx, key).Result()
			if err != nil {
				return nil, err
			}
			labels = append(labels, val)
		}
		if cursor == uint64(0) {
			return labels, err
		}
	}
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
	if err := redis.client.Del(ctx, prefixUser+userId).Err(); err != nil {
		return err
	}
	if err := redis.client.Del(ctx, prefixUserIndex+userId).Err(); err != nil {
		return err
	}
	return redis.client.Del(ctx, prefixIgnore+userId).Err()
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

func (redis *Redis) GetUsers() ([]User, error) {
	var ctx = context.Background()
	users := make([]User, 0)
	cursor := uint64(0)
	for {
		keys, cursor, err := redis.client.Scan(ctx, cursor, prefixUser+"*", count).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			var user User
			val, err := redis.client.Get(ctx, key).Result()
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal([]byte(val), &user)
			if err != nil {
				return nil, err
			}
			users = append(users, user)
		}
		if cursor == uint64(0) {
			return users, err
		}
	}
}

func (redis *Redis) GetUserFeedback(userId string) ([]Feedback, error) {
	var ctx = context.Background()
	feedback := make([]Feedback, 0)

	// get itemId list by userId
	itemIds, err := redis.client.LRange(ctx, prefixUserIndex+userId, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	// get feedback by itemId and userId
	for _, itemId := range itemIds {
		val, err := redis.getFeedback(userId, itemId)
		if err != nil {
			return nil, err
		}
		feedback = append(feedback, val)
	}
	return feedback, err
}

func (redis *Redis) getFeedback(userId string, itemId string) (Feedback, error) {
	var ctx = context.Background()
	feedbackKey := FeedbackKey{UserId: userId, ItemId: itemId}
	key, err := json.Marshal(feedbackKey)
	if err != nil {
		return Feedback{}, err
	}
	// get feedback by feedbackKey
	val, err := redis.client.Get(ctx, prefixFeedback+string(key)).Result()
	var feedback Feedback
	err = json.Unmarshal([]byte(val), &feedback)
	if err != nil {
		return Feedback{}, err
	}
	return feedback, err
}

func (redis *Redis) InsertUserIgnore(userId string, items []string) error {
	var ctx = context.Background()
	for item := range items {
		err := redis.client.RPush(ctx, prefixIgnore+userId, item).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

func (redis *Redis) GetUserIgnore(userId string) ([]string, error) {
	var ctx = context.Background()
	ignore, err := redis.client.LRange(ctx, prefixIgnore+userId, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	return ignore, err
}

func (redis *Redis) CountUserIgnore(userId string) (int, error) {
	var ctx = context.Background()
	len, err := redis.client.LLen(ctx, prefixIgnore+userId).Result()
	if err != nil {
		return -1, err
	}
	return int(len), nil
}

func (redis *Redis) InsertFeedback(feedback Feedback) error {
	var ctx = context.Background()
	key, err := json.Marshal(FeedbackKey{feedback.UserId, feedback.ItemId})
	if err != nil {
		return err
	}
	val, err := json.Marshal(feedback)
	if err != nil {
		return err
	}
	// insert feedback
	err = redis.client.Set(ctx, prefixFeedback+string(key), val, 0).Err()
	if err != nil {
		return err
	}
	// insert user
	user := new(User)
	user.UserId = feedback.UserId
	val, err = json.Marshal(user)
	if err != nil {
		return err
	}
	if err = redis.client.Set(ctx, prefixUser+feedback.UserId, val, 0).Err(); err != nil {
		return err
	}
	// insert user index
	if err = redis.client.RPush(ctx, prefixUserIndex+feedback.UserId, feedback.ItemId).Err(); err != nil {
		return err
	}
	// insert item index
	if err = redis.client.RPush(ctx, prefixItemIndex+feedback.ItemId, feedback.UserId).Err(); err != nil {
		return err
	}
	return nil
}

func (redis *Redis) BatchInsertFeedback(feedback []Feedback) error {
	for _, temp := range feedback {
		redis.InsertFeedback(temp)
	}
	return nil
}

func (redis *Redis) GetFeedback() ([]Feedback, error) {
	var ctx = context.Background()
	feedback := make([]Feedback, 0)
	cursor := uint64(0)
	for {
		keys, cursor, err := redis.client.Scan(ctx, cursor, prefixFeedback+"*", count).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			val, err := redis.client.Get(ctx, key).Result()
			if err != nil {
				return nil, err
			}
			var data Feedback
			err = json.Unmarshal([]byte(val), &data)
			if err != nil {
				return nil, err
			}
			feedback = append(feedback, data)
		}
		if cursor == uint64(0) {
			return feedback, err
		}
	}
}

func (redis *Redis) GetString(name string) (string, error) {
	var ctx = context.Background()
	val, err := redis.client.Get(ctx, prefixMeta+name).Result()
	if err != nil {
		return string(""), err
	}
	return val, err
}

func (redis *Redis) SetString(name string, val string) error {
	var ctx = context.Background()
	if err := redis.client.Set(ctx, prefixMeta+name, val, 0).Err(); err != nil {
		return err
	}
	return nil
}

func (redis *Redis) GetInt(name string) (int, error) {
	val, err := redis.GetString(name)
	if err != nil {
		return -1, nil
	}
	buf, err := strconv.Atoi(val)
	if err != nil {
		return -1, err
	}
	return buf, err
}

func (redis *Redis) SetInt(name string, val int) error {
	var ctx = context.Background()
	if err := redis.client.Set(ctx, prefixMeta+name, val, 0).Err(); err != nil {
		return err
	}
	return nil
}

func (redis *Redis) SetNeighbors(itemId string, items []RecommendedItem) error {
	return redis.SetList(prefixNeighbors, itemId, items)
}

func (redis *Redis) SetPop(label string, items []RecommendedItem) error {
	return redis.SetList(prefixPop, label, items)
}

func (redis *Redis) SetLatest(label string, items []RecommendedItem) error {
	return redis.SetList(prefixLatest, label, items)
}

func (redis *Redis) SetRecommend(userId string, items []RecommendedItem) error {
	return redis.SetList(prefixRecommends, userId, items)
}

func (redis *Redis) GetNeighbors(itemId string, n int, offset int) ([]RecommendedItem, error) {
	return redis.GetList(prefixNeighbors, itemId, n, offset)
}

func (redis *Redis) GetPop(label string, n int, offset int) ([]RecommendedItem, error) {
	return redis.GetList(prefixPop, label, n, offset)
}

func (redis *Redis) GetLatest(label string, n int, offset int) ([]RecommendedItem, error) {
	return redis.GetList(prefixLatest, label, n, offset)
}

func (redis *Redis) GetRecommend(userId string, n int, offset int) ([]RecommendedItem, error) {
	return redis.GetList(prefixRecommends, userId, n, offset)
}

func (redis *Redis) SetList(prefix string, listId string, items []RecommendedItem) error {
	var ctx = context.Background()
	for _, item := range items {
		buf, err := json.Marshal(item)
		if err != nil {
			return err
		}
		err = redis.client.RPush(ctx, prefix+listId, buf).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

func (redis *Redis) GetList(prefix string, listId string, n int, offset int) ([]RecommendedItem, error) {
	var ctx = context.Background()
	recommendItems := make([]RecommendedItem, 0)
	if n == 0 {
		val, err := redis.client.LLen(ctx, prefix+listId).Result()
		if err != nil {
			return nil, err
		}
		n = int(val) - offset
	}
	data, err := redis.client.LRange(ctx, prefix+listId, int64(offset), int64(n+offset-1)).Result()

	if err != nil {
		return nil, err
	}

	for _, item := range data {
		var recommendItem RecommendedItem
		err := json.Unmarshal([]byte(item), &recommendItem)
		if err != nil {
			return nil, err
		}
		recommendItems = append(recommendItems, recommendItem)
	}
	return recommendItems, err
}
