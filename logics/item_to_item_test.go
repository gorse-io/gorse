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

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/stretchr/testify/suite"
)

type ItemToItemTestSuite struct {
	suite.Suite
	vectorClient vectors.Database
}

func (suite *ItemToItemTestSuite) SetupTest() {
	log.SetTestLogger(suite.T())
	var err error
	suite.vectorClient, err = vectors.Open(fmt.Sprintf("xvec://%s/vectors", suite.T().TempDir()), "")
	suite.NoError(err)
	suite.NoError(suite.vectorClient.Init())
}

func (suite *ItemToItemTestSuite) TearDownTest() {
	suite.NoError(suite.vectorClient.Close())
}

func (suite *ItemToItemTestSuite) TestColumnFunc() {
	ctx := suite.T().Context()
	cfg := config.ItemToItemConfig{Name: "column", Type: "embedding", Column: "item.Labels.description"}
	item2item, err := newEmbeddingItemToItem(cfg, time.Now(), &ItemToItemOptions{
		Context: ctx, VectorClient: suite.vectorClient,
	})
	suite.NoError(err)

	// Add success
	item2item.Add(&data.Item{
		ItemId: "1",
		Labels: map[string]any{
			"description": []float32{0.1, 0.2, 0.3},
		},
	}, nil)

	// Hidden
	item2item.Add(&data.Item{
		ItemId:   "2",
		IsHidden: true,
		Labels: map[string]any{
			"description": []float32{0.1, 0.2, 0.3},
		},
	}, nil)

	// Dimension does not match
	item2item.Add(&data.Item{
		ItemId: "1",
		Labels: map[string]any{
			"description": []float32{0.1, 0.2},
		},
	}, nil)

	// Type does not match
	item2item.Add(&data.Item{
		ItemId: "1",
		Labels: map[string]any{
			"description": "hello",
		},
	}, nil)

	// Column does not exist
	item2item.Add(&data.Item{
		ItemId: "2",
		Labels: []float32{0.1, 0.2, 0.3},
	}, nil)

	suite.NoError(item2item.Clean())
	stored, err := suite.vectorClient.GetVectors(ctx, vectors.ItemToItemCollection(cfg.Name), []string{"1", "2"})
	suite.NoError(err)
	suite.Require().Len(stored, 2)
	suite.Len(stored[0].Values, 3)
	suite.False(stored[0].IsHidden)
	suite.Len(stored[1].Values, 3)
	suite.True(stored[1].IsHidden)
}

func (suite *ItemToItemTestSuite) TestEmbedding() {
	timestamp := time.Now()
	cfg := config.ItemToItemConfig{Name: "embedding", Type: "embedding", Column: "item.Labels.description"}
	item2item, err := newEmbeddingItemToItem(cfg, timestamp, &ItemToItemOptions{
		Context: suite.T().Context(), VectorClient: suite.vectorClient,
	})
	suite.NoError(err)

	for i := range 100 {
		item2item.Add(&data.Item{
			ItemId: strconv.Itoa(i),
			Labels: map[string]any{
				"description": []float32{0.1 * float32(i), 0.2 * float32(i), 0.3 * float32(i)},
			},
		}, nil)
	}
	suite.NoError(item2item.Clean())

	scores, err := QueryItemToItem(suite.T().Context(), suite.vectorClient, cfg, "0", nil, 10)
	suite.NoError(err)
	suite.Len(scores, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i), scores[i-1].Id)
	}
}

func (suite *ItemToItemTestSuite) TestClean() {
	ctx := suite.T().Context()
	timestamp := time.Now().UTC().Truncate(time.Millisecond)
	cfg := config.ItemToItemConfig{Name: "cleanup", Type: "embedding", Column: "item.Labels.embedding"}
	collection := vectors.ItemToItemCollection(cfg.Name)
	suite.NoError(suite.vectorClient.AddCollection(ctx, collection, 2, vectors.Euclidean, vectors.VectorConfig{}))
	suite.NoError(suite.vectorClient.AddVectors(ctx, collection, []vectors.Vector{
		{Id: "stale", Values: []float32{0, 0}, Timestamp: timestamp.Add(-time.Hour)},
		{Id: "current", Values: []float32{1, 0}, Timestamp: timestamp},
	}))

	item2item, err := NewItemToItem(cfg, timestamp, &ItemToItemOptions{Context: ctx, VectorClient: suite.vectorClient})
	suite.NoError(err)
	item2item.Add(&data.Item{ItemId: "new", Labels: map[string]any{"embedding": []float32{2, 0}}}, nil)
	suite.NoError(item2item.Clean())

	stored, err := suite.vectorClient.GetVectors(ctx, collection, []string{"stale", "current", "new"})
	suite.NoError(err)
	suite.Require().Len(stored, 2)
	suite.Equal("current", stored[0].Id)
	suite.Equal("new", stored[1].Id)
}

