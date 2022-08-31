package cache

import (
	"github.com/stretchr/testify/assert"
	"os"
	"strconv"
	"testing"
)

func init() {
	// get environment variables
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	redisDSN = env("REDIS_URI", "redis://127.0.0.1:6379/,")
}

type testRedisCluster struct {
	Database
}

func newMockRedisCluster(t *testing.T) *testRedisCluster {
	var err error
	db := new(testRedisCluster)
	assert.NoError(t, err)
	database.Inc()
	db.Database, err = Open(redisDSN+strconv.Itoa(int(database.Load())), "gorse_")
	assert.NoError(t, err)
	return db
}

func (db *testRedisCluster) Close(t *testing.T) {
	err := db.Database.Close()
	assert.NoError(t, err)
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
