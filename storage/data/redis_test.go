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
package data

import (
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/storage"
	"testing"
)

type mockRedis struct {
	Database
	server *miniredis.Miniredis
}

func newMockRedis(t *testing.T) *mockRedis {
	var err error
	db := new(mockRedis)
	db.server, err = miniredis.Run()
	assert.NoError(t, err)
	db.Database, err = Open(storage.RedisPrefix + db.server.Addr())
	assert.NoError(t, err)
	return db
}

func (db *mockRedis) Close(t *testing.T) {
	err := db.Database.Close()
	assert.NoError(t, err)
	db.server.Close()
}

func TestRedis_Users(t *testing.T) {
	db := newMockRedis(t)
	defer db.Close(t)
	testUsers(t, db.Database)
}

func TestRedis_Feedback(t *testing.T) {
	db := newMockRedis(t)
	defer db.Close(t)
	testFeedback(t, db.Database)
}

func TestRedis_Item(t *testing.T) {
	db := newMockRedis(t)
	defer db.Close(t)
	testItems(t, db.Database)
}

func TestRedis_DeleteUser(t *testing.T) {
	db := newMockRedis(t)
	defer db.Close(t)
	testDeleteUser(t, db.Database)
}

func TestRedis_DeleteItem(t *testing.T) {
	db := newMockRedis(t)
	defer db.Close(t)
	testDeleteItem(t, db.Database)
}

func TestRedis_DeleteFeedback(t *testing.T) {
	db := newMockRedis(t)
	defer db.Close(t)
	testDeleteFeedback(t, db.Database)
}

func TestRedis_TimeLimit(t *testing.T) {
	db := newMockRedis(t)
	defer db.Close(t)
	testTimeLimit(t, db.Database)
}
