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
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/suite"
	"github.com/zhenghaoz/gorse/storage"
)

type RedisTestSuite struct {
	baseTestSuite
	server *miniredis.Miniredis
}

func (suite *RedisTestSuite) SetupSuite() {
	var err error
	suite.server, err = miniredis.Run()
	suite.NoError(err)
	suite.Database, err = Open(storage.RedisPrefix+suite.server.Addr(), "")
	suite.NoError(err)
}

func (suite *RedisTestSuite) TearDownSuite() {
	err := suite.Database.Close()
	suite.NoError(err)
	suite.server.Close()
}

func TestRedis(t *testing.T) {
	suite.Run(t, new(RedisTestSuite))
}
