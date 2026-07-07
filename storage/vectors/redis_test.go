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
	"os"
	"testing"

	"github.com/gorse-io/gorse/common/log"
	"github.com/juju/errors"
	"github.com/stretchr/testify/suite"
)

var redisUri string

func init() {
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	redisUri = env("REDIS_URI", "redis://127.0.0.1:6379/")
}

type RedisTestSuite struct {
	vectorsTestSuite
}

func (suite *RedisTestSuite) SetupSuite() {
	log.SetTestLogger(suite.T())
	var err error
	suite.Database, err = Open(redisUri, "gorse_")
	suite.NoError(err)
	suite.NoError(suite.Database.Init())
}

func (suite *RedisTestSuite) TearDownSuite() {
	if suite.Database != nil {
		suite.NoError(suite.Database.Close())
	}
}

func (suite *RedisTestSuite) TestQuantization() {
	suite.testQuantization(QuantizationNone, 0)

	ctx := suite.T().Context()
	err := suite.Database.AddCollection(ctx, "unsupported_quantization", defaultVectorSize, Cosine, VectorConfig{Type: QuantizationSQ})
	suite.Error(err)
}

func (suite *RedisTestSuite) TestDistances() {
	ctx := suite.T().Context()

	err := suite.Database.AddCollection(ctx, "euclidean", defaultVectorSize, Euclidean, VectorConfig{})
	suite.NoError(err)
	info, err := suite.Database.DescribeCollection(ctx, "euclidean")
	suite.NoError(err)
	suite.Equal(Euclidean, info.Distance)

	err = suite.Database.AddCollection(ctx, "dot", defaultVectorSize, Dot, VectorConfig{})
	suite.NoError(err)
	info, err = suite.Database.DescribeCollection(ctx, "dot")
	suite.NoError(err)
	suite.Equal(Dot, info.Distance)

	err = suite.Database.AddCollection(ctx, "unsupported_distance", defaultVectorSize, Distance(100), VectorConfig{})
	suite.Error(err)
}

func (suite *RedisTestSuite) TestEmptyInputs() {
	ctx := suite.T().Context()
	suite.NoError(suite.Database.Optimize(ctx, "test"))
	suite.NoError(suite.Database.AddVectors(ctx, "test", nil))
	results, err := suite.Database.QueryVectors(ctx, "test", []float32{1, 0, 0, 0}, nil, 0)
	suite.NoError(err)
	suite.Empty(results)
}

func TestRedis(t *testing.T) {
	suite.Run(t, new(RedisTestSuite))
}

func TestRedisOpen(t *testing.T) {
	for _, uri := range []string{
		"rediss://localhost:6379/0",
		"redis+cluster://:password@192.168.1.11:6379?addr=192.168.0.5:6379&addr=192.168.0.7:6379",
		"rediss+cluster://:password@192.168.1.11:6379?addr=192.168.0.5:6379",
	} {
		database, err := Open(uri, "gorse_")
		if err != nil {
			t.Fatal(err)
		}
		if err = database.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRedisHelpers(t *testing.T) {
	blob := redisVectorBlob([]float32{1, 2})
	if len(blob) != 8 {
		t.Fatalf("unexpected vector blob size: %d", len(blob))
	}

	for _, tc := range []struct {
		distance Distance
		metric   string
	}{
		{Cosine, "COSINE"},
		{Euclidean, "L2"},
		{Dot, "IP"},
	} {
		metric, err := distanceToRedisDistance(tc.distance)
		if err != nil {
			t.Fatal(err)
		}
		if metric != tc.metric {
			t.Fatalf("expected %s, got %s", tc.metric, metric)
		}
		distance, err := redisDistanceToDistance(metric)
		if err != nil {
			t.Fatal(err)
		}
		if distance != tc.distance {
			t.Fatalf("expected %v, got %v", tc.distance, distance)
		}
	}
	if _, err := distanceToRedisDistance(Distance(100)); err == nil {
		t.Fatal("expected unsupported distance error")
	}
	if _, err := redisDistanceToDistance("unknown"); err == nil {
		t.Fatal("expected unsupported distance error")
	}

	categories := []string{"cat,a", "space cat", "中文"}
	encoded := encodeRedisCategories(categories)
	decoded := decodeRedisCategories(encoded)
	if len(decoded) != len(categories) {
		t.Fatalf("expected %d categories, got %d", len(categories), len(decoded))
	}
	for i := range categories {
		if decoded[i] != categories[i] {
			t.Fatalf("expected %q, got %q", categories[i], decoded[i])
		}
	}
	if decoded = decodeRedisCategories("not-hex"); decoded[0] != "not-hex" {
		t.Fatalf("unexpected decode fallback: %v", decoded)
	}

	for _, message := range []string{"Unknown index name", "no such index", "SEARCH_INDEX_NOT_FOUND"} {
		if !isRedisIndexNotFound(errors.New(message)) {
			t.Fatalf("expected index not found: %s", message)
		}
	}
	if isRedisIndexNotFound(nil) || isRedisIndexNotFound(errors.New("other")) {
		t.Fatal("unexpected index not found")
	}
}
