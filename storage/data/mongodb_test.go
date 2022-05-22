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
	"context"
	"github.com/stretchr/testify/assert"
	"os"
	"runtime"
	"strings"
	"testing"
)

var (
	mongoUri string
)

func init() {
	// get environment variables
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	mongoUri = env("MONGO_URI", "mongodb://root:password@127.0.0.1:27017/")
}

type testMongoDatabase struct {
	Database
}

func (db *testMongoDatabase) GetMongoDB(t *testing.T) *MongoDB {
	var mongoDatabase *MongoDB
	var ok bool
	mongoDatabase, ok = db.Database.(*MongoDB)
	assert.True(t, ok)
	return mongoDatabase
}

func (db *testMongoDatabase) Close(t *testing.T) {
	err := db.Database.Close()
	assert.NoError(t, err)
}

func newTestMongoDatabase(t *testing.T) *testMongoDatabase {
	// retrieve test name
	var testName string
	pc, _, _, ok := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	if ok && details != nil {
		splits := strings.Split(details.Name(), ".")
		testName = splits[len(splits)-1]
	} else {
		t.Fatalf("failed to retrieve test name")
	}

	ctx := context.Background()
	database := new(testMongoDatabase)
	var err error
	// create database
	database.Database, err = Open(mongoUri)
	assert.NoError(t, err)
	dbName := "gorse_" + testName
	databaseComm := database.GetMongoDB(t)
	assert.NoError(t, err)
	err = databaseComm.client.Database(dbName).Drop(ctx)
	if err == nil {
		t.Log("delete existed database:", dbName)
	}
	err = database.Database.Close()
	assert.NoError(t, err)
	// create schema
	database.Database, err = Open(mongoUri + dbName + "?authSource=admin&connect=direct")
	assert.NoError(t, err)
	err = database.Init()
	assert.NoError(t, err)
	return database
}

func TestMongoDatabase_Users(t *testing.T) {
	db := newTestMongoDatabase(t)
	defer db.Close(t)
	testUsers(t, db.Database)
}

func TestMongoDatabase_Feedback(t *testing.T) {
	db := newTestMongoDatabase(t)
	defer db.Close(t)
	testFeedback(t, db.Database)
}

func TestMongoDatabase_Item(t *testing.T) {
	db := newTestMongoDatabase(t)
	defer db.Close(t)
	testItems(t, db.Database)
}

func TestMongoDatabase_DeleteUser(t *testing.T) {
	db := newTestMongoDatabase(t)
	defer db.Close(t)
	testDeleteUser(t, db.Database)
}

func TestMongoDatabase_DeleteItem(t *testing.T) {
	db := newTestMongoDatabase(t)
	defer db.Close(t)
	testDeleteItem(t, db.Database)
}

func TestMongoDatabase_DeleteFeedback(t *testing.T) {
	db := newTestMongoDatabase(t)
	defer db.Close(t)
	testDeleteFeedback(t, db.Database)
}

func TestMongoDatabase_TimeLimit(t *testing.T) {
	db := newTestMongoDatabase(t)
	defer db.Close(t)
	testTimeLimit(t, db.Database)
}

func TestMongoDatabase_Timezone(t *testing.T) {
	db := newTestMongoDatabase(t)
	defer db.Close(t)
	testTimeZone(t, db.Database)
}
