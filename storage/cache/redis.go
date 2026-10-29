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
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/util"
	"github.com/gorse-io/gorse/storage"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	semconv "go.opentelemetry.io/otel/semconv/v1.8.0"
	"go.uber.org/zap"
)

func init() {
	Register([]string{storage.RedisPrefix, storage.RedissPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		opt, err := redis.ParseURL(path)
		if err != nil {
			return nil, err
		}
		opt.Protocol = 2
		option := storage.NewOptions(opts...)
		if option.RedisClientName != "" {
			opt.ClientName = option.RedisClientName
		}
		database := new(Redis)
		database.client = redis.NewClient(opt)
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		database.maxSearchResults = option.MaxSearchResults
		if err = redisotel.InstrumentTracing(database.client, redisotel.WithAttributes(semconv.DBSystemRedis)); err != nil {
			log.Logger().Error("failed to add tracing for redis", zap.Error(err))
			return nil, errors.WithStack(err)
		}
		return database, nil
	})
	Register([]string{storage.RedisClusterPrefix, storage.RedissClusterPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		var newURL string
		if strings.HasPrefix(path, storage.RedisClusterPrefix) {
			newURL = strings.Replace(path, storage.RedisClusterPrefix, storage.RedisPrefix, 1)
		} else if strings.HasPrefix(path, storage.RedissClusterPrefix) {
			newURL = strings.Replace(path, storage.RedissClusterPrefix, storage.RedissPrefix, 1)
		}
		opt, err := redis.ParseClusterURL(newURL)
		if err != nil {
			return nil, err
		}
		opt.Protocol = 2
		option := storage.NewOptions(opts...)
		if option.RedisClientName != "" {
			opt.ClientName = option.RedisClientName
		}
		database := new(Redis)
		database.client = redis.NewClusterClient(opt)
		database.TablePrefix = storage.TablePrefix(tablePrefix)
		database.maxSearchResults = option.MaxSearchResults
		if err = redisotel.InstrumentTracing(database.client, redisotel.WithAttributes(semconv.DBSystemRedis)); err != nil {
			log.Logger().Error("failed to add tracing for redis", zap.Error(err))
			return nil, errors.WithStack(err)
		}
		return database, nil
	})
}

// Redis cache storage.
type Redis struct {
	storage.TablePrefix
	client           redis.UniversalClient
	maxSearchResults int
}

// Close redis connection.
func (r *Redis) Close() error {
	return r.client.Close()
}

func (r *Redis) Ping() error {
	return r.client.Ping(context.Background()).Err()
}

// Init nothing.
func (r *Redis) Init() error {
	// list indices
	indices, err := r.client.FT_List(context.Background()).Result()
	if err != nil {
		return errors.WithStack(err)
	}
	// create index
	if !lo.Contains(indices, r.DocumentTable()) {
		_, err = r.client.FTCreate(context.TODO(), r.DocumentTable(),
			&redis.FTCreateOptions{
				OnHash: true,
				Prefix: []any{r.DocumentTable() + ":"},
			},
			&redis.FieldSchema{FieldName: "collection", FieldType: redis.SearchFieldTypeTag},
			&redis.FieldSchema{FieldName: "subset", FieldType: redis.SearchFieldTypeTag},
			&redis.FieldSchema{FieldName: "id", FieldType: redis.SearchFieldTypeTag},
			&redis.FieldSchema{FieldName: "score", FieldType: redis.SearchFieldTypeNumeric},
			&redis.FieldSchema{FieldName: "is_hidden", FieldType: redis.SearchFieldTypeNumeric},
			&redis.FieldSchema{FieldName: "categories", FieldType: redis.SearchFieldTypeTag, Separator: ";"},
			&redis.FieldSchema{FieldName: "timestamp", FieldType: redis.SearchFieldTypeNumeric},
		).Result()
		if err != nil && !strings.Contains(err.Error(), "Index already exists") {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (r *Redis) Scan(work func(string) error) error {
	ctx := context.Background()
	if clusterClient, isCluster := r.client.(*redis.ClusterClient); isCluster {
		return clusterClient.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			return r.scan(ctx, client, work)
		})
	} else {
		return r.scan(ctx, r.client, work)
	}
}

func (r *Redis) scan(ctx context.Context, client redis.UniversalClient, work func(string) error) error {
	var (
		result []string
		cursor uint64
		err    error
	)
	for {
		result, cursor, err = client.Scan(ctx, cursor, string(r.TablePrefix)+"*", 0).Result()
		if err != nil {
			return errors.WithStack(err)
		}
		for _, key := range result {
			if err = work(key[len(r.TablePrefix):]); err != nil {
				return errors.WithStack(err)
			}
		}
		if cursor == 0 {
			return nil
		}
	}
}

func (r *Redis) Purge() error {
	ctx := context.Background()
	if clusterClient, isCluster := r.client.(*redis.ClusterClient); isCluster {
		return clusterClient.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			return r.purge(ctx, client, isCluster)
		})
	} else {
		return r.purge(ctx, r.client, isCluster)
	}
}

