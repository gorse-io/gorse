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
	"io"
	"math"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/fxtlabs/primes"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type baseTestSuite struct {
	suite.Suite
	Database
}

func (suite *baseTestSuite) TearDownSuite() {
	err := suite.Database.Close()
	suite.NoError(err)
}

func (suite *baseTestSuite) SetupTest() {
	err := suite.Database.Ping()
	suite.NoError(err)
	err = suite.Database.Purge()
	suite.NoError(err)
}

func (suite *baseTestSuite) TearDownTest() {
	err := suite.Database.Purge()
	suite.NoError(err)
}

func (suite *baseTestSuite) TestInit() {
	err := suite.Database.Init()
	suite.NoError(err)
}

func (suite *baseTestSuite) TestMeta() {
	ctx := context.Background()
	// Set meta string
	err := suite.Database.Set(ctx, String(Key("meta", "1"), "2"), String(Key("meta", "1000"), "10"))
	suite.NoError(err)
	// Get meta string
	value, err := suite.Database.Get(ctx, Key("meta", "1")).String()
	suite.NoError(err)
	suite.Equal("2", value)
	value, err = suite.Database.Get(ctx, Key("meta", "1000")).String()
	suite.NoError(err)
	suite.Equal("10", value)
	// Delete string
	err = suite.Database.Delete(ctx, Key("meta", "1"))
	suite.NoError(err)
	// Get meta not existed
	value, err = suite.Database.Get(ctx, Key("meta", "1")).String()
	suite.True(errors.Is(err, errors.NotFound), err)
	suite.Equal("", value)
	// Set meta int
	err = suite.Database.Set(ctx, Integer(Key("meta", "1"), 2))
	suite.NoError(err)
	// Get meta int
	valInt, err := suite.Database.Get(ctx, Key("meta", "1")).Integer()
	suite.NoError(err)
	suite.Equal(2, valInt)
	// set meta time
	err = suite.Database.Set(ctx, Time(Key("meta", "1"), time.Date(1996, 4, 8, 0, 0, 0, 0, time.UTC)))
	suite.NoError(err)
	// get meta time
	valTime, err := suite.Database.Get(ctx, Key("meta", "1")).Time()
	suite.NoError(err)
	suite.Equal(1996, valTime.Year())
	suite.Equal(time.Month(4), valTime.Month())
	suite.Equal(8, valTime.Day())

	// test set empty
	err = suite.Database.Set(ctx)
	suite.NoError(err)
	// test set duplicate
	err = suite.Database.Set(ctx, String("100", "1"), String("100", "2"))
	suite.NoError(err)
}

func (suite *baseTestSuite) TestSet() {
	ctx := context.Background()
	err := suite.Database.SetSet(ctx, "set", "1")
	suite.NoError(err)
	// test add
	err = suite.Database.AddSet(ctx, "set", "2")
	suite.NoError(err)
	var members []string
	members, err = suite.Database.GetSet(ctx, "set")
	suite.NoError(err)
	suite.Equal([]string{"1", "2"}, members)
	// test rem
	err = suite.Database.RemSet(ctx, "set", "1")
	suite.NoError(err)
	members, err = suite.Database.GetSet(ctx, "set")
	suite.NoError(err)
	suite.Equal([]string{"2"}, members)
	// test set
	err = suite.Database.SetSet(ctx, "set", "3")
	suite.NoError(err)
	members, err = suite.Database.GetSet(ctx, "set")
	suite.NoError(err)
	suite.Equal([]string{"3"}, members)

	// test add empty
	err = suite.Database.AddSet(ctx, "set")
	suite.NoError(err)
	// test set empty
	err = suite.Database.SetSet(ctx, "set")
	suite.NoError(err)
	// test get empty
	members, err = suite.Database.GetSet(ctx, "unknown_set")
	suite.NoError(err)
	suite.Empty(members)
	// test rem empty
	err = suite.Database.RemSet(ctx, "set")
	suite.NoError(err)
}

func (suite *baseTestSuite) TestScan() {
	ctx := context.Background()
	err := suite.Database.Set(ctx, String("1", "1"))
	suite.NoError(err)
	err = suite.Database.SetSet(ctx, "2", "21", "22", "23")
	suite.NoError(err)

	var keys []string
	err = suite.Database.Scan(func(s string) error {
		keys = append(keys, s)
		return nil
	})
	suite.NoError(err)
	suite.ElementsMatch([]string{"1", "2"}, keys)
}

