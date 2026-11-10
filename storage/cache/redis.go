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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
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
	geoMutex         sync.RWMutex
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
	r.geoMutex.Lock()
	defer r.geoMutex.Unlock()
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

// Redis GEO uses a Mercator latitude range. Polar points remain fully indexed
// by the shared 52-bit Morton cell (exactly representable as a Search NUMERIC).
const redisGeoLatitudeLimit = 85.05112878

func (r *Redis) geoKeyPrefix() string {
	return "gorse_geo_" + geoNameDigest(string(r.TablePrefix)) + ":"
}

// Encode untrusted names/IDs and put a nonempty hash tag before them. All point
// hashes belonging to an index share a Cluster slot, independent of raw braces.
func (r *Redis) geoIndex(name string) string {
	return r.geoKeyPrefix() + "{" + geoNameDigest(string(r.TablePrefix), name) + "}:" +
		base64.RawURLEncoding.EncodeToString([]byte(name))
}

func (r *Redis) geoPointKey(name, id string) string {
	return r.geoIndex(name) + ":" + base64.RawURLEncoding.EncodeToString([]byte(id))
}

func redisGeoError(name string, err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unknown index name") || strings.Contains(message, "no such index") ||
		strings.Contains(message, "search_index_not_found") {
		return fmt.Errorf("geo index %q %w", name, storage.ErrNotFound)
	}
	if strings.Contains(message, "index already exists") {
		return fmt.Errorf("geo index %q %w", name, storage.ErrAlreadyExists)
	}
	return fmt.Errorf("geo index %q %w", name, err)
}

// AddGeoIndex creates an empty Search index, whose existence does not depend on
// point count. Lifecycle changes are coordinated within this client; callers
// must coordinate drop/recreate against writes from other clients/processes.
func (r *Redis) AddGeoIndex(ctx context.Context, name string) error {
	if err := validateGeoName(name); err != nil {
		return err
	}
	r.geoMutex.Lock()
	defer r.geoMutex.Unlock()
	index := r.geoIndex(name)
	return redisGeoError(name, r.client.FTCreate(ctx, index,
		&redis.FTCreateOptions{OnHash: true, Prefix: []any{index + ":"}},
		&redis.FieldSchema{FieldName: "location", FieldType: redis.SearchFieldTypeGeo},
		&redis.FieldSchema{FieldName: "latitude", FieldType: redis.SearchFieldTypeNumeric},
		&redis.FieldSchema{FieldName: "cell", FieldType: redis.SearchFieldTypeNumeric},
	).Err())
}

// ListGeoIndex discovers only this namespace's indexes, including empty ones.
func (r *Redis) ListGeoIndex(ctx context.Context) ([]string, error) {
	indexes, err := r.client.FT_List(ctx).Result()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	names := make([]string, 0)
	for _, index := range indexes {
		if !strings.HasPrefix(index, r.geoKeyPrefix()) {
			continue
		}
		_, encoded, ok := strings.Cut(strings.TrimPrefix(index, r.geoKeyPrefix()), "}:")
		if !ok {
			continue
		}
		decoded, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil || validateGeoName(string(decoded)) != nil || r.geoIndex(string(decoded)) != index {
			continue
		}
		names = append(names, string(decoded))
	}
	sort.Strings(names)
	return names, nil
}

func (r *Redis) checkGeoIndex(ctx context.Context, name string) error {
	return redisGeoError(name, r.client.Do(ctx, "FT.INFO", r.geoIndex(name)).Err())
}

// AddGeoPoints validates the complete batch before writing. Each transaction
// updates the exact payload and indexed fields together, including removing an
// obsolete GEO field when a point moves outside Redis's supported latitudes.
func (r *Redis) AddGeoPoints(ctx context.Context, name string, points []GeoPoint) error {
	records, err := prepareGeoPoints(name, points)
	if err != nil {
		return err
	}
	r.geoMutex.RLock()
	defer r.geoMutex.RUnlock()
	if err = r.checkGeoIndex(ctx, name); err != nil {
		return err
	}
	for start := 0; start < len(records); start += redisGeoBatchSize {
		pipeline := r.client.TxPipeline()
		for _, record := range records[start:min(start+redisGeoBatchSize, len(records))] {
			payload, err := json.Marshal(record)
			if err != nil {
				return errors.WithStack(err)
			}
			key := r.geoPointKey(name, record.Id)
			fields := []any{"payload", string(payload), "cell", record.Cell, "latitude", record.Latitude}
			if math.Abs(record.Latitude) <= redisGeoLatitudeLimit {
				fields = append(fields, "location", fmt.Sprintf("%.17g,%.17g", record.Longitude, record.Latitude))
			} else {
				pipeline.HDel(ctx, key, "location")
			}
			pipeline.HSet(ctx, key, fields...)
		}
		if _, err = pipeline.Exec(ctx); err != nil {
			return redisGeoError(name, err)
		}
	}
	return nil
}