func (r *Redis) purge(ctx context.Context, client redis.UniversalClient, isCluster bool) error {
	if err := r.purgeGeoIndexes(ctx, client); err != nil {
		return errors.WithStack(err)
	}
	var (
		result []string
		cursor uint64
		err    error
	)
	for {
		result, cursor, err = client.Scan(ctx, cursor, string(r.TablePrefix)+"*", 0).Result()
		if err != nil {
			return errors.WithStack(err)
		}
		if len(result) > 0 {
			if isCluster {
				p := client.Pipeline()
				for _, key := range result {
					if err = p.Del(ctx, key).Err(); err != nil {
						return errors.WithStack(err)
					}
				}
				if _, err = p.Exec(ctx); err != nil {
					return errors.WithStack(err)
				}
			} else {
				if err = client.Del(ctx, result...).Err(); err != nil {
					return errors.WithStack(err)
				}
			}
		}
		if cursor == 0 {
			return nil
		}
	}
}

func (r *Redis) Set(ctx context.Context, values ...Value) error {
	p := r.client.Pipeline()
	for _, v := range values {
		if err := p.Set(ctx, r.Key(v.name), v.value, 0).Err(); err != nil {
			return errors.WithStack(err)
		}
	}
	_, err := p.Exec(ctx)
	return errors.WithStack(err)
}

// Get returns a value from Redis.
func (r *Redis) Get(ctx context.Context, key string) *ReturnValue {
	val, err := r.client.Get(ctx, r.Key(key)).Result()
	if err != nil {
		if err == redis.Nil {
			return &ReturnValue{value: "", exists: false}
		}
		return &ReturnValue{err: err, exists: false}
	}
	return &ReturnValue{value: val, exists: true}
}

// Delete object from Redis.
func (r *Redis) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, r.Key(key)).Err()
}

func (r *Redis) documentKey(collection, subset, value string) string {
	return r.DocumentTable() + ":" + collection + ":" + subset + ":" + value
}

func (r *Redis) AddScores(ctx context.Context, collection, subset string, documents []Score) error {
	p := r.client.Pipeline()
	for _, document := range documents {
		p.HSet(ctx, r.documentKey(collection, subset, document.Id),
			"collection", collection,
			"subset", subset,
			"id", document.Id,
			"score", document.Score,
			"is_hidden", document.IsHidden,
			"categories", encodeCategories(document.Categories),
			"timestamp", document.Timestamp.UnixMicro())
	}
	_, err := p.Exec(ctx)
	return errors.WithStack(err)
}

func (r *Redis) SearchScores(ctx context.Context, collection, subset string, query []string, begin, end int) ([]Score, error) {
	var builder strings.Builder
	fmt.Fprintf(&builder, "@collection:{ %s } @is_hidden:[0 0]", escape(collection))
	if subset != "" {
		fmt.Fprintf(&builder, " @subset:{ %s }", escape(subset))
	}
	for _, q := range query {
		fmt.Fprintf(&builder, " @categories:{ %s }", escape(encodeCategory(q)))
	}
	options := &redis.FTSearchOptions{
		SortBy:      []redis.FTSearchSortBy{{FieldName: "score", Desc: true}},
		LimitOffset: begin,
	}
	if end == -1 {
		options.Limit = 10000
	} else {
		options.Limit = end - begin
	}
	result, err := r.client.FTSearchWithArgs(ctx, r.DocumentTable(), builder.String(), options).Result()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	documents := make([]Score, 0, len(result.Docs))
	for _, doc := range result.Docs {
		var document Score
		document.Id = doc.Fields["id"]
		score, err := strconv.ParseFloat(doc.Fields["score"], 64)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		document.Score = score
		isHidden, err := strconv.ParseInt(doc.Fields["is_hidden"], 10, 64)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		document.IsHidden = isHidden != 0
		categories, err := decodeCategories(doc.Fields["categories"])
		if err != nil {
			return nil, errors.WithStack(err)
		}
		document.Categories = categories
		timestamp, err := strconv.ParseInt(doc.Fields["timestamp"], 10, 64)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		document.Timestamp = time.UnixMicro(timestamp).In(time.UTC)
		documents = append(documents, document)
	}
	return documents, nil
}

