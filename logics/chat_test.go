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
	"net/http"
	"testing"

	"github.com/gorse-io/gorse/common/reranker"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/assert"
)

func TestChatReranker(t *testing.T) {
	s := reranker.NewMockServer()
	go func() {
		err := s.Start()
		if err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
	s.Ready()
	defer s.Close()

	reranker, err := NewChatReranker(config.RerankerAPIConfig{
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
		{Item: data.Item{ItemId: "tt0387564", Comment: "Saw"}},
		{Item: data.Item{ItemId: "tt0432348", Comment: "Saw II"}},
		{Item: data.Item{ItemId: "tt0435761", Comment: "Saw III"}},
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

func TestParseArrayFromCompletion(t *testing.T) {
	// parse JSON object
	completion := "```json\n{\"a\": 1, \"b\": 2}\n```"
	parsed := parseArrayFromCompletion(completion)
	assert.Equal(t, []string{"{\"a\": 1, \"b\": 2}\n"}, parsed)

	// parse JSON array
	completion = "```json\n[1, 2]\n```"
	parsed = parseArrayFromCompletion(completion)
	assert.Equal(t, []string{"1", "2"}, parsed)

	// parse CSV
	completion = "```csv\n1\n2\n3\n```"
	parsed = parseArrayFromCompletion(completion)
	assert.Equal(t, []string{"1", "2", "3"}, parsed)

	// parse text
	completion = "Hello, world!\nThis is a test."
	parsed = parseArrayFromCompletion(completion)
	assert.Equal(t, []string{"Hello, world!", "This is a test."}, parsed)

	// strip think
	completion = "<think>hello</think>World!"
	assert.Equal(t, "World!", stripThinkInCompletion(completion))
}
