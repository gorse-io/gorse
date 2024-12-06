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
	"fmt"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type SQLiteTestSuite struct {
	baseTestSuite
}

func (suite *SQLiteTestSuite) SetupTest() {
	var err error
	// create database
	path := fmt.Sprintf("sqlite://%s/sqlite.db", suite.T().TempDir())
	suite.Database, err = Open(path, time.Second)
	suite.NoError(err)
	// create schema
	err = suite.Database.Init()
	suite.NoError(err)
}

func (suite *SQLiteTestSuite) TearDownTest() {
	suite.NoError(suite.Database.Close())
}

func TestSQLite(t *testing.T) {
	suite.Run(t, new(SQLiteTestSuite))
}
