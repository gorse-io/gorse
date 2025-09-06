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
	"context"
	"os"
	"testing"

	"github.com/gorse-io/gorse/base/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
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

type MongoTestSuite struct {
	baseTestSuite
}

func (suite *MongoTestSuite) SetupSuite() {
	ctx := context.Background()
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

func TestMongo(t *testing.T) {
	suite.Run(t, new(MongoTestSuite))
}

func BenchmarkMongo(b *testing.B) {
	log.CloseLogger()
	ctx := context.Background()
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
