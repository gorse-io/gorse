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
		suite.Equal("/api/dashboard/categories", r.URL.Path)
		suite.Equal("secret", r.Header.Get("X-Api-Key"))
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`["news","tech"]`))
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

func TestAdminClient(t *testing.T) {
	suite.Run(t, new(AdminClientTestSuite))
}
