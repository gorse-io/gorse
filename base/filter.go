// Copyright 2020 gorse Project Authors
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

package base

type weightedItem struct {
	item   int32
	weight float32
}

// TopKFilter filters out top k items with maximum weights.
type TopKFilter struct {
	items []weightedItem
	k     int
}

// NewTopKFilter creates a top k filter.
func NewTopKFilter(k int) *TopKFilter {
	filter := new(TopKFilter)
	filter.items = make([]weightedItem, 0, k+1)
	filter.k = k
	return filter
}

func (filter *TopKFilter) Len() int {
	return len(filter.items)
}

func (filter *TopKFilter) Swap(i, j int) {
	filter.items[i], filter.items[j] = filter.items[j], filter.items[i]
}

func (filter *TopKFilter) Less(i, j int) bool {
	return filter.items[i].weight < filter.items[j].weight
}

// Push pushes the element x onto the heap.
// The complexity is O(log n) where n = h.Count().
func (filter *TopKFilter) Push(item int32, weight float32) {
	filter.items = append(filter.items, weightedItem{item, weight})
	filter.up(filter.Len() - 1)
	if filter.Len() > filter.k {
		filter.pop()
	}
}

// Pop removes and returns the minimum element (according to Less) from the heap.
// The complexity is O(log n) where n = h.Count().
// Pop is equivalent to Remove(h, 0).
func (filter *TopKFilter) pop() (int32, float32) {
	n := filter.Len() - 1
	filter.Swap(0, n)
	filter.down(0, n)
	item := filter.items[filter.Len()-1]
	filter.items = filter.items[:filter.Len()-1]
	return item.item, item.weight
}

func (filter *TopKFilter) up(j int) {
	for {
		i := (j - 1) / 2 // parent
		if i == j || !filter.Less(j, i) {
			break
		}
		filter.Swap(i, j)
		j = i
	}
}

func (filter *TopKFilter) down(i0, n int) bool {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 after int overflow
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < n && filter.Less(j2, j1) {
			j = j2 // = 2*i + 2  // right child
		}
		if !filter.Less(j, i) {
			break
		}
		filter.Swap(i, j)
		i = j
	}
	return i > i0
}

// PopAll pops all items in the filter with decreasing order.
func (filter *TopKFilter) PopAll() ([]int32, []float32) {
	items := make([]int32, filter.Len())
	weights := make([]float32, filter.Len())
	for i := len(items) - 1; i >= 0; i-- {
		items[i], weights[i] = filter.pop()
	}
	return items, weights
}

type weightedString struct {
	item   string
	weight float32
}

// TopKStringFilter filters out top k strings with maximum weights.
type TopKStringFilter struct {
	items []weightedString
	k     int
}

// NewTopKStringFilter creates a top k string filter.
func NewTopKStringFilter(k int) *TopKStringFilter {
	filter := new(TopKStringFilter)
	filter.items = make([]weightedString, 0, k+1)
	filter.k = k
	return filter
}

func (filter *TopKStringFilter) Len() int {
	return len(filter.items)
}

func (filter *TopKStringFilter) Swap(i, j int) {
	filter.items[i], filter.items[j] = filter.items[j], filter.items[i]
}

func (filter *TopKStringFilter) Less(i, j int) bool {
	return filter.items[i].weight < filter.items[j].weight
}

// Push pushes the element x onto the heap.
// The complexity is O(log n) where n = h.Count().
func (filter *TopKStringFilter) Push(item string, weight float32) {
	filter.items = append(filter.items, weightedString{item, weight})
	filter.up(filter.Len() - 1)
	if filter.Len() > filter.k {
		filter.pop()
	}
}

// Pop removes and returns the minimum element (according to Less) from the heap.
// The complexity is O(log n) where n = h.Count().
// Pop is equivalent to Remove(h, 0).
func (filter *TopKStringFilter) pop() (string, float32) {
	n := filter.Len() - 1
	filter.Swap(0, n)
	filter.down(0, n)
	item := filter.items[filter.Len()-1]
	filter.items = filter.items[:filter.Len()-1]
	return item.item, item.weight
}

func (filter *TopKStringFilter) up(j int) {
	for {
		i := (j - 1) / 2 // parent
		if i == j || !filter.Less(j, i) {
			break
		}
		filter.Swap(i, j)
		j = i
	}
}

func (filter *TopKStringFilter) down(i0, n int) bool {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 after int overflow
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < n && filter.Less(j2, j1) {
			j = j2 // = 2*i + 2  // right child
		}
		if !filter.Less(j, i) {
			break
		}
		filter.Swap(i, j)
		i = j
	}
	return i > i0
}

// PopAll pops all strings in the filter with decreasing order.
func (filter *TopKStringFilter) PopAll() ([]string, []float32) {
	items := make([]string, filter.Len())
	weights := make([]float32, filter.Len())
	for i := len(items) - 1; i >= 0; i-- {
		items[i], weights[i] = filter.pop()
	}
	return items, weights
}
