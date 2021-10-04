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
	"github.com/juju/errors"
	"strconv"
	"time"

	"github.com/araddon/dateparse"

	"github.com/go-redis/redis/v8"
)

// Redis cache storage.
type Redis struct {
	client *redis.Client
}

// Close redis connection.
func (r *Redis) Close() error {
	return r.client.Close()
}

// SetScores save a list of scored items to Redis.
func (r *Redis) SetScores(prefix, name string, items []Scored) error {
	var ctx = context.Background()
	key := prefix + "/" + name
	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return errors.Trace(err)
	}
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return errors.Trace(err)
		}
		err = r.client.RPush(ctx, key, data).Err()
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// GetScores returns a list of scored items from Redis.
func (r *Redis) GetScores(prefix, name string, begin, end int) ([]Scored, error) {
	var ctx = context.Background()
	key := prefix + "/" + name
	res := make([]Scored, 0)
	data, err := r.client.LRange(ctx, key, int64(begin), int64(end)).Result()
	if err != nil {
		return nil, err
	}
	for _, s := range data {
		var item Scored
		err = json.Unmarshal([]byte(s), &item)
		if err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, err
}

// ClearScores clears a list of scored items in Redis.
func (r *Redis) ClearScores(prefix, name string) error {
	var ctx = context.Background()
	key := prefix + "/" + name
	return r.client.Del(ctx, key).Err()
}

// AppendScores appends a list of scored items to Redis.
func (r *Redis) AppendScores(prefix, name string, items ...Scored) error {
	var ctx = context.Background()
	key := prefix + "/" + name
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return errors.Trace(err)
		}
		err = r.client.RPush(ctx, key, data).Err()
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// GetString returns a string from Redis.
func (r *Redis) GetString(prefix, name string) (string, error) {
	var ctx = context.Background()
	key := prefix + "/" + name
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", ErrObjectNotExist
		}
		return "", err
	}
	return val, err
}

// SetString saves a string to Redis.
func (r *Redis) SetString(prefix, name, val string) error {
	var ctx = context.Background()
	key := prefix + "/" + name
	if err := r.client.Set(ctx, key, val, 0).Err(); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// GetInt returns a integer from Redis.
func (r *Redis) GetInt(prefix, name string) (int, error) {
	val, err := r.GetString(prefix, name)
	if err != nil {
		return 0, nil
	}
	buf, err := strconv.Atoi(val)
	if err != nil {
		return 0, err
	}
	return buf, err
}

// SetInt saves a integer from Redis.
func (r *Redis) SetInt(prefix, name string, val int) error {
	return r.SetString(prefix, name, strconv.Itoa(val))
}

// IncrInt increase a integer in Redis.
func (r *Redis) IncrInt(prefix, name string) error {
	var ctx = context.Background()
	key := prefix + "/" + name
	if err := r.client.Incr(ctx, key).Err(); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// GetTime returns a time from Redis.
func (r *Redis) GetTime(prefix, name string) (time.Time, error) {
	val, err := r.GetString(prefix, name)
	if err != nil {
		return time.Time{}, nil
	}
	tm, err := dateparse.ParseAny(val)
	if err != nil {
		return time.Time{}, nil
	}
	return tm, nil
}

// SetTime saves a time from Redis.
func (r *Redis) SetTime(prefix, name string, val time.Time) error {
	return r.SetString(prefix, name, val.String())
}