func (r *Redis) UpdateScores(ctx context.Context, collections []string, subset *string, id string, patch ScorePatch) error {
	if len(collections) == 0 {
		return nil
	}
	if patch.Score == nil && patch.IsHidden == nil && patch.Categories == nil {
		return nil
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "@collection:{ %s }", escape(strings.Join(collections, " | ")))
	fmt.Fprintf(&builder, " @id:{ %s }", escape(id))
	if subset != nil {
		fmt.Fprintf(&builder, " @subset:{ %s }", escape(*subset))
	}
	limit := r.maxSearchResults
	if limit <= 0 {
		limit = 10000
	}

	// Two-phase update:
	// 1) collect matched document IDs with pagination,
	// 2) mutate documents by key.
	// This avoids pagination drift when patch.Score changes the sort order.
	keys := make([]string, 0)
	keySet := make(map[string]struct{})
	offset := 0
	for {
		// search documents
		result, err := r.client.FTSearchWithArgs(ctx, r.DocumentTable(), builder.String(), &redis.FTSearchOptions{
			SortBy:      []redis.FTSearchSortBy{{FieldName: "score", Desc: true}},
			LimitOffset: offset,
			Limit:       limit,
		}).Result()
		if err != nil {
			return errors.WithStack(err)
		}
		if len(result.Docs) == 0 {
			break
		}
		newKeys := 0
		for _, doc := range result.Docs {
			if _, exists := keySet[doc.ID]; !exists {
				keySet[doc.ID] = struct{}{}
				keys = append(keys, doc.ID)
				newKeys++
			}
		}
		offset += len(result.Docs)
		// Stop when:
		// 1) the last page is shorter than the limit (common Redis behavior), or
		// 2) no new keys are discovered (defensive for engines with non-standard total/offset semantics).
		if len(result.Docs) < limit || newKeys == 0 {
			break
		}
	}

	values := make([]any, 0)
	if patch.Score != nil {
		values = append(values, "score", *patch.Score)
	}
	if patch.IsHidden != nil {
		values = append(values, "is_hidden", *patch.IsHidden)
	}
	if patch.Categories != nil {
		values = append(values, "categories", encodeCategories(patch.Categories))
	}
	for _, key := range keys {
		if err := r.client.Watch(ctx, func(tx *redis.Tx) error {
			if exist, err := tx.Exists(ctx, key).Result(); err != nil {
				return err
			} else if exist == 0 {
				return nil
			}
			return tx.HSet(ctx, key, values...).Err()
		}, key); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (r *Redis) DeleteScores(ctx context.Context, collections []string, condition ScoreCondition) error {
	if err := condition.Check(); err != nil {
		return errors.WithStack(err)
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "@collection:{ %s }", escape(strings.Join(collections, " | ")))
	if condition.Subset != nil {
		fmt.Fprintf(&builder, " @subset:{ %s }", escape(*condition.Subset))
	}
	if condition.Id != nil {
		fmt.Fprintf(&builder, " @id:{ %s }", escape(*condition.Id))
	}
	if condition.Before != nil {
		fmt.Fprintf(&builder, " @timestamp:[-inf (%d]", condition.Before.UnixMicro())
	}
	for {
		// search documents
		result, err := r.client.FTSearchWithArgs(ctx, r.DocumentTable(), builder.String(), &redis.FTSearchOptions{
			SortBy:      []redis.FTSearchSortBy{{FieldName: "score", Desc: true}},
			LimitOffset: 0,
			Limit:       10000,
		}).Result()
		if err != nil {
			return errors.WithStack(err)
		}
		// delete documents
		p := r.client.Pipeline()
		for _, doc := range result.Docs {
			p.Del(ctx, doc.ID)
		}
		_, err = p.Exec(ctx)
		if err != nil {
			return errors.WithStack(err)
		}
		// break if no more documents
		if result.Total == len(result.Docs) {
			break
		}
	}
	return nil
}

func (r *Redis) ScanScores(ctx context.Context, callback func(collection string, id string, subset string, timestamp time.Time) error) error {
	if clusterClient, isCluster := r.client.(*redis.ClusterClient); isCluster {
		return clusterClient.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			return r.scanScores(ctx, client, callback)
		})
	} else {
		return r.scanScores(ctx, r.client, callback)
	}
}

