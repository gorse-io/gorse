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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorse-io/gorse/client"
	"github.com/stretchr/testify/suite"
)

type AdminClientTestSuite struct {
	suite.Suite
	client *client.AdminClient
	server *httptest.Server
}

func (suite *AdminClientTestSuite) SetupSuite() {
	suite.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.Equal("secret", r.Header.Get("X-Api-Key"))
		w.Header().Set("Content-Type", "application/json")
		var response string
		switch r.URL.Path {
		case "/api/dashboard/categories":
			response = `["news","tech"]`
		case "/api/dashboard/timeseries/requests":
			suite.Equal("2026-01-01T00:00:00Z", r.URL.Query().Get("begin"))
			suite.Equal("2026-01-02T00:00:00Z", r.URL.Query().Get("end"))
			suite.Equal("1h", r.URL.Query().Get("duration"))
			response = `[{"Name":"requests","Timestamp":"2026-01-01T01:00:00Z","Value":1}]`
		case "/api/user/alice":
			response = `{"UserId":"alice"}`
		default:
			suite.Fail("unexpected request path", r.URL.Path)
		}
		_, err := w.Write([]byte(response))
		suite.NoError(err)
	}))
	suite.client = client.NewAdminClient(suite.server.URL, "secret")
}

func (suite *AdminClientTestSuite) TearDownSuite() {
	suite.server.Close()
}

func (suite *AdminClientTestSuite) TestGetCategories() {
	categories, err := suite.client.GetCategories()
	suite.NoError(err)
	suite.Equal([]string{"news", "tech"}, categories)
}

func (suite *AdminClientTestSuite) TestGetTimeseries() {
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
