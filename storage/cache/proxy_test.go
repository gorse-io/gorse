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
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gorse-io/gorse/common/log"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ProxyTestSuite struct {
	baseTestSuite
	sqlite     Database
	server     *ProxyServer
	clientConn *grpc.ClientConn
}

func (suite *ProxyTestSuite) SetupSuite() {
	log.SetTestLogger(suite.T())
	// create database
	var err error
	path := fmt.Sprintf("sqlite://%s/sqlite.db", suite.T().TempDir())
	suite.sqlite, err = Open(path, "gorse_")
	suite.NoError(err)
	// create schema
	err = suite.sqlite.Init()
	suite.NoError(err)
	// start server
	lis, err := net.Listen("tcp", "localhost:0")
	suite.NoError(err)
	suite.server = NewProxyServer(suite.sqlite)
	go func() {
		err = suite.server.Serve(lis)
		suite.NoError(err)
	}()
	// create proxy client
	suite.clientConn, err = grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	suite.NoError(err)
	suite.Database = NewProxyClient(suite.clientConn)
}

func (suite *ProxyTestSuite) TearDownSuite() {
	suite.server.Stop()
	suite.NoError(suite.clientConn.Close())
	suite.NoError(suite.sqlite.Close())
}

func (suite *ProxyTestSuite) SetupTest() {
	log.SetTestLogger(suite.T())
	err := suite.sqlite.Ping()
	suite.NoError(err)
	err = suite.sqlite.Purge()
	suite.NoError(err)
}

func (suite *ProxyTestSuite) TearDownTest() {
	err := suite.sqlite.Purge()
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

func (suite *ProxyTestSuite) TestAddEvent() {
	ctx := suite.T().Context()
	timestamp := time.Unix(1_700_000_000, 123_456_789)
	suite.Require().NoError(suite.Database.AddEvent(ctx, DeleteItems, "item-id", timestamp))

	db := suite.sqlite.(*SQLDatabase)
	var events []SQLEvent
	suite.Require().NoError(db.gormDB.Table(db.EventsTable()).Find(&events).Error)
	suite.Equal([]SQLEvent{{Key: DeleteItems, Member: "item-id", Score: timestamp.UnixMilli()}}, events)

	ctx, cancel := context.WithCancel(ctx)
	cancel()
	suite.Equal(codes.Canceled, status.Code(suite.Database.AddEvent(ctx, DeleteItems, "another-item", timestamp)))
}

func TestProxy(t *testing.T) {
	suite.Run(t, new(ProxyTestSuite))
}