// scanScoresPageSize is the COUNT hint given to SCAN, and therefore the size of each
// pipelined batch of HGETALL. It replaces a COUNT of 0, i.e. Redis' default of 10.
const scanScoresPageSize = 1000

func (r *Redis) scanScores(ctx context.Context, client redis.UniversalClient, callback func(collection string, id string, subset string, timestamp time.Time) error) error {
	var (
		result []string
		cursor uint64
		err    error
	)
	for {
		result, cursor, err = client.Scan(ctx, cursor, r.DocumentTable()+"*", scanScoresPageSize).Result()
		if err != nil {
			return errors.WithStack(err)
		}
		if len(result) > 0 {
			// Fetch the whole page in one round trip. The callback still runs once per
			// key, in scan order, so the caller's semantics are unchanged -- only the
			// number of round trips is.
			pipe := client.Pipeline()
			cmds := make([]*redis.MapStringStringCmd, len(result))
			for i, key := range result {
				cmds[i] = pipe.HGetAll(ctx, key)
			}
			if _, err = pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
				return errors.WithStack(err)
			}
			for _, cmd := range cmds {
				// The page was fetched in one shot, so the per-key Redis calls no longer
				// observe cancellation on the caller's behalf. Check it here: callers rely
				// on ScanScores returning promptly once their context is done, not only at
				// page boundaries (see TestScanScores in database_test.go).
				if err = ctx.Err(); err != nil {
					return errors.WithStack(err)
				}
				var row map[string]string
				row, err = cmd.Result()
				if err != nil {
					if errors.Is(err, redis.Nil) {
						// The key was deleted between SCAN and HGETALL. Scanning is
						// inherently racy, so skip it rather than failing the pass.
						continue
					}
					return errors.WithStack(err)
				}
				var usec int64
				usec, err = util.ParseInt[int64](row["timestamp"])
				if err != nil {
					return errors.WithStack(err)
				}
				if err = callback(row["collection"], row["id"], row["subset"], time.UnixMicro(usec).In(time.UTC)); err != nil {
					return errors.WithStack(err)
				}
			}
		}
		if cursor == 0 {
			return nil
		}
	}
}

func (r *Redis) AddTimeSeriesPoints(ctx context.Context, points []TimeSeriesPoint) error {
	p := r.client.Pipeline()
	opt := &redis.TSOptions{DuplicatePolicy: "LAST"}
	for _, point := range points {
		if err := p.TSAddWithArgs(ctx, r.PointsTable()+":"+point.Name, point.Timestamp.UnixMilli(), point.Value, opt).Err(); err != nil {
			return errors.WithStack(err)
		}
	}
	_, err := p.Exec(ctx)
	return errors.WithStack(err)
}

func (r *Redis) GetTimeSeriesPoints(ctx context.Context, name string, begin, end time.Time, duration time.Duration) ([]TimeSeriesPoint, error) {
	result, err := r.client.TSRangeWithArgs(ctx, r.PointsTable()+":"+name, int(begin.UnixMilli()), int(end.UnixMilli()),
		&redis.TSRangeOptions{Aggregator: redis.Last, BucketDuration: int(duration / time.Millisecond)}).Result()
	if err != nil {
		if redis.HasErrorPrefix(err, "TSDB: the key does not exist") ||
			redis.HasErrorPrefix(err, "key does not exist") {
			return []TimeSeriesPoint{}, nil
		}
		return nil, errors.WithStack(err)
	}
	points := make([]TimeSeriesPoint, 0, len(result))
	for _, doc := range result {
		var point TimeSeriesPoint
		point.Name = name
		point.Value = doc.Value
		point.Timestamp = time.UnixMilli(doc.Timestamp).UTC()
		points = append(points, point)
	}
	return points, nil
}

