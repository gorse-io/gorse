// Copyright 2022 gorse Project Authors
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

package heap

import (
	"container/heap"
	"golang.org/x/exp/constraints"
)

// TopKFilter filters out top k items with maximum weights.
type TopKFilter[T any, W constraints.Ordered] struct {
	_heap[T, W]
	k int
}

// NewTopKFilter creates a top k filter.
func NewTopKFilter[T any, W constraints.Ordered](k int) *TopKFilter[T, W] {
	return &TopKFilter[T, W]{k: k}
}

// Push pushes the element x onto the heap.
// The complexity is O(log n) where n = h.Count().
func (filter *TopKFilter[T, W]) Push(item T, weight W) {
	heap.Push(&filter._heap, Elem[T, W]{item, weight})
	if filter.Len() > filter.k {
		heap.Pop(&filter._heap)
	}
}

// PopAll pops all items in the filter with decreasing order.
func (filter *TopKFilter[T, W]) PopAll() ([]T, []W) {
	items := make([]T, filter.Len())
	weights := make([]W, filter.Len())
	for i := len(items) - 1; i >= 0; i-- {
		elem := heap.Pop(&filter._heap).(Elem[T, W])
		items[i], weights[i] = elem.Value, elem.Weight
	}
	return items, weights
}
