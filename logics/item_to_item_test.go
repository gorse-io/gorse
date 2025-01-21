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
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/dataset"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"strconv"
	"testing"
	"time"
)

func TestColumnFunc(t *testing.T) {
	item2item, err := newEmbeddingItemToItem(config.ItemToItemConfig{
		Column: "item.Labels.description",
	}, 10, time.Now())
	assert.NoError(t, err)

	// Push success
	item2item.Push(data.Item{
		ItemId: "1",
		Labels: map[string]any{
			"description": []float32{0.1, 0.2, 0.3},
		},
	})
	assert.Len(t, item2item.Items(), 1)

	// Hidden
	item2item.Push(data.Item{
		ItemId:   "2",
		IsHidden: true,
		Labels: map[string]any{
			"description": []float32{0.1, 0.2, 0.3},
		},
	})
	assert.Len(t, item2item.Items(), 1)

	// Dimension does not match
	item2item.Push(data.Item{
		ItemId: "1",
		Labels: map[string]any{
			"description": []float32{0.1, 0.2},
		},
	})
	assert.Len(t, item2item.Items(), 1)

	// Type does not match
	item2item.Push(data.Item{
		ItemId: "1",
		Labels: map[string]any{
			"description": "hello",
		},
	})
	assert.Len(t, item2item.Items(), 1)

	// Column does not exist
	item2item.Push(data.Item{
		ItemId: "2",
		Labels: []float32{0.1, 0.2, 0.3},
	})
	assert.Len(t, item2item.Items(), 1)
}

func TestEmbedding(t *testing.T) {
	timestamp := time.Now()
	item2item, err := newEmbeddingItemToItem(config.ItemToItemConfig{
		Column: "item.Labels.description",
	}, 10, timestamp)
	assert.NoError(t, err)

	for i := 0; i < 100; i++ {
		item2item.Push(data.Item{
			ItemId: strconv.Itoa(i),
			Labels: map[string]any{
				"description": []float32{0.1 * float32(i), 0.2 * float32(i), 0.3 * float32(i)},
			},
		})
	}

	var scores []cache.Score
	item2item.PopAll(func(itemId string, score []cache.Score) {
		if itemId == "0" {
			scores = score
		}
	})
	assert.Len(t, scores, 10)
	for i := 1; i <= 10; i++ {
		assert.Equal(t, strconv.Itoa(i), scores[i-1].Id)
	}
}

func TestTags(t *testing.T) {
	timestamp := time.Now()
	idf := make([]float32, 101)
	for i := range idf {
		idf[i] = 1
	}
	item2item, err := newTagsItemToItem(config.ItemToItemConfig{
		Column: "item.Labels",
	}, 10, timestamp, idf)
	assert.NoError(t, err)

	for i := 0; i < 100; i++ {
		labels := make(map[string]any)
		for j := 1; j <= 100-i; j++ {
			labels[strconv.Itoa(j)] = []dataset.ID{dataset.ID(j)}
		}
		item2item.Push(data.Item{
			ItemId: strconv.Itoa(i),
			Labels: labels,
		})
	}

	var scores []cache.Score
	item2item.PopAll(func(itemId string, score []cache.Score) {
		if itemId == "0" {
			scores = score
		}
	})
	assert.Len(t, scores, 10)
	for i := 1; i <= 10; i++ {
		assert.Equal(t, strconv.Itoa(i), scores[i-1].Id)
	}
}

func TestUsers(t *testing.T) {

}