func (suite *baseTestSuite) TestPurge() {
	ctx := context.Background()
	// insert data
	err := suite.Database.Set(ctx, String("key", "value"))
	suite.NoError(err)
	ret := suite.Database.Get(ctx, "key")
	suite.NoError(ret.err)
	suite.Equal("value", ret.value)

	err = suite.Database.AddSet(ctx, "set", "a", "b", "c")
	suite.NoError(err)
	s, err := suite.Database.GetSet(ctx, "set")
	suite.NoError(err)
	suite.ElementsMatch([]string{"a", "b", "c"}, s)

	// purge data
	err = suite.Database.Purge()
	suite.NoError(err)
	ret = suite.Database.Get(ctx, "key")
	suite.ErrorIs(ret.err, errors.NotFound)
	s, err = suite.Database.GetSet(ctx, "set")
	suite.NoError(err)
	suite.Empty(s)

	// purge empty dataset
	err = suite.Database.Purge()
	suite.NoError(err)
}

func (suite *baseTestSuite) TestPushPop() {
	ctx := context.Background()
	err := suite.Push(ctx, "a", "1")
	suite.NoError(err)
	err = suite.Push(ctx, "a", "2")
	suite.NoError(err)
	count, err := suite.Remain(ctx, "a")
	suite.NoError(err)
	suite.Equal(int64(2), count)

	err = suite.Push(ctx, "b", "1")
	suite.NoError(err)
	err = suite.Push(ctx, "b", "2")
	suite.NoError(err)
	err = suite.Push(ctx, "b", "1")
	suite.NoError(err)
	count, err = suite.Remain(ctx, "b")
	suite.NoError(err)
	suite.Equal(int64(2), count)

	value, err := suite.Pop(ctx, "a")
	suite.NoError(err)
	suite.Equal("1", value)
	value, err = suite.Pop(ctx, "a")
	suite.NoError(err)
	suite.Equal("2", value)
	_, err = suite.Pop(ctx, "a")
	suite.ErrorIs(err, io.EOF)

	value, err = suite.Pop(ctx, "b")
	suite.NoError(err)
	suite.Equal("2", value)
	value, err = suite.Pop(ctx, "b")
	suite.NoError(err)
	suite.Equal("1", value)
	_, err = suite.Pop(ctx, "b")
	suite.ErrorIs(err, io.EOF)
}

