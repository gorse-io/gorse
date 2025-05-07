// Copyright 2024 gorse Project Authors
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

package ann

import (
	"github.com/chewxy/math32"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base/heap"
	"math/rand"
	"modernc.org/mathutil"
	"sync"
)

// HNSW is a vector index based on Hierarchical Navigable Small Worlds.
type HNSW[T any] struct {
	distanceFunc    func(a, b T) float32
	vectors         []T
	bottomNeighbors []*heap.PriorityQueue
	upperNeighbors  []sync.Map
	enterPoint      int32
	initOnce        sync.Once
	indexMutex      sync.Mutex
	rootMutex       sync.Mutex
	bottomMutex     []*sync.RWMutex

	levelFactor    float32
	maxConnection  int // maximum number of connections for each element per layer
	maxConnection0 int
	ef             int
	efConstruction int
}

func NewHNSW[T any](distanceFunc func(a, b T) float32) *HNSW[T] {
	return &HNSW[T]{
		distanceFunc:   distanceFunc,
		levelFactor:    1.0 / math32.Log(48),
		maxConnection:  48,
		maxConnection0: 96,
		efConstruction: 100,
	}
}

func (h *HNSW[T]) Add(v T) int {
	// Add vector
	h.indexMutex.Lock()
	h.vectors = append(h.vectors, v)
	h.bottomNeighbors = append(h.bottomNeighbors, heap.NewPriorityQueue(false))
	h.bottomMutex = append(h.bottomMutex, new(sync.RWMutex))
	q := len(h.vectors) - 1
	h.indexMutex.Unlock()
	h.insert(int32(q))
	return q
}

func (h *HNSW[T]) SearchIndex(q, k int, prune0 bool) ([]lo.Tuple2[int, float32], error) {
	// Check index
	if q < 0 || q >= len(h.vectors) {
		return nil, errors.Errorf("index out of range: %v", q)
	}
	w := h.knnSearch(h.vectors[q], k, h.efSearchValue(k))
	scores := make([]lo.Tuple2[int, float32], 0)
	for w.Len() > 0 {
		value, score := w.Pop()
		if !prune0 || score > 0 {
			scores = append(scores, lo.Tuple2[int, float32]{A: int(value), B: score})
		}
	}
	return scores, nil
}

func (h *HNSW[T]) SearchVector(q T, k int, prune0 bool) []lo.Tuple2[int, float32] {
	w := h.knnSearch(q, k, h.efSearchValue(k))
	scores := make([]lo.Tuple2[int, float32], 0)
	for w.Len() > 0 {
		value, score := w.Pop()
		if !prune0 || score > 0 {
			scores = append(scores, lo.Tuple2[int, float32]{A: int(value), B: score})
		}
	}
	return scores
}

func (h *HNSW[T]) knnSearch(q T, k, ef int) *heap.PriorityQueue {
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

// insert i-th vector into the vector index.
func (h *HNSW[T]) insert(q int32) {
	// insert first point
	var isFirstPoint bool
	h.initOnce.Do(func() {
		if h.upperNeighbors == nil {
			h.bottomNeighbors[q] = heap.NewPriorityQueue(false)
			h.upperNeighbors = make([]sync.Map, 0)
			h.enterPoint = q
			isFirstPoint = true
			return
		}
	})
	if isFirstPoint {
		return
	}

	h.rootMutex.Lock()
	var (
		w           *heap.PriorityQueue                               // list for the currently found nearest elements
		enterPoints = h.distance(h.vectors[q], []int32{h.enterPoint}) // get enter point for hnsw
		l           = int(math32.Floor(-math32.Log(rand.Float32()) * h.levelFactor))
		topLayer    = len(h.upperNeighbors)
	)
	if l <= topLayer {
		h.rootMutex.Unlock()
	} else {
		defer h.rootMutex.Unlock()
	}

	for currentLayer := topLayer; currentLayer >= l+1; currentLayer-- {
		w = h.searchLayer(h.vectors[q], enterPoints, 1, currentLayer)
		enterPoints = h.selectNeighbors(h.vectors[q], w, 1)
	}

	h.bottomMutex[q].Lock()
	for currentLayer := mathutil.Min(topLayer, l); currentLayer >= 0; currentLayer-- {
		w = h.searchLayer(h.vectors[q], enterPoints, h.efConstruction, currentLayer)
		neighbors := h.selectNeighbors(h.vectors[q], w, h.maxConnection)
		// add bidirectional connections from upperNeighbors to q at layer l_c
		h.setNeighbourhood(q, currentLayer, neighbors)
		for _, e := range neighbors.Elems() {
			h.bottomMutex[e.Value].Lock()
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
			h.bottomMutex[e.Value].Unlock()
		}
		enterPoints = w
	}
	h.bottomMutex[q].Unlock()

	if l > topLayer {
		// set enter point for hnsw to q
		h.enterPoint = q
		h.upperNeighbors = append(h.upperNeighbors, sync.Map{})
		h.setNeighbourhood(q, topLayer+1, heap.NewPriorityQueue(false))
	}
}

func (h *HNSW[T]) searchLayer(q T, enterPoints *heap.PriorityQueue, ef, currentLayer int) *heap.PriorityQueue {
	var (
		v          = mapset.NewSet(enterPoints.Values()...) // set of visited elements
		candidates = enterPoints.Clone()                    // set of candidates
		w          = enterPoints.Reverse()                  // dynamic list of found nearest upperNeighbors
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
		h.bottomMutex[c].RLock()
		neighbors := h.getNeighbourhood(c, currentLayer).Values()
		h.bottomMutex[c].RUnlock()
		for _, e := range neighbors {
			if !v.Contains(e) {
				v.Add(e)
				// get the furthest element from w to q
				_, fq = w.Peek()
				if eq := h.distanceFunc(h.vectors[e], q); eq < fq || w.Len() < ef {
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

func (h *HNSW[T]) setNeighbourhood(e int32, currentLayer int, connections *heap.PriorityQueue) {
	if currentLayer == 0 {
		h.bottomNeighbors[e] = connections
	} else {
		h.upperNeighbors[currentLayer-1].Store(e, connections)
	}
}

func (h *HNSW[T]) getNeighbourhood(e int32, currentLayer int) *heap.PriorityQueue {
	if currentLayer == 0 {
		return h.bottomNeighbors[e]
	} else {
		if connections, ok := h.upperNeighbors[currentLayer-1].Load(e); ok {
			return connections.(*heap.PriorityQueue)
		}
		return nil
	}
}

func (h *HNSW[T]) selectNeighbors(_ T, candidates *heap.PriorityQueue, m int) *heap.PriorityQueue {
	pq := candidates.Reverse()
	for pq.Len() > m {
		pq.Pop()
	}
	return pq.Reverse()
}

func (h *HNSW[T]) distance(q T, points []int32) *heap.PriorityQueue {
	pq := heap.NewPriorityQueue(false)
	for _, point := range points {
		pq.Push(point, h.distanceFunc(h.vectors[point], q))
	}
	return pq
}

// efSearchValue returns the efSearch value to use, given the current number of elements desired.
func (h *HNSW[T]) efSearchValue(n int) int {
	if h.ef > 0 {
		return mathutil.Max(h.ef, n)
	}
	return mathutil.Max(h.efConstruction, n)
}
