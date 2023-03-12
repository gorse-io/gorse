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
	"github.com/go-redis/redis/v9"
	"github.com/juju/errors"
	"github.com/zhenghaoz/gorse/storage"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func ParseRedisClusterURL(redisURL string) (*redis.ClusterOptions, error) {
	options := &redis.ClusterOptions{}
	uri := redisURL

	var err error
	if strings.HasPrefix(redisURL, storage.RedisClusterPrefix) {
		uri = uri[len(storage.RedisClusterPrefix):]
	} else {
		return nil, fmt.Errorf("scheme must be \"redis+cluster\"")
	}

	if idx := strings.Index(uri, "@"); idx != -1 {
		userInfo := uri[:idx]
		uri = uri[idx+1:]

		username := userInfo
		var password string

		if idx := strings.Index(userInfo, ":"); idx != -1 {
			username = userInfo[:idx]
			password = userInfo[idx+1:]
		}

		// Validate and process the username.
		if strings.Contains(username, "/") {
			return nil, fmt.Errorf("unescaped slash in username")
		}
		options.Username, err = url.PathUnescape(username)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Errorf("invalid username"))
		}

		// Validate and process the password.
		if strings.Contains(password, ":") {
			return nil, fmt.Errorf("unescaped colon in password")
		}
		if strings.Contains(password, "/") {
			return nil, fmt.Errorf("unescaped slash in password")
		}
		options.Password, err = url.PathUnescape(password)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Errorf("invalid password"))
		}
	}

	// fetch the hosts field
	hosts := uri
	if idx := strings.IndexAny(uri, "/?@"); idx != -1 {
		if uri[idx] == '@' {
			return nil, fmt.Errorf("unescaped @ sign in user info")
		}
		hosts = uri[:idx]
	}

	options.Addrs = strings.Split(hosts, ",")
	uri = uri[len(hosts):]
	if len(uri) > 0 && uri[0] == '/' {
		uri = uri[1:]
	}

	// grab connection arguments from URI
	connectionArgsFromQueryString, err := extractQueryArgsFromURI(uri)
	if err != nil {
		return nil, err
	}
	for _, pair := range connectionArgsFromQueryString {
		err = addOption(options, pair)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

func extractQueryArgsFromURI(uri string) ([]string, error) {
	if len(uri) == 0 {
		return nil, nil
	}

	if uri[0] != '?' {
		return nil, errors.New("must have a ? separator between path and query")
	}

	uri = uri[1:]
	if len(uri) == 0 {
		return nil, nil
	}
	return strings.FieldsFunc(uri, func(r rune) bool { return r == ';' || r == '&' }), nil
}

type optionHandler struct {
	int      *int
	bool     *bool
	duration *time.Duration
}

func addOption(options *redis.ClusterOptions, pair string) error {
	kv := strings.SplitN(pair, "=", 2)
	if len(kv) != 2 || kv[0] == "" {
		return fmt.Errorf("invalid option")
	}

	key, err := url.QueryUnescape(kv[0])
	if err != nil {
		return errors.Wrap(err, errors.Errorf("invalid option key %q", kv[0]))
	}

	value, err := url.QueryUnescape(kv[1])
	if err != nil {
		return errors.Wrap(err, errors.Errorf("invalid option value %q", kv[1]))
	}

	handlers := map[string]optionHandler{
		"max_retries":        {int: &options.MaxRetries},
		"min_retry_backoff":  {duration: &options.MinRetryBackoff},
		"max_retry_backoff":  {duration: &options.MaxRetryBackoff},
		"dial_timeout":       {duration: &options.DialTimeout},
		"read_timeout":       {duration: &options.ReadTimeout},
		"write_timeout":      {duration: &options.WriteTimeout},
		"pool_fifo":          {bool: &options.PoolFIFO},
		"pool_size":          {int: &options.PoolSize},
		"pool_timeout":       {duration: &options.PoolTimeout},
		"min_idle_conns":     {int: &options.MinIdleConns},
		"max_idle_conns":     {int: &options.MaxIdleConns},
		"conn_max_idle_time": {duration: &options.ConnMaxIdleTime},
		"conn_max_lifetime":  {duration: &options.ConnMaxLifetime},
	}

	lowerKey := strings.ToLower(key)
	if handler, ok := handlers[lowerKey]; ok {
		if handler.int != nil {
			*handler.int, err = strconv.Atoi(value)
			if err != nil {
				return errors.Wrap(err, fmt.Errorf("invalid '%s' value: %q", key, value))
			}
		} else if handler.duration != nil {
			*handler.duration, err = time.ParseDuration(value)
			if err != nil {
				return errors.Wrap(err, fmt.Errorf("invalid '%s' value: %q", key, value))
			}
		} else if handler.bool != nil {
			*handler.bool, err = strconv.ParseBool(value)
			if err != nil {
				return errors.Wrap(err, fmt.Errorf("invalid '%s' value: %q", key, value))
			}
		} else {
			return fmt.Errorf("redis: unexpected option: %s", key)
		}
	} else {
		return fmt.Errorf("redis: unexpected option: %s", key)
	}

	return nil
}

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
	// create index
	_, err := r.client.Do(context.TODO(), "FT.CREATE", r.DocumentTable(),
		"ON", "HASH", "PREFIX", "1", r.DocumentTable()+":", "SCHEMA",
		"name", "TEXT", "NOSTEM",
		"value", "TEXT",
		"score", "NUMERIC", "SORTABLE",
		"categories", "TAG", "SEPARATOR", ";").
		Result()
	return errors.Trace(err)
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
	if clusterClient, isCluster := r.client.(*redis.ClusterClient); isCluster {
		return clusterClient.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			return r.purge(ctx, client, isCluster)
		})
	} else {
		return r.purge(ctx, r.client, isCluster)
	}
}

