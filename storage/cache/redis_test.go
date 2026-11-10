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
	"math/rand/v2"
	"os"
	"sort"
	"strconv"
	"strings"
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
	// os.Setenv("REDIS_URI", "redis://127.0.0.1:6379/")
	redisDSN = os.Getenv("REDIS_URI")
}

type RedisTestSuite struct {
	baseTestSuite
}

func (suite *RedisTestSuite) SetupSuite() {
	log.SetTestLogger(suite.T())
	var err error
	suite.Database, err = Open(redisDSN, "gorse_")
	suite.NoError(err)
	// flush db
	redisClient, ok := suite.Database.(*Redis)
	suite.True(ok)
	if clusterClient, ok := redisClient.client.(*redis.ClusterClient); ok {
		err = clusterClient.ForEachMaster(suite.T().Context(), func(ctx context.Context, client *redis.Client) error {
			return client.FlushDB(ctx).Err()
		})
		suite.NoError(err)
	} else {
		err = redisClient.client.FlushDB(suite.T().Context()).Err()
		suite.NoError(err)
	}
	// create schema
	err = suite.Database.Init()
	suite.NoError(err)
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

// TestScanScoresMultiplePages covers the case TestScanScores cannot: a corpus larger
// than one SCAN page. Every document must come back exactly once, whatever the page
// size or the number of pages, and the ids must be the full set that was written.
func (suite *RedisTestSuite) TestScanScoresMultiplePages() {
	const numScores = 2500
	timestamp := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	written := make([]Score, 0, numScores)
	for i := range numScores {
		written = append(written, Score{
			Id:         strconv.Itoa(i),
			Score:      float64(i),
			Categories: []string{"a"},
			Timestamp:  timestamp,
		})
	}
	ctx := suite.T().Context()
	suite.NoError(suite.AddScores(ctx, "collection", "subset", written))

	seen := make(map[string]bool, numScores)
	scanned := 0
	err := suite.ScanScores(ctx, func(collection, id, subset string, t time.Time) error {
		scanned++
		seen[id] = true
		suite.Equal("collection", collection)
		suite.Equal("subset", subset)
		suite.Equal(timestamp, t.UTC())
		return nil
	})
	suite.NoError(err)
	// No duplicates: a document returned twice would make scanned exceed the set size.
	suite.Equal(numScores, scanned)
	suite.Equal(numScores, len(seen))
	for i := range numScores {
		suite.True(seen[strconv.Itoa(i)], "missing document %d", i)
	}
}

func (suite *RedisTestSuite) TestUpdateScoresWithPagination() {
	ctx := suite.T().Context()
	db, ok := suite.Database.(*Redis)
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

func (suite *RedisTestSuite) TestUpdateScoresWithPaginationAndScorePatch() {
	ctx := suite.T().Context()
	db, ok := suite.Database.(*Redis)
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

func (suite *RedisTestSuite) TestUpdateScoresWithPaginationAndTiedScores() {
	ctx := suite.T().Context()
	db, ok := suite.Database.(*Redis)
	suite.True(ok)
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

// The geo lifecycle is backed by Search indexes, including empty indexes.
func (suite *RedisTestSuite) TestGeoSearchIndex() {
	ctx := suite.T().Context()
	r := suite.Database.(*Redis)
	name := "nearby:{}:*?[]\x00東京"
	suite.Require().NoError(r.AddGeoIndex(ctx, name))
	defer func() { suite.NoError(r.DeleteGeoIndex(context.Background(), name)) }()
	suite.ErrorIs(r.AddGeoIndex(ctx, name), storage.ErrAlreadyExists)
	names, err := r.ListGeoIndex(ctx)
	suite.Require().NoError(err)
	suite.Equal([]string{name}, names)
	indexes, err := r.client.FT_List(ctx).Result()
	suite.Require().NoError(err)
	var index string
	for _, candidate := range indexes {
		if strings.HasPrefix(candidate, r.geoKeyPrefix()) {
			index = candidate
		}
	}
	suite.Require().NotEmpty(index, "geography must create a RediSearch index")
	// Normal coordinates must be searchable through a genuine GEO field.
	points := []GeoPoint{
		{Id: "a:{}:*?[]\x00東京", Longitude: 12.123456789012345, Latitude: 45.123456789012345},
		{Id: "north", Longitude: 18, Latitude: 90},
		{Id: "south", Longitude: -18, Latitude: -90},
		{Id: "edge", Longitude: 10, Latitude: 85.05112878},
		{Id: "polar", Longitude: 10, Latitude: 85.05112879},
	}
	suite.Require().NoError(r.AddGeoPoints(ctx, name, points))
	native, err := r.client.FTSearchWithArgs(ctx, index, "@location:[12.123456789012345 45.123456789012345 1 m]", &redis.FTSearchOptions{Limit: 10}).Result()
	suite.Require().NoError(err)
	suite.Equal(1, native.Total)
	for _, p := range points {
		results, err := r.SearchGeoPoints(ctx, name, GeoQuery{Longitude: p.Longitude, Latitude: p.Latitude, N: 1, MaxDistance: 0.001})
		suite.Require().NoError(err)
		suite.Require().Len(results, 1)
		canonical, err := prepareGeoPoints(name, []GeoPoint{p})
		suite.Require().NoError(err)
		suite.Equal(canonical[0].GeoPoint, results[0].GeoPoint)
		suite.Zero(results[0].Distance)
	}
	// Moving from GEO to polar and back must not leave a stale GEO value.
	p := points[0]
	p.Latitude = 89
	suite.Require().NoError(r.AddGeoPoints(ctx, name, []GeoPoint{p}))
	native, err = r.client.FTSearchWithArgs(ctx, index, "@location:[12.123456789012345 45.123456789012345 1 m]", &redis.FTSearchOptions{Limit: 10}).Result()
	suite.Require().NoError(err)
	suite.Zero(native.Total)
	suite.Require().NoError(r.AddGeoPoints(ctx, name, points[:1]))
	suite.Require().NoError(r.DeleteGeoPoints(ctx, name, points[0].Id))
	results, err := r.SearchGeoPoints(ctx, name, GeoQuery{Longitude: points[0].Longitude, Latitude: points[0].Latitude, N: 1, MaxDistance: 1})
	suite.NoError(err)
	suite.Empty(results)
	suite.Require().NoError(r.DeleteGeoIndex(ctx, name))
	indexes, err = r.client.FT_List(ctx).Result()
	suite.Require().NoError(err)
	suite.NotContains(indexes, index)
	keys, err := r.client.Keys(ctx, r.geoKeyPrefix()+"*").Result()
	suite.NoError(err)
	suite.Empty(keys)
	_, err = r.SearchGeoPoints(ctx, name, GeoQuery{N: 0})
	suite.ErrorIs(err, storage.ErrNotFound)
	suite.ErrorIs(r.AddGeoPoints(ctx, name, nil), storage.ErrNotFound)
	suite.ErrorIs(r.DeleteGeoPoints(ctx, name), storage.ErrNotFound)
}

func (suite *RedisTestSuite) TestGeoSearchPagesAndBoundaries() {
	ctx := suite.T().Context()
	r := suite.Database.(*Redis)
	name := "pages"
	suite.Require().NoError(r.AddGeoIndex(ctx, name))
	defer func() { suite.NoError(r.DeleteGeoIndex(context.Background(), name)) }()
	points := make([]GeoPoint, 1100)
	for i := range points {
		points[i] = GeoPoint{Id: fmt.Sprintf("%04d", i), Longitude: 179.999, Latitude: 0}
	}
	suite.Require().NoError(r.AddGeoPoints(ctx, name, points))
	q := GeoQuery{Longitude: -179.999, N: len(points)}
	q.MaxDistance = geoDistance(q, points[0])
	results, err := r.SearchGeoPoints(ctx, name, q)
	suite.Require().NoError(err)
	suite.Require().Len(results, len(points))
	for i, result := range results {
		suite.Equal(points[i], result.GeoPoint)
		suite.Equal(q.MaxDistance, result.Distance)
	}
	q.MaxDistance = math.Nextafter(q.MaxDistance, 0)
	results, err = r.SearchGeoPoints(ctx, name, q)
	suite.NoError(err)
	suite.Empty(results)
	// Unlimited queries must also traverse all cursor pages and break ties by ID.
	results, err = r.SearchGeoPoints(ctx, name, GeoQuery{N: 3})
	suite.Require().NoError(err)
	suite.Require().Len(results, 3)
	suite.Equal(points[0].Id, results[0].Id)
	suite.Equal(points[1].Id, results[1].Id)
	suite.Equal(points[2].Id, results[2].Id)
	suite.ErrorIs(r.AddGeoPoints(ctx, name, []GeoPoint{{Id: "new"}, {Id: "bad", Latitude: 91}}), storage.ErrInvalidArgument)
	suite.ErrorIs(r.DeleteGeoPoints(ctx, name, points[0].Id, ""), storage.ErrInvalidArgument)
	results, err = r.SearchGeoPoints(ctx, name, GeoQuery{Longitude: 179.999, N: 1})
	suite.Require().NoError(err)
	suite.Require().Len(results, 1)
	suite.Equal(points[0].Id, results[0].Id)
	// An interrupted scan must release its server-side cursor even after the
	// request context is canceled.
	canceled, cancel := context.WithCancel(ctx)
	err = r.scanGeoPoints(canceled, name, "*", func(geoRecord) error {
		cancel()
		return canceled.Err()
	})
	cancel()
	suite.ErrorIs(err, context.Canceled)
	info, err := r.client.FTInfo(ctx, r.geoIndex(name)).Result()
	suite.Require().NoError(err)
	suite.Zero(info.CursorStats.IndexTotal)
	ids := make([]string, len(points))
	for i, point := range points {
		ids[i] = point.Id
	}
	suite.Require().NoError(r.DeleteGeoPoints(ctx, name, ids...))
	results, err = r.SearchGeoPoints(ctx, name, GeoQuery{N: 10})
	suite.NoError(err)
	suite.Empty(results)
	names, err := r.ListGeoIndex(ctx)
	suite.NoError(err)
	suite.Equal([]string{name}, names, "deleting all points must retain the empty index")
}

func (suite *RedisTestSuite) TestGeoSearchAgainstFullScan() {
	ctx := suite.T().Context()
	r := suite.Database.(*Redis)
	name := "oracle"
	suite.Require().NoError(r.AddGeoIndex(ctx, name))
	defer func() { suite.NoError(r.DeleteGeoIndex(context.Background(), name)) }()
	rng := rand.New(rand.NewPCG(42, 17))
	points := []GeoPoint{
		{Id: "north", Latitude: 90}, {Id: "south", Latitude: -90},
		{Id: "polar-n", Latitude: 86}, {Id: "polar-s", Latitude: -86},
		{Id: "dateline", Longitude: 180}, {Id: "max-cell", Longitude: math.Nextafter(180, 0), Latitude: math.Nextafter(90, 0)},
	}
	for i := range 300 {
		points = append(points, GeoPoint{Id: strconv.Itoa(i), Longitude: rng.Float64()*360 - 180, Latitude: rng.Float64()*180 - 90})
	}
	// Duplicate IDs use the final coordinates, even across the polar boundary.
	points = append(points, GeoPoint{Id: "polar-n", Longitude: 10, Latitude: 85.05}, GeoPoint{Id: "polar-n", Longitude: 0, Latitude: 86})
	suite.Require().NoError(r.AddGeoPoints(ctx, name, points))
	records, err := prepareGeoPoints(name, points)
	suite.Require().NoError(err)
	queries := []GeoQuery{
		{Latitude: 85, N: 20, MaxDistance: 200000}, {Latitude: -85, N: 20, MaxDistance: 200000},
		{Latitude: 90, N: 10}, {Latitude: -90, N: 10}, {Longitude: -180, N: 10, MaxDistance: 1000000},
		{N: 1000}, {N: 0}, {Longitude: math.Nextafter(180, 0), Latitude: math.Nextafter(90, 0), N: 1, MaxDistance: 0.0001},
	}
	for i := range 40 {
		q := GeoQuery{Longitude: rng.Float64()*360 - 180, Latitude: rng.Float64()*180 - 90, N: 1 + i%20}
		if i%2 == 0 {
			q.MaxDistance = rng.Float64() * math.Pi * geoEarthRadius
		}
		queries = append(queries, q)
	}
	for _, q := range queries {
		want := make([]GeoResult, 0)
		for _, record := range records {
			distance := geoDistance(q, record.GeoPoint)
			if q.MaxDistance == 0 || distance <= q.MaxDistance {
				want = append(want, GeoResult{GeoPoint: record.GeoPoint, Distance: distance})
			}
		}
		sort.Slice(want, func(i, j int) bool { return geoResultLess(want[i], want[j]) })
		want = want[:min(q.N, len(want))]
		got, err := r.SearchGeoPoints(ctx, name, q)
		suite.Require().NoError(err)
		suite.Equal(want, got, "query %+v", q)
	}
	info, err := r.client.FTInfo(ctx, r.geoIndex(name)).Result()
	suite.Require().NoError(err)
	suite.Zero(info.HashIndexingFailures, "polar points must not cause Search indexing failures")
}

func (suite *RedisTestSuite) TestGeoSearchPurgeNamespace() {
	ctx := suite.T().Context()
	r := suite.Database.(*Redis)
	// Prefixes with glob metacharacters/braces must remain isolated.
	a := &Redis{client: r.client, TablePrefix: storage.TablePrefix("{}*?[]:one")}
	b := &Redis{client: r.client, TablePrefix: storage.TablePrefix("{}*?[]:two")}
	for _, db := range []*Redis{a, b} {
		suite.Require().NoError(db.AddGeoIndex(ctx, "same"))
		suite.Require().NoError(db.AddGeoPoints(ctx, "same", []GeoPoint{{Id: "point", Latitude: 90}}))
		defer func() { suite.NoError(db.DeleteGeoIndex(context.Background(), "same")) }()
	}
	suite.Require().NoError(a.Purge())
	names, err := a.ListGeoIndex(ctx)
	suite.NoError(err)
	suite.Empty(names)
	names, err = b.ListGeoIndex(ctx)
	suite.NoError(err)
	suite.Equal([]string{"same"}, names)
	results, err := b.SearchGeoPoints(ctx, "same", GeoQuery{Latitude: 90, N: 1})
	suite.NoError(err)
	suite.Len(results, 1)
	indexes, err := r.client.FT_List(ctx).Result()
	suite.NoError(err)
	sort.Strings(indexes)
	for _, index := range indexes {
		suite.False(strings.HasPrefix(index, a.geoKeyPrefix()))
	}
}

func TestRedis(t *testing.T) {
	if redisDSN == "" {
		t.Skip("REDIS_URI is not set, skipping Redis test")
	}
	suite.Run(t, new(RedisTestSuite))
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

func BenchmarkRedis(b *testing.B) {
	log.CloseLogger()
	// open db
	database, err := Open(redisDSN, "gorse_")
	assert.NoError(b, err)
	// flush db
	err = database.(*Redis).client.FlushDB(b.Context()).Err()
	assert.NoError(b, err)
	// create schema
	err = database.Init()
	assert.NoError(b, err)
	// benchmark
	benchmark(b, database)
}