func (suite *ItemToItemTestSuite) TestHidden() {
	timestamp := time.Now()
	cfg := config.ItemToItemConfig{Name: "hidden", Type: "embedding", Column: "item.Labels.description"}
	item2item, err := newEmbeddingItemToItem(cfg, timestamp, &ItemToItemOptions{
		Context: suite.T().Context(), VectorClient: suite.vectorClient,
	})
	suite.NoError(err)

	item2item.Add(&data.Item{
		ItemId: "visible_1",
		Labels: map[string]any{
			"description": []float32{0.0, 0.0, 0.0},
		},
	}, nil)
	item2item.Add(&data.Item{
		ItemId: "visible_2",
		Labels: map[string]any{
			"description": []float32{0.1, 0.0, 0.0},
		},
	}, nil)
	item2item.Add(&data.Item{
		ItemId:   "hidden_1",
		IsHidden: true,
		Labels: map[string]any{
			"description": []float32{0.05, 0.0, 0.0},
		},
	}, nil)
	suite.NoError(item2item.Clean())

	// hidden item should have similar items generated from non-hidden index
	hiddenScores, err := QueryItemToItem(suite.T().Context(), suite.vectorClient, cfg, "hidden_1", nil, 2)
	suite.NoError(err)
	suite.Len(hiddenScores, 2)
	for _, score := range hiddenScores {
		suite.NotEqual("hidden_1", score.Id)
	}

	// non-hidden item should never get hidden item in similarity results
	visibleScores, err := QueryItemToItem(suite.T().Context(), suite.vectorClient, cfg, "visible_1", nil, 2)
	suite.NoError(err)
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
	cfg := config.ItemToItemConfig{Name: "tags", Type: "tags", Column: "item.Labels"}
	item2item, err := newTagsItemToItem(cfg, timestamp, &ItemToItemOptions{
		Context: suite.T().Context(), VectorClient: suite.vectorClient,
	}, idf)
	suite.NoError(err)

	for i := range 100 {
		labels := make(map[string]any)
		for j := 1; j <= 100-i; j++ {
			labels[strconv.Itoa(j)] = []dataset.ID{dataset.ID(j)}
		}
		item2item.Add(&data.Item{
			ItemId: strconv.Itoa(i),
			Labels: labels,
		}, nil)
	}
	suite.NoError(item2item.Clean())

	scores, err := QueryItemToItem(suite.T().Context(), suite.vectorClient, cfg, "0", nil, 10)
	suite.NoError(err)
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
	cfg := config.ItemToItemConfig{Name: "users", Type: "users"}
	item2item, err := newUsersItemToItem(cfg, timestamp, &ItemToItemOptions{
		Context: suite.T().Context(), VectorClient: suite.vectorClient,
	}, idf)
	suite.NoError(err)

	for i := range 100 {
		feedback := make([]int32, 0, 100-i)
		for j := 1; j <= 100-i; j++ {
			feedback = append(feedback, int32(j))
		}
		item2item.Add(&data.Item{ItemId: strconv.Itoa(i)}, feedback)
	}
	suite.NoError(item2item.Clean())

	scores, err := QueryItemToItem(suite.T().Context(), suite.vectorClient, cfg, "0", nil, 10)
	suite.NoError(err)
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
	cfg := config.ItemToItemConfig{Name: "auto", Type: "auto"}
	item2item, err := newAutoItemToItem(cfg, timestamp, &ItemToItemOptions{
		Context: suite.T().Context(), VectorClient: suite.vectorClient,
	}, idf, idf)
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
		item2item.Add(item, feedback)
	}
	suite.NoError(item2item.Clean())

	scores0, err := QueryItemToItem(suite.T().Context(), suite.vectorClient, cfg, "0", nil, 10)
	suite.NoError(err)
	suite.Len(scores0, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i*2), scores0[i-1].Id)
	}

	scores1, err := QueryItemToItem(suite.T().Context(), suite.vectorClient, cfg, "1", nil, 10)
	suite.NoError(err)
	suite.Len(scores1, 10)
	for i := 1; i <= 10; i++ {
		suite.Equal(strconv.Itoa(i*2+1), scores1[i-1].Id)
	}
}

func TestItemToItem(t *testing.T) {
	suite.Run(t, new(ItemToItemTestSuite))
}
