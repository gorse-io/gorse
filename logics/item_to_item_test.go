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
	"github.com/zhenghaoz/gorse/storage/data"
	"testing"
)

func TestColumnFunc(t *testing.T) {
	item2item, err := NewItemToItem(config.ItemToItemConfig{
		Column: "item.Labels.description",
	})
	assert.NoError(t, err)

	// Column exists
	item2item.Push(data.Item{
		ItemId: "1",
		Labels: map[string]any{
			"description": "hello",
		},
	})

	// Column does not exist
	item2item.Push(data.Item{
		ItemId: "2",
		Labels: []float32{0.1, 0.2, 0.3},
	})
}

func TestEmbedding(t *testing.T) {
	item2item, err := NewItemToItem(config.ItemToItemConfig{
		Column: "item.Labels.description",
	})
	assert.NoError(t, err)

	item2item.Push(data.Item{
		ItemId: "1",
		Labels: map[string]any{
			"description": []float32{0.1, 0.2, 0.3},
		},
	})
}