func encodeCategory(category string) string {
	return base64.RawStdEncoding.EncodeToString([]byte("_" + category))
}

func decodeCategory(s string) (string, error) {
	b, err := base64.RawStdEncoding.DecodeString(s)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return string(b[1:]), nil
}

func encodeCategories(categories []string) string {
	var builder strings.Builder
	for i, category := range categories {
		if i > 0 {
			builder.WriteByte(';')
		}
		builder.WriteString(encodeCategory(category))
	}
	return builder.String()
}

func decodeCategories(s string) ([]string, error) {
	if s == "" {
		return []string{}, nil
	}
	var categories []string
	for category := range strings.SplitSeq(s, ";") {
		category, err := decodeCategory(category)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		categories = append(categories, category)
	}
	return categories, nil
}

// escape -:.
func escape(s string) string {
	r := strings.NewReplacer(
		"-", "\\-",
		":", "\\:",
		".", "\\.",
		"/", "\\/",
		"+", "\\+",
	)
	return r.Replace(s)
}

const redisGeoBatchSize = 512

// Encode the namespace before adding the first hash tag. Raw prefixes such as
// "{}" would otherwise disable Redis Cluster's hash-tag extraction entirely.
func (r *Redis) geoKeyPrefix() string {
	return "gorse_geo_" + geoNameDigest(string(r.TablePrefix)) + ":"
}

// All three keys share the same first nonempty hash tag. Neither namespace nor
// index names can inject braces or key separators.
func (r *Redis) geoKeys(name string) ([]string, string) {
	prefix := string(r.TablePrefix)
	identity := make([]byte, 0, 16+len(prefix)+len(name))
	identity = binary.BigEndian.AppendUint64(identity, uint64(len(prefix)))
	identity = append(identity, prefix...)
	identity = binary.BigEndian.AppendUint64(identity, uint64(len(name)))
	identity = append(identity, name...)
	digest := sha256.Sum256(identity)
	base := fmt.Sprintf("%s{%x}:%s:", r.geoKeyPrefix(), digest, base64.RawURLEncoding.EncodeToString([]byte(name)))
	return []string{base + "meta", base + "index", base + "points"},
		"gorse-geo-v1:" + base64.RawURLEncoding.EncodeToString(identity)
}

// Lua checks every argument and existing payload in a block before writing:
// Redis does not roll back earlier commands when a script returns an error.
// JSON is checked but remains opaque on the write path; it is never re-encoded
// in Lua, which would lose the original Go float64 coordinate precision.
const redisGeoLua = `
local function fail(message)
    error('GORSE_GEO_' .. message, 0)
end
local function keytype(key)
    return redis.call('TYPE', key).ok
end
local function types()
    local m = keytype(KEYS[1])
    local z = keytype(KEYS[2])
    local h = keytype(KEYS[3])
    if (m ~= 'none' and m ~= 'string') or
       (z ~= 'none' and z ~= 'zset') or
       (h ~= 'none' and h ~= 'hash') then
        fail('CORRUPT key type')
    end
end
local function exists()
    local t = keytype(KEYS[1])
    if t == 'none' then fail('NOT_FOUND') end
    if t ~= 'string' then fail('CORRUPT metadata type') end
    if redis.call('GET', KEYS[1]) ~= ARGV[1] then fail('CORRUPT metadata') end
    types()
end
local function unpackpoint(value, id)
    if type(value) ~= 'string' then fail('CORRUPT missing payload') end
    local colon = string.find(value, ':', 1, true)
    if not colon then fail('CORRUPT payload frame') end
    local digits = string.sub(value, 1, colon - 1)
    if not string.match(digits, '^[1-9][0-9]*$') then fail('CORRUPT payload length') end
    local n = tonumber(digits)
    if not n or n < 15 or n > string.len(value) - colon - 1 then
        fail('CORRUPT payload length')
    end
    local member = string.sub(value, colon + 1, colon + n)
    local hex = string.sub(member, 1, 13)
    if not string.match(hex, '^[0-9a-f]+$') or string.len(hex) ~= 13 or
       string.sub(member, 14, 14) ~= ':' or string.sub(member, 15) ~= id then
        fail('CORRUPT payload member')
    end
    local payload = string.sub(value, colon + n + 1)
    local ok, point = pcall(cjson.decode, payload)
    if not ok or type(point) ~= 'table' or point.Id ~= id or
       type(point.Cell) ~= 'number' or point.Cell ~= tonumber(hex, 16) or
       type(point.Longitude) ~= 'number' or type(point.Latitude) ~= 'number' or
       not (point.Longitude >= -180 and point.Longitude <= 180) or
       not (point.Latitude >= -90 and point.Latitude <= 90) then
        fail('CORRUPT payload JSON')
    end
    return member, payload
end
local function oldpoint(id)
    local value = redis.call('HGET', KEYS[3], id)
    if not value then return false end
    local member = unpackpoint(value, id)
    local score = redis.call('ZSCORE', KEYS[2], member)
    if not score or tonumber(score) ~= 0 then fail('CORRUPT missing index member') end
    return member
end
`

