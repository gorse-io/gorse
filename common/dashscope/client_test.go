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

package dashscope

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient_Rerank(t *testing.T) {
	s := NewMockServer()
	go func() {
		err := s.Start()
		assert.ErrorIs(t, err, http.ErrServerClosed)
	}()
	s.Ready()
	defer s.Close()

	client := NewClient(s.APIKey())
	client.SetBaseURL(s.URL())

	req := RerankRequest{
		Model: "gte-rerank",
		Input: Input{
			Query: "What is the capital of France?",
			Documents: []string{
				"Paris is the capital of France.",
				"Lyon is a city in France.",
			},
		},
	}

	resp, err := client.Rerank(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(resp.Output.Results))
	assert.Equal(t, 0, resp.Output.Results[0].Index)
	assert.Equal(t, 1.0, resp.Output.Results[0].RelevanceScore)
	assert.Equal(t, 1, resp.Output.Results[1].Index)
	assert.Equal(t, 0.5, resp.Output.Results[1].RelevanceScore)
}
