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

	"github.com/juju/errors"
	"github.com/redis/go-redis/v9"
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
	// list indices
	indices, err := r.client.FT_List(context.Background()).Result()
	if err != nil {
		return errors.Trace(err)
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
			&redis.FieldSchema{FieldName: "score", FieldType: redis.SearchFieldTypeNumeric, Sortable: true},
			&redis.FieldSchema{FieldName: "is_hidden", FieldType: redis.SearchFieldTypeNumeric},
			&redis.FieldSchema{FieldName: "categories", FieldType: redis.SearchFieldTypeTag, Separator: ";"},
			&redis.FieldSchema{FieldName: "timestamp", FieldType: redis.SearchFieldTypeNumeric, Sortable: true},
		).Result()
		if err != nil {
			return errors.Trace(err)
		}
	}
	if !lo.Contains(indices, r.PointsTable()) {
		_, err = r.client.FTCreate(context.TODO(), r.PointsTable(),
			&redis.FTCreateOptions{
				OnHash: true,
				Prefix: []any{r.PointsTable() + ":"},
			},
			&redis.FieldSchema{FieldName: "name", FieldType: redis.SearchFieldTypeTag},
			&redis.FieldSchema{FieldName: "timestamp", FieldType: redis.SearchFieldTypeNumeric, Sortable: true},
		).Result()
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
	return errors.Trace(err)
}

func (r *Redis) SearchScores(ctx context.Context, collection, subset string, query []string, begin, end int) ([]Score, error) {
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
		return nil, errors.Trace(err)
	}
	documents := make([]Score, 0, len(result.Docs))
	for _, doc := range result.Docs {
		var document Score
		document.Id = doc.Fields["id"]
		score, err := strconv.ParseFloat(doc.Fields["score"], 64)
		if err != nil {
			return nil, errors.Trace(err)
		}
		document.Score = score
		isHidden, err := strconv.ParseInt(doc.Fields["is_hidden"], 10, 64)
		if err != nil {
			return nil, errors.Trace(err)
		}
		document.IsHidden = isHidden != 0
		categories, err := decodeCategories(doc.Fields["categories"])
		if err != nil {
			return nil, errors.Trace(err)
		}
		document.Categories = categories
		timestamp, err := strconv.ParseInt(doc.Fields["timestamp"], 10, 64)
		if err != nil {
			return nil, errors.Trace(err)
		}
		document.Timestamp = time.UnixMicro(timestamp).In(time.UTC)
		documents = append(documents, document)
	}
	return documents, nil
}

func (r *Redis) UpdateScores(ctx context.Context, collections []string, id string, patch ScorePatch) error {
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
		result, err := r.client.FTSearchWithArgs(ctx, r.DocumentTable(), builder.String(), &redis.FTSearchOptions{
			SortBy:      []redis.FTSearchSortBy{{FieldName: "score", Desc: true}},
			LimitOffset: 0,
			Limit:       10000,
		}).Result()
		if err != nil {
			return errors.Trace(err)
		}
		// update documents
		for _, doc := range result.Docs {
			key := doc.ID
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
		if result.Total <= len(result.Docs) {
			break
		}
	}
	return nil
}

func (r *Redis) DeleteScores(ctx context.Context, collections []string, condition ScoreCondition) error {
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
		result, err := r.client.FTSearchWithArgs(ctx, r.DocumentTable(), builder.String(), &redis.FTSearchOptions{
			SortBy:      []redis.FTSearchSortBy{{FieldName: "score", Desc: true}},
			LimitOffset: 0,
			Limit:       10000,
		}).Result()
		if err != nil {
			return errors.Trace(err)
		}
		// delete documents
		p := r.client.Pipeline()
		for _, doc := range result.Docs {
			p.Del(ctx, doc.ID)
		}
		_, err = p.Exec(ctx)
		if err != nil {
			return errors.Trace(err)
		}
		// break if no more documents
		if result.Total == len(result.Docs) {
			break
		}
	}
	return nil
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
	result, err := r.client.FTSearchWithArgs(ctx, r.PointsTable(),
		fmt.Sprintf("@name:{ %s } @timestamp:[%d (%d]", escape(name), begin.UnixMicro(), end.UnixMicro()),
		&redis.FTSearchOptions{SortBy: []redis.FTSearchSortBy{{FieldName: "timestamp"}}}).Result()
	if err != nil {
		return nil, errors.Trace(err)
	}
	points := make([]TimeSeriesPoint, 0, len(result.Docs))
	for _, doc := range result.Docs {
		var point TimeSeriesPoint
		point.Name = doc.Fields["name"]
		point.Value, err = strconv.ParseFloat(doc.Fields["value"], 64)
		if err != nil {
			return nil, errors.Trace(err)
		}
		timestamp, err := strconv.ParseInt(doc.Fields["timestamp"], 10, 64)
		if err != nil {
			return nil, errors.Trace(err)
		}
		point.Timestamp = time.UnixMicro(timestamp).In(time.UTC)
		points = append(points, point)
	}
	return points, nil
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
