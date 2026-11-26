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
	"bytes"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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
	cfg.Database.VectorStore = "xvec://" + filepath.Join(tempDir, "vectors")
	cfg.Blob.URI = filepath.Join(tempDir, "blob")
	cfg.Master.Host = "127.0.0.1"
	cfg.Master.Port = 18086
	cfg.Master.HttpHost = "127.0.0.1"
	cfg.Master.HttpPort = 18088
	cfg.Master.AdminAPIKey = "secret"
	cfg.Master.DashboardUserName = "admin"
	cfg.Master.DashboardPassword = "pass"
	cfg.OpenAI.AuthToken = "test"

	suite.master = master.NewMaster(cfg, tempDir, true, "")
	go suite.master.Serve()
	endpoint := fmt.Sprintf("http://%s:%d", cfg.Master.HttpHost, cfg.Master.HttpPort)
	waitForMaster(suite.T(), endpoint)
	suite.client = client.NewAdminClient(endpoint, cfg.Master.AdminAPIKey)
}

func (suite *AdminClientTestSuite) TearDownSuite() {
	suite.master.Shutdown()
}

func (suite *AdminClientTestSuite) TearDownTest() {
	suite.NoError(suite.master.DataClient.Purge())
	suite.NoError(suite.master.CacheClient.Purge())
}

func (suite *AdminClientTestSuite) TestImportExportUsers() {
	input := `{"UserId":"bulk-user","Labels":{"role":"reader"},"Comment":"用户"}
`
	result, err := suite.client.ImportUsers(strings.NewReader(input))
	suite.Require().NoError(err)
	suite.Equal(1, result.RowAffected)
	var output bytes.Buffer
	suite.Require().NoError(suite.client.ExportUsers(&output))
	var user client.User
	suite.Require().NoError(json.Unmarshal(bytes.TrimSpace(output.Bytes()), &user))
	suite.Equal(client.User{UserId: "bulk-user", Labels: map[string]any{"role": "reader"}, Comment: "用户"}, user)
	suite.Require().NoError(suite.master.DataClient.Purge())
	result, err = suite.client.ImportUsers(&output)
	suite.Require().NoError(err)
	suite.Equal(1, result.RowAffected)
	stored, err := suite.master.DataClient.GetUser(suite.T().Context(), "bulk-user")
	suite.Require().NoError(err)
	suite.Equal(user.Labels, stored.Labels)
	suite.Equal(user.Comment, stored.Comment)
}

func (suite *AdminClientTestSuite) TestImportExportItems() {
	input := `{"ItemId":"bulk-item","IsHidden":true,"Categories":["news","tech"],"Timestamp":"2026-01-02T03:04:05Z","Labels":{"embedding":[0.1,0.2]},"Comment":"物品"}
`
	result, err := suite.client.ImportItems(strings.NewReader(input))
	suite.Require().NoError(err)
	suite.Equal(1, result.RowAffected)
	var output bytes.Buffer
	suite.Require().NoError(suite.client.ExportItems(&output))
	var item client.Item
	suite.Require().NoError(json.Unmarshal(bytes.TrimSpace(output.Bytes()), &item))
	suite.Equal(client.Item{ItemId: "bulk-item", IsHidden: true, Categories: []string{"news", "tech"},
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Labels: map[string]any{"embedding": []any{0.1, 0.2}}, Comment: "物品"}, item)
	suite.Require().NoError(suite.master.DataClient.Purge())
	result, err = suite.client.ImportItems(&output)
	suite.Require().NoError(err)
	suite.Equal(1, result.RowAffected)
	stored, err := suite.master.DataClient.GetItem(suite.T().Context(), "bulk-item")
	suite.Require().NoError(err)
	suite.Equal(item.IsHidden, stored.IsHidden)
	suite.Equal(item.Categories, stored.Categories)
	suite.Equal(item.Timestamp, stored.Timestamp)
	suite.Equal(map[string]any{"embedding": []any{0.1, 0.2}}, stored.Labels)
	suite.Equal(item.Comment, stored.Comment)
}

func (suite *AdminClientTestSuite) TestImportExportFeedback() {
	input := `{"FeedbackType":"click","UserId":"bulk-user","ItemId":"bulk-item","Value":0,"Timestamp":"2026-01-02T03:04:05Z","Labels":{"source":"web"},"Comment":"反馈"}
`
	result, err := suite.client.ImportFeedback(strings.NewReader(input))
	suite.Require().NoError(err)
	suite.Equal(1, result.RowAffected)
	var output bytes.Buffer
	suite.Require().NoError(suite.client.ExportFeedback(&output))
	var feedback client.Feedback
	suite.Require().NoError(json.Unmarshal(bytes.TrimSpace(output.Bytes()), &feedback))
	suite.Equal("click", feedback.FeedbackType)
	suite.Equal("bulk-user", feedback.UserId)
	suite.Equal("bulk-item", feedback.ItemId)
	suite.Zero(feedback.Value)
	suite.Equal(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), feedback.Timestamp)
	suite.Equal(map[string]any{"source": "web"}, feedback.Labels)
	suite.Equal("反馈", feedback.Comment)
	suite.Require().NoError(suite.master.DataClient.Purge())
	result, err = suite.client.ImportFeedback(&output)
	suite.Require().NoError(err)
	suite.Equal(1, result.RowAffected)
	stored, err := suite.master.DataClient.GetUserItemFeedback(suite.T().Context(), "bulk-user", "bulk-item")
	suite.Require().NoError(err)
	suite.Require().Len(stored, 1)
	suite.Equal(feedback.Labels, stored[0].Labels)
	suite.Equal(feedback.Comment, stored[0].Comment)
}

func (suite *AdminClientTestSuite) TestGetCategories() {
	suite.Require().NoError(suite.master.CacheClient.AddScores(suite.T().Context(), cache.ItemCategories, "", []cache.Score{
		{Id: "news", Score: 2},
		{Id: "tech", Score: 1},
	}))
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
	suite.Require().NoError(suite.master.DataClient.BatchInsertUsers(suite.T().Context(), []data.User{{UserId: "alice"}}))
	user, err := suite.client.GetUser(suite.T().Context(), "alice")
	suite.NoError(err)
	suite.Equal("alice", user.UserId)
}

func TestRestoreDuration(t *testing.T) {
	for _, wire := range []string{`"1.000000007s"`, `1000000007`} {
		t.Run(wire, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"Users":3,"Duration":%s}`, wire)
			}))
			defer srv.Close()
			stats, err := client.NewAdminClient(srv.URL, "").Restore(strings.NewReader("dump"))
			if wire[0] != '"' {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, time.Second+7, stats.Duration)
			require.Equal(t, 3, stats.Users)
		})
	}
}

func TestAdminClient(t *testing.T) {
	suite.Run(t, new(AdminClientTestSuite))
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
