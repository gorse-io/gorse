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
	"math"
	"os"
	"testing"
	"time"

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/storage"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var (
	redisDSN string
)

func init() {
	// get environment variables
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	redisDSN = env("REDIS_URI", "redis://127.0.0.1:6379/")
}

type RedisTestSuite struct {
	baseTestSuite
}

func (suite *RedisTestSuite) SetupSuite() {
	var err error
	suite.Database, err = Open(redisDSN, "gorse_")
	suite.NoError(err)
	// flush db -- handle both *Redis and *RedisValkey (auto-detected)
	var client redis.UniversalClient
	switch db := suite.Database.(type) {
	case *RedisValkey:
		client = db.client
	case *Redis:
		client = db.client
	default:
		suite.Fail("unexpected database type")
	}
	if clusterClient, ok := client.(*redis.ClusterClient); ok {
		err = clusterClient.ForEachMaster(suite.T().Context(), func(ctx context.Context, c *redis.Client) error {
			return c.FlushDB(ctx).Err()
		})
		suite.NoError(err)
	} else {
		err = client.FlushDB(suite.T().Context()).Err()
		suite.NoError(err)
	}
	// create schema
	err = suite.Database.Init()
	suite.NoError(err)
}

func (suite *RedisTestSuite) redisDB() *Redis {
	switch db := suite.Database.(type) {
	case *RedisValkey:
		return &db.Redis
	case *Redis:
		return db
	default:
		suite.Fail("unexpected database type")
		return nil
	}
}

func (suite *RedisTestSuite) TestEscapeCharacters() {
	ts := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx := suite.T().Context()
	for _, c := range []string{"-", ":", ".", "/"} {
		suite.Run(c, func() {
			collection := fmt.Sprintf("a%s1", c)
			subset := fmt.Sprintf("b%s2", c)
			id := fmt.Sprintf("c%s3", c)
			err := suite.AddScores(ctx, collection, subset, []Score{{
				Id:         id,
				Score:      math.MaxFloat64,
				Categories: []string{"a", "b"},
				Timestamp:  ts,
			}})
			suite.NoError(err)
			documents, err := suite.SearchScores(ctx, collection, subset, []string{"b"}, 0, -1)
			suite.NoError(err)
			suite.Equal([]Score{{Id: id, Score: math.MaxFloat64, Categories: []string{"a", "b"}, Timestamp: ts}}, documents)

			err = suite.UpdateScores(ctx, []string{collection}, nil, id, ScorePatch{Score: new(float64(1))})
			suite.NoError(err)
			documents, err = suite.SearchScores(ctx, collection, subset, []string{"b"}, 0, -1)
			suite.NoError(err)
			suite.Equal([]Score{{Id: id, Score: 1, Categories: []string{"a", "b"}, Timestamp: ts}}, documents)

			err = suite.DeleteScores(ctx, []string{collection}, ScoreCondition{
				Subset: new(subset),
				Id:     new(id),
			})
			suite.NoError(err)
			documents, err = suite.SearchScores(ctx, collection, subset, []string{"b"}, 0, -1)
			suite.NoError(err)
			suite.Empty(documents)
		})
	}
}

func (suite *RedisTestSuite) TestUpdateScoresWithPagination() {
	ctx := suite.T().Context()
	db := suite.redisDB()
	limit := db.maxSearchResults
	db.maxSearchResults = 2
	defer func() {
		db.maxSearchResults = limit
	}()

	for i := range 5 {
		subset := fmt.Sprintf("subset-%d", i)
		err := suite.AddScores(ctx, "collection-a", subset, []Score{{
			Id:         "shared-item",
			Score:      float64(i),
			Categories: []string{"old"},
			Timestamp:  time.Now().UTC(),
		}})
		suite.NoError(err)
	}

	err := suite.UpdateScores(ctx, []string{"collection-a"}, nil, "shared-item", ScorePatch{
		Categories: []string{"new"},
	})
	suite.NoError(err)

	for i := range 5 {
		subset := fmt.Sprintf("subset-%d", i)
		docs, err := suite.SearchScores(ctx, "collection-a", subset, []string{"new"}, 0, -1)
		suite.NoError(err)
		suite.Require().Len(docs, 1)
		suite.Equal("shared-item", docs[0].Id)
	}
}

