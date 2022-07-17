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
	"github.com/go-redis/redis/v8"
	"github.com/juju/errors"
	"github.com/zhenghaoz/gorse/storage"
	"strconv"
)

// Redis cache storage.
type Redis struct {
	storage.TablePrefix
	client *redis.Client
}

// Close redis connection.
func (r *Redis) Close() error {
	return r.client.Close()
}

// Init nothing.
func (r *Redis) Init() error {
	return nil
}

func (r *Redis) Scan(work func(string) error) error {
	var (
		ctx    = context.Background()
		result []string
		cursor uint64
		err    error
	)
	for {
		result, cursor, err = r.client.Scan(ctx, cursor, string(r.TablePrefix)+"*", 0).Result()
		if err != nil {
			return errors.Trace(err)
		}
		for _, key := range result {
			if err = work(key[len(r.TablePrefix):]); err != nil {
				return errors.Trace(err)
			}
		}
		if cursor == 0 {
			return nil
		}
	}
}

func (r *Redis) Set(values ...Value) error {
	var ctx = context.Background()
	p := r.client.Pipeline()
	for _, v := range values {
		if err := p.Set(ctx, r.Key(v.name), v.value, 0).Err(); err != nil {
			return errors.Trace(err)
		}
	}
	_, err := p.Exec(ctx)
	return errors.Trace(err)
}

// Get returns a value from Redis.
func (r *Redis) Get(key string) *ReturnValue {
	var ctx = context.Background()
	val, err := r.client.Get(ctx, r.Key(key)).Result()
	if err != nil {
		if err == redis.Nil {
			return &ReturnValue{err: errors.Annotate(ErrObjectNotExist, key)}
		}
		return &ReturnValue{err: err}
	}
	return &ReturnValue{value: val}
}

// Delete object from Redis.
func (r *Redis) Delete(key string) error {
	ctx := context.Background()
	return r.client.Del(ctx, r.Key(key)).Err()
}

// GetSet returns members of a set from Redis.
func (r *Redis) GetSet(key string) ([]string, error) {
	ctx := context.Background()
	return r.client.SMembers(ctx, r.Key(key)).Result()
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
	pipeline.Del(ctx, r.Key(key))
	pipeline.SAdd(ctx, r.Key(key), values...)
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
	return r.client.SAdd(ctx, r.Key(key), values...).Err()
}

// RemSet removes members from a set in Redis.
func (r *Redis) RemSet(key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	ctx := context.Background()
	return r.client.SRem(ctx, r.Key(key), members).Err()
}

// GetSorted get scores from sorted set.
func (r *Redis) GetSorted(key string, begin, end int) ([]Scored, error) {
	ctx := context.Background()
	members, err := r.client.ZRevRangeWithScores(ctx, r.Key(key), int64(begin), int64(end)).Result()
	if err != nil {
		return nil, err
	}
	results := make([]Scored, 0, len(members))
	for _, member := range members {
		results = append(results, Scored{Id: member.Member.(string), Score: member.Score})
	}
	return results, nil
}

func (r *Redis) GetSortedByScore(key string, begin, end float64) ([]Scored, error) {
	ctx := context.Background()
	members, err := r.client.ZRangeByScoreWithScores(ctx, r.Key(key), &redis.ZRangeBy{
		Min:    strconv.FormatFloat(begin, 'g', -1, 64),
		Max:    strconv.FormatFloat(end, 'g', -1, 64),
		Offset: 0,
		Count:  -1,
	}).Result()
	if err != nil {
		return nil, err
	}
	results := make([]Scored, 0, len(members))
	for _, member := range members {
		results = append(results, Scored{Id: member.Member.(string), Score: member.Score})
	}
	return results, nil
}

func (r *Redis) RemSortedByScore(key string, begin, end float64) error {
	ctx := context.Background()
	return r.client.ZRemRangeByScore(ctx, r.Key(key),
		strconv.FormatFloat(begin, 'g', -1, 64),
		strconv.FormatFloat(end, 'g', -1, 64)).
		Err()
}

// AddSorted add scores to sorted set.
func (r *Redis) AddSorted(sortedSets ...SortedSet) error {
	ctx := context.Background()
	p := r.client.Pipeline()
	for _, sorted := range sortedSets {
		if len(sorted.scores) > 0 {
			members := make([]*redis.Z, 0, len(sorted.scores))
			for _, score := range sorted.scores {
				members = append(members, &redis.Z{Member: score.Id, Score: score.Score})
			}
			p.ZAdd(ctx, r.Key(sorted.name), members...)
		}
	}
	_, err := p.Exec(ctx)
	return err
}

// SetSorted set scores in sorted set and clear previous scores.
func (r *Redis) SetSorted(key string, scores []Scored) error {
	members := make([]*redis.Z, 0, len(scores))
	for _, score := range scores {
		members = append(members, &redis.Z{Member: score.Id, Score: float64(score.Score)})
	}
	ctx := context.Background()
	pipeline := r.client.Pipeline()
	pipeline.Del(ctx, r.Key(key))
	if len(scores) > 0 {
		pipeline.ZAdd(ctx, r.Key(key), members...)
	}
	_, err := pipeline.Exec(ctx)
	return err
}

// RemSorted method of NoDatabase returns ErrNoDatabase.
func (r *Redis) RemSorted(members ...SetMember) error {
	if len(members) == 0 {
		return nil
	}
	ctx := context.Background()
	pipe := r.client.Pipeline()
	for _, member := range members {
		pipe.ZRem(ctx, r.Key(member.name), member.member)
	}
	_, err := pipe.Exec(ctx)
	return errors.Trace(err)
}
