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
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/storage/data"
	"modernc.org/strutil"
	"sort"
	"time"
)

type ID int

type Dataset struct {
	timestamp time.Time
	items     []data.Item
	fields    *strutil.Pool
	dict      *strutil.Dict
}

func NewDataset(timestamp time.Time, itemCount int) *Dataset {
	return &Dataset{
		timestamp: timestamp,
		items:     make([]data.Item, 0, itemCount),
		fields:    strutil.NewPool(),
		dict:      strutil.NewDict(),
	}
}

func (d *Dataset) GetTimestamp() time.Time {
	return d.timestamp
}

func (d *Dataset) GetItems() []data.Item {
	return d.items
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
			o[d.fields.Align(k)] = d.processLabels(v, parent+"."+k)
		}
		return o
	case []any:
		if isSliceOf[float64](typed) {
			return lo.Map(typed, func(e any, _ int) float32 {
				return float32(e.(float64))
			})
		} else if isSliceOf[string](typed) {
			ids := lo.Map(typed, func(e any, _ int) ID {
				return ID(d.dict.Id(parent + "." + e.(string)))
			})
			sort.Slice(ids, func(i, j int) bool {
				return ids[i] < ids[j]
			})
			return ids
		}
		return typed
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
