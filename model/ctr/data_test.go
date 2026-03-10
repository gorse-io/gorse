// Copyright 2021 gorse Project Authors
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
package ctr

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gorse-io/gorse/dataset"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestConvertLabels(t *testing.T) {
	features := ConvertLabels(nil)
	assert.Nil(t, features)

	// categorical features
	features = ConvertLabels("label")
	assert.ElementsMatch(t, []Label{{Name: "label", Value: 1}}, features)
	features = ConvertLabels([]any{"1", "2", "3"})
	assert.ElementsMatch(t, []Label{
		{Name: "1", Value: 1},
		{Name: "2", Value: 1},
		{Name: "3", Value: 1},
	}, features)
	features = ConvertLabels(map[string]any{"city": "wenzhou", "tags": []any{"1", "2", "3"}})
	assert.ElementsMatch(t, []Label{
		{Name: "city.wenzhou", Value: 1},
		{Name: "tags.1", Value: 1},
		{Name: "tags.2", Value: 1},
		{Name: "tags.3", Value: 1},
	}, features)
	features = ConvertLabels(map[string]any{"address": map[string]any{"province": "zhejiang", "city": "wenzhou"}})
	assert.ElementsMatch(t, []Label{
		{Name: "address.province.zhejiang", Value: 1},
		{Name: "address.city.wenzhou", Value: 1},
	}, features)

	// numerical features
	features = ConvertLabels(json.Number("1"))
	assert.Equal(t, []Label{{Name: "", Value: 1}}, features)
	features = ConvertLabels(map[string]any{"city": "wenzhou", "tags": json.Number("0.5")})
	assert.ElementsMatch(t, []Label{
		{Name: "city.wenzhou", Value: 1},
		{Name: "tags.", Value: 0.5},
	}, features)

	// not supported
	features = ConvertLabels([]any{float64(1), float64(2), float64(3)})
	assert.Empty(t, features)
	features = ConvertLabels(map[string]any{"city": "wenzhou", "tags": []any{float64(1), float64(2), float64(3)}})
	assert.ElementsMatch(t, []Label{{Name: "city.wenzhou", Value: 1}}, features)
}

func TestConvertEmbeddings(t *testing.T) {
	embeddings := ConvertEmbeddings(nil)
	assert.Nil(t, embeddings)

	embeddings = ConvertEmbeddings([]float32{1, 2, 3})
	if assert.Len(t, embeddings, 1) {
		assert.Equal(t, "", embeddings[0].Name)
		assert.Equal(t, []float32{1, 2, 3}, embeddings[0].Value)
	}

	embeddings = ConvertEmbeddings([]float64{1, 2, 3})
	if assert.Len(t, embeddings, 1) {
		assert.Equal(t, "", embeddings[0].Name)
		assert.Equal(t, []float32{1, 2, 3}, embeddings[0].Value)
	}

	embeddings = ConvertEmbeddings([]any{float64(1), float32(2), float64(3)})
	if assert.Len(t, embeddings, 1) {
		assert.Equal(t, "", embeddings[0].Name)
		assert.Equal(t, []float32{1, 2, 3}, embeddings[0].Value)
	}

	embeddings = ConvertEmbeddings(map[string]any{
		"embedding1": []float32{1, 2, 3},
		"a": map[string]any{
			"embedding2": []float64{4, 5, 6},
		},
		"no_embedding": "test",
	})
	if assert.Len(t, embeddings, 2) {
		assert.ElementsMatch(t, []Embedding{
			{Name: "embedding1", Value: []float32{1, 2, 3}},
			{Name: "a.embedding2", Value: []float32{4, 5, 6}},
		}, embeddings)
	}
}

func TestLoadDataFromBuiltIn(t *testing.T) {
	train, test, err := LoadDataFromBuiltIn("frappe")
	assert.NoError(t, err)
	assert.Equal(t, 202027, train.Count())
	assert.Equal(t, 28860, test.Count())
}