var redisGeoCreate = redis.NewScript(redisGeoLua + `
types()
local value = redis.call('GET', KEYS[1])
if value then
    if value ~= ARGV[1] then fail('CORRUPT metadata') end
    fail('ALREADY_EXISTS')
end
if redis.call('EXISTS', KEYS[2], KEYS[3]) ~= 0 then fail('CORRUPT orphan keys') end
redis.call('SET', KEYS[1], ARGV[1], 'NX')
return 1
`)

var redisGeoCheck = redis.NewScript(redisGeoLua + `
exists()
return 1
`)

var redisGeoAdd = redis.NewScript(redisGeoLua + `
exists()
if (#ARGV - 1) % 2 ~= 0 or (#ARGV - 1) / 2 > 512 then fail('CORRUPT write batch') end
local old = {}
local members = {}
local seen = {}
for i = 2, #ARGV, 2 do
    local id = ARGV[i]
    if id == '' or seen[id] then fail('CORRUPT write id') end
    seen[id] = true
    members[i] = unpackpoint(ARGV[i+1], id)
    old[i] = oldpoint(id)
end
for i = 2, #ARGV, 2 do
    if old[i] then redis.call('ZREM', KEYS[2], old[i]) end
    redis.call('HSET', KEYS[3], ARGV[i], ARGV[i+1])
    redis.call('ZADD', KEYS[2], 0, members[i])
end
return 1
`)

var redisGeoDeletePoints = redis.NewScript(redisGeoLua + `
exists()
if #ARGV - 1 > 512 then fail('CORRUPT delete batch') end
local old = {}
local seen = {}
for i = 2, #ARGV do
    if ARGV[i] == '' or seen[ARGV[i]] then fail('CORRUPT delete id') end
    seen[ARGV[i]] = true
    old[i] = oldpoint(ARGV[i])
end
for i = 2, #ARGV do
    if old[i] then
        redis.call('ZREM', KEYS[2], old[i])
        redis.call('HDEL', KEYS[3], ARGV[i])
    end
end
return 1
`)

var redisGeoDrop = redis.NewScript(redisGeoLua + `
types()
local value = redis.call('GET', KEYS[1])
if value and value ~= ARGV[1] then fail('CORRUPT metadata') end
redis.call('DEL', KEYS[1], KEYS[2], KEYS[3])
return 1
`)

// A page and its payloads are read in the same script. The continuation is the
// full last member, not an offset, so concurrent deletions cannot skip a page.
var redisGeoRead = redis.NewScript(redisGeoLua + `
exists()
local members = redis.call('ZRANGE', KEYS[2], ARGV[2], ARGV[3], 'BYLEX', 'LIMIT', 0, 512)
local result = {''}
for _, member in ipairs(members) do
    local id = string.sub(member, 15)
    local value = redis.call('HGET', KEYS[3], id)
    local stored, payload = unpackpoint(value, id)
    if stored ~= member then fail('CORRUPT inconsistent member') end
    local score = redis.call('ZSCORE', KEYS[2], member)
    if not score or tonumber(score) ~= 0 then fail('CORRUPT member score') end
    result[#result+1] = member
    result[#result+1] = payload
    result[1] = member
end
return result
`)

