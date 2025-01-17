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
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/storage/data"
	"testing"
	"time"
)

func TestDataset_AddItem(t *testing.T) {
	dataSet := NewDataset(time.Now(), 1)
	dataSet.AddItem(data.Item{
		ItemId:     "1",
		IsHidden:   false,
		Categories: []string{"a", "b"},
		Timestamp:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Labels: map[string]any{
			"a":        1,
			"embedded": []any{1.1, 2.2, 3.3},
		},
		Comment: "comment",
	})
	assert.Len(t, dataSet.GetItems(), 1)
	assert.Equal(t, data.Item{
		ItemId:     "1",
		IsHidden:   false,
		Categories: []string{"a", "b"},
		Timestamp:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Labels: map[string]any{
			"a":        1,
			"embedded": []float32{1.1, 2.2, 3.3},
		},
		Comment: "comment",
	}, dataSet.GetItems()[0])
}
