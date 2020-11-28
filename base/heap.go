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

import (
	"container/heap"
	"github.com/zhenghaoz/gorse/floats"
)

// MaxHeap is designed for store K maximal elements. Heap is used to reduce time complexity
// and memory complexity in top-K searching.
type MaxHeap struct {
	Elem  []interface{} // store elements
	Score []float32     // store scores
	K     int           // the size of heap
}

// NewMaxHeap creates a MaxHeap.
func NewMaxHeap(k int) *MaxHeap {
	knnHeap := new(MaxHeap)
	knnHeap.Elem = make([]interface{}, 0)
	knnHeap.Score = make([]float32, 0)
	knnHeap.K = k
	return knnHeap
}

// Less returns true if the score of i-th item is less than the score of j-th item.
// It is a method of heap.Interface.
func (maxHeap *MaxHeap) Less(i, j int) bool {
	return maxHeap.Score[i] < maxHeap.Score[j]
}

// Swap the i-th item with the j-th item. It is a method of heap.Interface.
func (maxHeap *MaxHeap) Swap(i, j int) {
	maxHeap.Elem[i], maxHeap.Elem[j] = maxHeap.Elem[j], maxHeap.Elem[i]
	maxHeap.Score[i], maxHeap.Score[j] = maxHeap.Score[j], maxHeap.Score[i]
}

// Len returns the size of heap. It is a method of heap.Interface.
func (maxHeap *MaxHeap) Len() int {
	return len(maxHeap.Elem)
}

// _HeapItem is designed for heap.Interface to pass neighborhoods.
type _HeapItem struct {
	Elem  interface{}
	Score float32
}

// Push a neighbors into the MaxHeap. It is a method of heap.Interface.
func (maxHeap *MaxHeap) Push(x interface{}) {
	item := x.(_HeapItem)
	maxHeap.Elem = append(maxHeap.Elem, item.Elem)
	maxHeap.Score = append(maxHeap.Score, item.Score)
}

// Pop the last item (the element with minimal score) in the MaxHeap.
// It is a method of heap.Interface.
func (maxHeap *MaxHeap) Pop() interface{} {
	// Extract the minimum
	n := maxHeap.Len()
	item := _HeapItem{
		Elem:  maxHeap.Elem[n-1],
		Score: maxHeap.Score[n-1],
	}
	// Remove last element
	maxHeap.Elem = maxHeap.Elem[0 : n-1]
	maxHeap.Score = maxHeap.Score[0 : n-1]
	// We never use returned item
	return item
}

// Add a new element to the MaxHeap.
func (maxHeap *MaxHeap) Add(elem interface{}, score float32) {
	// Insert item
	heap.Push(maxHeap, _HeapItem{elem, score})
	// Remove minimum
	if maxHeap.Len() > maxHeap.K {
		heap.Pop(maxHeap)
	}
}

// ToSorted returns a sorted slice od elements in the heap.
func (maxHeap *MaxHeap) ToSorted() ([]interface{}, []float32) {
	// sort indices
	scores := make([]float32, maxHeap.Len())
	indices := make([]int, maxHeap.Len())
	copy(scores, maxHeap.Score)
	floats.Argsort(scores, indices)
	// make output
	sorted := make([]interface{}, maxHeap.Len())
	for i := range indices {
		sorted[i] = maxHeap.Elem[indices[maxHeap.Len()-1-i]]
		scores[i] = maxHeap.Score[indices[maxHeap.Len()-1-i]]
	}
	return sorted, scores
}