func redisGeoError(name string, err error) error {
	if err == nil {
		return nil
	}
	// Redis may append script location details; use the unique error token,
	// not generic NOT_FOUND/EXISTS substrings from unrelated Redis errors.
	message := strings.TrimPrefix(err.Error(), "ERR ")
	if strings.HasPrefix(message, "GORSE_GEO_NOT_FOUND") {
		return fmt.Errorf("geo index %q %w", name, storage.ErrNotFound)
	}
	if strings.HasPrefix(message, "GORSE_GEO_ALREADY_EXISTS") {
		return fmt.Errorf("geo index %q %w", name, storage.ErrAlreadyExists)
	}
	return fmt.Errorf("geo index %q %w", name, err)
}

// AddGeoIndex explicitly creates an empty geographic index without a TTL.
func (r *Redis) AddGeoIndex(ctx context.Context, name string) error {
	if err := validateGeoName(name); err != nil {
		return err
	}
	keys, identity := r.geoKeys(name)
	return redisGeoError(name, redisGeoCreate.Run(ctx, r.client, keys, identity).Err())
}

// ListGeoIndex lists existing geographic indexes in this namespace.
func (r *Redis) ListGeoIndex(ctx context.Context) ([]string, error) {
	prefix := r.geoKeyPrefix()
	names := make(map[string]struct{})
	var mutex sync.Mutex
	scan := func(ctx context.Context, client redis.UniversalClient) error {
		var cursor uint64
		for {
			metadata, next, err := client.Scan(ctx, cursor, prefix+"*:meta", redisGeoBatchSize).Result()
			if err != nil {
				return errors.WithStack(err)
			}
			for _, key := range metadata {
				_, encoded, ok := strings.Cut(strings.TrimSuffix(strings.TrimPrefix(key, prefix), ":meta"), ":")
				if !ok {
					return fmt.Errorf("corrupt geo metadata key %q", key)
				}
				decoded, err := base64.RawURLEncoding.DecodeString(encoded)
				if err != nil {
					return fmt.Errorf("decode geo metadata key %q %w", key, err)
				}
				name := string(decoded)
				if err = validateGeoName(name); err != nil {
					return fmt.Errorf("corrupt geo metadata key %q %w", key, err)
				}
				keys, identity := r.geoKeys(name)
				if keys[0] != key {
					return fmt.Errorf("corrupt geo metadata key %q", key)
				}
				// Route validation through the universal client so Cluster can
				// follow redirects if the scanned slot moves to another master.
				if err = redisGeoError(name, redisGeoCheck.Run(ctx, r.client, keys, identity).Err()); err != nil {
					if errors.Is(err, storage.ErrNotFound) {
						// An index may be dropped between SCAN and validation.
						continue
					}
					return err
				}
				mutex.Lock()
				names[name] = struct{}{}
				mutex.Unlock()
			}
			cursor = next
			if cursor == 0 {
				return nil
			}
		}
	}
	if clusterClient, isCluster := r.client.(*redis.ClusterClient); isCluster {
		if err := clusterClient.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			return scan(ctx, client)
		}); err != nil {
			return nil, err
		}
	} else if err := scan(ctx, r.client); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

// AddGeoPoints upserts validated, last-write-wins points in bounded blocks.
func (r *Redis) AddGeoPoints(ctx context.Context, name string, points []GeoPoint) error {
	records, err := prepareGeoPoints(name, points)
	if err != nil {
		return err
	}
	keys, identity := r.geoKeys(name)
	// Encode the entire input before the first write as well as validating it.
	values := make([]any, 0, len(records)*2)
	for _, record := range records {
		payload, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("encode geo point %q %w", record.Id, err)
		}
		member := fmt.Sprintf("%013x:", record.Cell) + record.Id
		frame := strconv.Itoa(len(member)) + ":" + member + string(payload)
		values = append(values, record.Id, frame)
	}
	if len(values) == 0 {
		return redisGeoError(name, redisGeoCheck.Run(ctx, r.client, keys, identity).Err())
	}
	for start := 0; start < len(values); start += redisGeoBatchSize * 2 {
		end := min(start+redisGeoBatchSize*2, len(values))
		args := make([]any, 1, 1+end-start)
		args[0] = identity
		args = append(args, values[start:end]...)
		if err = redisGeoAdd.Run(ctx, r.client, keys, args...).Err(); err != nil {
			return redisGeoError(name, err)
		}
	}
	return nil
}

