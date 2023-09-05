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
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/storage"
)

// Redis cache storage.
type Redis struct {
	storage.TablePrefix
	client redis.UniversalClient
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
	// list index
	result, err := r.client.Do(context.TODO(), "FT._LIST").Result()
	if err != nil {
		return errors.Trace(err)
	}
	indices := lo.Map(result.([]any), func(s any, _ int) string {
		return s.(string)
	})
	// create index
	if !lo.Contains(indices, r.DocumentTable()) {
		_, err = r.client.Do(context.TODO(), "FT.CREATE", r.DocumentTable(),
			"ON", "HASH", "PREFIX", "1", r.DocumentTable()+":", "SCHEMA",
			"collection", "TAG",
			"subset", "TAG",
			"id", "TAG",
			"score", "NUMERIC", "SORTABLE",
			"is_hidden", "NUMERIC",
			"categories", "TAG", "SEPARATOR", ";",
			"timestamp", "NUMERIC", "SORTABLE").
			Result()
		if err != nil {
			return errors.Trace(err)
		}
	}
	if !lo.Contains(indices, r.PointsTable()) {
		_, err = r.client.Do(context.TODO(), "FT.CREATE", r.PointsTable(),
			"ON", "HASH", "PREFIX", "1", r.PointsTable()+":", "SCHEMA",
			"name", "TAG",
			"timestamp", "NUMERIC", "SORTABLE").
			Result()
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (r *Redis) Scan(work func(string) error) error {
	ctx := context.Background()
	return r.scan(ctx, r.client, work)
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

func (r *Redis) Purge() error {
	ctx := context.Background()
	return r.purge(ctx, r.client)
}

func (r *Redis) purge(ctx context.Context, client redis.UniversalClient) error {
	var (
		result []string
		cursor uint64
		err    error
	)
	for {
		result, cursor, err = client.Scan(ctx, cursor, string(r.TablePrefix)+"*", 0).Result()
		if err != nil {
			return errors.Trace(err)
		}
		if len(result) > 0 {
			if err = client.Del(ctx, result...).Err(); err != nil {
				return errors.Trace(err)
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
			return errors.Trace(err)
		}
	}
	_, err := p.Exec(ctx)
	return errors.Trace(err)
}

// Get returns a value from Redis.
func (r *Redis) Get(ctx context.Context, key string) *ReturnValue {
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
func (r *Redis) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, r.Key(key)).Err()
}

// GetSet returns members of a set from Redis.
func (r *Redis) GetSet(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, r.Key(key)).Result()
}

// SetSet overrides a set with members in Redis.
func (r *Redis) SetSet(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	// convert strings to interfaces
	values := make([]interface{}, 0, len(members))
	for _, member := range members {
		values = append(values, member)
	}
	// push set
	pipeline := r.client.Pipeline()
	pipeline.Del(ctx, r.Key(key))
	pipeline.SAdd(ctx, r.Key(key), values...)
	_, err := pipeline.Exec(ctx)
	return err
}

// AddSet adds members to a set in Redis.
func (r *Redis) AddSet(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	// convert strings to interfaces
	values := make([]interface{}, 0, len(members))
	for _, member := range members {
		values = append(values, member)
	}
	// push set
	return r.client.SAdd(ctx, r.Key(key), values...).Err()
}

// RemSet removes members from a set in Redis.
func (r *Redis) RemSet(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	return r.client.SRem(ctx, r.Key(key), members).Err()
}

func (r *Redis) Push(ctx context.Context, name string, message string) error {
	_, err := r.client.ZAdd(ctx, r.Key(name), redis.Z{Member: message, Score: float64(time.Now().UnixNano())}).Result()
	return err
}

func (r *Redis) Pop(ctx context.Context, name string) (string, error) {
	z, err := r.client.ZPopMin(ctx, r.Key(name), 1).Result()
	if err != nil {
		return "", errors.Trace(err)
	}
	if len(z) == 0 {
		return "", io.EOF
	}
	return z[0].Member.(string), nil
}

func (r *Redis) Remain(ctx context.Context, name string) (int64, error) {
	return r.client.ZCard(ctx, r.Key(name)).Result()
}

func (r *Redis) documentKey(collection, subset, value string) string {
	return r.DocumentTable() + ":" + collection + ":" + subset + ":" + value
}

func (r *Redis) AddDocuments(ctx context.Context, collection, subset string, documents []Document) error {
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
	return errors.Trace(err)
}

func (r *Redis) SearchDocuments(ctx context.Context, collection, subset string, query []string, begin, end int) ([]Document, error) {
	if len(query) == 0 {
		return nil, nil
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("@collection:{ %s } @is_hidden:[0 0]", escape(collection)))
	if subset != "" {
		builder.WriteString(fmt.Sprintf(" @subset:{ %s }", escape(subset)))
	}
	for _, q := range query {
		builder.WriteString(fmt.Sprintf(" @categories:{ %s }", escape(encdodeCategory(q))))
	}
	args := []any{"FT.SEARCH", r.DocumentTable(), builder.String(), "SORTBY", "score", "DESC", "LIMIT", begin}
	if end == -1 {
		args = append(args, 10000)
	} else {
		args = append(args, end-begin)
	}
	result, err := r.client.Do(ctx, args...).Result()
	if err != nil {
		return nil, errors.Trace(err)
	}
	_, _, documents, err := parseSearchDocumentsResult(result)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return documents, nil
}

func (r *Redis) UpdateDocuments(ctx context.Context, collections []string, id string, patch DocumentPatch) error {
	if len(collections) == 0 {
		return nil
	}
	if patch.Score == nil && patch.IsHidden == nil && patch.Categories == nil {
		return nil
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("@collection:{ %s }", escape(strings.Join(collections, " | "))))
	builder.WriteString(fmt.Sprintf(" @id:{ %s }", escape(id)))
	for {
		// search documents
		result, err := r.client.Do(ctx, "FT.SEARCH", r.DocumentTable(), builder.String(), "SORTBY", "score", "DESC", "LIMIT", 0, 10000).Result()
		if err != nil {
			return errors.Trace(err)
		}
		count, keys, _, err := parseSearchDocumentsResult(result)
		if err != nil {
			return errors.Trace(err)
		}
		// update documents
		for _, key := range keys {
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
			if err = r.client.Watch(ctx, func(tx *redis.Tx) error {
				if exist, err := tx.Exists(ctx, key).Result(); err != nil {
					return err
				} else if exist == 0 {
					return nil
				}
				return tx.HSet(ctx, key, values...).Err()
			}, key); err != nil {
				return errors.Trace(err)
			}
		}
		// break if no more documents
		if count <= int64(len(keys)) {
			break
		}
	}
	return nil
}

func (r *Redis) DeleteDocuments(ctx context.Context, collections []string, condition DocumentCondition) error {
	if err := condition.Check(); err != nil {
		return errors.Trace(err)
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("@collection:{ %s }", escape(strings.Join(collections, " | "))))
	if condition.Subset != nil {
		builder.WriteString(fmt.Sprintf(" @subset:{ %s }", escape(*condition.Subset)))
	}
	if condition.Id != nil {
		builder.WriteString(fmt.Sprintf(" @id:{ %s }", escape(*condition.Id)))
	}
	if condition.Before != nil {
		builder.WriteString(fmt.Sprintf(" @timestamp:[-inf (%d]", condition.Before.UnixMicro()))
	}
	for {
		// search documents
		result, err := r.client.Do(ctx, "FT.SEARCH", r.DocumentTable(), builder.String(), "SORTBY", "score", "DESC", "LIMIT", 0, 10000).Result()
		if err != nil {
			return errors.Trace(err)
		}
		count, keys, _, err := parseSearchDocumentsResult(result)
		if err != nil {
			return errors.Trace(err)
		}
		// delete documents
		p := r.client.Pipeline()
		for _, key := range keys {
			p.Del(ctx, key)
		}
		_, err = p.Exec(ctx)
		if err != nil {
			return errors.Trace(err)
		}
		// break if no more documents
		if count == int64(len(keys)) {
			break
		}
	}
	return nil
}

func parseSearchDocumentsResult(result any) (count int64, keys []string, documents []Document, err error) {
	rows, ok := result.([]any)
	if !ok {
		return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", result)
	}
	count, ok = rows[0].(int64)
	if !ok {
		return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", rows[0])
	}
	for i := 1; i < len(rows); i += 2 {
		key, ok := rows[i].(string)
		if !ok {
			return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", rows[i])
		}
		keys = append(keys, key)
		row, ok := rows[i+1].([]any)
		if !ok {
			return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", rows[i+1])
		}
		fields := make(map[string]any)
		for j := 0; j < len(row); j += 2 {
			fields[row[j].(string)] = row[j+1]
		}
		var document Document
		document.Id, ok = fields["id"].(string)
		if !ok {
			return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", fields["id"])
		}
		score, ok := fields["score"].(string)
		if !ok {
			return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", fields["score"])
		}
		document.Score, err = strconv.ParseFloat(score, 64)
		if err != nil {
			return 0, nil, nil, errors.Trace(err)
		}
		categories, ok := fields["categories"].(string)
		if !ok {
			return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", fields["categories"])
		}
		document.Categories, err = decodeCategories(categories)
		if err != nil {
			return 0, nil, nil, errors.Trace(err)
		}
		timestamp, ok := fields["timestamp"].(string)
		if !ok {
			return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", fields["timestamp"])
		}
		timestampMicros, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			return 0, nil, nil, errors.Trace(err)
		}
		document.Timestamp = time.UnixMicro(timestampMicros).In(time.UTC)
		documents = append(documents, document)
	}
	return
}

func (r *Redis) pointKey(name string, timestamp time.Time) string {
	return fmt.Sprintf("%s:%s:%d", r.PointsTable(), name, timestamp.UnixMicro())
}

func (r *Redis) AddTimeSeriesPoints(ctx context.Context, points []TimeSeriesPoint) error {
	p := r.client.Pipeline()
	for _, point := range points {
		p.HSet(ctx, r.pointKey(point.Name, point.Timestamp),
			"name", point.Name,
			"value", point.Value,
			"timestamp", point.Timestamp.UnixMicro())
	}
	_, err := p.Exec(ctx)
	return errors.Trace(err)
}

func (r *Redis) GetTimeSeriesPoints(ctx context.Context, name string, begin, end time.Time) ([]TimeSeriesPoint, error) {
	result, err := r.client.Do(ctx, "FT.SEARCH", r.PointsTable(),
		fmt.Sprintf("@name:{ %s } @timestamp:[%d (%d]", escape(name), begin.UnixMicro(), end.UnixMicro()),
		"SORTBY", "timestamp").Result()
	if err != nil {
		return nil, errors.Trace(err)
	}
	_, _, points, err := parseGetTimeSeriesPointsResult(result)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Results in the JSON serializer producing an empty array instead of a
	// null value. More info at:
	// https://github.com/golang/go/wiki/CodeReviewComments#declaring-empty-slices
	if points == nil {
		return []TimeSeriesPoint{}, nil
	}

	return points, nil
}

func parseGetTimeSeriesPointsResult(result any) (count int64, keys []string, points []TimeSeriesPoint, err error) {
	rows, ok := result.([]any)
	if !ok {
		return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", result)
	}
	count, ok = rows[0].(int64)
	if !ok {
		return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", rows[0])
	}
	for i := 1; i < len(rows); i += 2 {
		key, ok := rows[i].(string)
		if !ok {
			return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", rows[i])
		}
		keys = append(keys, key)
		row, ok := rows[i+1].([]any)
		if !ok {
			return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", rows[i+1])
		}
		fields := make(map[string]any)
		for j := 0; j < len(row); j += 2 {
			fields[row[j].(string)] = row[j+1]
		}
		var point TimeSeriesPoint
		point.Name, ok = fields["name"].(string)
		if !ok {
			return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", fields["name"])
		}
		value, ok := fields["value"].(string)
		if !ok {
			return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", fields["value"])
		}
		point.Value, err = strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, nil, nil, errors.Trace(err)
		}
		timestamp, ok := fields["timestamp"].(string)
		if !ok {
			return 0, nil, nil, errors.Errorf("invalid FT.SEARCH result: %#v", fields["timestamp"])
		}
		timestampMicros, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			return 0, nil, nil, errors.Trace(err)
		}
		point.Timestamp = time.UnixMicro(timestampMicros).In(time.UTC)
		points = append(points, point)
	}
	return
}

func encdodeCategory(category string) string {
	return base64.RawStdEncoding.EncodeToString([]byte("_" + category))
}

func decodeCategory(s string) (string, error) {
	b, err := base64.RawStdEncoding.DecodeString(s)
	if err != nil {
		return "", errors.Trace(err)
	}
	return string(b[1:]), nil
}

func encodeCategories(categories []string) string {
	var builder strings.Builder
	for i, category := range categories {
		if i > 0 {
			builder.WriteByte(';')
		}
		builder.WriteString(encdodeCategory(category))
	}
	return builder.String()
}

func decodeCategories(s string) ([]string, error) {
	var categories []string
	for _, category := range strings.Split(s, ";") {
		category, err := decodeCategory(category)
		if err != nil {
			return nil, errors.Trace(err)
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
