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
	startTime := time.Now()
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
	SetScoresSeconds.Observe(time.Since(startTime).Seconds())
	return nil
}

// GetScores returns a list of scored items from Redis.
func (r *Redis) GetScores(prefix, name string, begin, end int) ([]Scored, error) {
	startTime := time.Now()
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
	GetScoresSeconds.Observe(time.Since(startTime).Seconds())
	return res, err
}

// SetCategoryScores method of NoDatabase returns ErrNoDatabase.
func (r *Redis) SetCategoryScores(prefix, name, category string, items []Scored) error {
	if category != "" {
		name += "/" + category
	}
	return r.SetScores(prefix, name, items)
}

// GetCategoryScores method of NoDatabase returns ErrNoDatabase.
func (r *Redis) GetCategoryScores(prefix, name, category string, begin, end int) ([]Scored, error) {
	if category != "" {
		name += "/" + category
	}
	return r.GetScores(prefix, name, begin, end)
}

// ClearScores clears a list of scored items in Redis.
func (r *Redis) ClearScores(prefix, name string) error {
	startTime := time.Now()
	var ctx = context.Background()
	key := prefix + "/" + name
	err := r.client.Del(ctx, key).Err()
	if err == nil {
		ClearScoresSeconds.Observe(time.Since(startTime).Seconds())
	}
	return err
}

// AppendScores appends a list of scored items to Redis.
func (r *Redis) AppendScores(prefix, name string, items ...Scored) error {
	startTime := time.Now()
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
	AppendScoresSeconds.Observe(time.Since(startTime).Seconds())
	return nil
}

// GetString returns a string from Redis.
func (r *Redis) GetString(prefix, name string) (string, error) {
	var ctx = context.Background()
	key := prefix + "/" + name
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", errors.Annotate(ErrObjectNotExist, key)
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

// Exists check keys in Redis.
func (r *Redis) Exists(prefix string, names ...string) ([]int, error) {
	ctx := context.Background()
	pipeline := r.client.Pipeline()
	commands := make([]*redis.IntCmd, len(names))
	for i, name := range names {
		key := prefix + "/" + name
		commands[i] = pipeline.Exists(ctx, key)
	}
	_, err := pipeline.Exec(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	existences := make([]int, len(names))
	for i := range existences {
		existences[i] = int(commands[i].Val())
	}
	return existences, nil
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

// Delete object from Redis.
func (r *Redis) Delete(prefix, name string) error {
	ctx := context.Background()
	key := prefix + "/" + name
	return r.client.Del(ctx, key).Err()
}

// GetSet returns members of a set from Redis.
func (r *Redis) GetSet(key string) ([]string, error) {
	ctx := context.Background()
	return r.client.SMembers(ctx, key).Result()
}

// SetSet overrides a set with members in Redis.
func (r *Redis) SetSet(key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	// convert strings to interfaces
	values := make([]interface{}, 0, len(members))
	for _, member := range members {
		values = append(values, member)
	}
	// push set
	ctx := context.Background()
	pipeline := r.client.Pipeline()
	pipeline.Del(ctx, key)
	pipeline.SAdd(ctx, key, values...)
	_, err := pipeline.Exec(ctx)
	return err
}

// AddSet adds members to a set in Redis.
func (r *Redis) AddSet(key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	// convert strings to interfaces
	values := make([]interface{}, 0, len(members))
	for _, member := range members {
		values = append(values, member)
	}
	// push set
	ctx := context.Background()
	return r.client.SAdd(ctx, key, values...).Err()
}

// RemSet removes members from a set in Redis.
func (r *Redis) RemSet(key string, members ...string) error {
	ctx := context.Background()
	return r.client.SRem(ctx, key, members).Err()
}

// GetSortedScore get the score of a member from sorted set.
func (r *Redis) GetSortedScore(key, member string) (float32, error) {
	ctx := context.Background()
	score, err := r.client.ZScore(ctx, key, member).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, errors.Annotate(ErrObjectNotExist, key)
		}
		return 0, err
	}
	return float32(score), nil
}

// GetSorted get scores from sorted set.
func (r *Redis) GetSorted(key string, begin, end int) ([]Scored, error) {
	ctx := context.Background()
	members, err := r.client.ZRevRangeWithScores(ctx, key, int64(begin), int64(end)).Result()
	if err != nil {
		return nil, err
	}
	results := make([]Scored, 0, len(members))
	for _, member := range members {
		results = append(results, Scored{Id: member.Member.(string), Score: float32(member.Score)})
	}
	return results, nil
}

// SetSorted add scores to sorted set.
func (r *Redis) SetSorted(key string, scores []Scored) error {
	if len(scores) == 0 {
		return nil
	}
	ctx := context.Background()
	members := make([]*redis.Z, 0, len(scores))
	for _, score := range scores {
		members = append(members, &redis.Z{Member: score.Id, Score: float64(score.Score)})
	}
	return r.client.ZAdd(ctx, key, members...).Err()
}

// IncrSorted increase score in sorted set.
func (r *Redis) IncrSorted(key, member string) error {
	ctx := context.Background()
	return r.client.ZIncrBy(ctx, key, 1, member).Err()
}

// RemSorted method of NoDatabase returns ErrNoDatabase.
func (r *Redis) RemSorted(key, member string) error {
	ctx := context.Background()
	return r.client.ZRem(ctx, key, member).Err()
}
