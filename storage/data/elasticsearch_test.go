// Copyright 2021 gorse Project Authors
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
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

var (
	elasticUri string
)

func init() {
	// get environment variables
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	elasticUri = env("ELASTIC_URI", "elastic://elastic:password@127.0.0.1:9200/")
}

type ElasticTestSuite struct {
	baseTestSuite
}

func (suite *ElasticTestSuite) SetupSuite() {
	ctx := suite.T().Context()
	var err error
	// create database
	suite.Database, err = Open(elasticUri, "gorse_")
	suite.NoError(err)
	err = suite.Database.Init()
	suite.NoError(err)
}

func (suite *ElasticTestSuite) getElasticDB() *Elasticsearch {
	var elasticDatabase *Elasticsearch
	var ok bool
	elasticDatabase, ok = suite.Database.(*Elasticsearch)
	suite.True(ok)
	return elasticDatabase
}

func TestElastic(t *testing.T) {
	suite.Run(t, new(ElasticTestSuite))
}