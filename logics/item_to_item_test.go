// Copyright 2024 gorse Project Authors
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

	"github.com/gorse-io/gorse/common/floats"
	"github.com/gorse-io/gorse/common/mock"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ItemToItemTestSuite struct {
	suite.Suite
}

func (suite *ItemToItemTestSuite) TestColumnFunc() {
	item2item, err := newEmbeddingItemToItem(config.ItemToItemConfig{
		Column: "item.Labels.description",
	}, 10, time.Now())
	suite.NoError(err)

	// Push success
	item2item.Push(&data.Item{
		ItemId: "1",
		Labels: map[string]any{
			"description": []float32{0.1, 0.2, 0.3},
		},
	}, nil)
	suite.Len(item2item.Items(), 1)

	// Hidden
	item2item.Push(&data.Item{
		ItemId:   "2",
		IsHidden: true,
		Labels: map[string]any{
			"description": []float32{0.1, 0.2, 0.3},
		},
	}, nil)
	suite.Len(item2item.Items(), 1)

	// Dimension does not match
	item2item.Push(&data.Item{
		ItemId: "1",
		Labels: map[string]any{
			"description": []float32{0.1, 0.2},
		},
	}, nil)
	suite.Len(item2item.Items(), 1)

	// Type does not match
	item2item.Push(&data.Item{
		ItemId: "1",
		Labels: map[string]any{
			"description": "hello",
		},
	}, nil)
	suite.Len(item2item.Items(), 1)

	// Column does not exist
	item2item.Push(&data.Item{
		ItemId: "2",
		Labels: []float32{0.1, 0.2, 0.3},
	}, nil)
	suite.Len(item2item.Items(), 1)
}

func (suite *ItemToItemTestSuite) TestEmbedding() {
	timestamp := time.Now()
	item2item, err := newEmbeddingItemToItem(config.ItemToItemConfig{
		Column: "item.Labels.description",
	}, 10, timestamp)
	suite.NoError(err)
	suite.IsType(item2item.Pool(), &parallel.SequentialPool{})

	for i := 0; i < 100; i++ {
		item2item.Push(&data.Item{
			ItemId: strconv.Itoa(i),
			Labels: map[string]any{
				"description": []float32{0.1 * float32(i), 0.2 * float32(i), 0.3 * float32(i)},
			},
		}, nil)
	}

	scores := item2item.PopAll(0)
	suite.Len(scores, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i), scores[i-1].Id)
	}
}

func (suite *ItemToItemTestSuite) TestTags() {
	timestamp := time.Now()
	idf := make([]float32, 101)
	for i := range idf {
		idf[i] = 1
	}
	item2item, err := newTagsItemToItem(config.ItemToItemConfig{
		Column: "item.Labels",
	}, 10, timestamp, idf)
	suite.NoError(err)
	suite.IsType(item2item.Pool(), &parallel.SequentialPool{})

	for i := 0; i < 100; i++ {
		labels := make(map[string]any)
		for j := 1; j <= 100-i; j++ {
			labels[strconv.Itoa(j)] = []dataset.ID{dataset.ID(j)}
		}
		item2item.Push(&data.Item{
			ItemId: strconv.Itoa(i),
			Labels: labels,
		}, nil)
	}

	scores := item2item.PopAll(0)
	suite.Len(scores, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i), scores[i-1].Id)
	}
}

func (suite *ItemToItemTestSuite) TestUsers() {
	timestamp := time.Now()
	idf := make([]float32, 101)
	for i := range idf {
		idf[i] = 1
	}
	item2item, err := newUsersItemToItem(config.ItemToItemConfig{}, 10, timestamp, idf)
	suite.NoError(err)
	suite.IsType(item2item.Pool(), &parallel.SequentialPool{})

	for i := 0; i < 100; i++ {
		feedback := make([]int32, 0, 100-i)
		for j := 1; j <= 100-i; j++ {
			feedback = append(feedback, int32(j))
		}
		item2item.Push(&data.Item{ItemId: strconv.Itoa(i)}, feedback)
	}

	scores := item2item.PopAll(0)
	suite.Len(scores, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i), scores[i-1].Id)
	}
}

func (suite *ItemToItemTestSuite) TestAuto() {
	timestamp := time.Now()
	idf := make([]float32, 101)
	for i := range idf {
		idf[i] = 1
	}
	item2item, err := newAutoItemToItem(config.ItemToItemConfig{}, 10, timestamp, idf, idf)
	suite.NoError(err)
	suite.IsType(item2item.Pool(), &parallel.SequentialPool{})

	for i := 0; i < 100; i++ {
		item := &data.Item{ItemId: strconv.Itoa(i)}
		feedback := make([]int32, 0, 100-i)
		if i%2 == 0 {
			labels := make(map[string]any)
			for j := 1; j <= 100-i; j++ {
				labels[strconv.Itoa(j)] = []dataset.ID{dataset.ID(j)}
			}
			item.Labels = labels
		} else {
			for j := 1; j <= 100-i; j++ {
				feedback = append(feedback, int32(j))
			}
		}
		item2item.Push(item, feedback)
	}

	scores0 := item2item.PopAll(0)
	suite.Len(scores0, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i*2), scores0[i-1].Id)
	}
	scores1 := item2item.PopAll(1)
	suite.Len(scores1, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i*2+1), scores1[i-1].Id)
	}
}

func (suite *ItemToItemTestSuite) TestChat() {
	mockAI := mock.NewOpenAIServer()
	go func() {
		_ = mockAI.Start()
	}()
	mockAI.Ready()
	defer mockAI.Close()

	timestamp := time.Now()
	item2item, err := newChatItemToItem(config.ItemToItemConfig{
		Column: "item.Labels.embeddings",
		Prompt: "Please generate similar items for {{ item.Labels.title }}.",
	}, 10, timestamp, config.OpenAIConfig{
		BaseURL:             mockAI.BaseURL(),
		AuthToken:           mockAI.AuthToken(),
		ChatCompletionModel: "deepseek-r1",
		EmbeddingModel:      "text-similarity-ada-001",
	})
	suite.NoError(err)
	suite.IsType(item2item.Pool(), &parallel.ConcurrentPool{})

	for i := 0; i < 100; i++ {
		embedding := mock.Hash("Please generate similar items for item_0.")
		floats.AddConst(embedding, float32(i+1))
		item2item.Push(&data.Item{
			ItemId: strconv.Itoa(i),
			Labels: map[string]any{
				"title":      "item_" + strconv.Itoa(i),
				"embeddings": embedding,
			},
		}, nil)
	}

	scores := item2item.PopAll(0)
	suite.Len(scores, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i), scores[i-1].Id)
	}
}

func TestItemToItem(t *testing.T) {
	suite.Run(t, new(ItemToItemTestSuite))
}

func TestParseJSONArrayFromCompletion(t *testing.T) {
	// parse JSON object
	completion := "```json\n{\"a\": 1, \"b\": 2}\n```"
	parsed := parseJSONArrayFromCompletion(completion)
	assert.Equal(t, []string{"{\"a\": 1, \"b\": 2}\n"}, parsed)

	// parse JSON array
	completion = "```json\n[1, 2]\n```"
	parsed = parseJSONArrayFromCompletion(completion)
	assert.Equal(t, []string{"1", "2"}, parsed)

	// parse text
	completion = "Hello, world!"
	parsed = parseJSONArrayFromCompletion(completion)
	assert.Equal(t, []string{"Hello, world!"}, parsed)

	// strip think
	completion = "<think>hello</think>World!"
	assert.Equal(t, "World!", stripThinkInCompletion(completion))
}
