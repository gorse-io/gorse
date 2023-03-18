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
	"github.com/samber/lo"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/juju/errors"
	"github.com/mitchellh/mapstructure"
	"github.com/scylladb/go-set/strset"
	"github.com/thoas/go-funk"
	"github.com/zhenghaoz/gorse/storage"
)

const (
	prefixItem     = "item/"     // prefix for items
	prefixUser     = "user/"     // prefix for users
	prefixFeedback = "feedback/" // prefix for feedback
)

type JSONObject struct {
	any
}

func JSON(v any) JSONObject {
	return JSONObject{any: v}
}

func (j JSONObject) MarshalBinary() (data []byte, err error) {
	return json.Marshal(j.any)
}

func decodeMapStructure(input, output any) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           output,
		WeaklyTypedInput: true,
		DecodeHook: func(f reflect.Kind, t reflect.Kind, data any) (any, error) {
			if f == reflect.String && t != reflect.String {
				var v any
				if err := json.Unmarshal([]byte(data.(string)), &v); err != nil {
					return nil, err
				}
				return v, nil
			}
			return data, nil
		},
	})
	if err != nil {
		return err
	}
	return decoder.Decode(input)
}

func parseAggregateResult[T any](result any) (cursor string, a []T, _ error) {
	// fetch cursor
	resultAndCursor, ok := result.([]any)
	if !ok {
		return "", nil, errors.New("invalid FT.AGGREGATE result")
	}
	cursorId := resultAndCursor[len(resultAndCursor)-1].(int64)
	if cursorId > 0 {
		cursor = strconv.FormatInt(cursorId, 10)
	}
	// fetch rows
	rows, ok := resultAndCursor[0].([]any)
	if !ok {
		return "", nil, errors.New("invalid FT.AGGREGATE result")
	}
	for _, row := range rows[1:] {
		fields, ok := row.([]any)
		if !ok {
			return "", nil, errors.New("invalid FT.AGGREGATE result")
		}
		values := make(map[string]string)
		for i := 0; i < len(fields); i += 2 {
			key, ok := fields[i].(string)
			if !ok {
				return "", nil, errors.New("invalid FT.AGGREGATE result")
			}
			value, ok := fields[i+1].(string)
			if !ok {
				return "", nil, errors.New("invalid FT.AGGREGATE result")
			}
			values[key] = value
		}
		var e T
		if err := decodeMapStructure(values, &e); err != nil {
			return "", nil, errors.Trace(err)
		}
		a = append(a, e)
	}
	return
}

// Redis use Redis as data storage, but used for test only.
type Redis struct {
	storage.TablePrefix
	client *redis.Client
}

func (r *Redis) userDocId(userId string) string {
	return r.UsersTable() + ":" + userId
}

func (r *Redis) itemDocId(itemId string) string {
	return r.ItemsTable() + ":" + itemId
}

func (r *Redis) feedbackDocId(key FeedbackKey) string {
	return r.FeedbackTable() + ":" + key.FeedbackType + ":" + key.UserId + ":" + key.ItemId
}

// Init does nothing.
func (r *Redis) Init() error {
	result, err := r.client.Do(context.TODO(), "FT._LIST").Result()
	if err != nil {
		return errors.Trace(err)
	}
	rows, ok := result.([]any)
	if !ok {
		return errors.New("invalid FT._LIST result")
	}
	indices, ok := lo.FromAnySlice[string](rows)
	if !ok {
		return errors.New("invalid FT._LIST result")
	}
	// create user index
	if !lo.Contains(indices, r.UsersTable()) {
		_, err = r.client.Do(context.TODO(), "FT.CREATE", r.UsersTable(),
			"ON", "HASH", "PREFIX", "1", r.UsersTable()+":", "SCHEMA",
			"user_id", "TAG",
			"labels", "TEXT", "NOINDEX",
			"subscribe", "TEXT", "NOINDEX",
			"comment", "TEXT", "NOINDEX").
			Result()
		if err != nil {
			return errors.Trace(err)
		}
	}
	// create item index
	if !lo.Contains(indices, r.ItemsTable()) {
		_, err = r.client.Do(context.TODO(), "FT.CREATE", r.ItemsTable(),
			"ON", "HASH", "PREFIX", "1", r.ItemsTable()+":", "SCHEMA",
			"item_id", "TAG",
			"labels", "TEXT", "NOINDEX",
			"comment", "TEXT", "NOINDEX").
			Result()
		if err != nil {
			return errors.Trace(err)
		}
	}
	// create feedback index
	if !lo.Contains(indices, r.FeedbackTable()) {
		_, err = r.client.Do(context.TODO(), "FT.CREATE", r.FeedbackTable(),
			"ON", "HASH", "PREFIX", "1", r.FeedbackTable()+":", "SCHEMA",
			"feedback_id", "TAG",
			"feedback_type", "TAG",
			"user_id", "TAG",
			"item_id", "TAG",
			"labels", "TEXT", "NOINDEX",
			"comment", "TEXT", "NOINDEX").
			Result()
		if err != nil {
			return errors.Trace(err)
		}
	}
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
	if err := r.client.FlushDB(context.Background()).Err(); err != nil {
		return errors.Trace(err)
	}
	if err := r.Init(); err != nil {
		return errors.Trace(err)
	}
	return nil
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

// BatchInsertUsers inserts a batch pf user into Redis.
func (r *Redis) BatchInsertUsers(ctx context.Context, users []User) error {
	p := r.client.Pipeline()
	for _, user := range users {
		p.HSet(ctx, r.userDocId(user.UserId),
			"user_id", user.UserId,
			"labels", JSON(user.Labels),
			"subscribe", JSON(user.Subscribe),
			"comment", user.Comment)
	}
	_, err := p.Exec(ctx)
	return errors.Trace(err)
}

// DeleteUser deletes a user from Redis.
func (r *Redis) DeleteUser(ctx context.Context, userId string) error {
	return r.client.Del(ctx, r.userDocId(userId)).Err()
}

// GetUser returns a user from Redis.
func (r *Redis) GetUser(ctx context.Context, userId string) (User, error) {
	val, err := r.client.HGetAll(ctx, r.userDocId(userId)).Result()
	if err != nil {
		return User{}, err
	}
	if len(val) == 0 {
		return User{}, errors.Annotate(ErrUserNotExist, userId)
	}
	var user User
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &user,
		WeaklyTypedInput: true,
		DecodeHook: func(f reflect.Kind, t reflect.Kind, data any) (any, error) {
			if f == reflect.String && t != reflect.String {
				var v any
				if err := json.Unmarshal([]byte(data.(string)), &v); err != nil {
					return nil, err
				}
				return v, nil
			}
			return data, nil
		},
	})
	if err != nil {
		return User{}, err
	}
	err = decoder.Decode(val)
	return user, err
}

