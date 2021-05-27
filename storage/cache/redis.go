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

package cache

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/go-redis/redis/v8"
)

// Redis cache storage.
type Redis struct {
	client *redis.Client
}

// Close redis connection.
func (redis *Redis) Close() error {
	return redis.client.Close()
}

// SetScores save a list of scored items to redis.
func (redis *Redis) SetScores(prefix, name string, items []ScoredItem) error {
	var ctx = context.Background()
	key := prefix + "/" + name
	err := redis.client.Del(ctx, key).Err()
	if err != nil {
		return err
	}
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return err
		}
		err = redis.client.RPush(ctx, key, data).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

// GetScores returns a list of scored items from redis.
func (redis *Redis) GetScores(prefix, name string, begin, end int) ([]ScoredItem, error) {
	var ctx = context.Background()
	key := prefix + "/" + name
	res := make([]ScoredItem, 0)
	data, err := redis.client.LRange(ctx, key, int64(begin), int64(end)).Result()
	if err != nil {
		return nil, err
	}
	for _, s := range data {
		var item ScoredItem
		err = json.Unmarshal([]byte(s), &item)
		if err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, err
}

// ClearList clears a list of items in redis.
func (redis *Redis) ClearList(prefix, name string) error {
	var ctx = context.Background()
	key := prefix + "/" + name
	return redis.client.Del(ctx, key).Err()
}

// AppendList appends a list of scored items to redis.
func (redis *Redis) AppendList(prefix, name string, items ...string) error {
	var ctx = context.Background()
	key := prefix + "/" + name
	for _, item := range items {
		err := redis.client.RPush(ctx, key, item).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

// GetList returns a list of scored items from redis.
func (redis *Redis) GetList(prefix, name string) ([]string, error) {
	var ctx = context.Background()
	key := prefix + "/" + name
	res := make([]string, 0)
	data, err := redis.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	for _, s := range data {
		res = append(res, s)
	}
	return res, err
}

// GetString returns a string from redis.
func (redis *Redis) GetString(prefix, name string) (string, error) {
	var ctx = context.Background()
	key := prefix + "/" + name
	val, err := redis.client.Get(ctx, key).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return "", ErrObjectNotExist
		}
		return "", err
	}
	return val, err
}

// SetString saves a string to redis.
func (redis *Redis) SetString(prefix, name string, val string) error {
	var ctx = context.Background()
	key := prefix + "/" + name
	if err := redis.client.Set(ctx, key, val, 0).Err(); err != nil {
		return err
	}
	return nil
}

// GetInt returns a integer from redis.
func (redis *Redis) GetInt(prefix, name string) (int, error) {
	val, err := redis.GetString(prefix, name)
	if err != nil {
		return -1, nil
	}
	buf, err := strconv.Atoi(val)
	if err != nil {
		return -1, err
	}
	return buf, err
}

// SetInt saves a integer from redis.
func (redis *Redis) SetInt(prefix, name string, val int) error {
	return redis.SetString(prefix, name, strconv.Itoa(val))
}
