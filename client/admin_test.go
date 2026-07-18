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

package client_test

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorse-io/gorse/client"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/master"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AdminClientTestSuite struct {
	suite.Suite
	client *client.AdminClient
	master *master.Master
}

func (suite *AdminClientTestSuite) SetupSuite() {
	log.SetTestLogger(suite.T())
	tempDir := suite.T().TempDir()
	cfg := config.GetDefaultConfig()
	cfg.Database.DataStore = "sqlite://" + filepath.Join(tempDir, "data.db")
	cfg.Database.CacheStore = "sqlite://" + filepath.Join(tempDir, "cache.db")
	cfg.Blob.URI = filepath.Join(tempDir, "blob")
	cfg.Master.Host = "127.0.0.1"
	cfg.Master.Port = freePort(suite.T())
	cfg.Master.HttpHost = "127.0.0.1"
	cfg.Master.HttpPort = freePort(suite.T())
	cfg.Master.AdminAPIKey = "secret"
	cfg.OpenAI.AuthToken = "test"

	suite.master = master.NewMaster(cfg, tempDir, true, "")
	go suite.master.Serve()
	endpoint := fmt.Sprintf("http://%s:%d", cfg.Master.HttpHost, cfg.Master.HttpPort)
	waitForMaster(suite.T(), endpoint)
	suite.client = client.NewAdminClient(endpoint, cfg.Master.AdminAPIKey)

	ctx := suite.T().Context()
	suite.Require().NoError(suite.master.DataClient.BatchInsertUsers(ctx, []data.User{{UserId: "alice"}}))
	suite.Require().NoError(suite.master.CacheClient.AddScores(ctx, cache.ItemCategories, "", []cache.Score{
		{Id: "news", Score: 2},
		{Id: "tech", Score: 1},
	}))
}

func (suite *AdminClientTestSuite) TearDownSuite() {
	suite.master.Shutdown()
}

func (suite *AdminClientTestSuite) TestGetCategories() {
	categories, err := suite.client.GetCategories()
	suite.NoError(err)
	suite.Equal([]string{"news", "tech"}, categories)
}

func (suite *AdminClientTestSuite) TestGetTimeseries() {
	suite.Require().NoError(suite.master.CacheClient.AddTimeSeriesPoints(suite.T().Context(), []cache.TimeSeriesPoint{{
		Name:      "requests",
		Timestamp: time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC),
		Value:     1,
	}}))
	points, err := suite.client.GetTimeseries("requests", "2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z", "1h")
	suite.NoError(err)
	suite.Equal("requests", points[0].Name)
	suite.Equal(time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), points[0].Timestamp)
	suite.Equal(float64(1), points[0].Value)
}

func (suite *AdminClientTestSuite) TestGetUser() {
	user, err := suite.client.GetUser(suite.T().Context(), "alice")
	suite.NoError(err)
	suite.Equal("alice", user.UserId)
}

func TestAdminClient(t *testing.T) {
	suite.Run(t, new(AdminClientTestSuite))
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitForMaster(t *testing.T, endpoint string) {
	t.Helper()
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(endpoint + "/api/health/live")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.Fail(t, "master didn't become ready")
}
