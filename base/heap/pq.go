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
	"github.com/chewxy/math32"
	"github.com/scylladb/go-set/i32set"
	"golang.org/x/exp/constraints"
)

type Elem[E any, W constraints.Ordered] struct {
	Value  E
	Weight W
}

type _heap[T any, W constraints.Ordered] struct {
	elems []Elem[T, W]
	desc  bool
}

func (e *_heap[T, W]) Len() int {
	return len(e.elems)
}

func (e *_heap[T, W]) Less(i, j int) bool {
	if e.desc {
		return e.elems[i].Weight > e.elems[j].Weight
	} else {
		return e.elems[i].Weight < e.elems[j].Weight
	}
}

func (e *_heap[T, W]) Swap(i, j int) {
	e.elems[i], e.elems[j] = e.elems[j], e.elems[i]
}

func (e *_heap[T, W]) Push(x interface{}) {
	it := x.(Elem[T, W])
	e.elems = append(e.elems, it)
}

func (e *_heap[T, W]) Pop() interface{} {
	old := e.elems
	item := e.elems[len(old)-1]
	e.elems = old[0 : len(old)-1]
	return item
}

// PriorityQueue represents the priority queue.
type PriorityQueue struct {
	_heap[int32, float32]
	lookup *i32set.Set
}

// NewPriorityQueue initializes an empty priority queue.
func NewPriorityQueue(desc bool) *PriorityQueue {
	return &PriorityQueue{
		_heap:  _heap[int32, float32]{desc: desc},
		lookup: i32set.New(),
	}
}

// Push inserts a new element into the queue. No action is performed on duplicate elements.
func (p *PriorityQueue) Push(v int32, weight float32) {
	if math32.IsNaN(weight) {
		panic("NaN weight is forbidden")
	} else if !p.lookup.Has(v) {
		newItem := Elem[int32, float32]{
			Value:  v,
			Weight: weight,
		}
		heap.Push(&p._heap, newItem)
		p.lookup.Add(v)
	}
}

// Pop removes the element with the highest priority from the queue and returns it.
// In case of an empty queue, an error is returned.
func (p *PriorityQueue) Pop() (int32, float32) {
	item := heap.Pop(&p._heap).(Elem[int32, float32])
	return item.Value, item.Weight
}

func (p *PriorityQueue) Peek() (int32, float32) {
	return p.elems[0].Value, p.elems[0].Weight
}

func (p *PriorityQueue) Values() []int32 {
	values := make([]int32, 0, p.Len())
	for _, elem := range p.elems {
		values = append(values, elem.Value)
	}
	return values
}

func (p *PriorityQueue) Elems() []Elem[int32, float32] {
	return p.elems
}

func (p *PriorityQueue) Clone() *PriorityQueue {
	pq := NewPriorityQueue(p.desc)
	pq.elems = make([]Elem[int32, float32], p.Len())
	copy(pq.elems, p.elems)
	return pq
}

func (p *PriorityQueue) Reverse() *PriorityQueue {
	pq := NewPriorityQueue(!p.desc)
	pq.elems = make([]Elem[int32, float32], 0, p.Len())
	for _, elem := range p.elems {
		pq.Push(elem.Value, elem.Weight)
	}
	return pq
}
