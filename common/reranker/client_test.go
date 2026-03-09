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

package reranker

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ClientTestSuite struct {
	suite.Suite
	s *MockServer
}

func (suite *ClientTestSuite) SetupSuite() {
	suite.s = NewMockServer()
	go func() {
		err := suite.s.Start()
		suite.ErrorIs(err, http.ErrServerClosed)
	}()
	suite.s.Ready()
}

func (suite *ClientTestSuite) TearDownSuite() {
	suite.s.Close()
}

func (suite *ClientTestSuite) TestRerank() {
	client := NewClient(suite.s.AuthToken(), suite.s.URL())

	req := RerankRequest{
		Model: "jina-reranker-v2-base-multilingual",
		Query: "What is the capital of France?",
		Documents: []string{
			"Paris is the capital of France.",
			"Lyon is a city in France.",
		},
	}

	resp, err := client.Rerank(suite.T().Context(), req)
	suite.NoError(err)
	suite.Equal(2, len(resp.Results))
	suite.Equal(0, resp.Results[0].Index)
	suite.Equal(1.0, resp.Results[0].RelevanceScore)
	suite.Equal(1, resp.Results[1].Index)
	suite.Equal(0.5, resp.Results[1].RelevanceScore)
}

func TestClient(t *testing.T) {
	suite.Run(t, new(ClientTestSuite))
}
