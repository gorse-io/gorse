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

package dataset

import (
	"strconv"
	"testing"
	"time"

	"github.com/chewxy/math32"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/storage/data"
)

func TestDataset_AddItem(t *testing.T) {
	dataSet := NewDataset(time.Now(), 0, 1)
	dataSet.AddItem(data.Item{
		ItemId:     "1",
		IsHidden:   false,
		Categories: []string{"a", "b"},
		Timestamp:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Labels: map[string]any{
			"a":        1,
			"embedded": []any{1.1, 2.2, 3.3},
			"tags":     []any{"a", "b", "c"},
		},
		Comment: "comment",
	})
	dataSet.AddItem(data.Item{
		ItemId:     "2",
		IsHidden:   true,
		Categories: []string{"a", "b"},
		Timestamp:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Labels: map[string]any{
			"a":        1,
			"embedded": []any{1.1, 2.2, 3.3},
			"tags":     []any{"b", "c", "a"},
			"topics":   []any{"a", "b", "c"},
		},
		Comment: "comment",
	})
	assert.Len(t, dataSet.GetItems(), 2)
	assert.Equal(t, data.Item{
		ItemId:     "1",
		IsHidden:   false,
		Categories: []string{"a", "b"},
		Timestamp:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Labels: map[string]any{
			"a":        1,
			"embedded": []float32{1.1, 2.2, 3.3},
			"tags":     []ID{0, 1, 2},
		},
		Comment: "comment",
	}, dataSet.GetItems()[0])
	assert.Equal(t, data.Item{
		ItemId:     "2",
		IsHidden:   true,
		Categories: []string{"a", "b"},
		Timestamp:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Labels: map[string]any{
			"a":        1,
			"embedded": []float32{1.1, 2.2, 3.3},
			"tags":     []ID{1, 2, 0},
			"topics":   []ID{3, 4, 5},
		},
		Comment: "comment",
	}, dataSet.GetItems()[1])
}

func TestDataset_GetItemColumnValuesIDF(t *testing.T) {
	dataSet := NewDataset(time.Now(), 0, 1)
	dataSet.AddItem(data.Item{
		ItemId:     "1",
		IsHidden:   false,
		Categories: []string{"a", "b"},
		Timestamp:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Labels: map[string]any{
			"tags": []any{"a", "b", "c"},
		},
		Comment: "comment",
	})
	dataSet.AddItem(data.Item{
		ItemId:     "2",
		IsHidden:   false,
		Categories: []string{"a", "b"},
		Timestamp:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Labels: map[string]any{
			"tags": []any{"a", "e"},
		},
		Comment: "comment",
	})
	idf := dataSet.GetItemColumnValuesIDF()
	assert.Len(t, idf, 4)
	assert.InDelta(t, 1e-3, idf[0], 1e-6)
	assert.InDelta(t, math32.Log(2), idf[1], 1e-6)
}

func TestDataset_AddUser(t *testing.T) {
	dataSet := NewDataset(time.Now(), 1, 0)
	dataSet.AddUser(data.User{
		UserId:  "1",
		Labels:  map[string]any{"a": 1, "b": "a"},
		Comment: "comment",
	})
	assert.Len(t, dataSet.users, 1)
	assert.Equal(t, data.User{
		UserId:  "1",
		Labels:  map[string]any{"a": 1, "b": ID(0)},
		Comment: "comment",
	}, dataSet.users[0])
}

func TestDataset_GetUserColumnValuesIDF(t *testing.T) {
	dataSet := NewDataset(time.Now(), 1, 0)
	dataSet.AddUser(data.User{
		UserId: "1",
		Labels: map[string]any{
			"tags": []any{"a", "b", "c"},
		},
		Comment: "comment",
	})
	dataSet.AddUser(data.User{
		UserId: "2",
		Labels: map[string]any{
			"tags": []any{"a", "e"},
		},
		Comment: "comment",
	})
	idf := dataSet.GetUserColumnValuesIDF()
	assert.Len(t, idf, 4)
	assert.InDelta(t, 1e-3, idf[0], 1e-6)
	assert.InDelta(t, math32.Log(2), idf[1], 1e-6)
}

func TestDataset_AddFeedback(t *testing.T) {
	dataSet := NewDataset(time.Now(), 10, 10)
	for i := 0; i < 10; i++ {
		dataSet.AddUser(data.User{
			UserId: strconv.Itoa(i),
		})
	}
	for i := 0; i < 10; i++ {
		dataSet.AddItem(data.Item{
			ItemId: strconv.Itoa(i),
		})
	}
	for i := 0; i < 10; i++ {
		for j := i; j < 10; j++ {
			dataSet.AddFeedback(strconv.Itoa(i), strconv.Itoa(j))
		}
	}
	userIDF := dataSet.GetUserIDF()
	itemIDF := dataSet.GetItemIDF()
	for i := 0; i < 10; i++ {
		assert.Len(t, dataSet.GetUserFeedback()[i], 10-i)
		assert.Len(t, dataSet.GetItemFeedback()[i], i+1)
		assert.InDelta(t, math32.Log(float32(10)/float32(10-i)), userIDF[i], 1e-2)
		assert.InDelta(t, math32.Log(float32(10)/float32(i+1)), itemIDF[i], 1e-2)
	}
}