// GetUsers returns users from Redis.
func (r *Redis) GetUsers(ctx context.Context, cursor string, n int) (string, []User, error) {
	var (
		result any
		err    error
	)
	if cursor == "" {
		result, err = r.client.Do(ctx, "FT.AGGREGATE", r.UsersTable(), "*",
			"LOAD", "3", "user_id", "labels", "comment",
			"SORTBY", "1", "@user_id",
			"WITHCURSOR", "COUNT", n).Result()
	} else {
		result, err = r.client.Do(ctx, "FT.CURSOR", "READ", r.UsersTable(), cursor, "COUNT", n).Result()
	}
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	return parseAggregateResult[User](result)
}

// GetUserStream read users from Redis by stream.
func (r *Redis) GetUserStream(ctx context.Context, batchSize int) (chan []User, chan error) {
	userChan := make(chan []User, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(userChan)
		defer close(errChan)
		var (
			cursor string
			users  []User
			result any
			err    error
		)
		for {
			if cursor == "" {
				result, err = r.client.Do(ctx, "FT.AGGREGATE", r.UsersTable(), "*",
					"LOAD", "3", "user_id", "labels", "comment",
					"SORTBY", "1", "@user_id",
					"WITHCURSOR", "COUNT", batchSize).Result()
			} else {
				result, err = r.client.Do(ctx, "FT.CURSOR", "READ", r.UsersTable(), cursor, "COUNT", batchSize).Result()
			}
			if err != nil {
				errChan <- errors.Trace(err)
				return
			}
			cursor, users, err = parseAggregateResult[User](result)
			if err != nil {
				errChan <- errors.Trace(err)
				return
			}
			userChan <- users
			if cursor == "" {
				break
			}
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
	p := r.client.Pipeline()
	// apply patch
	if patch.IsHidden != nil {
		p.HSet(ctx, r.itemDocId(itemId), "is_hidden", *patch.IsHidden)
	}
	if patch.Categories != nil {
		p.HSet(ctx, r.itemDocId(itemId), "categories", JSON(patch.Categories))
	}
	if patch.Comment != nil {
		p.HSet(ctx, r.itemDocId(itemId), "comment", *patch.Comment)
	}
	if patch.Labels != nil {
		p.HSet(ctx, r.itemDocId(itemId), "labels", JSON(patch.Labels))
	}
	if patch.Timestamp != nil {
		p.HSet(ctx, r.itemDocId(itemId), "timestamp", patch.Timestamp)
	}
	_, err := p.Exec(ctx)
	return errors.Trace(err)
}

// ModifyUser modify a user in Redis.
func (r *Redis) ModifyUser(ctx context.Context, userId string, patch UserPatch) error {
	p := r.client.Pipeline()
	if patch.Comment != nil {
		p.HSet(ctx, r.userDocId(userId), "comment", *patch.Comment)
	}
	if patch.Labels != nil {
		p.HSet(ctx, r.userDocId(userId), "labels", JSON(patch.Labels))
	}
	if patch.Subscribe != nil {
		p.HSet(ctx, r.userDocId(userId), "subscribe", JSON(patch.Subscribe))
	}
	_, err := p.Exec(ctx)
	return errors.Trace(err)
}
