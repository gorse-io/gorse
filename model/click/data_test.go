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
package click

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLoadDataFromBuiltIn(t *testing.T) {
	train, test, err := LoadDataFromBuiltIn("frappe")
	assert.NoError(t, err)
	assert.Equal(t, 202027, train.Count())
	assert.Equal(t, 28860, test.Count())
}

func TestDataset_Split(t *testing.T) {
	// create dataset
	unifiedIndex := NewUnifiedMapIndexBuilder()
	dataset := NewMapIndexDataset()
	numUsers, numItems := 5, 6
	for i := 0; i < numUsers; i++ {
		unifiedIndex.AddUser(fmt.Sprintf("user%v", i))
		unifiedIndex.AddUserLabel(fmt.Sprintf("user_label%v", 2*i))
		unifiedIndex.AddUserLabel(fmt.Sprintf("user_label%v", 2*i+1))
		dataset.UserFeatures = append(dataset.UserFeatures, []int32{int32(2 * i), int32(2*i + 1)})
	}
	for i := 0; i < numItems; i++ {
		unifiedIndex.AddItem(fmt.Sprintf("item%v", i))
		unifiedIndex.AddItemLabel(fmt.Sprintf("item_label%v", 3*i))
		unifiedIndex.AddItemLabel(fmt.Sprintf("item_label%v", 3*i+1))
		unifiedIndex.AddItemLabel(fmt.Sprintf("item_label%v", 3*i+2))
		dataset.ItemFeatures = append(dataset.ItemFeatures, []int32{int32(3 * i), int32(3*i + 1), int32(3*i + 2)})
	}
	for i := 0; i < numUsers; i++ {
		for j := 0; j < numItems; j++ {
			if i+j > 4 {
				dataset.Users.Append(int32(i))
				dataset.Items.Append(int32(j))
				dataset.CtxFeatures = append(dataset.CtxFeatures, []int32{int32(i * j)})
				dataset.CtxValues = append(dataset.CtxValues, []float32{float32(i + j)})
				dataset.NormValues.Append(1.5)
				dataset.Target.Append(1)
				dataset.PositiveCount++
			} else {
				dataset.Users.Append(int32(i))
				dataset.Items.Append(int32(j))
				dataset.CtxFeatures = append(dataset.CtxFeatures, []int32{int32(i * j)})
				dataset.CtxValues = append(dataset.CtxValues, []float32{float32(i + j)})
				dataset.NormValues.Append(1.5)
				dataset.Target.Append(-1)
				dataset.NegativeCount++
			}
		}
	}
	dataset.Index = unifiedIndex.Build()

	assert.Equal(t, numUsers*numItems, dataset.Count())
	assert.Equal(t, numUsers, dataset.UserCount())
	assert.Equal(t, numItems, dataset.ItemCount())
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
	assert.Equal(t, []float32{1, 1, 1.5, 1.5, 1.5, 1.5, 1.5, 2}, values)
	assert.Equal(t, float32(-1), target)

	// split
	train, test := dataset.Split(0.2, 0)
	assert.Equal(t, numUsers, train.UserCount())
	assert.Equal(t, numItems, train.ItemCount())
	assert.Equal(t, 24, train.Count())
	assert.Equal(t, 12, train.PositiveCount)
	assert.Equal(t, 12, train.NegativeCount)
	assert.Equal(t, numUsers, test.UserCount())
	assert.Equal(t, numItems, test.ItemCount())
	assert.Equal(t, 6, test.Count())
	assert.Equal(t, 3, test.PositiveCount)
	assert.Equal(t, 3, test.NegativeCount)
}
