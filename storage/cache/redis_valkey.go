// Copyright 2025 gorse Project Authors
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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	semconv "go.opentelemetry.io/otel/semconv/v1.8.0"
	"go.uber.org/zap"
)

func init() {
	// Valkey is wire-compatible with Redis. We reuse the go-redis client but
	// wrap it in RedisValkey which provides a sorted-set-based time series
	// implementation (Valkey lacks the Redis TimeSeries module).
	Register([]string{storage.ValkeyPrefix, storage.ValkeysPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		// Rewrite valkey:// or valkeys:// to redis:// or rediss:// for go-redis URL parsing.
		var newURL string
		if strings.HasPrefix(path, storage.ValkeysPrefix) {
			newURL = strings.Replace(path, storage.ValkeysPrefix, storage.RedissPrefix, 1)
		} else {
			newURL = strings.Replace(path, storage.ValkeyPrefix, storage.RedisPrefix, 1)
		}
		opt, err := redis.ParseURL(newURL)
		if err != nil {
			return nil, err
		}
		opt.Protocol = 2
		database := &RedisValkey{}
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		database.client = redis.NewClient(opt)
		database.maxSearchResults = storage.NewOptions(opts...).MaxSearchResults
		if err = redisotel.InstrumentTracing(database.client, redisotel.WithAttributes(semconv.DBSystemRedis)); err != nil {
			log.Logger().Error("failed to add tracing for valkey", zap.Error(err))
			return nil, errors.Trace(err)
		}
		return database, nil
	})
	Register([]string{storage.ValkeyClusterPrefix, storage.ValkeysClusterPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		var newURL string
		if strings.HasPrefix(path, storage.ValkeyClusterPrefix) {
			newURL = strings.Replace(path, storage.ValkeyClusterPrefix, storage.RedisPrefix, 1)
		} else if strings.HasPrefix(path, storage.ValkeysClusterPrefix) {
			newURL = strings.Replace(path, storage.ValkeysClusterPrefix, storage.RedissPrefix, 1)
		}
		opt, err := redis.ParseClusterURL(newURL)
		if err != nil {
			return nil, err
		}
		opt.Protocol = 2
		database := &RedisValkey{}
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		database.client = redis.NewClusterClient(opt)
		database.maxSearchResults = storage.NewOptions(opts...).MaxSearchResults
		if err = redisotel.InstrumentTracing(database.client, redisotel.WithAttributes(semconv.DBSystemRedis)); err != nil {
			log.Logger().Error("failed to add tracing for valkey", zap.Error(err))
			return nil, errors.Trace(err)
		}
		return database, nil
	})
}

// RedisValkey embeds Redis and overrides only the time series methods.
// All other operations (KV, queue, scores/search, scan, purge) are inherited
// from Redis since Valkey is wire-compatible for those commands.
type RedisValkey struct {
	Redis
}

// AddTimeSeriesPoints stores time series points using sorted sets + hashes.
// Each series uses two keys:
//   - Sorted set (ts_index:{name}): score = timestamp_ms, member = timestamp_ms string
//   - Hash (ts_data:{name}): field = timestamp_ms string, value = float64 string
//
// ZADD naturally handles duplicate timestamps (updates score), and HSET
// naturally overwrites (last-write-wins), matching Redis TimeSeries LAST policy.
func (v *RedisValkey) AddTimeSeriesPoints(ctx context.Context, points []TimeSeriesPoint) error {
	if len(points) == 0 {
		return nil
	}
	p := v.client.Pipeline()
	for _, point := range points {
		tsMsStr := strconv.FormatInt(point.Timestamp.UnixMilli(), 10)
		indexKey := v.PointsTable() + ":ts_index:" + point.Name
		dataKey := v.PointsTable() + ":ts_data:" + point.Name
		p.ZAdd(ctx, indexKey, redis.Z{Score: float64(point.Timestamp.UnixMilli()), Member: tsMsStr})
		p.HSet(ctx, dataKey, tsMsStr, strconv.FormatFloat(point.Value, 'g', -1, 64))
	}
	_, err := p.Exec(ctx)
	return errors.Trace(err)
}

// GetTimeSeriesPoints retrieves time series points within [begin, end] and
// aggregates them into buckets of the given duration, returning the last value
// per bucket. This mirrors the behavior of Redis TS.RANGE with LAST aggregator.
func (v *RedisValkey) GetTimeSeriesPoints(ctx context.Context, name string, begin, end time.Time, duration time.Duration) ([]TimeSeriesPoint, error) {
	indexKey := v.PointsTable() + ":ts_index:" + name
	dataKey := v.PointsTable() + ":ts_data:" + name

	beginMs := begin.UnixMilli()
	endMs := end.UnixMilli()

	// Fetch all timestamps in range from sorted set.
	members, err := v.client.ZRangeByScore(ctx, indexKey, &redis.ZRangeBy{
		Min: strconv.FormatInt(beginMs, 10),
		Max: strconv.FormatInt(endMs, 10),
	}).Result()
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(members) == 0 {
		return nil, nil
	}

	// Fetch corresponding values from hash.
	vals, err := v.client.HMGet(ctx, dataKey, members...).Result()
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Bucket aggregation: group by (timestamp / duration) * duration,
	// keeping the last (highest timestamp) value per bucket.
	durationMs := duration.Milliseconds()
	type bucketEntry struct {
		timestamp int64
		value     float64
	}
	buckets := make(map[int64]*bucketEntry)

	for i, member := range members {
		if vals[i] == nil {
			continue
		}
		ts, err := strconv.ParseInt(member, 10, 64)
		if err != nil {
			continue
		}
		valStr, ok := vals[i].(string)
		if !ok {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}
		bucketKey := (ts / durationMs) * durationMs
		if existing, ok := buckets[bucketKey]; !ok || ts > existing.timestamp {
			buckets[bucketKey] = &bucketEntry{timestamp: ts, value: val}
		}
	}

	// Sort bucket keys and build result.
	sortedKeys := make([]int64, 0, len(buckets))
	for k := range buckets {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Slice(sortedKeys, func(i, j int) bool { return sortedKeys[i] < sortedKeys[j] })

	points := make([]TimeSeriesPoint, 0, len(sortedKeys))
	for _, bk := range sortedKeys {
		entry := buckets[bk]
		points = append(points, TimeSeriesPoint{
			Name:      name,
			Value:     entry.value,
			Timestamp: time.UnixMilli(bk).UTC(),
		})
	}
	return points, nil
}
