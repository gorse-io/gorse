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
	"os"
	"testing"
	"time"

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
	// flush db
	err = suite.Database.(*Redis).client.FlushDB(context.TODO()).Err()
	suite.NoError(err)
	// create schema
	err = suite.Database.Init()
	suite.NoError(err)
}

func TestRedis(t *testing.T) {
	suite.Run(t, new(RedisTestSuite))
}

func TestParseRedisClusterURL(t *testing.T) {
	options, err := ParseRedisClusterURL("redis+cluster://username:password@127.0.0.1:6379,127.0.0.1:6380,127.0.0.1:6381/?" +
		"max_retries=1000&dial_timeout=1h&pool_fifo=true")
	if assert.NoError(t, err) {
		assert.Equal(t, "username", options.Username)
		assert.Equal(t, "password", options.Password)
		assert.Equal(t, []string{"127.0.0.1:6379", "127.0.0.1:6380", "127.0.0.1:6381"}, options.Addrs)
		assert.Equal(t, 1000, options.MaxRetries)
		assert.Equal(t, time.Hour, options.DialTimeout)
		assert.True(t, options.PoolFIFO)
	}

	_, err = ParseRedisClusterURL("redis://")
	assert.Error(t, err)
	_, err = ParseRedisClusterURL("redis+cluster://username:password@127.0.0.1:6379/?max_retries=a")
	assert.Error(t, err)
	_, err = ParseRedisClusterURL("redis+cluster://username:password@127.0.0.1:6379/?dial_timeout=a")
	assert.Error(t, err)
	_, err = ParseRedisClusterURL("redis+cluster://username:password@127.0.0.1:6379/?pool_fifo=a")
	assert.Error(t, err)
	_, err = ParseRedisClusterURL("redis+cluster://username:password@127.0.0.1:6379/?a=1")
	assert.Error(t, err)
}
