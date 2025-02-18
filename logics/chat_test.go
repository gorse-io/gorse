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
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base/floats"
	"github.com/zhenghaoz/gorse/common/mock"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/data"
)

func TestChat(t *testing.T) {
	mockAI := mock.NewOpenAIServer()
	go func() {
		_ = mockAI.Start()
	}()
	mockAI.Ready()
	defer mockAI.Close()

	chat, err := NewChat(config.ChatConfig{
		Column: "item.Labels.embeddings",
		Prompt: `You are a ecommerce recommender system. I have purchased:
{%- for item in items %}
	{%- if loop.index0 == 0 %}
		{{- ' ' + item.ItemId -}}
	{%- else %}
		{{- ', ' + item.ItemId -}}
	{%- endif %}
{%- endfor %}. Please recommend me more items.
`,
	}, 10, time.Now(), config.OpenAIConfig{
		BaseURL:             mockAI.BaseURL(),
		AuthToken:           mockAI.AuthToken(),
		ChatCompletionModel: "deepseek-r1",
		EmbeddingsModel:     "text-similarity-ada-001",
	})
	assert.NoError(t, err)

	for i := 0; i < 100; i++ {
		embedding := mock.Hash("You are a ecommerce recommender system. I have purchased: 3, 1, 5. Please recommend me more items.")
		floats.AddConst(embedding, float32(i))
		chat.Push(&data.Item{
			ItemId: strconv.Itoa(i),
			Labels: map[string]any{
				"embeddings": embedding,
			},
		}, nil)
	}
	assert.Len(t, chat.Items(), 100)

	scores := chat.PopAll([]int{3, 1, 5})
	assert.Len(t, scores, 10)
	for i := 0; i < 10; i++ {
		assert.Equal(t, strconv.Itoa(i), scores[i].Id)
	}
}

func TestParseMessage(t *testing.T) {
	// parse JSON object
	message := "```json\n{\"a\": 1, \"b\": 2}\n```"
	contents := parseMessage(message)
	assert.Equal(t, []string{"{\"a\": 1, \"b\": 2}\n"}, contents)

	// parse JSON array
	message = "```json\n[1, 2]\n```"
	contents = parseMessage(message)
	assert.Equal(t, []string{"1", "2"}, contents)

	// parse text
	message = "Hello, world!"
	contents = parseMessage(message)
	assert.Equal(t, []string{"Hello, world!"}, contents)

	// strip think
	message = "<think>hello</think>World!"
	content := stripThink(message)
	assert.Equal(t, "World!", content)
}