// SearchGeoPoints scans the shared cell ranges, then uses the shared exact
// spherical-distance and bounded-heap implementation to select results.
func (r *Redis) SearchGeoPoints(ctx context.Context, name string, query GeoQuery) ([]GeoResult, error) {
	query, err := normalizeGeoQuery(name, query)
	if err != nil {
		return nil, err
	}
	keys, identity := r.geoKeys(name)
	// The common search helper may short-circuit N == 0; existence still matters.
	if err = redisGeoCheck.Run(ctx, r.client, keys, identity).Err(); err != nil {
		return nil, redisGeoError(name, err)
	}
	results, err := searchGeoPoints(ctx, query, func(ctx context.Context, ranges []geoRange, yield func(geoRecord) error) error {
		for _, interval := range ranges {
			lower := fmt.Sprintf("[%013x:", interval.Min)
			upper := "+"
			if interval.Max < (1<<52)-1 {
				upper = fmt.Sprintf("(%013x:", interval.Max+1)
			}
			for {
				page, err := redisGeoRead.Run(ctx, r.client, keys, identity, lower, upper).Slice()
				if err != nil {
					return redisGeoError(name, err)
				}
				if len(page) == 0 || len(page)%2 != 1 || len(page) > 1+2*redisGeoBatchSize {
					return fmt.Errorf("geo index %q corrupt range response", name)
				}
				last, ok := page[0].(string)
				if !ok || (len(page) > 1 && last == "") {
					return fmt.Errorf("geo index %q corrupt range continuation", name)
				}
				for i := 1; i < len(page); i += 2 {
					member, memberOK := page[i].(string)
					payload, payloadOK := page[i+1].(string)
					if !memberOK || !payloadOK {
						return fmt.Errorf("geo index %q corrupt point response", name)
					}
					var record geoRecord
					if err = json.Unmarshal([]byte(payload), &record); err != nil {
						return fmt.Errorf("decode geo index %q %w", name, err)
					}
					canonical, err := prepareGeoPoints(name, []GeoPoint{record.GeoPoint})
					if err != nil {
						return fmt.Errorf("decode geo index %q %w", name, err)
					}
					if len(canonical) != 1 || canonical[0] != record || record.Cell < interval.Min || record.Cell > interval.Max ||
						member != fmt.Sprintf("%013x:", record.Cell)+record.Id {
						return fmt.Errorf("geo index %q corrupt point coordinates or member", name)
					}
					if err = yield(record); err != nil {
						return err
					}
				}
				if len(page) < 1+2*redisGeoBatchSize {
					break
				}
				lower = "(" + last
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// DeleteGeoPoints removes IDs without implicitly creating an index.
func (r *Redis) DeleteGeoPoints(ctx context.Context, name string, ids ...string) error {
	ids, err := prepareGeoIDs(name, ids)
	if err != nil {
		return err
	}
	keys, identity := r.geoKeys(name)
	if len(ids) == 0 {
		return redisGeoError(name, redisGeoCheck.Run(ctx, r.client, keys, identity).Err())
	}
	for start := 0; start < len(ids); start += redisGeoBatchSize {
		end := min(start+redisGeoBatchSize, len(ids))
		args := make([]any, 1, 1+end-start)
		args[0] = identity
		for _, id := range ids[start:end] {
			args = append(args, id)
		}
		if err = redisGeoDeletePoints.Run(ctx, r.client, keys, args...).Err(); err != nil {
			return redisGeoError(name, err)
		}
	}
	return nil
}

// DeleteGeoIndex atomically drops all three keys; absence is successful.
func (r *Redis) DeleteGeoIndex(ctx context.Context, name string) error {
	if err := validateGeoName(name); err != nil {
		return err
	}
	keys, identity := r.geoKeys(name)
	return redisGeoError(name, redisGeoDrop.Run(ctx, r.client, keys, identity).Err())
}

// Geographic keys use an encoded namespace, independent from the raw KV prefix.
// Purge visits this namespace on each master and deletes one key per command so
// the pipeline remains valid across different indexes' Cluster slots.
func (r *Redis) purgeGeoIndexes(ctx context.Context, client redis.UniversalClient) error {
	var cursor uint64
	for {
		keys, next, err := client.Scan(ctx, cursor, r.geoKeyPrefix()+"*", redisGeoBatchSize).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			pipeline := client.Pipeline()
			for _, key := range keys {
				pipeline.Del(ctx, key)
			}
			if _, err = pipeline.Exec(ctx); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}
