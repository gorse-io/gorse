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
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"
	"os"
	"strconv"
	"strings"
	"testing"
)

var (
	redisDSNCluster string
	databaseCluster atomic.Int64
)

func init() {
	// get environment variables
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	redisDSNCluster = env("REDIS_URI", "redis://127.0.0.1:6379/")
}

type testRedisCluster struct {
	Database
}

func newMockRedisCluster(t *testing.T) *testRedisCluster {
	var err error
	db := new(testRedisCluster)
	assert.NoError(t, err)
	databaseCluster.Inc()
	db.Database, err = Open(redisDSNCluster+strconv.Itoa(int(databaseCluster.Load())), "gorse_")
	assert.NoError(t, err)
	return db
}

func (db *testRedisCluster) Close(t *testing.T) {
	err := db.Database.Close()
	assert.NoError(t, err)
}
func TestPathsSplite(t *testing.T) {
	paths := strings.Split("redis://test.example.com:6379/0,", ",")
	addrs := make([]string, 0, len(paths))
	for _, path := range paths {
		if path != "" {
			addrs = append(addrs, path)
		}
	}
	if len(addrs) == 0 {
		t.Errorf("addrs parse error")
		return
	}

	println(len(addrs))
	fmt.Printf("%v", addrs)
}

func TestRedisCluster_Meta(t *testing.T) {
	db := newMockRedisCluster(t)
	defer db.Close(t)
	testMeta(t, db.Database)
}

func TestRedisCluster_Sort(t *testing.T) {
	db := newMockRedisCluster(t)
	defer db.Close(t)
	testSort(t, db.Database)
}

func TestRedisCluster_Set(t *testing.T) {
	db := newMockRedisCluster(t)
	defer db.Close(t)
	testSet(t, db.Database)
}

func TestRedisCluster_Scan(t *testing.T) {
	db := newMockRedisCluster(t)
	defer db.Close(t)
	testScan(t, db.Database)
}