// SearchGeoPoints uses Search NUMERIC cell ranges for complete spherical
// coverage, including poles and unlimited nearest-neighbor queries. For bounded
// queries inside Redis's latitude range, GEO additionally prunes normal points;
// the OR branch retains polar points. The generous outward radius guard covers
// Redis's different Earth radius and sub-meter GEO quantization. Only the shared
// exact distance/ID ordering decides inclusion and top N, never native distance.
func (r *Redis) SearchGeoPoints(ctx context.Context, name string, query GeoQuery) ([]GeoResult, error) {
	query, err := normalizeGeoQuery(name, query)
	if err != nil {
		return nil, err
	}
	r.geoMutex.RLock()
	defer r.geoMutex.RUnlock()
	if err = r.checkGeoIndex(ctx, name); err != nil {
		return nil, err
	}
	return searchGeoPoints(ctx, query, func(ctx context.Context, ranges []geoRange, yield func(geoRecord) error) error {
		var builder strings.Builder
		builder.WriteByte('(')
		for i, interval := range ranges {
			if i > 0 {
				builder.WriteString(" | ")
			}
			fmt.Fprintf(&builder, "@cell:[%d %d]", interval.Min, interval.Max)
		}
		builder.WriteByte(')')
		if query.MaxDistance > 0 && query.MaxDistance < math.Pi*geoEarthRadius && math.Abs(query.Latitude) <= redisGeoLatitudeLimit {
			fmt.Fprintf(&builder, " (@location:[%.17g %.17g %.17g m] | @latitude:[-90 (-%.17g] | @latitude:[(%.17g 90])",
				query.Longitude, query.Latitude, query.MaxDistance*1.01+1, redisGeoLatitudeLimit, redisGeoLatitudeLimit)
		}
		return r.scanGeoPoints(ctx, name, builder.String(), yield)
	})
}

// Cursors avoid offset/result ceilings and exhaust every candidate before the
// shared bounded heap selects top N. LOAD returns the original float64 JSON,
// not Redis GEO's quantized coordinates. No concurrent-write snapshot is promised.
func (r *Redis) scanGeoPoints(ctx context.Context, name, query string, yield func(geoRecord) error) error {
	index := r.geoIndex(name)
	var cursor int64
	defer func() {
		if cursor != 0 {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			r.client.Do(cleanup, "FT.CURSOR", "DEL", index, cursor)
		}
	}()
	args := []any{"FT.AGGREGATE", index, query, "LOAD", 1, "@payload", "TIMEOUT", 0,
		"WITHCURSOR", "COUNT", redisGeoBatchSize, "DIALECT", 2}
	for {
		page, err := r.client.Do(ctx, args...).Slice()
		if err != nil {
			return redisGeoError(name, err)
		}
		if len(page) != 2 {
			return fmt.Errorf("geo index %q invalid cursor response", name)
		}
		rows, rowsOK := page[0].([]any)
		var cursorOK bool
		cursor, cursorOK = page[1].(int64)
		if !rowsOK || !cursorOK || len(rows) == 0 {
			return fmt.Errorf("geo index %q invalid cursor page", name)
		}
		for _, row := range rows[1:] {
			fields, ok := row.([]any)
			if !ok || len(fields) != 2 || fields[0] != "payload" {
				return fmt.Errorf("geo index %q invalid point response", name)
			}
			payload, ok := fields[1].(string)
			if !ok {
				return fmt.Errorf("geo index %q missing point payload", name)
			}
			var record geoRecord
			if err = json.Unmarshal([]byte(payload), &record); err != nil {
				return errors.WithStack(err)
			}
			if err = yield(record); err != nil {
				return err
			}
		}
		if cursor == 0 {
			return nil
		}
		args = []any{"FT.CURSOR", "READ", index, cursor, "COUNT", redisGeoBatchSize}
	}
}

func (r *Redis) DeleteGeoPoints(ctx context.Context, name string, ids ...string) error {
	ids, err := prepareGeoIDs(name, ids)
	if err != nil {
		return err
	}
	r.geoMutex.RLock()
	defer r.geoMutex.RUnlock()
	if err = r.checkGeoIndex(ctx, name); err != nil {
		return err
	}
	for start := 0; start < len(ids); start += redisGeoBatchSize {
		pipeline := r.client.Pipeline()
		for _, id := range ids[start:min(start+redisGeoBatchSize, len(ids))] {
			pipeline.Del(ctx, r.geoPointKey(name, id))
		}
		if _, err = pipeline.Exec(ctx); err != nil {
			return redisGeoError(name, err)
		}
	}
	return nil
}

// DeleteGeoIndex drops the Search index and its per-point hashes; absence is OK.
func (r *Redis) DeleteGeoIndex(ctx context.Context, name string) error {
	if err := validateGeoName(name); err != nil {
		return err
	}
	r.geoMutex.Lock()
	defer r.geoMutex.Unlock()
	err := redisGeoError(name, r.client.FTDropIndexWithArgs(ctx, r.geoIndex(name), &redis.FTDropIndexOptions{DeleteDocs: true}).Err())
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	return err
}

func (r *Redis) purgeGeoIndexes(ctx context.Context, client redis.UniversalClient) error {
	indexes, err := client.FT_List(ctx).Result()
	if err != nil {
		return err
	}
	for _, index := range indexes {
		if strings.HasPrefix(index, r.geoKeyPrefix()) {
			err = redisGeoError(index, client.FTDropIndexWithArgs(ctx, index, &redis.FTDropIndexOptions{DeleteDocs: true}).Err())
			if err != nil && !errors.Is(err, storage.ErrNotFound) {
				return err
			}
		}
	}
	return nil
}
