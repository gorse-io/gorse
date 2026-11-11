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

func (suite *MongoTestSuite) getMongoDB() *MongoDB {
	var mongoDatabase *MongoDB
	var ok bool
	mongoDatabase, ok = suite.Database.(*MongoDB)
	suite.True(ok)
	return mongoDatabase
}

func (suite *MongoTestSuite) TestBinaryStorage() {
	ctx := suite.T().Context()
	collection := suite.getMongoDB().client.Database(suite.getMongoDB().dbName).Collection(suite.getMongoDB().ValuesTable())
	payload := make([]byte, 256)
	for i := range payload {
		payload[i] = byte(i)
	}
	for _, value := range []string{"", "text value", string(payload)} {
		suite.Require().NoError(suite.Database.Set(ctx, String("binary", value)))
		raw, err := collection.FindOne(ctx, bson.M{"_id": "binary"}).DecodeBytes()
		suite.Require().NoError(err)
		subtype, data, ok := raw.Lookup("value").BinaryOK()
		suite.Require().True(ok, "cache values must be stored as BSON binary")
		suite.Equal(byte(0), subtype)
		suite.Equal([]byte(value), data)
		result := suite.Database.Get(ctx, "binary")
		suite.True(result.Exists())
		got, err := result.String()
		suite.Require().NoError(err)
		suite.Equal(value, got)
	}
}

func (suite *MongoTestSuite) TestLegacyValue() {
	ctx := suite.T().Context()
	database := suite.getMongoDB()
	collection := database.client.Database(database.dbName).Collection(database.ValuesTable())
	_, err := collection.InsertOne(ctx, bson.M{"_id": "legacy", "value": "old text"})
	suite.Require().NoError(err)
	result := suite.Database.Get(ctx, "legacy")
	suite.True(result.Exists())
	value, err := result.String()
	suite.Require().NoError(err)
	suite.Equal("old text", value)

	// Replacing a legacy text value stores the new value as BSON binary.
	suite.Require().NoError(suite.Database.Set(ctx, String("legacy", "new text")))
	raw, err := collection.FindOne(ctx, bson.M{"_id": "legacy"}).DecodeBytes()
	suite.Require().NoError(err)
	_, data, ok := raw.Lookup("value").BinaryOK()
	suite.Require().True(ok)
	suite.Equal([]byte("new text"), data)
	value, err = suite.Database.Get(ctx, "legacy").String()
	suite.Require().NoError(err)
	suite.Equal("new text", value)
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
