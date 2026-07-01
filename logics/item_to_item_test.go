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
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gorse-io/gorse/common/floats"
	"github.com/gorse-io/gorse/common/mock"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/samber/lo"
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
	suite.Equal(1, item2item.Count())

	// Hidden
	item2item.Push(&data.Item{
		ItemId:   "2",
		IsHidden: true,
		Labels: map[string]any{
			"description": []float32{0.1, 0.2, 0.3},
		},
	}, nil)
	suite.Equal(2, item2item.Count())

	// Dimension does not match
	item2item.Push(&data.Item{
		ItemId: "1",
		Labels: map[string]any{
			"description": []float32{0.1, 0.2},
		},
	}, nil)
	suite.Equal(2, item2item.Count())

	// Type does not match
	item2item.Push(&data.Item{
		ItemId: "1",
		Labels: map[string]any{
			"description": "hello",
		},
	}, nil)
	suite.Equal(2, item2item.Count())

	// Column does not exist
	item2item.Push(&data.Item{
		ItemId: "2",
		Labels: []float32{0.1, 0.2, 0.3},
	}, nil)
	suite.Equal(2, item2item.Count())
}

func (suite *ItemToItemTestSuite) TestEmbeddingItemToItemVectorWriter() {
	ctx := suite.T().Context()
	vectorClient, err := vectors.Open(fmt.Sprintf("sqlite://%s/vector.db", suite.T().TempDir()), "")
	suite.NoError(err)
	suite.NoError(vectorClient.Init())
	defer func() {
		suite.NoError(vectorClient.Close())
	}()

	timestamp := time.Now()
	writer, err := NewEmbeddingItemToItemVectorWriter(ctx, config.ItemToItemConfig{
		Name:   "embedding",
		Type:   "embedding",
		Column: "item.Labels.embedding",
	}, timestamp, vectorClient, vectors.VectorConfig{}, 2)
	suite.NoError(err)

	writer.Push(&data.Item{ItemId: "a", Labels: map[string]any{"embedding": []float32{0, 0}}, Categories: []string{"movie"}, Timestamp: timestamp}, nil)
	writer.Push(&data.Item{ItemId: "b", Labels: map[string]any{"embedding": []float32{0.1, 0}}, Categories: []string{"movie"}, Timestamp: timestamp}, nil)
	writer.Push(&data.Item{ItemId: "bad", Labels: map[string]any{"embedding": []float32{0, 0, 0}}, Categories: []string{"movie"}, Timestamp: timestamp}, nil)
	writer.Push(&data.Item{ItemId: "hidden", Labels: map[string]any{"embedding": []float32{0.05, 0}}, Categories: []string{"movie"}, IsHidden: true, Timestamp: timestamp}, nil)
	suite.Equal(2, writer.Dimension())
	suite.NoError(writer.Flush())

	results, err := vectorClient.QueryVectors(ctx, vectors.ItemToItemCollection("embedding"), []float32{0, 0}, []string{"movie"}, 10)
	suite.NoError(err)
	suite.Equal([]string{"a", "b"}, lo.Map(results, func(result vectors.Vector, _ int) string { return result.Id }))
}

func (suite *ItemToItemTestSuite) TestEmbedding() {
	timestamp := time.Now()
	item2item, err := newEmbeddingItemToItem(config.ItemToItemConfig{
		Column: "item.Labels.description",
	}, 10, timestamp)
	suite.NoError(err)

	for i := range 100 {
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

func (suite *ItemToItemTestSuite) TestHidden() {
	timestamp := time.Now()
	item2item, err := newEmbeddingItemToItem(config.ItemToItemConfig{
		Column: "item.Labels.description",
	}, 2, timestamp)
	suite.NoError(err)

	item2item.Push(&data.Item{
		ItemId: "visible_1",
		Labels: map[string]any{
			"description": []float32{0.0, 0.0, 0.0},
		},
	}, nil)
	item2item.Push(&data.Item{
		ItemId: "visible_2",
		Labels: map[string]any{
			"description": []float32{0.1, 0.0, 0.0},
		},
	}, nil)
	item2item.Push(&data.Item{
		ItemId:   "hidden_1",
		IsHidden: true,
		Labels: map[string]any{
			"description": []float32{0.05, 0.0, 0.0},
		},
	}, nil)

	suite.Equal(3, item2item.Count())

	// hidden item should have similar items generated from non-hidden index
	hiddenScores := item2item.PopAll(2)
	suite.Len(hiddenScores, 2)
	for _, score := range hiddenScores {
		suite.NotEqual("hidden_1", score.Id)
	}

	// non-hidden item should never get hidden item in similarity results
	visibleScores := item2item.PopAll(0)
	suite.Len(visibleScores, 1)
	for _, score := range visibleScores {
		suite.NotEqual("hidden_1", score.Id)
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

	for i := range 100 {
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

	for i := range 100 {
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

	for i := range 100 {
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

	for i := range 100 {
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
