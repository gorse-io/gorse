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
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"testing"
)

func TestConvertLabelsToFeatures(t *testing.T) {
	features := ConvertLabelsToFeatures(nil)
	assert.Nil(t, features)

	// categorical features
	features = ConvertLabelsToFeatures("label")
	assert.ElementsMatch(t, []Feature{{Name: "label", Value: 1}}, features)
	features = ConvertLabelsToFeatures([]any{"1", "2", "3"})
	assert.ElementsMatch(t, []Feature{
		{Name: "1", Value: 1},
		{Name: "2", Value: 1},
		{Name: "3", Value: 1},
	}, features)
	features = ConvertLabelsToFeatures(map[string]any{"city": "wenzhou", "tags": []any{"1", "2", "3"}})
	assert.ElementsMatch(t, []Feature{
		{Name: "city.wenzhou", Value: 1},
		{Name: "tags.1", Value: 1},
		{Name: "tags.2", Value: 1},
		{Name: "tags.3", Value: 1},
	}, features)
	features = ConvertLabelsToFeatures(map[string]any{"address": map[string]any{"province": "zhejiang", "city": "wenzhou"}})
	assert.ElementsMatch(t, []Feature{
		{Name: "address.province.zhejiang", Value: 1},
		{Name: "address.city.wenzhou", Value: 1},
	}, features)

	// numerical features
	features = ConvertLabelsToFeatures(json.Number("1"))
	assert.Equal(t, []Feature{{Name: "", Value: 1}}, features)
	features = ConvertLabelsToFeatures(map[string]any{"city": "wenzhou", "tags": json.Number("0.5")})
	assert.ElementsMatch(t, []Feature{
		{Name: "city.wenzhou", Value: 1},
		{Name: "tags.", Value: 0.5},
	}, features)

	// not supported
	features = ConvertLabelsToFeatures([]any{float64(1), float64(2), float64(3)})
	assert.Empty(t, features)
	features = ConvertLabelsToFeatures(map[string]any{"city": "wenzhou", "tags": []any{float64(1), float64(2), float64(3)}})
	assert.ElementsMatch(t, []Feature{{Name: "city.wenzhou", Value: 1}}, features)
}

func TestLoadDataFromBuiltIn(t *testing.T) {
	train, test, err := LoadDataFromBuiltIn("frappe")
	assert.NoError(t, err)
	assert.Equal(t, 202027, train.Count())
	assert.Equal(t, 28860, test.Count())
}

func TestDataset_Split(t *testing.T) {
	// create dataset
	unifiedIndex := base.NewUnifiedMapIndexBuilder()
	dataset := NewMapIndexDataset()
	numUsers, numItems := 5, 6
	for i := 0; i < numUsers; i++ {
		unifiedIndex.AddUser(fmt.Sprintf("user%v", i))
		unifiedIndex.AddUserLabel(fmt.Sprintf("user_label%v", 2*i))
		unifiedIndex.AddUserLabel(fmt.Sprintf("user_label%v", 2*i+1))
		dataset.UserFeatures = append(dataset.UserFeatures, []lo.Tuple2[int32, float32]{
			{A: int32(2 * i), B: 1},
			{A: int32(2*i + 1), B: 1},
		})
	}
	for i := 0; i < numItems; i++ {
		unifiedIndex.AddItem(fmt.Sprintf("item%v", i))
		unifiedIndex.AddItemLabel(fmt.Sprintf("item_label%v", 3*i))
		unifiedIndex.AddItemLabel(fmt.Sprintf("item_label%v", 3*i+1))
		unifiedIndex.AddItemLabel(fmt.Sprintf("item_label%v", 3*i+2))
		dataset.ItemFeatures = append(dataset.ItemFeatures, []lo.Tuple2[int32, float32]{
			{A: int32(3 * i), B: 1},
			{A: int32(3*i + 1), B: 1},
			{A: int32(3*i + 2), B: 1},
		})
	}
	for i := 0; i < numUsers; i++ {
		for j := 0; j < numItems; j++ {
			if i+j > 4 {
				dataset.Users.Append(int32(i))
				dataset.Items.Append(int32(j))
				dataset.ContextFeatures = append(dataset.ContextFeatures, []lo.Tuple2[int32, float32]{{A: int32(i * j), B: 0.5}})
				dataset.Target.Append(1)
				dataset.PositiveCount++
			} else {
				dataset.Users.Append(int32(i))
				dataset.Items.Append(int32(j))
				dataset.ContextFeatures = append(dataset.ContextFeatures, []lo.Tuple2[int32, float32]{{A: int32(i * j), B: 0.5}})
				dataset.Target.Append(-1)
				dataset.NegativeCount++
			}
		}
	}
	dataset.Index = unifiedIndex.Build()

	assert.Equal(t, numUsers*numItems, dataset.Count())
	assert.Equal(t, numUsers, dataset.CountUsers())
	assert.Equal(t, numItems, dataset.CountItems())
	assert.Equal(t, numUsers*numItems/2, dataset.PositiveCount)
	assert.Equal(t, numUsers*numItems/2, dataset.NegativeCount)

	features, values, target := dataset.Get(2)
	assert.Equal(t, []int32{
		0,
		dataset.Index.CountUsers() + 2,
		dataset.Index.CountUsers() + dataset.Index.CountItems() + 0,
		dataset.Index.CountUsers() + dataset.Index.CountItems() + 1,
		dataset.Index.CountUsers() + dataset.Index.CountItems() + dataset.Index.CountUserLabels() + 6,
		dataset.Index.CountUsers() + dataset.Index.CountItems() + dataset.Index.CountUserLabels() + 7,
		dataset.Index.CountUsers() + dataset.Index.CountItems() + dataset.Index.CountUserLabels() + 8,
		0,
	}, features)
	assert.Equal(t, []float32{1, 1, 1, 1, 1, 1, 1, 0.5}, values)
	assert.Equal(t, float32(-1), target)

	// split
	train, test := dataset.Split(0.2, 0)
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
