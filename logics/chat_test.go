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
	"testing"

	"github.com/gorse-io/gorse/common/mock"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/assert"
)

func TestChatRankerhat(t *testing.T) {
	mockAI := mock.NewOpenAIServer()
	go func() {
		_ = mockAI.Start()
	}()
	mockAI.Ready()
	defer mockAI.Close()

	ranker, err := NewChatRanker(config.OpenAIConfig{
		BaseURL:             mockAI.BaseURL(),
		AuthToken:           mockAI.AuthToken(),
		ChatCompletionModel: "deepseek-r1",
		EmbeddingModel:      "text-similarity-ada-001",
	}, `{{ user.UserId }} is a {{ user.Comment }} watched the following movies recently:
{% for item in feedback -%}
- {{ item.Comment }}
{% endfor -%}
Please sort the following movies based on his or her preference:
| ID | Title |
{% for item in items -%}
| {{ item.ItemId }} | {{ item.Comment }} |
{% endfor -%}
Return IDs as a JSON array. For example:
`+"```json\n"+`["tt1233227", "tt0926084", "tt0890870", "tt1132626", "tt0435761"]`+"\n```")
	assert.NoError(t, err)
	items, err := ranker.Rank(&data.User{
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
	assert.Equal(t, []string{"tt1233227", "tt0926084", "tt0890870", "tt1132626", "tt0435761"}, items)
}