func (suite *RedisTestSuite) TestUpdateScoresWithPaginationAndScorePatch() {
	ctx := suite.T().Context()
	db := suite.redisDB()
	limit := db.maxSearchResults
	db.maxSearchResults = 1
	defer func() {
		db.maxSearchResults = limit
	}()

	initialScores := []float64{3, 2, 1}
	for i, score := range initialScores {
		subset := fmt.Sprintf("score-subset-%d", i)
		err := suite.AddScores(ctx, "collection-b", subset, []Score{{
			Id:         "shared-item",
			Score:      score,
			Categories: []string{"score-old"},
			Timestamp:  time.Now().UTC(),
		}})
		suite.NoError(err)
	}

	targetScore := float64(0)
	err := suite.UpdateScores(ctx, []string{"collection-b"}, nil, "shared-item", ScorePatch{
		Score: &targetScore,
	})
	suite.NoError(err)

	for i := range initialScores {
		subset := fmt.Sprintf("score-subset-%d", i)
		docs, err := suite.SearchScores(ctx, "collection-b", subset, nil, 0, -1)
		suite.NoError(err)
		suite.Require().Len(docs, 1)
		suite.Equal(targetScore, docs[0].Score)
	}
}

func (suite *RedisTestSuite) TestUpdateScoresWithPaginationAndTiedScores() {
	ctx := suite.T().Context()
	db := suite.redisDB()
	limit := db.maxSearchResults
	db.maxSearchResults = 2
	defer func() {
		db.maxSearchResults = limit
	}()

	for i := range 5 {
		subset := fmt.Sprintf("tie-subset-%d", i)
		err := suite.AddScores(ctx, "collection-c", subset, []Score{{
			Id:         "shared-item",
			Score:      1,
			Categories: []string{"tie-old"},
			Timestamp:  time.Now().UTC(),
		}})
		suite.NoError(err)
	}

	err := suite.UpdateScores(ctx, []string{"collection-c"}, nil, "shared-item", ScorePatch{
		Categories: []string{"tie-new"},
	})
	suite.NoError(err)

	for i := range 5 {
		subset := fmt.Sprintf("tie-subset-%d", i)
		docs, err := suite.SearchScores(ctx, "collection-c", subset, []string{"tie-new"}, 0, -1)
		suite.NoError(err)
		suite.Require().Len(docs, 1)
		suite.Equal("shared-item", docs[0].Id)
	}
}

func TestRedis(t *testing.T) {
	suite.Run(t, new(RedisTestSuite))
}

// ValkeyViaRedisTestSuite tests auto-detection of Valkey when connected via a redis:// DSN.
// The redis:// URL points at a Valkey server; isValkey() should detect it via
// INFO server and return a *RedisValkey with sorted-set time series.
type ValkeyViaRedisTestSuite struct {
	baseTestSuite
}

func (suite *ValkeyViaRedisTestSuite) SetupSuite() {
	dsn := os.Getenv("VALKEY_URI")
	if dsn == "" {
		dsn = "redis://127.0.0.1:6380/"
	}
	var err error
	suite.Database, err = Open(dsn, "gorse_valkey_")
	suite.Require().NoError(err)
	redisDB, ok := suite.Database.(*RedisValkey)
	suite.Require().True(ok, "redis:// DSN pointing at Valkey should produce a *RedisValkey instance")
	err = redisDB.client.FlushDB(suite.T().Context()).Err()
	suite.Require().NoError(err)
	err = suite.Database.Init()
	suite.Require().NoError(err)
}

func (suite *ValkeyViaRedisTestSuite) TestEscapeCharacters() {
	ts := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx := suite.T().Context()
	for _, c := range []string{"-", ":", ".", "/"} {
		suite.Run(c, func() {
			collection := fmt.Sprintf("a%s1", c)
			subset := fmt.Sprintf("b%s2", c)
			id := fmt.Sprintf("c%s3", c)
			err := suite.AddScores(ctx, collection, subset, []Score{{
				Id:         id,
				Score:      math.MaxFloat64,
				Categories: []string{"a", "b"},
				Timestamp:  ts,
			}})
			suite.NoError(err)
			documents, err := suite.SearchScores(ctx, collection, subset, []string{"b"}, 0, -1)
			suite.NoError(err)
			suite.Equal([]Score{{Id: id, Score: math.MaxFloat64, Categories: []string{"a", "b"}, Timestamp: ts}}, documents)

			err = suite.UpdateScores(ctx, []string{collection}, nil, id, ScorePatch{Score: new(float64(1))})
			suite.NoError(err)
			documents, err = suite.SearchScores(ctx, collection, subset, []string{"b"}, 0, -1)
			suite.NoError(err)
			suite.Equal([]Score{{Id: id, Score: 1, Categories: []string{"a", "b"}, Timestamp: ts}}, documents)

			err = suite.DeleteScores(ctx, []string{collection}, ScoreCondition{
				Subset: new(subset),
				Id:     new(id),
			})
			suite.NoError(err)
			documents, err = suite.SearchScores(ctx, collection, subset, []string{"b"}, 0, -1)
			suite.NoError(err)
			suite.Empty(documents)
		})
	}
}

func TestValkeyViaRedis(t *testing.T) {
	suite.Run(t, new(ValkeyViaRedisTestSuite))
}

