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
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/juju/errors"
	"github.com/mitchellh/mapstructure"
	"github.com/samber/lo"
	"github.com/scylladb/go-set/strset"
	"github.com/zhenghaoz/gorse/storage"
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
		Result: output,
		Squash: true,
		DecodeHook: func(f reflect.Type, t reflect.Type, data any) (any, error) {
			if f.Kind() == reflect.String && t.Kind() == reflect.Bool {
				return strconv.ParseBool(data.(string))
			} else if f.Kind() == reflect.String && t == reflect.TypeOf(time.Time{}) {
				nanos, err := strconv.ParseInt(data.(string), 10, 64)
				if err != nil {
					return nil, err
				}
				return time.Unix(0, nanos).In(time.UTC), nil
			} else if f.Kind() == reflect.String && t.Kind() == reflect.Interface {
				var v any
				if err := json.Unmarshal([]byte(data.(string)), &v); err != nil {
					return nil, err
				}
				return v, nil
			} else if f.Kind() == reflect.String && t.Kind() == reflect.Slice {
				var v any
				if err := json.Unmarshal([]byte(data.(string)), &v); err != nil {
					return nil, err
				}
				if v == nil {
					return reflect.New(t).Interface(), nil
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

func parseSearchResult[T any](result any) (keys []string, docs []T, _ error) {
	// fetch rows
	rows, ok := result.([]any)
	if !ok {
		return nil, nil, errors.New("invalid FT.SEARCH result")
	}
	for i := 1; i < len(rows); i += 2 {
		key, ok := rows[i].(string)
		if !ok {
			return nil, nil, errors.New("invalid FT.SEARCH result")
		}
		keys = append(keys, key)
		fields, ok := rows[i+1].([]any)
		if !ok {
			return nil, nil, errors.New("invalid FT.SEARCH result")
		}
		values := make(map[string]string)
		for i := 0; i < len(fields); i += 2 {
			key, ok := fields[i].(string)
			if !ok {
				return nil, nil, errors.New("invalid FT.SEARCH result")
			}
			value, ok := fields[i+1].(string)
			if !ok {
				return nil, nil, errors.New("invalid FT.SEARCH result")
			}
			values[key] = value
		}
		var e T
		if err := decodeMapStructure(values, &e); err != nil {
			return nil, nil, errors.Trace(err)
		}
		docs = append(docs, e)
	}
	return
}

func parseAggregateResult[T any](result any) (a []T, _ error) {
	// fetch rows
	rows, ok := result.([]any)
	if !ok {
		return nil, errors.New("invalid FT.AGGREGATE result")
	}
	for _, row := range rows[1:] {
		fields, ok := row.([]any)
		if !ok {
			return nil, errors.New("invalid FT.AGGREGATE result")
		}
		values := make(map[string]string)
		for i := 0; i < len(fields); i += 2 {
			key, ok := fields[i].(string)
			if !ok {
				return nil, errors.New("invalid FT.AGGREGATE result")
			}
			value, ok := fields[i+1].(string)
			if !ok {
				return nil, errors.New("invalid FT.AGGREGATE result")
			}
			values[key] = value
		}
		var e T
		if err := decodeMapStructure(values, &e); err != nil {
			return nil, errors.Trace(err)
		}
		a = append(a, e)
	}
	return
}

func parseAggregateResultWithCursor[T any](result any) (cursor string, a []T, err error) {
	// fetch cursor
	resultAndCursor, ok := result.([]any)
	if !ok {
		return "", nil, errors.New("invalid FT.AGGREGATE result")
	}
	cursorId := resultAndCursor[len(resultAndCursor)-1].(int64)
	if cursorId > 0 {
		cursor = strconv.FormatInt(cursorId, 10)
	}
	a, err = parseAggregateResult[T](resultAndCursor[0])
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

func (r *Redis) ftList() (*strset.Set, error) {
	result, err := r.client.Do(context.TODO(), "FT._LIST").Result()
	if err != nil {
		return nil, errors.Trace(err)
	}
	rows, ok := result.([]any)
	if !ok {
		return nil, errors.New("invalid FT._LIST result")
	}
	indices, ok := lo.FromAnySlice[string](rows)
	if !ok {
		return nil, errors.New("invalid FT._LIST result")
	}
	return strset.New(indices...), nil
}

// Init does nothing.
func (r *Redis) Init() error {
	indices, err := r.ftList()
	if err != nil {
		return errors.Trace(err)
	}
	// create user index
	if !indices.Has(r.UsersTable()) {
		_, err = r.client.Do(context.TODO(), "FT.CREATE", r.UsersTable(),
			"ON", "HASH", "PREFIX", "1", r.UsersTable()+":", "SCHEMA", "user_id", "TAG").
			Result()
		if err != nil {
			return errors.Trace(err)
		}
	}
	// create item index
	if !indices.Has(r.ItemsTable()) {
		_, err = r.client.Do(context.TODO(), "FT.CREATE", r.ItemsTable(),
			"ON", "HASH", "PREFIX", "1", r.ItemsTable()+":", "SCHEMA", "item_id", "TAG").
			Result()
		if err != nil {
			return errors.Trace(err)
		}
	}
	// create feedback index
	if !indices.Has(r.FeedbackTable()) {
		_, err = r.client.Do(context.TODO(), "FT.CREATE", r.FeedbackTable(),
			"ON", "HASH", "PREFIX", "1", r.FeedbackTable()+":", "SCHEMA",
			"feedback_type", "TAG",
			"user_id", "TAG",
			"item_id", "TAG",
			"timestamp", "NUMERIC").
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
	// drop index
	indices, err := r.ftList()
	if err != nil {
		return errors.Trace(err)
	}
	for _, index := range []string{r.UsersTable(), r.ItemsTable(), r.FeedbackTable()} {
		if indices.Has(index) {
			if err = r.client.Do(context.Background(), "FT.DROPINDEX", index).Err(); err != nil {
				return errors.Trace(err)
			}
		}
	}
	// recreate index
	if err := r.Init(); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// BatchInsertItems inserts a batch of items into Redis.
func (r *Redis) BatchInsertItems(ctx context.Context, items []Item) error {
	p := r.client.Pipeline()
	for _, item := range items {
		p.HSet(ctx, r.itemDocId(item.ItemId),
			"item_id", item.ItemId,
			"is_hidden", item.IsHidden,
			"categories", JSON(item.Categories),
			"labels", JSON(item.Labels),
			"timestamp", item.Timestamp.UnixNano(),
			"comment", item.Comment)
	}
	_, err := p.Exec(ctx)
	return errors.Trace(err)
}

func (r *Redis) BatchGetItems(ctx context.Context, itemIds []string) ([]Item, error) {
	if len(itemIds) == 0 {
		return nil, nil
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("@item_id:{ %s }", strings.Join(itemIds, " | ")))
	result, err := r.client.Do(ctx, "FT.SEARCH", r.ItemsTable(), builder.String(), "LIMIT", 0, len(itemIds)).Result()
	if err != nil {
		return nil, errors.Trace(err)
	}
	_, items, err := parseSearchResult[Item](result)
	return items, err
}

// DeleteItem deletes a item from Redis.
func (r *Redis) DeleteItem(ctx context.Context, itemId string) error {
	if err := r.client.Del(ctx, r.itemDocId(itemId)).Err(); err != nil {
		return errors.Trace(err)
	}
	var (
		builder strings.Builder
		result  any
		err     error
	)
	builder.WriteString(fmt.Sprintf("@item_id:{ %s }", itemId))
	result, err = r.client.Do(ctx, "FT.SEARCH", r.FeedbackTable(), builder.String()).Result()
	if err != nil {
		return errors.Trace(err)
	}
	keys, _, err := parseSearchResult[Feedback](result)
	if err != nil {
		return errors.Trace(err)
	}
	p := r.client.Pipeline()
	for _, key := range keys {
		p.Del(ctx, key)
	}
	_, err = p.Exec(ctx)
	return errors.Trace(err)
}

// GetItem get a item from Redis.
func (r *Redis) GetItem(ctx context.Context, itemId string) (Item, error) {
	result, err := r.client.HGetAll(ctx, r.itemDocId(itemId)).Result()
	if err != nil {
		return Item{}, err
	}
	if len(result) == 0 {
		return Item{}, errors.Annotate(ErrItemNotExist, itemId)
	}
	var item Item
	err = decodeMapStructure(result, &item)
	return item, errors.Trace(err)
}

// GetItems returns items from Redis.
func (r *Redis) GetItems(ctx context.Context, cursor string, n int, timeLimit *time.Time) (string, []Item, error) {
	var (
		result any
		err    error
	)
	if cursor == "" {
		result, err = r.client.Do(ctx, "FT.AGGREGATE", r.ItemsTable(), "*",
			"LOAD", "6", "item_id", "is_hidden", "categories", "labels", "timestamp", "comment",
			"SORTBY", "1", "@item_id",
			"LIMIT", 0, n,
			"WITHCURSOR").Result()
	} else {
		result, err = r.client.Do(ctx, "FT.CURSOR", "READ", r.ItemsTable(), cursor, "COUNT", n).Result()
	}
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	return parseAggregateResultWithCursor[Item](result)
}

// GetItemStream read items from Redis by stream.
func (r *Redis) GetItemStream(ctx context.Context, batchSize int, timeLimit *time.Time) (chan []Item, chan error) {
	itemChan := make(chan []Item, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(itemChan)
		defer close(errChan)
		ctx := context.Background()
		var (
			cursor string
			result any
			err    error
			items  []Item
		)
		for {
			if cursor == "" {
				result, err = r.client.Do(ctx, "FT.AGGREGATE", r.ItemsTable(), "*",
					"LOAD", "6", "item_id", "is_hidden", "categories", "labels", "timestamp", "comment",
					"SORTBY", "1", "@item_id",
					"WITHCURSOR", "COUNT", batchSize).Result()
			} else {
				result, err = r.client.Do(ctx, "FT.CURSOR", "READ", r.ItemsTable(), cursor, "COUNT", batchSize).Result()
			}
			if err != nil {
				errChan <- errors.Trace(err)
				return
			}
			cursor, items, err = parseAggregateResultWithCursor[Item](result)
			if err != nil {
				errChan <- errors.Trace(err)
				return
			}
			itemChan <- items
			if cursor == "" {
				break
			}
		}
		errChan <- nil
	}()
	return itemChan, errChan
}

// GetItemFeedback returns feedback of an item from Redis.
func (r *Redis) GetItemFeedback(ctx context.Context, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	var (
		builder strings.Builder
		result  any
		err     error
	)
	builder.WriteString(fmt.Sprintf("@item_id:{ %s } @timestamp:[-inf %d]", itemId, time.Now().UnixNano()))
	if len(feedbackTypes) > 0 {
		builder.WriteString(fmt.Sprintf(" @feedback_type:{ %s }", strings.Join(feedbackTypes, "|")))
	}
	result, err = r.client.Do(ctx, "FT.SEARCH", r.FeedbackTable(), builder.String()).Result()
	if err != nil {
		return nil, errors.Trace(err)
	}
	_, rows, err := parseSearchResult[Feedback](result)
	return rows, errors.Trace(err)
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
	if err := r.client.Del(ctx, r.userDocId(userId)).Err(); err != nil {
		return errors.Trace(err)
	}
	var (
		builder strings.Builder
		result  any
		err     error
	)
	builder.WriteString(fmt.Sprintf("@user_id:{ %s }", userId))
	result, err = r.client.Do(ctx, "FT.SEARCH", r.FeedbackTable(), builder.String()).Result()
	if err != nil {
		return errors.Trace(err)
	}
	keys, _, err := parseSearchResult[Feedback](result)
	if err != nil {
		return errors.Trace(err)
	}
	p := r.client.Pipeline()
	for _, key := range keys {
		p.Del(ctx, key)
	}
	_, err = p.Exec(ctx)
	return errors.Trace(err)
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
	return parseAggregateResultWithCursor[User](result)
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
			cursor, users, err = parseAggregateResultWithCursor[User](result)
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
	var (
		builder strings.Builder
		result  any
		err     error
	)
	builder.WriteString(fmt.Sprintf("@user_id:{ %s }", userId))
	if endTime != nil {
		builder.WriteString(fmt.Sprintf(" @timestamp:[-inf %d]", endTime.UnixNano()))
	}
	if len(feedbackTypes) > 0 {
		builder.WriteString(fmt.Sprintf(" @feedback_type:{ %s }", strings.Join(feedbackTypes, "|")))
	}
	result, err = r.client.Do(ctx, "FT.SEARCH", r.FeedbackTable(), builder.String()).Result()
	if err != nil {
		return nil, errors.Trace(err)
	}
	_, rows, err := parseSearchResult[Feedback](result)
	return rows, errors.Trace(err)
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

// insertFeedback insert a feedback into Redis.
// If insertUser set, a new user will be inserted to user table.
// If insertItem set, a new item will be inserted to item table.
func (r *Redis) insertFeedback(ctx context.Context, feedback Feedback, insertUser, insertItem, overwrite bool) error {
	p := r.client.Pipeline()
	// locate user
	_, err := r.GetUser(ctx, feedback.UserId)
	if errors.Is(err, errors.NotFound) {
		if !insertUser {
			return nil
		}
	} else if err != nil {
		return errors.Trace(err)
	}
	// locate item
	_, err = r.GetItem(ctx, feedback.ItemId)
	if errors.Is(err, errors.NotFound) {
		if !insertItem {
			return nil
		}
	} else if err != nil {
		return errors.Trace(err)
	}
	// insert feedback
	if exists, err := r.client.Exists(ctx, r.feedbackDocId(feedback.FeedbackKey)).Result(); err != nil {
		return errors.Trace(err)
	} else if exists > 0 && !overwrite {
		return nil
	}
	p.HSet(ctx, r.feedbackDocId(feedback.FeedbackKey),
		"feedback_type", feedback.FeedbackType,
		"user_id", feedback.UserId,
		"item_id", feedback.ItemId,
		"timestamp", feedback.Timestamp.UnixNano(),
		"comment", feedback.Comment)
	// insert user
	if insertUser {
		p.HSet(ctx, r.userDocId(feedback.UserId), "user_id", feedback.UserId)
	}
	// Insert item
	if insertItem {
		p.HSet(ctx, r.itemDocId(feedback.ItemId), "item_id", feedback.ItemId)
	}
	_, err = p.Exec(ctx)
	return err
}

// BatchInsertFeedback insert a batch feedback into Redis.
// If insertUser set, new users will be inserted to user table.
// If insertItem set, new items will be inserted to item table.
func (r *Redis) BatchInsertFeedback(ctx context.Context, feedback []Feedback, insertUser, insertItem, overwrite bool) error {
	for _, temp := range feedback {
		if err := r.insertFeedback(ctx, temp, insertUser, insertItem, overwrite); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// GetFeedback returns feedback from Redis.
func (r *Redis) GetFeedback(ctx context.Context, cursor string, n int, beginTime, endTime *time.Time, feedbackTypes ...string) (string, []Feedback, error) {
	var (
		builder strings.Builder
		result  any
		err     error
	)
	if len(feedbackTypes) > 0 {
		builder.WriteString(fmt.Sprintf(" @feedback_type:{ %s }", strings.Join(feedbackTypes, "|")))
	}
	if beginTime != nil {
		builder.WriteString(fmt.Sprintf(" @timestamp:[ %d +inf ]", beginTime.UnixNano()))
	}
	if endTime != nil {
		builder.WriteString(fmt.Sprintf(" @timestamp:[ -inf %d ]", endTime.UnixNano()))
	}
	if builder.Len() == 0 {
		builder.WriteString("*")
	}
	if cursor == "" {
		result, err = r.client.Do(ctx, "FT.AGGREGATE", r.FeedbackTable(), builder.String(),
			"LOAD", "5", "feedback_type", "user_id", "item_id", "timestamp", "comment",
			"SORTBY", "2", "@user_id", "@item_id",
			"WITHCURSOR", "COUNT", n).Result()
	} else {
		result, err = r.client.Do(ctx, "FT.CURSOR", "READ", r.FeedbackTable(), cursor, "COUNT", n).Result()
	}
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	return parseAggregateResultWithCursor[Feedback](result)
}

// GetFeedbackStream reads feedback by stream.
func (r *Redis) GetFeedbackStream(ctx context.Context, batchSize int, beginTime, endTime *time.Time, feedbackTypes ...string) (chan []Feedback, chan error) {
	feedbackChan := make(chan []Feedback, bufSize)
	errChan := make(chan error, 1)
	var (
		builder  strings.Builder
		feedback []Feedback
		cursor   string
		result   any
		err      error
	)
	if len(feedbackTypes) > 0 {
		builder.WriteString(fmt.Sprintf(" @feedback_type:{ %s }", strings.Join(feedbackTypes, "|")))
	}
	if beginTime != nil {
		builder.WriteString(fmt.Sprintf(" @timestamp:[ %d +inf ]", beginTime.UnixNano()))
	}
	if endTime != nil {
		builder.WriteString(fmt.Sprintf(" @timestamp:[ -inf %d ]", endTime.UnixNano()))
	}
	if builder.Len() == 0 {
		builder.WriteString("*")
	}
	go func() {
		defer close(feedbackChan)
		defer close(errChan)
		for {
			if cursor == "" {
				result, err = r.client.Do(ctx, "FT.AGGREGATE", r.FeedbackTable(), builder.String(),
					"LOAD", "5", "feedback_type", "user_id", "item_id", "timestamp", "comment",
					"SORTBY", "2", "@user_id", "@item_id",
					"WITHCURSOR", "COUNT", batchSize).Result()
			} else {
				result, err = r.client.Do(ctx, "FT.CURSOR", "READ", r.FeedbackTable(), cursor, "COUNT", batchSize).Result()
			}
			if err != nil {
				errChan <- errors.Trace(err)
				return
			}
			cursor, feedback, err = parseAggregateResultWithCursor[Feedback](result)
			if err != nil {
				errChan <- errors.Trace(err)
				return
			}
			feedbackChan <- feedback
			if cursor == "" {
				break
			}
		}
		errChan <- nil
	}()
	return feedbackChan, errChan
}

// GetUserItemFeedback gets a feedback by user id and item id from Redis.
func (r *Redis) GetUserItemFeedback(ctx context.Context, userId, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	var (
		builder strings.Builder
		result  any
		err     error
	)
	builder.WriteString(fmt.Sprintf("@item_id:{ %s } @user_id:{ %s } @timestamp:[-inf %d]", itemId, userId, time.Now().UnixNano()))
	if len(feedbackTypes) > 0 {
		builder.WriteString(fmt.Sprintf(" @feedback_type:{ %s }", strings.Join(feedbackTypes, "|")))
	}
	result, err = r.client.Do(ctx, "FT.SEARCH", r.FeedbackTable(), builder.String()).Result()
	if err != nil {
		return nil, errors.Trace(err)
	}
	_, rows, err := parseSearchResult[Feedback](result)
	return rows, errors.Trace(err)
}

// DeleteUserItemFeedback deletes a feedback by user id and item id from Redis.
func (r *Redis) DeleteUserItemFeedback(ctx context.Context, userId, itemId string, feedbackTypes ...string) (int, error) {
	var (
		builder strings.Builder
		result  any
		err     error
	)
	builder.WriteString(fmt.Sprintf("@item_id:{ %s } @user_id:{ %s }", itemId, userId))
	if len(feedbackTypes) > 0 {
		builder.WriteString(fmt.Sprintf(" @feedback_type:{ %s }", strings.Join(feedbackTypes, "|")))
	}
	result, err = r.client.Do(ctx, "FT.SEARCH", r.FeedbackTable(), builder.String()).Result()
	if err != nil {
		return 0, errors.Trace(err)
	}
	keys, _, err := parseSearchResult[Feedback](result)
	if err != nil {
		return 0, errors.Trace(err)
	}
	p := r.client.Pipeline()
	for _, key := range keys {
		p.Del(ctx, key)
	}
	_, err = p.Exec(ctx)
	return len(keys), errors.Trace(err)
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
		p.HSet(ctx, r.itemDocId(itemId), "timestamp", patch.Timestamp.UnixNano())
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
