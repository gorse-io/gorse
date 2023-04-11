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
			"value", "TAG",
			"score", "NUMERIC", "SORTABLE",
			"is_hidden", "NUMERIC",
			"categories", "TAG", "SEPARATOR", ";",
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

// GetSorted get scores from sorted set.
func (r *Redis) GetSorted(ctx context.Context, key string, begin, end int) ([]Scored, error) {
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

func (r *Redis) GetSortedByScore(ctx context.Context, key string, begin, end float64) ([]Scored, error) {
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

func (r *Redis) RemSortedByScore(ctx context.Context, key string, begin, end float64) error {
	return r.client.ZRemRangeByScore(ctx, r.Key(key),
		strconv.FormatFloat(begin, 'g', -1, 64),
		strconv.FormatFloat(end, 'g', -1, 64)).
		Err()
}

// AddSorted add scores to sorted set.
func (r *Redis) AddSorted(ctx context.Context, sortedSets ...SortedSet) error {
	p := r.client.Pipeline()
	for _, sorted := range sortedSets {
		if len(sorted.scores) > 0 {
			members := make([]redis.Z, 0, len(sorted.scores))
			for _, score := range sorted.scores {
				members = append(members, redis.Z{Member: score.Id, Score: score.Score})
			}
			p.ZAdd(ctx, r.Key(sorted.name), members...)
		}
	}
	_, err := p.Exec(ctx)
	return err
}

// SetSorted set scores in sorted set and clear previous scores.
func (r *Redis) SetSorted(ctx context.Context, key string, scores []Scored) error {
	members := make([]redis.Z, 0, len(scores))
	for _, score := range scores {
		members = append(members, redis.Z{Member: score.Id, Score: float64(score.Score)})
	}
	pipeline := r.client.Pipeline()
	pipeline.Del(ctx, r.Key(key))
	if len(scores) > 0 {
		pipeline.ZAdd(ctx, r.Key(key), members...)
	}
	_, err := pipeline.Exec(ctx)
	return err
}

// RemSorted method of NoDatabase returns ErrNoDatabase.
func (r *Redis) RemSorted(ctx context.Context, members ...SetMember) error {
	if len(members) == 0 {
		return nil
	}
	pipe := r.client.Pipeline()
	for _, member := range members {
		pipe.ZRem(ctx, r.Key(member.name), member.member)
	}
	_, err := pipe.Exec(ctx)
	return errors.Trace(err)
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

func (r *Redis) AddDocuments(ctx context.Context, collection, subset string, documents ...Document) error {
	p := r.client.Pipeline()
	for _, document := range documents {
		p.Do(ctx, "HSET", r.documentKey(collection, subset, document.Value),
			"collection", collection,
			"subset", subset,
			"value", document.Value,
			"score", document.Score,
			"is_hidden", document.IsHidden,
			"categories", strings.Join(document.Categories, ";"),
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
	builder.WriteString(fmt.Sprintf("@collection:{ %s } @is_hidden:[0 0]", collection))
	if subset != "" {
		builder.WriteString(fmt.Sprintf(" @subset:{ %s }", subset))
	}
	for _, q := range query {
		builder.WriteString(fmt.Sprintf(" @categories:{ %s }", q))
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
	_, _, documents, err := parseSearchResult(result)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return documents, nil
}

func (r *Redis) UpdateDocuments(ctx context.Context, collections []string, value string, patch DocumentPatch) error {
	if len(collections) == 0 {
		return nil
	}
	if patch.Score == nil && patch.IsHidden == nil && patch.Categories == nil {
		return nil
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("@collection:{ %s }", strings.Join(collections, " | ")))
	builder.WriteString(fmt.Sprintf(" @value:{ %s }", value))
	for {
		// search documents
		result, err := r.client.Do(ctx, "FT.SEARCH", r.DocumentTable(), builder.String(), "SORTBY", "score", "DESC", "LIMIT", 0, 10000).Result()
		if err != nil {
			return errors.Trace(err)
		}
		count, keys, _, err := parseSearchResult(result)
		if err != nil {
			return errors.Trace(err)
		}
		// update documents
		p := r.client.Pipeline()
		for _, key := range keys {
			if patch.Score != nil {
				p.Do(ctx, "HSET", key, "score", *patch.Score)
			}
			if patch.IsHidden != nil {
				p.Do(ctx, "HSET", key, "is_hidden", *patch.IsHidden)
			}
			if patch.Categories != nil {
				p.Do(ctx, "HSET", key, "categories", strings.Join(patch.Categories, ";"))
			}
		}
		_, err = p.Exec(ctx)
		if err != nil {
			return errors.Trace(err)
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
	builder.WriteString(fmt.Sprintf("@collection:{ %s }", strings.Join(collections, " | ")))
	if condition.Subset != nil {
		builder.WriteString(fmt.Sprintf(" @subset:{ %s }", *condition.Subset))
	}
	if condition.Value != nil {
		builder.WriteString(fmt.Sprintf(" @value:{ %s }", *condition.Value))
	}
	if condition.Before != nil {
		builder.WriteString(fmt.Sprintf(" @timestamp:[-inf,%d]", condition.Before.UnixMicro()))
	}
	for {
		// search documents
		result, err := r.client.Do(ctx, "FT.SEARCH", r.DocumentTable(), builder.String(), "SORTBY", "score", "DESC", "LIMIT", 0, 10000).Result()
		if err != nil {
			return errors.Trace(err)
		}
		count, keys, _, err := parseSearchResult(result)
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

func parseSearchResult(result any) (count int64, keys []string, documents []Document, err error) {
	rows, ok := result.([]any)
	if !ok {
		return 0, nil, nil, errors.New("invalid FT.SEARCH result")
	}
	count, ok = rows[0].(int64)
	if !ok {
		return 0, nil, nil, errors.New("invalid FT.SEARCH result")
	}
	for i := 1; i < len(rows); i += 2 {
		key, ok := rows[i].(string)
		if !ok {
			return 0, nil, nil, errors.New("invalid FT.SEARCH result")
		}
		keys = append(keys, key)
		row, ok := rows[i+1].([]any)
		if !ok {
			return 0, nil, nil, errors.New("invalid FT.SEARCH result")
		}
		fields := make(map[string]any)
		for j := 0; j < len(row); j += 2 {
			fields[row[j].(string)] = row[j+1]
		}
		var document Document
		document.Value, ok = fields["value"].(string)
		if !ok {
			return 0, nil, nil, errors.New("invalid FT.SEARCH result")
		}
		score, ok := fields["score"].(string)
		if !ok {
			return 0, nil, nil, errors.New("invalid FT.SEARCH result")
		}
		document.Score, err = strconv.ParseFloat(score, 64)
		if err != nil {
			return 0, nil, nil, errors.Trace(err)
		}
		categories, ok := fields["categories"].(string)
		if !ok {
			return 0, nil, nil, errors.New("invalid FT.SEARCH result")
		}
		document.Categories = strings.Split(categories, ";")
		timestamp, ok := fields["timestamp"].(string)
		if !ok {
			return 0, nil, nil, errors.New("invalid FT.SEARCH result")
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