func (suite *ValkeyViaRedisTestSuite) TestUpdateScoresWithPagination() {
	ctx := suite.T().Context()
	db, ok := suite.Database.(*RedisValkey)
	suite.True(ok)
	limit := db.maxSearchResults
	db.maxSearchResults = 2
	defer func() {
		db.maxSearchResults = limit
	}()

	for i := range 5 {
		subset := fmt.Sprintf("subset-%d", i)
		err := suite.AddScores(ctx, "collection-a", subset, []Score{{
			Id:         "shared-item",
			Score:      float64(i),
			Categories: []string{"old"},
			Timestamp:  time.Now().UTC(),
		}})
		suite.NoError(err)
	}

	err := suite.UpdateScores(ctx, []string{"collection-a"}, nil, "shared-item", ScorePatch{
		Categories: []string{"new"},
	})
	suite.NoError(err)

	for i := range 5 {
		subset := fmt.Sprintf("subset-%d", i)
		docs, err := suite.SearchScores(ctx, "collection-a", subset, []string{"new"}, 0, -1)
		suite.NoError(err)
		suite.Require().Len(docs, 1)
		suite.Equal("shared-item", docs[0].Id)
	}
}

func (suite *ValkeyViaRedisTestSuite) TestUpdateScoresWithPaginationAndScorePatch() {
	ctx := suite.T().Context()
	db, ok := suite.Database.(*RedisValkey)
	suite.True(ok)
	limit := db.maxSearchResults
	db.maxSearchResults = 1
	defer func() {
		db.maxSearchResults = limit
	}()

	initialScores := []float64{3, 2, 1}
	for i, score := range initialScores {
		subset := fmt.Sprintf("score-subset-%d", i)
		err := suite.AddScores(ctx, "collection-b", subset, []Score{{
			Id:         "shared-item",
			Score:      score,
			Categories: []string{"score-old"},
			Timestamp:  time.Now().UTC(),
		}})
		suite.NoError(err)
	}

	targetScore := float64(0)
	err := suite.UpdateScores(ctx, []string{"collection-b"}, nil, "shared-item", ScorePatch{
		Score: &targetScore,
	})
	suite.NoError(err)

	for i := range initialScores {
		subset := fmt.Sprintf("score-subset-%d", i)
		docs, err := suite.SearchScores(ctx, "collection-b", subset, nil, 0, -1)
		suite.NoError(err)
		suite.Require().Len(docs, 1)
		suite.Equal(targetScore, docs[0].Score)
	}
}

func TestEncodeDecodeCategories(t *testing.T) {
	encoded := encodeCategories([]string{"z", "h"})
	decoded, err := decodeCategories(encoded)
	assert.NoError(t, err)
	assert.Equal(t, []string{"z", "h"}, decoded)

	encoded = encodeCategories(nil)
	decoded, err = decodeCategories(encoded)
	assert.NoError(t, err)
	assert.Equal(t, []string{}, decoded)
}

// RedisSortedSetTSTestSuite runs the full test suite using the RedisValkey type
// (sorted-set time series) against a real Redis server. This validates the
// fallback implementation works correctly using Redis's ZADD/HSET commands.
type RedisSortedSetTSTestSuite struct {
	baseTestSuite
}

func (suite *RedisSortedSetTSTestSuite) SetupSuite() {
	// Open as a regular Redis connection, then wrap in RedisValkey.
	opt, err := redis.ParseURL(redisDSN)
	suite.Require().NoError(err)
	opt.Protocol = 2
	database := &RedisValkey{}
	database.TablePrefix = storage.TablePrefix("gorse_ss_ts_")
	database.client = redis.NewClient(opt)
	database.maxSearchResults = 0
	suite.Database = database
	// flush db
	err = database.client.FlushDB(suite.T().Context()).Err()
	suite.Require().NoError(err)
	// create schema
	err = suite.Database.Init()
	suite.Require().NoError(err)
}

func TestRedisSortedSetTS(t *testing.T) {
	suite.Run(t, new(RedisSortedSetTSTestSuite))
}

// TestRedisSortedSetTS_NonExistentSeries validates that the sorted-set TS
// implementation returns empty results (not an error) for non-existent series.
func (suite *RedisSortedSetTSTestSuite) TestNonExistentSeries() {
	ctx := suite.T().Context()
	ts := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	points, err := suite.GetTimeSeriesPoints(ctx, "nonexistent",
		ts, ts.Add(time.Hour), time.Second)
	suite.NoError(err)
	suite.Empty(points)
}

func BenchmarkRedis(b *testing.B) {
	log.CloseLogger()
	// open db
	database, err := Open(redisDSN, "gorse_")
	assert.NoError(b, err)
	// flush db
	switch db := database.(type) {
	case *RedisValkey:
		err = db.client.FlushDB(b.Context()).Err()
	case *Redis:
		err = db.client.FlushDB(b.Context()).Err()
	}
	assert.NoError(b, err)
	// create schema
	err = database.Init()
	assert.NoError(b, err)
	// benchmark
	benchmark(b, database)
}
