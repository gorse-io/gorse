// Copyright 2026 gorse Project Authors
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

package vectors

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"math"
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

const (
	redisVectorField     = "vector"
	redisIdField         = "id"
	redisCategoriesField = "categories"
	redisTimestampField  = "timestamp"
	redisDistanceField   = "distance"
	redisVectorDataType  = "FLOAT32"
	redisTagSeparator    = ","
	redisIndexPrefix     = "vectors_"
)

func init() {
	Register([]string{storage.RedisPrefix, storage.RedissPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		opt, err := redis.ParseURL(path)
		if err != nil {
			return nil, errors.Trace(err)
		}
		opt.Protocol = 2
		option := storage.NewOptions(opts...)
		if option.RedisClientName != "" {
			opt.ClientName = option.RedisClientName
		}
		database := &Redis{
			client:           redis.NewClient(opt),
			TablePrefix:      storage.TablePrefix(tablePrefix),
			maxSearchResults: option.MaxSearchResults,
		}
		if err = redisotel.InstrumentTracing(database.client, redisotel.WithAttributes(semconv.DBSystemRedis)); err != nil {
			log.Logger().Error("failed to add tracing for redis", zap.Error(err))
			return nil, errors.Trace(err)
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
			return nil, errors.Trace(err)
		}
		opt.Protocol = 2
		option := storage.NewOptions(opts...)
		if option.RedisClientName != "" {
			opt.ClientName = option.RedisClientName
		}
		database := &Redis{
			client:           redis.NewClusterClient(opt),
			TablePrefix:      storage.TablePrefix(tablePrefix),
			maxSearchResults: option.MaxSearchResults,
		}
		if err = redisotel.InstrumentTracing(database.client, redisotel.WithAttributes(semconv.DBSystemRedis)); err != nil {
			log.Logger().Error("failed to add tracing for redis", zap.Error(err))
			return nil, errors.Trace(err)
		}
		return database, nil
	})
}

type Redis struct {
	storage.TablePrefix
	client           redis.UniversalClient
	maxSearchResults int
}

func (db *Redis) Init() error {
	return errors.Trace(db.client.FT_List(context.Background()).Err())
}

func (db *Redis) Optimize(_ context.Context, _ string) error {
	return nil
}

func (db *Redis) Close() error {
	return db.client.Close()
}

func (db *Redis) ListCollections(ctx context.Context) ([]string, error) {
	indices, err := db.client.FT_List(ctx).Result()
	if err != nil {
		return nil, errors.Trace(err)
	}
	prefix := db.collectionIndex("")
	collections := make([]string, 0)
	for _, index := range indices {
		if strings.HasPrefix(index, prefix) {
			collections = append(collections, strings.TrimPrefix(index, prefix))
		}
	}
	return collections, nil
}

func (db *Redis) DescribeCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	index := db.collectionIndex(name)
	info, err := db.client.FTInfo(ctx, index).Result()
	if err != nil {
		if isRedisIndexNotFound(err) {
			return nil, errors.NotFoundf("collection %s", name)
		}
		return nil, errors.Trace(err)
	}
	for _, attr := range info.Attributes {
		if attr.Identifier == redisVectorField || attr.Attribute == redisVectorField {
			distance, err := redisDistanceToDistance(attr.DistanceMetric)
			if err != nil {
				return nil, errors.Trace(err)
			}
			return &CollectionInfo{
				Name:      name,
				Dimension: attr.Dim,
				Distance:  distance,
				VectorConfig: VectorConfig{
					Type: QuantizationNone,
				},
			}, nil
		}
	}
	return nil, errors.NotFoundf("vector field in collection %s", name)
}

func (db *Redis) AddCollection(ctx context.Context, name string, dimensions int, distance Distance, config VectorConfig) error {
	if config.Type != QuantizationNone {
		return errors.NotSupportedf("quantization type %s for Redis", config.Type)
	}
	metric, err := distanceToRedisDistance(distance)
	if err != nil {
		return errors.Trace(err)
	}
	_, err = db.client.FTCreate(ctx, db.collectionIndex(name),
		&redis.FTCreateOptions{
			OnHash: true,
			Prefix: []any{db.collectionKeyPrefix(name)},
		},
		&redis.FieldSchema{FieldName: redisIdField, FieldType: redis.SearchFieldTypeTag},
		&redis.FieldSchema{FieldName: redisCategoriesField, FieldType: redis.SearchFieldTypeTag, Separator: redisTagSeparator},
		&redis.FieldSchema{FieldName: redisTimestampField, FieldType: redis.SearchFieldTypeNumeric},
		&redis.FieldSchema{
			FieldName: redisVectorField,
			FieldType: redis.SearchFieldTypeVector,
			VectorArgs: &redis.FTVectorArgs{HNSWOptions: &redis.FTHNSWOptions{
				Type:                   redisVectorDataType,
				Dim:                    dimensions,
				DistanceMetric:         metric,
				MaxEdgesPerNode:        16,
				MaxAllowedEdgesPerNode: 200,
			}},
		},
	).Result()
	return errors.Trace(err)
}

func (db *Redis) DeleteCollection(ctx context.Context, name string) error {
	_, err := db.client.FTDropIndexWithArgs(ctx, db.collectionIndex(name), &redis.FTDropIndexOptions{DeleteDocs: true}).Result()
	if err != nil {
		if isRedisIndexNotFound(err) {
			return errors.NotFoundf("collection %s", name)
		}
		return errors.Trace(err)
	}
	return nil
}

