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
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/gorse-io/gorse/common/log"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
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

func (suite *ProxyTestSuite) TestBinaryValues() {
	ctx := context.Background()
	for _, size := range []int{0, 256, 10 * 1024 * 1024} {
		suite.Run(fmt.Sprintf("%d_bytes", size), func() {
			value := make([]byte, size)
			for i := range value {
				value[i] = byte(i)
			}
			name := fmt.Sprintf("binary_%d", size)
			suite.Require().NoError(suite.Database.Set(ctx, String(name, string(value))))
			result := suite.Database.Get(ctx, name)
			suite.Require().True(result.Exists())
			actual, err := result.String()
			suite.Require().NoError(err)
			suite.Equal(value, []byte(actual))
			suite.Require().NoError(suite.Database.Delete(ctx, name))
			suite.False(suite.Database.Get(ctx, name).Exists())
		})
	}
}

func (suite *ProxyTestSuite) TestLargeValueWrite() {
	ctx := context.Background()
	value := string(bytes.Repeat([]byte("x"), 10*1024*1024))
	suite.Require().NoError(suite.Database.Set(ctx, String("large_write", value)))
	actual, err := suite.sqlite.Get(ctx, "large_write").String()
	suite.Require().NoError(err)
	suite.Equal(value, actual)
}

func (suite *ProxyTestSuite) TestLargeValueRead() {
	ctx := context.Background()
	value := string(bytes.Repeat([]byte("x"), 10*1024*1024))
	suite.Require().NoError(suite.sqlite.Set(ctx, String("large_read", value)))
	actual, err := suite.Database.Get(ctx, "large_read").String()
	suite.Require().NoError(err)
	suite.Equal(value, actual)
}

func TestProxy(t *testing.T) {
	suite.Run(t, new(ProxyTestSuite))
}