func (suite *baseTestSuite) TestDocument() {
	ts := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx := context.Background()
	err := suite.AddScores(ctx, "a", "", []Score{{
		Id:         "0",
		Score:      math.MaxFloat64,
		IsHidden:   true,
		Categories: []string{"a", "b"},
		Timestamp:  ts,
	}})
	suite.NoError(err)
	err = suite.AddScores(ctx, "a", "", []Score{{
		Id:         "1",
		Score:      100,
		Categories: []string{"a", "b"},
		Timestamp:  ts,
	}})
	suite.NoError(err)
	err = suite.AddScores(ctx, "a", "", []Score{
		{
			Id:         "1",
			Score:      1,
			Categories: []string{"a", "b"},
			Timestamp:  ts,
		},
		{
			Id:         "2",
			Score:      2,
			Categories: []string{"b", "c"},
			Timestamp:  ts,
		},
		{
			Id:         "3",
			Score:      3,
			Categories: []string{"b"},
			Timestamp:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Id:         "4",
			Score:      4,
			Categories: []string{""},
			Timestamp:  ts,
		},
		{
			Id:         "5",
			Score:      5,
			Categories: []string{"b"},
			Timestamp:  ts,
		},
	})
	suite.NoError(err)
	err = suite.AddScores(ctx, "b", "", []Score{{
		Id:         "6",
		Score:      6,
		Categories: []string{"b"},
		Timestamp:  ts,
	}})
	suite.NoError(err)

	// search documents
	documents, err := suite.SearchScores(ctx, "a", "", []string{"b"}, 1, 3)
	suite.NoError(err)
	suite.Equal([]Score{
		{Id: "3", Score: 3, Categories: []string{"b"}, Timestamp: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Id: "2", Score: 2, Categories: []string{"b", "c"}, Timestamp: ts},
	}, documents)
	documents, err = suite.SearchScores(ctx, "a", "", []string{"b"}, 0, -1)
	suite.NoError(err)
	suite.Equal([]Score{
		{Id: "5", Score: 5, Categories: []string{"b"}, Timestamp: ts},
		{Id: "3", Score: 3, Categories: []string{"b"}, Timestamp: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Id: "2", Score: 2, Categories: []string{"b", "c"}, Timestamp: ts},
		{Id: "1", Score: 1, Categories: []string{"a", "b"}, Timestamp: ts},
	}, documents)

	// search documents with nil category
	documents, err = suite.SearchScores(ctx, "a", "", nil, 0, -1)
	suite.NoError(err)
	suite.Equal([]Score{
		{Id: "5", Score: 5, Categories: []string{"b"}, Timestamp: ts},
		{Id: "4", Score: 4, Categories: []string{""}, Timestamp: ts},
		{Id: "3", Score: 3, Categories: []string{"b"}, Timestamp: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Id: "2", Score: 2, Categories: []string{"b", "c"}, Timestamp: ts},
		{Id: "1", Score: 1, Categories: []string{"a", "b"}, Timestamp: ts},
	}, documents)

	// search documents with empty category
	documents, err = suite.SearchScores(ctx, "a", "", []string{""}, 0, -1)
	suite.NoError(err)
	suite.Equal([]Score{{Id: "4", Score: 4, Categories: []string{""}, Timestamp: ts}}, documents)

	// delete nothing
	err = suite.DeleteScores(ctx, []string{"a"}, ScoreCondition{})
	suite.ErrorIs(err, errors.NotValid)
	// delete by value
	err = suite.DeleteScores(ctx, []string{"a"}, ScoreCondition{Id: proto.String("5")})
	suite.NoError(err)
	documents, err = suite.SearchScores(ctx, "a", "", []string{"b"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("3", documents[0].Id)
	// delete by timestamp
	err = suite.DeleteScores(ctx, []string{"a"}, ScoreCondition{Before: lo.ToPtr(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC))})
	suite.NoError(err)
	documents, err = suite.SearchScores(ctx, "a", "", []string{"b"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Id)

	// update categories
	err = suite.UpdateScores(ctx, []string{"a"}, nil, "2", ScorePatch{Categories: []string{"c", "s"}})
	suite.NoError(err)
	documents, err = suite.SearchScores(ctx, "a", "", []string{"s"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Id)
	err = suite.UpdateScores(ctx, []string{"a"}, nil, "2", ScorePatch{Categories: []string{"c"}})
	suite.NoError(err)
	documents, err = suite.SearchScores(ctx, "a", "", []string{"s"}, 0, 1)
	suite.NoError(err)
	suite.Empty(documents)

	// update is hidden
	err = suite.UpdateScores(ctx, []string{"a"}, nil, "0", ScorePatch{IsHidden: proto.Bool(false)})
	suite.NoError(err)
	documents, err = suite.SearchScores(ctx, "a", "", []string{"b"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("0", documents[0].Id)
}

func (suite *baseTestSuite) TestSubsetDocument() {
	ts := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx := context.Background()
	err := suite.AddScores(ctx, "a", "a", []Score{
		{
			Id:         "1",
			Score:      1,
			Categories: []string{"a", "b"},
			Timestamp:  ts,
		},
		{
			Id:         "2",
			Score:      2,
			Categories: []string{"b", "c"},
			Timestamp:  ts,
		},
		{
			Id:         "3",
			Score:      3,
			Categories: []string{"b"},
			Timestamp:  ts,
		},
	})
	suite.NoError(err)
	err = suite.AddScores(ctx, "b", "", []Score{
		{
			Id:         "4",
			Score:      4,
			Categories: []string{},
			Timestamp:  ts,
		},
		{
			Id:         "3",
			Score:      3,
			Categories: []string{"b"},
			Timestamp:  ts,
		},
		{
			Id:         "2",
			Score:      2,
			Categories: []string{"b"},
			Timestamp:  ts,
		},
	})
	suite.NoError(err)

	// search documents
	documents, err := suite.SearchScores(ctx, "a", "a", []string{"b"}, 0, -1)
	suite.NoError(err)
	suite.Equal([]Score{
		{Id: "3", Score: 3, Categories: []string{"b"}, Timestamp: ts},
		{Id: "2", Score: 2, Categories: []string{"b", "c"}, Timestamp: ts},
		{Id: "1", Score: 1, Categories: []string{"a", "b"}, Timestamp: ts},
	}, documents)

	// update categories
	err = suite.UpdateScores(ctx, []string{"a", "b"}, nil, "2", ScorePatch{Categories: []string{"b", "s"}})
	suite.NoError(err)
	documents, err = suite.SearchScores(ctx, "a", "a", []string{"s"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Id)
	documents, err = suite.SearchScores(ctx, "b", "", []string{"s"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Id)

	// update categories in subset
	err = suite.UpdateScores(ctx, []string{"a", "b"}, proto.String("a"), "2", ScorePatch{Categories: []string{"b", "x"}})
	suite.NoError(err)
	documents, err = suite.SearchScores(ctx, "a", "a", []string{"x"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Id)
	documents, err = suite.SearchScores(ctx, "b", "", []string{"x"}, 0, 1)
	suite.NoError(err)
	suite.Empty(documents)

	// delete by value
	err = suite.DeleteScores(ctx, []string{"a", "b"}, ScoreCondition{Id: proto.String("3")})
	suite.NoError(err)
	documents, err = suite.SearchScores(ctx, "a", "a", []string{"b"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Id)
	documents, err = suite.SearchScores(ctx, "b", "", []string{"b"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Id)

	// delete in subset
	err = suite.DeleteScores(ctx, []string{"a", "b"}, ScoreCondition{
		Subset: proto.String("a"),
		Id:     proto.String("2"),
	})
	suite.NoError(err)
	documents, err = suite.SearchScores(ctx, "a", "a", []string{"b"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("1", documents[0].Id)
	documents, err = suite.SearchScores(ctx, "b", "", []string{"b"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Id)
}

func (suite *baseTestSuite) TestScanScores() {
	// add scores
	timestamp := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	scores := map[lo.Tuple2[string, string]][]Score{
		{"a", "b"}: {
			{Id: "1", Score: 1, Categories: []string{"a", "b"}, Timestamp: timestamp},
			{Id: "2", Score: 2, Categories: []string{"b", "c"}, Timestamp: timestamp},
			{Id: "3", Score: 3, Categories: []string{"b"}, Timestamp: timestamp},
		},
		{"a", "c"}: {
			{Id: "4", Score: 4, Categories: []string{"a", "b"}, Timestamp: timestamp},
			{Id: "5", Score: 5, Categories: []string{"b", "c"}, Timestamp: timestamp},
			{Id: "6", Score: 6, Categories: []string{"b"}, Timestamp: timestamp},
		},
		{"b", "c"}: {
			{Id: "7", Score: 7, Categories: []string{"a", "b"}, Timestamp: timestamp},
			{Id: "8", Score: 8, Categories: []string{"b", "c"}, Timestamp: timestamp},
			{Id: "9", Score: 9, Categories: []string{"b"}, Timestamp: timestamp},
		},
	}
	for k, v := range scores {
		err := suite.AddScores(context.Background(), k.A, k.B, v)
		suite.NoError(err)
	}

	// scan scores
	totalScores := 0
	ctx := context.Background()
	err := suite.ScanScores(ctx, func(collection, id, subset string, t time.Time) error {
		totalScores++
		suite.Equal(timestamp, t.UTC())
		return nil
	})
	suite.NoError(err)
	suite.Equal(9, totalScores)

	// scan scores with timeout
	scanScores := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	err = suite.ScanScores(ctx, func(collection, id, subset string, timestamp time.Time) error {
		time.Sleep(time.Millisecond)
		scanScores++
		return nil
	})
	if err != nil && status.Code(err) != codes.DeadlineExceeded {
		suite.ErrorIs(err, context.DeadlineExceeded)
	}
	suite.Less(scanScores, 9)
}

func (suite *baseTestSuite) TestTimeSeries() {
	ts := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx := context.Background()
	err := suite.AddTimeSeriesPoints(ctx, []TimeSeriesPoint{
		{Name: "a", Value: 1, Timestamp: ts.Add(1 * time.Second)},
		{Name: "a", Value: 2, Timestamp: ts.Add(2 * time.Second)},
		{Name: "a", Value: 3, Timestamp: ts.Add(3 * time.Second)},
		{Name: "a", Value: 4, Timestamp: ts.Add(4 * time.Second)},
		{Name: "a", Value: 5, Timestamp: ts.Add(5 * time.Second)},
		{Name: "b", Value: 3, Timestamp: ts.Add(3 * time.Second)},
	})
	suite.NoError(err)

	points, err := suite.GetTimeSeriesPoints(ctx, "a", ts.Add(2*time.Second), ts.Add(4*time.Second), time.Second)
	suite.NoError(err)
	suite.Equal([]TimeSeriesPoint{
		{Name: "a", Value: 2, Timestamp: ts.Add(2 * time.Second)},
		{Name: "a", Value: 3, Timestamp: ts.Add(3 * time.Second)},
		{Name: "a", Value: 4, Timestamp: ts.Add(4 * time.Second)},
	}, points)

	points, err = suite.GetTimeSeriesPoints(ctx, "a", ts.Add(2*time.Second), ts.Add(4*time.Second), 2*time.Second)
	suite.NoError(err)
	suite.Equal([]TimeSeriesPoint{
		{Name: "a", Value: 3, Timestamp: ts.Add(2 * time.Second)},
		{Name: "a", Value: 4, Timestamp: ts.Add(4 * time.Second)},
	}, points)
}

func (suite *baseTestSuite) TestTimestampPrecision() {
	ctx := context.Background()
	timestamp := time.Date(2023, 1, 1, 0, 0, 0, 500, time.UTC)
	// add scores
	err := suite.Database.AddScores(ctx, "a", "s", []Score{
		{Id: "1", Score: 1, Categories: []string{""}, Timestamp: timestamp},
	})
	suite.NoError(err)
	// remove by timestamp
	err = suite.Database.DeleteScores(ctx, []string{"a"}, ScoreCondition{
		Subset: proto.String("s"),
		Before: lo.ToPtr(timestamp)})
	suite.NoError(err)
	// search scores
	documents, err := suite.Database.SearchScores(ctx, "a", "s", nil, 0, -1)
	suite.NoError(err)
	suite.NotEmpty(documents)
}

func TestKey(t *testing.T) {
	assert.Empty(t, Key())
	assert.Equal(t, "a", Key("a"))
	assert.Equal(t, "a", Key("a", ""))
	assert.Equal(t, "a/b", Key("a", "b"))
}

var (
	benchmarkDataSize = 100000
	primeTable        []int
)

func init() {
	benchmarkDataSizeStr := os.Getenv("BENCHMARK_DATA_SIZE")
	if benchmarkDataSizeStr != "" {
		benchmarkDataSize, _ = strconv.Atoi(benchmarkDataSizeStr)
	}
	primeTable = primes.Sieve(benchmarkDataSize)
}

func primeFactor(n int) []int {
	var factors []int
	for _, p := range primeTable {
		if n%p == 0 {
			factors = append(factors, p)
		}
	}
	return factors
}

func benchmark(b *testing.B, database Database) {
	b.Run("AddScores", func(b *testing.B) {
		benchmarkAddDocuments(b, database)
	})
	b.Run("SearchScores", func(b *testing.B) {
		benchmarkSearchDocuments(b, database)
	})
	b.Run("UpdateScores", func(b *testing.B) {
		benchmarkUpdateDocuments(b, database)
	})
}

func benchmarkAddDocuments(b *testing.B, database Database) {
	ctx := context.Background()
	var documents []Score
	for i := 1; i <= b.N; i++ {
		documents = append(documents, Score{
			Id:         strconv.Itoa(i),
			Score:      float64(-i),
			Categories: lo.Map(primeFactor(i), func(n, _ int) string { return strconv.Itoa(n) }),
			Timestamp:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		})
	}
	b.ResetTimer()
	err := database.AddScores(ctx, "a", "", documents)
	assert.NoError(b, err)
}

func benchmarkSearchDocuments(b *testing.B, database Database) {
	// insert data
	ctx := context.Background()
	var documents []Score
	for i := 1; i <= benchmarkDataSize; i++ {
		documents = append(documents, Score{
			Id:         strconv.Itoa(i),
			Score:      float64(-i),
			Categories: lo.Map(primeFactor(i), func(n, _ int) string { return strconv.Itoa(n) }),
			Timestamp:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		})
	}
	err := database.AddScores(ctx, "a", "", documents)
	assert.NoError(b, err)
	// search data
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// select a random prime
		p := primeTable[rand.Intn(len(primeTable))]
		// search documents
		r, err := database.SearchScores(ctx, "a", "", []string{strconv.Itoa(p)}, 0, 10)
		assert.NoError(b, err)
		assert.NotEmpty(b, r)
	}
}

func benchmarkUpdateDocuments(b *testing.B, database Database) {
	ctx := context.Background()
	b.ResetTimer()
	for i := 1; i <= b.N; i++ {
		// select a random number
		n := rand.Intn(benchmarkDataSize) + 1
		// update documents
		err := database.UpdateScores(ctx, []string{"a"}, nil, strconv.Itoa(n), ScorePatch{
			Score: proto.Float64(float64(n)),
		})
		assert.NoError(b, err)
	}
}