func (db *Redis) AddVectors(ctx context.Context, collection string, vectors []Vector) error {
	if len(vectors) == 0 {
		return nil
	}
	p := db.client.Pipeline()
	for _, vector := range vectors {
		if err := p.HSet(ctx, db.vectorKey(collection, vector.Id), map[string]any{
			redisIdField:         vector.Id,
			redisCategoriesField: encodeRedisCategories(vector.Categories),
			redisTimestampField:  vector.Timestamp.UnixMilli(),
			redisVectorField:     redisVectorBlob(vector.Vector),
		}).Err(); err != nil {
			return errors.Trace(err)
		}
	}
	_, err := p.Exec(ctx)
	return errors.Trace(err)
}

func (db *Redis) DeleteVectors(ctx context.Context, collection string, timestamp time.Time) error {
	limit := db.maxSearchResults
	if limit <= 0 {
		limit = storage.NewOptions().MaxSearchResults
	}
	query := "@" + redisTimestampField + ":[-inf (" + strconv.FormatInt(timestamp.UnixMilli(), 10) + "]"
	for {
		result, err := db.client.FTSearchWithArgs(ctx, db.collectionIndex(collection), query, &redis.FTSearchOptions{
			NoContent:      true,
			LimitOffset:    0,
			Limit:          limit,
			DialectVersion: 2,
		}).Result()
		if err != nil {
			return errors.Trace(err)
		}
		if len(result.Docs) == 0 {
			return nil
		}
		keys := make([]string, 0, len(result.Docs))
		for _, doc := range result.Docs {
			keys = append(keys, doc.ID)
		}
		if err = db.client.Del(ctx, keys...).Err(); err != nil {
			return errors.Trace(err)
		}
		if len(result.Docs) < limit {
			return nil
		}
	}
}

func (db *Redis) QueryVectors(ctx context.Context, collection string, q []float32, categories []string, topK int) ([]Vector, error) {
	if topK <= 0 {
		return []Vector{}, nil
	}
	filter := "*"
	if len(categories) > 0 {
		encoded := make([]string, 0, len(categories))
		for _, category := range categories {
			encoded = append(encoded, encodeRedisCategory(category))
		}
		filter = "@" + redisCategoriesField + ":{" + strings.Join(encoded, "|") + "}"
	}
	query := filter + "=>[KNN " + strconv.Itoa(topK) + " @" + redisVectorField + " $query_vector AS " + redisDistanceField + "]"
	result, err := db.client.FTSearchWithArgs(ctx, db.collectionIndex(collection), query, &redis.FTSearchOptions{
		Return: []redis.FTSearchReturn{
			{FieldName: redisIdField},
			{FieldName: redisCategoriesField},
		},
		SortBy:         []redis.FTSearchSortBy{{FieldName: redisDistanceField, Asc: true}},
		LimitOffset:    0,
		Limit:          topK,
		Params:         map[string]any{"query_vector": redisVectorBlob(q)},
		DialectVersion: 2,
	}).Result()
	if err != nil {
		return nil, errors.Trace(err)
	}
	vectors := make([]Vector, 0, len(result.Docs))
	for _, doc := range result.Docs {
		vectors = append(vectors, Vector{
			Id:         doc.Fields[redisIdField],
			Categories: decodeRedisCategories(doc.Fields[redisCategoriesField]),
		})
	}
	return vectors, nil
}

func (db *Redis) collectionIndex(name string) string {
	return string(db.TablePrefix) + redisIndexPrefix + name
}

func (db *Redis) collectionKeyPrefix(collection string) string {
	return db.collectionIndex(collection) + ":"
}

func (db *Redis) vectorKey(collection, id string) string {
	sum := md5.Sum([]byte(id))
	return db.collectionKeyPrefix(collection) + hex.EncodeToString(sum[:])
}

func redisVectorBlob(vector []float32) []byte {
	buf := make([]byte, len(vector)*4)
	for i, v := range vector {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

func distanceToRedisDistance(distance Distance) (string, error) {
	switch distance {
	case Cosine:
		return "COSINE", nil
	case Euclidean:
		return "L2", nil
	case Dot:
		return "IP", nil
	default:
		return "", errors.NotSupportedf("distance method %v", distance)
	}
}

func redisDistanceToDistance(distance string) (Distance, error) {
	switch strings.ToUpper(distance) {
	case "", "COSINE":
		return Cosine, nil
	case "L2":
		return Euclidean, nil
	case "IP":
		return Dot, nil
	default:
		return Cosine, errors.NotSupportedf("distance method %s", distance)
	}
}

func encodeRedisCategories(categories []string) string {
	encoded := make([]string, 0, len(categories))
	for _, category := range categories {
		encoded = append(encoded, encodeRedisCategory(category))
	}
	return strings.Join(encoded, redisTagSeparator)
}

func encodeRedisCategory(category string) string {
	return hex.EncodeToString([]byte(category))
}

func decodeRedisCategories(encoded string) []string {
	if encoded == "" {
		return []string{}
	}
	parts := strings.Split(encoded, redisTagSeparator)
	categories := make([]string, 0, len(parts))
	for _, part := range parts {
		decoded, err := hex.DecodeString(part)
		if err != nil {
			categories = append(categories, part)
			continue
		}
		categories = append(categories, string(decoded))
	}
	return categories
}

func isRedisIndexNotFound(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "Unknown index name") || strings.Contains(message, "no such index") || strings.Contains(message, "SEARCH_INDEX_NOT_FOUND")
}
