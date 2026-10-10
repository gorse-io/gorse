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

package logics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/require"
)

func TestDecisionReranker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		var request struct {
			Model     string `json:"model"`
			State     string `json:"state"`
			Questions map[string]struct {
				Type         string   `json:"type"`
				Instructions string   `json:"instructions"`
				Criteria     []string `json:"criteria"`
			} `json:"questions"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, "test-model", request.Model)
		require.Equal(t, "u1: history", request.State)
		require.Len(t, request.Questions, 2)
		require.Equal(t, "score", request.Questions["a"].Type)
		require.Contains(t, request.Questions["a"].Instructions, "first")
		require.Len(t, request.Questions["a"].Criteria, 5)
		_, _ = w.Write([]byte(`{"answers":{"a":{"type":"score","score":1.5,"probabilities":{},"legend":{},"confidence":0.9},"b":{"type":"score","score":3.5,"probabilities":{},"legend":{},"confidence":0.9}}}`))
	}))
	defer server.Close()
	ranker, err := NewDecisionReranker(config.DecisionAPIConfig{AuthToken: "test-key", URL: server.URL, Model: "test-model"}, "{{user.UserId}}: {% for f in feedback %}{{f.ItemId}}{% endfor %}", "{{item.Comment}}")
	require.NoError(t, err)
	scores, err := ranker.Rank(t.Context(), &data.User{UserId: "u1"}, []*FeedbackItem{{Item: data.Item{ItemId: "history"}}}, []*data.Item{{ItemId: "a", Comment: "first"}, {ItemId: "b", Comment: "second"}})
	require.NoError(t, err)
	require.Equal(t, []cache.Score{{Id: "b", Score: 3.5}, {Id: "a", Score: 1.5}}, scores)
	scores, err = ranker.Rank(t.Context(), nil, nil, nil)
	require.NoError(t, err)
	require.Empty(t, scores)
}

func TestDecisionRerankerInvalidAnswers(t *testing.T) {
	for _, body := range []string{`{"answers":{}}`, `{"answers":{"a":{"type":"noul","noul":0.5}}}`, `{"answers":{"a":{"type":"score","score":5,"probabilities":{},"legend":{},"confidence":1}}}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			ranker, err := NewDecisionReranker(config.DecisionAPIConfig{URL: server.URL}, "", "{{item.ItemId}}")
			require.NoError(t, err)
			_, err = ranker.Rank(t.Context(), &data.User{}, nil, []*data.Item{{ItemId: "a"}})
			require.Error(t, err)
		})
	}
}
