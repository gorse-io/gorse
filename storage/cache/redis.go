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
	"strconv"

	"github.com/go-redis/redis/v8"
)

type Redis struct {
	client *redis.Client
}

func (redis *Redis) Close() error {
	return redis.client.Close()
}

func (redis *Redis) SetList(prefix, name string, items []string) error {
	var ctx = context.Background()
	key := prefix + "/" + name
	err := redis.client.Del(ctx, key).Err()
	if err != nil {
		return err
	}
	for _, itemId := range items {
		err = redis.client.RPush(ctx, key, itemId).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

func (redis *Redis) GetList(prefix, name string, begin, end int) ([]string, error) {
	var ctx = context.Background()
	key := prefix + "/" + name
	res := make([]string, 0)
	data, err := redis.client.LRange(ctx, key, int64(begin), int64(end)).Result()

	if err != nil {
		return nil, err
	}

	res = append(res, data...)
	return res, err
}

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

func (redis *Redis) SetString(prefix, name string, val string) error {
	var ctx = context.Background()
	key := prefix + "/" + name
	if err := redis.client.Set(ctx, key, val, 0).Err(); err != nil {
		return err
	}
	return nil
}

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

func (redis *Redis) SetInt(prefix, name string, val int) error {
	return redis.SetString(prefix, name, strconv.Itoa(val))
}
