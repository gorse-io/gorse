// Copyright 2024 gorse Project Authors
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

package meta

import (
	"github.com/stretchr/testify/suite"
	"time"
)

type baseTestSuite struct {
	suite.Suite
	Database
}

func (suite *baseTestSuite) TestNodes() {
	// Add node
	err := suite.Database.UpdateNode(&Node{
		UUID:       "node-1",
		Hostname:   "localhost",
		Type:       "master",
		Version:    "v0.1.0",
		UpdateTime: time.Now(),
	})
	suite.NoError(err)
	// Add duplicate node
	err = suite.Database.UpdateNode(&Node{
		UUID:       "node-1",
		Hostname:   "localhost",
		Type:       "master",
		Version:    "v0.1.1",
		UpdateTime: time.Now(),
	})
	suite.NoError(err)
	// Add outdated node
	err = suite.Database.UpdateNode(&Node{
		UUID:       "node-2",
		Hostname:   "localhost",
		Type:       "master",
		Version:    "v0.1.0",
		UpdateTime: time.Now().Add(-time.Hour),
	})
	suite.NoError(err)
	// List nodes
	nodes, err := suite.Database.ListNodes()
	suite.NoError(err)
	if suite.Equal(1, len(nodes)) {
		suite.Equal("node-1", nodes[0].UUID)
		suite.Equal("localhost", nodes[0].Hostname)
		suite.Equal("master", nodes[0].Type)
		suite.Equal("v0.1.1", nodes[0].Version)
	}
}

func (suite *baseTestSuite) TestKeyValues() {
	err := suite.Database.Put("key1", "value1")
	suite.NoError(err)
	err = suite.Database.Put("key2", "value2")
	suite.NoError(err)
	err = suite.Database.Put("key3", "value3")
	suite.NoError(err)

	value, err := suite.Database.Get("key1")
	suite.NoError(err)
	suite.Equal("value1", *value)

	value, err = suite.Database.Get("key2")
	suite.NoError(err)
	suite.Equal("value2", *value)

	value, err = suite.Database.Get("key3")
	suite.NoError(err)
	suite.Equal("value3", *value)

	// Test non-existing key
	value, err = suite.Database.Get("non-existing-key")
	suite.NoError(err)
	suite.Nil(value)
}
