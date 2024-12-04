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

package cache

import (
	"fmt"
	"github.com/stretchr/testify/suite"
	"net"
	"testing"
)

type ProxyTestSuite struct {
	baseTestSuite
	SQLite Database
	Server *ProxyServer
}

func (suite *ProxyTestSuite) SetupSuite() {
	// create database
	var err error
	path := fmt.Sprintf("sqlite://%s/sqlite.db", suite.T().TempDir())
	suite.SQLite, err = Open(path, "gorse_")
	suite.NoError(err)
	// create schema
	err = suite.SQLite.Init()
	suite.NoError(err)
	// start server
	lis, err := net.Listen("tcp", "localhost:0")
	suite.NoError(err)
	suite.Server = NewProxyServer(suite.SQLite)
	go func() {
		err = suite.Server.Serve(lis)
		suite.NoError(err)
	}()
	// create proxy
	suite.Database, err = OpenProxyClient(lis.Addr().String())
	suite.NoError(err)
}

func (suite *ProxyTestSuite) TearDownSuite() {
	suite.Server.Stop()
	suite.NoError(suite.Database.Close())
	suite.NoError(suite.SQLite.Close())
}

func (suite *ProxyTestSuite) SetupTest() {
	err := suite.SQLite.Ping()
	suite.NoError(err)
	err = suite.SQLite.Purge()
	suite.NoError(err)
}

func (suite *ProxyTestSuite) TearDownTest() {
	err := suite.SQLite.Purge()
	suite.NoError(err)
}

func (suite *ProxyTestSuite) TestInit() {
	suite.T().Skip()
}

func (suite *ProxyTestSuite) TestPurge() {
	suite.T().Skip()
}

func (suite *ProxyTestSuite) TestScan() {
	suite.T().Skip()
}

func TestProxy(t *testing.T) {
	suite.Run(t, new(ProxyTestSuite))
}
