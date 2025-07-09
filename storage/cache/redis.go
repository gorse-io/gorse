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
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/redis/rueidis"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/common/util"
	"github.com/zhenghaoz/gorse/storage"
)

// Redis cache storage.
type Redis struct {
	storage.TablePrefix
	client rueidis.Client
}

// Close redis connection.
func (r *Redis) Close() error {
	r.client.Close()
	return nil
}

func (r *Redis) Ping() error {
	return r.client.Do(context.Background(), r.client.B().Ping().Build()).Error()
}

// Init nothing.
func (r *Redis) Init() error {
	// list indices
	indices, err := r.client.Do(context.Background(), r.client.B().FtList().Build()).AsStrSlice()
	if err != nil && !rueidis.IsRedisNil(err) {
		return errors.Trace(err)
	}
	// create index
	if !lo.Contains(indices, r.DocumentTable()) {
		cmd := r.client.B().FtCreate().Index(r.DocumentTable()).
			OnHash().
			Prefix(1).Prefix(r.DocumentTable() + ":").
			Schema().
			FieldName("collection").Tag().
			FieldName("subset").Tag().
			FieldName("id").Tag().
			FieldName("score").Numeric().Sortable().
			FieldName("is_hidden").Numeric().
			FieldName("categories").Tag().Separator(";").
			FieldName("timestamp").Numeric().Sortable().
			Build()
		if err := r.client.Do(context.Background(), cmd).Error(); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (r *Redis) Scan(work func(string) error) error {
	return r.scan(context.Background(), r.client, string(r.TablePrefix)+"*", work)
}

func (r *Redis) scan(ctx context.Context, client rueidis.Client, pattern string, work func(string) error) error {
	var cursor uint64
	for {
		cmd := client.B().Scan().Cursor(cursor).Match(pattern).Count(100).Build()
		resp := client.Do(ctx, cmd)
		if err := resp.Error(); err != nil {
			return errors.Trace(err)
		}
		e, err := resp.AsScanEntry()
		if err != nil {
			return errors.Trace(err)
		}
		for _, key := range e.Elements {
			if err = work(key[len(r.TablePrefix):]); err != nil {
				return errors.Trace(err)
			}
		}
		cursor = e.Cursor
		if cursor == 0 {
			return nil
		}
	}
}

func (r *Redis) Purge() error {
	ctx := context.Background()
	return r.scan(ctx, r.client, string(r.TablePrefix)+"*", func(key string) error {
		cmd := r.client.B().Del().Key(string(r.TablePrefix) + key).Build()
		return r.client.Do(ctx, cmd).Error()
	})
}

func (r *Redis) Set(ctx context.Context, values ...Value) error {
	cmds := make(rueidis.Commands, 0, len(values))
	for _, v := range values {
		cmds = append(cmds, r.client.B().Set().Key(r.Key(v.name)).Value(v.value).Build())
	}
	for _, resp := range r.client.DoMulti(ctx, cmds...) {
		if err := resp.Error(); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// Get returns a value from Redis.
func (r *Redis) Get(ctx context.Context, key string) *ReturnValue {
	cmd := r.client.B().Get().Key(r.Key(key)).Cache()
	res := r.client.DoCache(ctx, cmd, time.Minute)
	if res.IsCacheHit() && rueidis.IsRedisNil(res.Error()) {
		// Cache miss, try fetching from redis directly
		res = r.client.Do(ctx, r.client.B().Get().Key(r.Key(key)).Build())
	}
	val, err := res.ToString()
	if err != nil {
		if rueidis.IsRedisNil(err) {
			return &ReturnValue{err: errors.Annotate(ErrObjectNotExist, key)}
		}
		return &ReturnValue{err: err}
	}
	return &ReturnValue{value: val}
}

// Delete object from Redis.
func (r *Redis) Delete(ctx context.Context, key string) error {
	cmd := r.client.B().Del().Key(r.Key(key)).Build()
	return r.client.Do(ctx, cmd).Error()
}

// GetSet returns members of a set from Redis.
func (r *Redis) GetSet(ctx context.Context, key string) ([]string, error) {
	cmd := r.client.B().Smembers().Key(r.Key(key)).Cache()
	return r.client.DoCache(ctx, cmd, time.Minute).AsStrSlice()
}

// SetSet overrides a set with members in Redis.
func (r *Redis) SetSet(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	cmds := make(rueidis.Commands, 0, 2)
	cmds = append(cmds, r.client.B().Del().Key(r.Key(key)).Build())
	cmds = append(cmds, r.client.B().Sadd().Key(r.Key(key)).Member(members...).Build())
	for _, resp := range r.client.DoMulti(ctx, cmds...) {
		if err := resp.Error(); err != nil {
			return err
		}
	}
	return nil
}

// AddSet adds members to a set in Redis.
func (r *Redis) AddSet(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	cmd := r.client.B().Sadd().Key(r.Key(key)).Member(members...).Build()
	return r.client.Do(ctx, cmd).Error()
}

// RemSet removes members from a set in Redis.
func (r *Redis) RemSet(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	cmd := r.client.B().Srem().Key(r.Key(key)).Member(members...).Build()
	return r.client.Do(ctx, cmd).Error()
}

func (r *Redis) Push(ctx context.Context, name string, message string) error {
	cmd := r.client.B().Zadd().Key(r.Key(name)).
		ScoreMember().ScoreMember(float64(time.Now().UnixNano()), message).Build()
	return r.client.Do(ctx, cmd).Error()
}

func (r *Redis) Pop(ctx context.Context, name string) (string, error) {
	cmd := r.client.B().Zpopmin().Key(r.Key(name)).Count(1).Build()
	z, err := r.client.Do(ctx, cmd).AsZScores()
	if err != nil {
		return "", errors.Trace(err)
	}
	if len(z) == 0 {
		return "", io.EOF
	}
	return z[0].Member, nil
}

func (r *Redis) Remain(ctx context.Context, name string) (int64, error) {
	cmd := r.client.B().Zcard().Key(r.Key(name)).Build()
	return r.client.Do(ctx, cmd).AsInt64()
}

func (r *Redis) documentKey(collection, subset, value string) string {
	return r.DocumentTable() + ":" + collection + ":" + subset + ":" + value
}

func (r *Redis) AddScores(ctx context.Context, collection, subset string, documents []Score) error {
	cmds := make(rueidis.Commands, 0, len(documents))
	for _, document := range documents {
		key := r.documentKey(collection, subset, document.Id)
		fields := map[string]string{
			"collection": collection,
			"subset":     subset,
			"id":         document.Id,
			"score":      strconv.FormatFloat(document.Score, 'f', -1, 64),
			"categories": encodeCategories(document.Categories),
			"timestamp":  strconv.FormatInt(document.Timestamp.UnixMicro(), 10),
		}
		if document.IsHidden {
			fields["is_hidden"] = "1"
		} else {
			fields["is_hidden"] = "0"
		}
		cmd := r.client.B().Hset().Key(key).FieldValue().FieldValueIter(maps.All(fields)).Build()
		cmds = append(cmds, cmd)
	}
	for _, resp := range r.client.DoMulti(ctx, cmds...) {
		if err := resp.Error(); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (r *Redis) SearchScores(ctx context.Context, collection, subset string, query []string, begin, end int) ([]Score, error) {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("@collection:{ %s } @is_hidden:[0 0]", escape(collection)))
	if subset != "" {
		builder.WriteString(fmt.Sprintf(" @subset:{ %s }", escape(subset)))
	}
	for _, q := range query {
		builder.WriteString(fmt.Sprintf(" @categories:{ %s }", escape(encdodeCategory(q))))
	}
	limit := int64(end - begin)
	if limit < 0 {
		limit = 10000
	}

	cmd := r.client.B().FtSearch().
		Index(r.DocumentTable()).
		Query(builder.String()).
		Sortby("score").Desc().Limit().OffsetNum(int64(begin), limit).
		Build()
	_, docs, err := r.client.Do(ctx, cmd).AsFtSearch()
	if err != nil {
		return nil, errors.Trace(err)
	}
	documents := make([]Score, 0, len(docs))
	for _, doc := range docs {
		var document Score
		document.Id = doc.Doc["id"]
		score, err := strconv.ParseFloat(doc.Doc["score"], 64)
		if err != nil {
			return nil, errors.Trace(err)
		}
		document.Score = score
		isHidden, err := strconv.ParseInt(doc.Doc["is_hidden"], 10, 64)
		if err != nil {
			return nil, errors.Trace(err)
		}
		document.IsHidden = isHidden != 0
		categories, err := decodeCategories(doc.Doc["categories"])
		if err != nil {
			return nil, errors.Trace(err)
		}
		document.Categories = categories
		timestamp, err := strconv.ParseInt(doc.Doc["timestamp"], 10, 64)
		if err != nil {
			return nil, errors.Trace(err)
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
	builder.WriteString(fmt.Sprintf("@collection:{ %s }", escape(strings.Join(collections, " | "))))
	builder.WriteString(fmt.Sprintf(" @id:{ %s }", escape(id)))
	if subset != nil {
		builder.WriteString(fmt.Sprintf(" @subset:{ %s }", escape(*subset)))
	}
	offset := int64(0)
	limit := int64(10000)
	for {
		cmd := r.client.B().FtSearch().
			Index(r.DocumentTable()).
			Query(builder.String()).
			Sortby("score").Desc().Limit().OffsetNum(offset, limit).
			Build()
		_, result, err := r.client.Do(ctx, cmd).AsFtSearch()
		if err != nil {
			return errors.Trace(err)
		}
		// update documents
		for _, doc := range result {
			key := doc.Key
			values := make(map[string]string)
			if patch.Score != nil {
				values["score"] = strconv.FormatFloat(*patch.Score, 'f', -1, 64)
			}
			if patch.IsHidden != nil {
				if *patch.IsHidden {
					values["is_hidden"] = "1"
				} else {
					values["is_hidden"] = "0"
				}
			}
			if patch.Categories != nil {
				values["categories"] = encodeCategories(patch.Categories)
			}
			if len(values) > 0 {
				cmd := r.client.B().Hset().Key(key).FieldValue().FieldValueIter(maps.All(values)).Build()
				if err := r.client.Do(ctx, cmd).Error(); err != nil {
					return errors.Trace(err)
				}
			}
		}
		// break if no more documents
		if len(result) < int(limit) {
			break
		}
		offset += limit
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
	offset := int64(0)
	limit := int64(10000)
	for {
		cmd := r.client.B().FtSearch().
			Index(r.DocumentTable()).
			Query(builder.String()).
			Sortby("score").Desc().Limit().OffsetNum(offset, limit).
			Build()
		resp := r.client.Do(ctx, cmd)
		_, result, err := resp.AsFtSearch()
		if err != nil {
			return errors.Trace(err)
		}
		// delete documents
		if len(result) > 0 {
			keys := make([]string, len(result))
			for i, doc := range result {
				keys[i] = doc.Key
			}
			cmd := r.client.B().Del().Key(keys...).Build()
			if err := r.client.Do(ctx, cmd).Error(); err != nil {
				return err
			}
		}
		// break if no more documents
		if len(result) < int(limit) {
			break
		}
		offset += limit
	}
	return nil
}

func (r *Redis) ScanScores(ctx context.Context, callback func(collection string, id string, subset string, timestamp time.Time) error) error {
	return r.scan(ctx, r.client, r.DocumentTable()+"*", func(key string) error {
		cmd := r.client.B().Hgetall().Key(string(r.TablePrefix) + key).Build()
		row, err := r.client.Do(ctx, cmd).AsStrMap()
		if err != nil {
			return errors.Trace(err)
		}
		usec, err := util.ParseInt[int64](row["timestamp"])
		if err != nil {
			return errors.Trace(err)
		}
		if err = callback(row["collection"], row["id"], row["subset"], time.UnixMicro(usec).In(time.UTC)); err != nil {
			return errors.Trace(err)
		}
		return nil
	})
}

func (r *Redis) AddTimeSeriesPoints(ctx context.Context, points []TimeSeriesPoint) error {
	cmds := make(rueidis.Commands, 0, len(points))
	for _, point := range points {
		key := r.PointsTable() + ":" + point.Name
		ts := strconv.FormatInt(point.Timestamp.UnixMilli(), 10)
		cmd := r.client.B().TsAdd().Key(key).Timestamp(ts).Value(point.Value).
			OnDuplicateLast().Build()
		cmds = append(cmds, cmd)
	}
	for _, resp := range r.client.DoMulti(ctx, cmds...) {
		if err := resp.Error(); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (r *Redis) GetTimeSeriesPoints(ctx context.Context, name string, begin, end time.Time, duration time.Duration) ([]TimeSeriesPoint, error) {
	key := r.PointsTable() + ":" + name
	begin_ts := strconv.FormatInt(begin.UnixMilli(), 10)
	end_ts := strconv.FormatInt(end.UnixMilli(), 10)
	cmd := r.client.B().TsRange().Key(key).
		Fromtimestamp(begin_ts).
		Totimestamp(end_ts).
		AggregationLast().Bucketduration(int64(duration / time.Millisecond)).
		Build()
	result, err := r.client.Do(ctx, cmd).ToArray()
	if err != nil {
		return nil, errors.Trace(err)
	}
	points := make([]TimeSeriesPoint, 0, len(result))
	for _, doc := range result {
		var point TimeSeriesPoint
		msg, err := doc.ToArray()
		if err != nil {
			return nil, errors.Trace(err)
		}
		tstmp, err := msg[0].AsInt64()
		if err != nil {
			return nil, errors.Trace(err)
		}
		val, err := msg[1].AsFloat64()
		if err != nil {
			return nil, errors.Trace(err)
		}
		point.Name = name
		point.Value = val
		point.Timestamp = time.UnixMilli(tstmp).UTC()
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