func (r *Redis) purge(ctx context.Context, client redis.UniversalClient, isCluster bool) error {
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
			if isCluster {
				p := client.Pipeline()
				for _, key := range result {
					if err = p.Del(ctx, key).Err(); err != nil {
						return errors.Trace(err)
					}
				}
				if _, err = p.Exec(ctx); err != nil {
					return errors.Trace(err)
				}
			} else {
				if err = client.Del(ctx, result...).Err(); err != nil {
					return errors.Trace(err)
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

func (r *Redis) AddDocuments(ctx context.Context, name string, documents ...Document) error {
	p := r.client.Pipeline()
	for _, document := range documents {
		p.Do(ctx, "HSET", r.DocumentTable()+":"+name+":"+document.Value,
			"name", name,
			"value", document.Value,
			"score", document.Score,
			"categories", strings.Join(document.Categories, ";"))
	}
	_, err := p.Exec(ctx)
	return errors.Trace(err)
}

func (r *Redis) SearchDocuments(ctx context.Context, name string, query []string, begin, end int) ([]Document, error) {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("@name:%s", "a"))
	for _, q := range query {
		builder.WriteString(fmt.Sprintf(" @categories:{ %s }", q))
	}
	fmt.Println(builder.String())
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
	rows := result.([]any)
	documents := make([]Document, 0)
	fmt.Println(rows)
	for i := 2; i < len(rows); i += 2 {
		row := rows[i]
		fields := make(map[string]any)
		for j := 0; j < len(row.([]any)); j += 2 {
			fields[row.([]any)[j].(string)] = row.([]any)[j+1]
		}
		var document Document
		document.Value = fields["value"].(string)
		document.Score, err = strconv.ParseFloat(fields["score"].(string), 64)
		if err != nil {
			return nil, errors.Trace(err)
		}
		document.Categories = strings.Split(fields["categories"].(string), ";")
		documents = append(documents, document)
	}
	return documents, nil
}
