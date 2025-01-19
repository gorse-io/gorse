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
	"github.com/chewxy/math32"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/storage/data"
	"modernc.org/strutil"
	"time"
)

type ID int

type Dataset struct {
	timestamp    time.Time
	items        []data.Item
	columnNames  *strutil.Pool
	columnValues *FreqDict
}

func NewDataset(timestamp time.Time, itemCount int) *Dataset {
	return &Dataset{
		timestamp:    timestamp,
		items:        make([]data.Item, 0, itemCount),
		columnNames:  strutil.NewPool(),
		columnValues: NewFreqDict(),
	}
}

func (d *Dataset) GetTimestamp() time.Time {
	return d.timestamp
}

func (d *Dataset) GetItems() []data.Item {
	return d.items
}

func (d *Dataset) GetItemColumnValuesIDF() []float32 {
	idf := make([]float32, d.columnValues.Count())
	for i := 0; i < d.columnValues.Count(); i++ {
		// Since zero IDF will cause NaN in the future, we set the minimum value to 1e-3.
		idf[i] = max(math32.Log(float32(len(d.items)/(d.columnValues.Freq(i)))), 1e-3)
	}
	return idf
}

func (d *Dataset) AddItem(item data.Item) {
	d.items = append(d.items, data.Item{
		ItemId:     item.ItemId,
		IsHidden:   item.IsHidden,
		Categories: item.Categories,
		Timestamp:  item.Timestamp,
		Labels:     d.processLabels(item.Labels, ""),
		Comment:    item.Comment,
	})
}

func (d *Dataset) processLabels(labels any, parent string) any {
	switch typed := labels.(type) {
	case map[string]any:
		o := make(map[string]any)
		for k, v := range typed {
			o[d.columnNames.Align(k)] = d.processLabels(v, parent+"."+k)
		}
		return o
	case []any:
		if isSliceOf[float64](typed) {
			return lo.Map(typed, func(e any, _ int) float32 {
				return float32(e.(float64))
			})
		} else if isSliceOf[string](typed) {
			return lo.Map(typed, func(e any, _ int) ID {
				return ID(d.columnValues.Id(parent + ":" + e.(string)))
			})
		}
		return typed
	case string:
		return ID(d.columnValues.Id(parent + ":" + typed))
	default:
		return labels
	}
}

func isSliceOf[T any](v []any) bool {
	for _, e := range v {
		if _, ok := e.(T); !ok {
			return false
		}
	}
	return true
}