func TestDataset_Split(t *testing.T) {
	// create dataset
	unifiedIndex := dataset.NewUnifiedMapIndexBuilder()
	dataSet := NewMapIndexDataset()
	numUsers, numItems := 5, 6
	for i := 0; i < numUsers; i++ {
		unifiedIndex.AddUser(fmt.Sprintf("user%v", i))
		unifiedIndex.AddUserLabel(fmt.Sprintf("user_label%v", 2*i))
		unifiedIndex.AddUserLabel(fmt.Sprintf("user_label%v", 2*i+1))
		dataSet.UserLabels = append(dataSet.UserLabels, []lo.Tuple2[int32, float32]{
			{A: int32(2 * i), B: 1},
			{A: int32(2*i + 1), B: 1},
		})
	}
	for i := 0; i < numItems; i++ {
		unifiedIndex.AddItem(fmt.Sprintf("item%v", i))
		unifiedIndex.AddItemLabel(fmt.Sprintf("item_label%v", 3*i))
		unifiedIndex.AddItemLabel(fmt.Sprintf("item_label%v", 3*i+1))
		unifiedIndex.AddItemLabel(fmt.Sprintf("item_label%v", 3*i+2))
		dataSet.ItemLabels = append(dataSet.ItemLabels, []lo.Tuple2[int32, float32]{
			{A: int32(3 * i), B: 1},
			{A: int32(3*i + 1), B: 1},
			{A: int32(3*i + 2), B: 1},
		})
		dataSet.ItemEmbeddings = append(dataSet.ItemEmbeddings, [][]float32{
			{float32(i), float32(i) + 0.1, float32(i) + 0.2},
		})
	}
	for i := 0; i < numUsers; i++ {
		for j := 0; j < numItems; j++ {
			if i+j > 4 {
				dataSet.Users = append(dataSet.Users, int32(i))
				dataSet.Items = append(dataSet.Items, int32(j))
				dataSet.ContextLabels = append(dataSet.ContextLabels, []lo.Tuple2[int32, float32]{{A: int32(i * j), B: 0.5}})
				dataSet.Target = append(dataSet.Target, 1)
				dataSet.PositiveCount++
			} else {
				dataSet.Users = append(dataSet.Users, int32(i))
				dataSet.Items = append(dataSet.Items, int32(j))
				dataSet.ContextLabels = append(dataSet.ContextLabels, []lo.Tuple2[int32, float32]{{A: int32(i * j), B: 0.5}})
				dataSet.Target = append(dataSet.Target, -1)
				dataSet.NegativeCount++
			}
		}
	}
	dataSet.Index = unifiedIndex.Build()

	assert.Equal(t, numUsers*numItems, dataSet.Count())
	assert.Equal(t, numUsers, dataSet.CountUsers())
	assert.Equal(t, numItems, dataSet.CountItems())
	assert.Equal(t, numUsers*numItems/2, dataSet.PositiveCount)
	assert.Equal(t, numUsers*numItems/2, dataSet.NegativeCount)

	features, values, embeddings, target := dataSet.Get(2)
	assert.Equal(t, []int32{
		0,
		dataSet.Index.CountUsers() + 2,
		dataSet.Index.CountUsers() + dataSet.Index.CountItems() + 0,
		dataSet.Index.CountUsers() + dataSet.Index.CountItems() + 1,
		dataSet.Index.CountUsers() + dataSet.Index.CountItems() + dataSet.Index.CountUserLabels() + 6,
		dataSet.Index.CountUsers() + dataSet.Index.CountItems() + dataSet.Index.CountUserLabels() + 7,
		dataSet.Index.CountUsers() + dataSet.Index.CountItems() + dataSet.Index.CountUserLabels() + 8,
		0,
	}, features)
	assert.Equal(t, [][]float32{{2, 2.1, 2.2}}, embeddings)
	assert.Equal(t, []float32{1, 1, 1, 1, 1, 1, 1, 0.5}, values)
	assert.Equal(t, float32(-1), target)

	// split
	train, test := dataSet.Split(0.2, 0)
	assert.Equal(t, numUsers, train.CountUsers())
	assert.Equal(t, numItems, train.CountItems())
	assert.Equal(t, 24, train.Count())
	assert.Equal(t, 12, train.PositiveCount)
	assert.Equal(t, 12, train.NegativeCount)
	assert.Equal(t, numUsers, test.CountUsers())
	assert.Equal(t, numItems, test.CountItems())
	assert.Equal(t, 6, test.Count())
	assert.Equal(t, 3, test.PositiveCount)
	assert.Equal(t, 3, test.NegativeCount)
}
