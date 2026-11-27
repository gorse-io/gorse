// Copyright 2022 gorse Project Authors
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
	"os"
	"testing"
	"time"

	"github.com/gorse-io/gorse/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson"
)

var (
	mongoUri string
)

func init() {
	// os.Setenv("MONGO_URI", "mongodb://root:password@127.0.0.1:27017/")
	mongoUri = os.Getenv("MONGO_URI")
}

type MongoTestSuite struct {
	baseTestSuite
}

func (suite *MongoTestSuite) SetupSuite() {
	log.SetTestLogger(suite.T())
	ctx := suite.T().Context()
	var err error
	// create database
	suite.Database, err = Open(mongoUri, "gorse_")
	suite.NoError(err)
	dbName := "gorse_cache_test"
	databaseComm := suite.getMongoDB()
	suite.NoError(err)
	err = databaseComm.client.Database(dbName).Drop(ctx)
	if err == nil {
		suite.T().Log("delete existed database:", dbName)
	}
	err = suite.Database.Close()
	suite.NoError(err)
	// create schema
	suite.Database, err = Open(mongoUri+dbName+"?authSource=admin&connect=direct", "gorse_")
	suite.NoError(err)
	err = suite.Database.Init()
	suite.NoError(err)
}

func (suite *MongoTestSuite) TestAddEvent() {
	ctx := suite.T().Context()
	db := suite.getMongoDB()
	ts := time.Unix(1700000000, 123456789)
	suite.Require().NoError(db.AddEvent(ctx, "first", "item", ts))
	suite.Require().NoError(db.AddEvent(ctx, "first", "Item", ts))
	suite.Require().NoError(db.AddEvent(ctx, "second", "item", ts))
	suite.Require().NoError(db.AddEvent(ctx, "first", "item", ts.Add(-time.Second)))
	events := db.client.Database(db.dbName).Collection("gorse_events")
	for _, want := range []struct {
		key, member string
		score       int64
	}{{"first", "item", 1699999999123}, {"first", "Item", 1700000000123}, {"second", "item", 1700000000123}} {
		n, err := events.CountDocuments(ctx, bson.M{"_id.key": want.key, "_id.member": want.member, "score": want.score})
		suite.Require().NoError(err)
		suite.EqualValues(1, n)
	}
	suite.Require().NoError(db.Purge())
	n, err := events.CountDocuments(ctx, bson.M{})
	suite.Require().NoError(err)
	suite.Zero(n)
}

func (suite *MongoTestSuite) getMongoDB() *MongoDB {
	var mongoDatabase *MongoDB
	var ok bool
	mongoDatabase, ok = suite.Database.(*MongoDB)
	suite.True(ok)
	return mongoDatabase
}

func TestMongo(t *testing.T) {
	if mongoUri == "" {
		t.Skip("MONGO_URI is not set, skipping MongoDB test")
	}
	suite.Run(t, new(MongoTestSuite))
}

func BenchmarkMongo(b *testing.B) {
	if mongoUri == "" {
		b.Skip("MONGO_URI is not set, skipping MongoDB benchmark")
	}
	log.CloseLogger()
	ctx := b.Context()
	// create database
	database, err := Open(mongoUri, "gorse_")
	assert.NoError(b, err)
	dbName := "gorse_cache_benchmark"
	databaseComm := database.(*MongoDB)
	_ = databaseComm.client.Database(dbName).Drop(ctx)
	err = database.Close()
	assert.NoError(b, err)
	// create schema
	database, err = Open(mongoUri+dbName+"?authSource=admin&connect=direct", "gorse_")
	assert.NoError(b, err)
	err = database.Init()
	assert.NoError(b, err)
	// benchmark
	benchmark(b, database)
}
