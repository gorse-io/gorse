package data

import (
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/storage"
	"testing"
)

type mockRedisCluster struct {
	Database
	server *miniredis.Miniredis
}

func newMockRedisCluster(t *testing.T) *mockRedisCluster {
	var err error
	db := new(mockRedisCluster)
	db.server, err = miniredis.Run()
	assert.NoError(t, err)
	db.Database, err = Open(storage.RedisPrefix+db.server.Addr()+",", "")
	assert.NoError(t, err)
	return db
}

func (db *mockRedisCluster) Close(t *testing.T) {
	err := db.Database.Close()
	assert.NoError(t, err)
	db.server.Close()
}

func TestRedisCluster_Users(t *testing.T) {
	db := newMockRedisCluster(t)
	defer db.Close(t)
	testUsers(t, db.Database)
}

func TestRedisCluster_Feedback(t *testing.T) {
	db := newMockRedisCluster(t)
	defer db.Close(t)
	testFeedback(t, db.Database)
}

func TestRedisCluster_Item(t *testing.T) {
	db := newMockRedisCluster(t)
	defer db.Close(t)
	testItems(t, db.Database)
}

func TestRedisCluster_DeleteUser(t *testing.T) {
	db := newMockRedisCluster(t)
	defer db.Close(t)
	testDeleteUser(t, db.Database)
}

func TestRedisCluster_DeleteItem(t *testing.T) {
	db := newMockRedisCluster(t)
	defer db.Close(t)
	testDeleteItem(t, db.Database)
}

func TestRedisCluster_DeleteFeedback(t *testing.T) {
	db := newMockRedisCluster(t)
	defer db.Close(t)
	testDeleteFeedback(t, db.Database)
}

func TestRedisCluster_TimeLimit(t *testing.T) {
	db := newMockRedisCluster(t)
	defer db.Close(t)
	testTimeLimit(t, db.Database)
}
