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
	"github.com/go-redis/redis/v9"
	"github.com/juju/errors"
	"github.com/scylladb/go-set/strset"
	"github.com/thoas/go-funk"
	"strconv"
	"strings"
	"time"
)

const (
	prefixItem     = "item/"     // prefix for items
	prefixUser     = "user/"     // prefix for users
	prefixFeedback = "feedback/" // prefix for feedback
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

func (r *Redis) Ping() error {
	return r.client.Ping(context.Background()).Err()
}

// Close Redis connection.
func (r *Redis) Close() error {
	return r.client.Close()
}

func (r *Redis) Purge() error {
	return r.client.FlushDB(context.Background()).Err()
}

// insertItem inserts an item into Redis.
func (r *Redis) insertItem(ctx context.Context, item Item) error {
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
func (r *Redis) BatchInsertItems(ctx context.Context, items []Item) error {
	for _, item := range items {
		if err := r.insertItem(ctx, item); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (r *Redis) BatchGetItems(ctx context.Context, itemIds []string) ([]Item, error) {
	var (
		items  []Item
		cursor uint64
		err    error
		keys   []string
	)
	for {
		keys, cursor, err = r.client.Scan(ctx, cursor, prefixItem+"*", -1).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			data, err := r.client.Get(ctx, key).Result()
			if err != nil {
				return nil, err
			}
			var item Item
			err = json.Unmarshal([]byte(data), &item)
			if err != nil {
				return nil, err
			}
			if funk.ContainsString(itemIds, item.ItemId) {
				items = append(items, item)
			}
		}
		if cursor == 0 {
			break
		}
	}
	return items, nil
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
func (r *Redis) DeleteItem(ctx context.Context, itemId string) error {
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
func (r *Redis) GetItem(ctx context.Context, itemId string) (Item, error) {
	data, err := r.client.Get(ctx, prefixItem+itemId).Result()
	if err != nil {
		if err == redis.Nil {
			return Item{}, errors.Annotate(ErrItemNotExist, itemId)
		}
		return Item{}, err
	}
	var item Item
	err = json.Unmarshal([]byte(data), &item)
	return item, err
}

// GetItems returns items from Redis.
func (r *Redis) GetItems(ctx context.Context, cursor string, n int, timeLimit *time.Time) (string, []Item, error) {
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
func (r *Redis) GetItemStream(ctx context.Context, batchSize int, timeLimit *time.Time) (chan []Item, chan error) {
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
func (r *Redis) GetItemFeedback(ctx context.Context, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	feedback := make([]Feedback, 0)
	feedbackTypeSet := strset.New(feedbackTypes...)
	err := r.ForFeedback(ctx, func(key, thisFeedbackType, _, thisItemId string) error {
		if itemId == thisItemId && (feedbackTypeSet.IsEmpty() || feedbackTypeSet.Has(thisFeedbackType)) {
			val, err := r.getFeedbackInternal(key)
			if err != nil {
				return errors.Trace(err)
			}
			if val.Timestamp.Before(time.Now()) {
				feedback = append(feedback, val)
			}
		}
		return nil
	})
	return feedback, err
}

// insertUser inserts a user into Redis.
func (r *Redis) insertUser(ctx context.Context, user User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return errors.Trace(err)
	}
	return r.client.Set(ctx, prefixUser+user.UserId, data, 0).Err()
}

// BatchInsertUsers inserts a batch pf user into Redis.
func (r *Redis) BatchInsertUsers(ctx context.Context, users []User) error {
	for _, user := range users {
		if err := r.insertUser(ctx, user); err != nil {
			return err
		}
	}
	return nil
}

// DeleteUser deletes a user from Redis.
func (r *Redis) DeleteUser(ctx context.Context, userId string) error {
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
func (r *Redis) GetUser(ctx context.Context, userId string) (User, error) {
	val, err := r.client.Get(ctx, prefixUser+userId).Result()
	if err != nil {
		if err == redis.Nil {
			return User{}, errors.Annotate(ErrUserNotExist, userId)
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
func (r *Redis) GetUsers(ctx context.Context, cursor string, n int) (string, []User, error) {
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
func (r *Redis) GetUserStream(ctx context.Context, batchSize int) (chan []User, chan error) {
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
func (r *Redis) GetUserFeedback(ctx context.Context, userId string, endTime *time.Time, feedbackTypes ...string) ([]Feedback, error) {
	feedback := make([]Feedback, 0)
	feedbackTypeSet := strset.New(feedbackTypes...)
	// get itemId list by userId
	err := r.ForFeedback(ctx, func(key, thisFeedbackType, thisUserId, thisItemId string) error {
		if thisUserId == userId && (feedbackTypeSet.IsEmpty() || feedbackTypeSet.Has(thisFeedbackType)) {
			val, err := r.getFeedbackInternal(key)
			if err != nil {
				return errors.Trace(err)
			}
			if endTime == nil || val.Timestamp.Before(*endTime) {
				feedback = append(feedback, val)
			}
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
func (r *Redis) insertFeedback(ctx context.Context, feedback Feedback, insertUser, insertItem, overwrite bool) error {
	// locate user
	_, err := r.GetUser(ctx, feedback.UserId)
	if errors.Is(err, errors.NotFound) {
		if !insertUser {
			return nil
		}
	} else if err != nil {
		return err
	}
	// locate item
	_, err = r.GetItem(ctx, feedback.ItemId)
	if errors.Is(err, errors.NotFound) {
		if !insertItem {
			return nil
		}
	} else if err != nil {
		return err
	}
	val, err := json.Marshal(feedback)
	if err != nil {
		return errors.Trace(err)
	}
	// insert feedback
	feedbackKey := createFeedbackKey(feedback.FeedbackKey)
	if exists, err := r.client.Exists(ctx, feedbackKey).Result(); err != nil {
		return errors.Trace(err)
	} else if exists > 0 && !overwrite {
		return nil
	}
	err = r.client.Set(ctx, feedbackKey, val, 0).Err()
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
func (r *Redis) BatchInsertFeedback(ctx context.Context, feedback []Feedback, insertUser, insertItem, overwrite bool) error {
	for _, temp := range feedback {
		if err := r.insertFeedback(ctx, temp, insertUser, insertItem, overwrite); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// GetFeedback returns feedback from Redis.
func (r *Redis) GetFeedback(ctx context.Context, _ string, _ int, beginTime, endTime *time.Time, feedbackTypes ...string) (string, []Feedback, error) {
	feedback := make([]Feedback, 0)
	feedbackTypeSet := strset.New(feedbackTypes...)
	err := r.ForFeedback(ctx, func(key, thisFeedbackType, thisUserId, thisItemId string) error {
		if feedbackTypeSet.IsEmpty() || feedbackTypeSet.Has(thisFeedbackType) {
			val, err := r.getFeedbackInternal(key)
			if err != nil {
				return errors.Trace(err)
			}
			if beginTime != nil && val.Timestamp.Before(*beginTime) {
				return nil
			}
			if endTime != nil && val.Timestamp.After(*endTime) {
				return nil
			}
			feedback = append(feedback, val)
		}
		return nil
	})
	return "", feedback, err
}

// GetFeedbackStream reads feedback by stream.
func (r *Redis) GetFeedbackStream(ctx context.Context, batchSize int, beginTime, endTime *time.Time, feedbackTypes ...string) (chan []Feedback, chan error) {
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
				if beginTime != nil && val.Timestamp.Before(*beginTime) {
					return nil
				}
				if endTime != nil && val.Timestamp.After(*endTime) {
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
func (r *Redis) GetUserItemFeedback(ctx context.Context, userId, itemId string, feedbackTypes ...string) ([]Feedback, error) {
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
func (r *Redis) DeleteUserItemFeedback(ctx context.Context, userId, itemId string, feedbackTypes ...string) (int, error) {
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

// ModifyItem modify an item in Redis.
func (r *Redis) ModifyItem(ctx context.Context, itemId string, patch ItemPatch) error {
	// read item
	item, err := r.GetItem(ctx, itemId)
	if err != nil {
		return err
	}
	// apply patch
	if patch.IsHidden != nil {
		item.IsHidden = *patch.IsHidden
	}
	if patch.Categories != nil {
		item.Categories = patch.Categories
	}
	if patch.Comment != nil {
		item.Comment = *patch.Comment
	}
	if patch.Labels != nil {
		item.Labels = patch.Labels
	}
	if patch.Timestamp != nil {
		item.Timestamp = *patch.Timestamp
	}
	// write back
	return r.insertItem(ctx, item)
}

// ModifyUser modify a user in Redis.
func (r *Redis) ModifyUser(ctx context.Context, userId string, patch UserPatch) error {
	// read user
	user, err := r.GetUser(ctx, userId)
	if err != nil {
		return err
	}
	// apply patch
	if patch.Comment != nil {
		user.Comment = *patch.Comment
	}
	if patch.Labels != nil {
		user.Labels = patch.Labels
	}
	if patch.Subscribe != nil {
		user.Subscribe = patch.Subscribe
	}
	// write back
	return r.insertUser(ctx, user)
}
