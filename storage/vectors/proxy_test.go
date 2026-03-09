// Copyright 2026 gorse Project Authors
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

package vectors

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
)

type ProxyTestSuite struct {
	vectorsTestSuite
	sqlite     Database
	server     *ProxyServer
	clientConn *grpc.ClientConn
}

func (suite *ProxyTestSuite) SetupSuite() {
	var err error
	path := fmt.Sprintf("sqlite://%s/sqlite.db", suite.T().TempDir())
	suite.sqlite, err = Open(path, "gorse_")
	suite.NoError(err)
	lis, err := net.Listen("tcp", "localhost:0")
	suite.NoError(err)
	suite.server = NewProxyServer(suite.sqlite)
	go func() {
		err = suite.server.Serve(lis)
		suite.NoError(err)
	}()
	suite.clientConn, err = grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	suite.NoError(err)
	suite.Database = NewProxyClient(suite.clientConn)
}

func (suite *ProxyTestSuite) TearDownSuite() {
	suite.server.Stop()
	suite.NoError(suite.clientConn.Close())
	suite.NoError(suite.sqlite.Close())
}

func TestProxy(t *testing.T) {
	suite.Run(t, new(ProxyTestSuite))
}
