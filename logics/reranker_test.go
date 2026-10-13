// Copyright 2025 gorse Project Authors
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

	"github.com/gorse-io/gorse/common/reranker"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReranker(t *testing.T) {
	s := reranker.NewMockServer()
	go func() {
		err := s.Start()
		if err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
	s.Ready()
	defer s.Close()

	reranker, err := NewReranker(config.RerankerAPIConfig{
		AuthToken: s.AuthToken(),
		URL:       s.URL(),
		Model:     "gte-rerank",
	},
		"{{ user.UserId }} is a {{ user.Comment }} watched the following movies recently: {% for item in feedback %}{{ item.Comment }}, {% endfor %}",
		"{{ item.Comment }}")
	assert.NoError(t, err)

	items, err := reranker.Rank(t.Context(), &data.User{
		UserId:  "Tom",
		Comment: "horror movie enthusiast",
	}, []*FeedbackItem{
		{ItemId: "tt0387564", Comment: "Saw"},
		{ItemId: "tt0432348", Comment: "Saw II"},
		{ItemId: "tt0435761", Comment: "Saw III"},
	}, []*data.Item{
		{ItemId: "tt1233227", Comment: "Harry Potter and the Half-Blood Prince"},
		{ItemId: "tt0926084", Comment: "Harry Potter and the Deathly Hallows: Part 1"},
		{ItemId: "tt0890870", Comment: "Saw IV"},
		{ItemId: "tt1132626", Comment: "Saw VI"},
		{ItemId: "tt0435761", Comment: "Saw V"},
	})
	assert.NoError(t, err)
	assert.Equal(t, []cache.Score{
		{Id: "tt1233227", Score: 1},
		{Id: "tt0926084", Score: 0.5},
		{Id: "tt0890870", Score: 0.3333333333333333},
		{Id: "tt1132626", Score: 0.25},
		{Id: "tt0435761", Score: 0.2},
	}, items)
}

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
