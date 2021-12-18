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

package hnsw

import (
	"github.com/chewxy/math32"
	"github.com/scylladb/go-set/i32set"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/heap"
	"math/rand"
)

var _ VectorIndex = &HierarchicalNSW{}

type HierarchicalNSW struct {
	vectors         []Vector
	bottomNeighbors []*heap.PriorityQueue
	upperNeighbors  []map[int32]*heap.PriorityQueue
	enterPoint      int32

	levelFactor    float32
	maxConnection  int // maximum number of connections for each element per layer
	maxConnection0 int
	efConstruction int
}

type Config func(*HierarchicalNSW)

func SetMaxConnection(maxConnection int) Config {
	return func(h *HierarchicalNSW) {
		h.levelFactor = 1.0 / math32.Log(float32(maxConnection))
		h.maxConnection = maxConnection
		h.maxConnection0 = maxConnection * 2
	}
}

func SetEFConstruction(efConstruction int) Config {
	return func(h *HierarchicalNSW) {
		h.efConstruction = efConstruction
	}
}

func NewHierarchicalNSW(vectors []Vector, configs ...Config) VectorIndex {
	h := &HierarchicalNSW{
		vectors:        vectors,
		levelFactor:    1.0 / math32.Log(48),
		maxConnection:  48,
		maxConnection0: 96,
		efConstruction: 100,
	}
	for _, config := range configs {
		config(h)
	}
	for i := range vectors {
		h.insert(int32(i))
	}
	return h
}

func (h *HierarchicalNSW) AddVector(vector Vector) {
	//TODO implement me
	panic("implement me")
}

func (h *HierarchicalNSW) Search(q Vector, n int) (values []int32, scores []float32) {
	w := h.knnSearch(q, n, h.efConstruction)
	for w.Len() > 0 {
		value, score := w.Pop()
		values = append(values, value)
		scores = append(scores, score)
	}
	return
}

func (h *HierarchicalNSW) knnSearch(q Vector, k, ef int) *heap.PriorityQueue {
	var (
		w           *heap.PriorityQueue                    // set for the current the nearest element
		enterPoints = h.distance(q, []int32{h.enterPoint}) // get enter point for hnsw
		topLayer    = len(h.upperNeighbors)                // top layer for hnsw
	)
	for currentLayer := topLayer; currentLayer > 0; currentLayer-- {
		w = h.searchLayer(q, enterPoints, 1, currentLayer)
		enterPoints = heap.NewPriorityQueue(false)
		enterPoints.Push(w.Peek())
	}
	w = h.searchLayer(q, enterPoints, ef, 0)
	return h.selectNeighbors(q, w, k)
}

func (h *HierarchicalNSW) insert(q int32) {
	if h.bottomNeighbors == nil {
		h.bottomNeighbors = make([]*heap.PriorityQueue, 1)
		h.bottomNeighbors[q] = heap.NewPriorityQueue(false)
		h.upperNeighbors = make([]map[int32]*heap.PriorityQueue, 0)
		h.enterPoint = q
		return
	}

	var (
		w           *heap.PriorityQueue                               // list for the currently found nearest elements
		enterPoints = h.distance(h.vectors[q], []int32{h.enterPoint}) // get enter point for hnsw
		l           = int(math32.Floor(-math32.Log(rand.Float32()) * h.levelFactor))
		topLayer    = len(h.upperNeighbors)
	)

	for currentLayer := topLayer; currentLayer >= l+1; currentLayer-- {
		w = h.searchLayer(h.vectors[q], enterPoints, 1, currentLayer)
		enterPoints = h.selectNeighbors(h.vectors[q], w, 1)
	}

	for currentLayer := base.Min(topLayer, l); currentLayer >= 0; currentLayer-- {
		w = h.searchLayer(h.vectors[q], enterPoints, h.efConstruction, currentLayer)
		neighbors := h.selectNeighbors(h.vectors[q], w, h.maxConnection)
		// add bidirectional connections from upperNeighbors to q at layer l_c
		h.setNeighbourhood(q, currentLayer, neighbors)
		for _, e := range neighbors.Elems() {
			h.getNeighbourhood(e.Value, currentLayer).Push(q, e.Weight)
			connections := h.getNeighbourhood(e.Value, currentLayer)
			var currentMaxConnection int
			if currentLayer == 0 {
				currentMaxConnection = h.maxConnection0
			} else {
				currentMaxConnection = h.maxConnection
			}
			if connections.Len() > currentMaxConnection {
				// shrink connections of e if lc = 0 then M_max = M_max0
				newConnections := h.selectNeighbors(h.vectors[q], connections, h.maxConnection)
				h.setNeighbourhood(e.Value, currentLayer, newConnections)
			}
		}
		enterPoints = w
	}

	if l > topLayer {
		// set enter point for hnsw to q
		h.enterPoint = q
	}
}

func (h *HierarchicalNSW) searchLayer(q Vector, enterPoints *heap.PriorityQueue, ef, currentLayer int) *heap.PriorityQueue {
	var (
		v          = i32set.New(enterPoints.Values()...) // set of visited elements
		candidates = enterPoints.Clone()                 // set of candidates
		w          = enterPoints.Reverse()               // dynamic list of found nearest upperNeighbors
	)
	for candidates.Len() > 0 {
		// extract nearest element from candidates to q
		c, cq := candidates.Pop()
		// get the furthest element from w to q
		_, fq := w.Peek()

		if cq > fq {
			break // all elements in w are evaluated
		}

		// update candidates and w
		for _, e := range h.getNeighbourhood(c, currentLayer).Values() {
			if !v.Has(e) {
				v.Add(e)
				// get the furthest element from w to q
				_, fq = w.Peek()
				if eq := h.vectors[e].Distance(q); eq < fq || w.Len() < ef {
					candidates.Push(e, eq)
					w.Push(e, eq)
					if w.Len() > ef {
						// remove the furthest element from w to q
						w.Pop()
					}
				}
			}
		}
	}
	return w.Reverse()
}

func (h *HierarchicalNSW) setNeighbourhood(e int32, currentLayer int, connections *heap.PriorityQueue) {
	if currentLayer == 0 {
		for len(h.bottomNeighbors) <= int(e) {
			h.bottomNeighbors = append(h.bottomNeighbors, nil)
		}
		h.bottomNeighbors[e] = connections
	} else {
		for len(h.upperNeighbors) < currentLayer {
			h.upperNeighbors = append(h.upperNeighbors, make(map[int32]*heap.PriorityQueue))
		}
		h.upperNeighbors[currentLayer][e] = connections
	}
}

func (h *HierarchicalNSW) getNeighbourhood(e int32, currentLayer int) *heap.PriorityQueue {
	if currentLayer == 0 {
		return h.bottomNeighbors[e]
	} else {
		return h.upperNeighbors[currentLayer-1][e]
	}
}

func (h *HierarchicalNSW) selectNeighbors(q Vector, candidates *heap.PriorityQueue, m int) *heap.PriorityQueue {
	pq := candidates.Reverse()
	for pq.Len() > m {
		pq.Pop()
	}
	return pq.Reverse()
}

func (h *HierarchicalNSW) distance(q Vector, points []int32) *heap.PriorityQueue {
	pq := heap.NewPriorityQueue(false)
	for _, point := range points {
		pq.Push(point, h.vectors[point].Distance(q))
	}
	return pq
}
