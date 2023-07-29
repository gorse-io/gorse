// Copyright 2023 gorse Project Authors
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
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/chewxy/math32"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/heap"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/parallel"
	"go.uber.org/zap"
	"modernc.org/mathutil"
)

// HNSW is a vector index based on Hierarchical Navigable Small Worlds.
type HNSW struct {
	vectors         []Vector
	bottomNeighbors []*heap.PriorityQueue
	upperNeighbors  []sync.Map
	enterPoint      int32

	nodeMutexes []sync.RWMutex
	globalMutex sync.RWMutex
	initOnce    sync.Once

	levelFactor    float32
	maxConnection  int // maximum number of connections for each element per layer
	maxConnection0 int
	efConstruction int
	numJobs        int
}

// HNSWConfig is the configuration function for HNSW.
type HNSWConfig func(*HNSW)

// SetHNSWNumJobs sets the number of jobs for building index.
func SetHNSWNumJobs(numJobs int) HNSWConfig {
	return func(h *HNSW) {
		h.numJobs = numJobs
	}
}

// SetMaxConnection sets the number of connections in HNSW.
func SetMaxConnection(maxConnection int) HNSWConfig {
	return func(h *HNSW) {
		h.levelFactor = 1.0 / math32.Log(float32(maxConnection))
		h.maxConnection = maxConnection
		h.maxConnection0 = maxConnection * 2
	}
}

// SetEFConstruction sets efConstruction in HNSW.
func SetEFConstruction(efConstruction int) HNSWConfig {
	return func(h *HNSW) {
		h.efConstruction = efConstruction
	}
}

// NewHNSW builds a vector index based on Hierarchical Navigable Small Worlds.
func NewHNSW(vectors []Vector, configs ...HNSWConfig) *HNSW {
	h := &HNSW{
		vectors:        vectors,
		levelFactor:    1.0 / math32.Log(48),
		maxConnection:  48,
		maxConnection0: 96,
		efConstruction: 100,
		numJobs:        runtime.NumCPU(),
	}
	for _, config := range configs {
		config(h)
	}
	return h
}

// Search a vector in Hierarchical Navigable Small Worlds.
func (h *HNSW) Search(q Vector, n int) []Result {
	w := h.knnSearch(q, n, mathutil.Max(h.efConstruction, n))
	results := make([]Result, 0, w.Len())
	for w.Len() > 0 {
		value, score := w.Pop()
		results = append(results, Result{
			Index:    value,
			Distance: score,
		})
	}
	return results
}

func (h *HNSW) knnSearch(q Vector, k, ef int) *heap.PriorityQueue {
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

// Build a vector index on data.
func (h *HNSW) Build() {
	completed := make(chan struct{}, h.numJobs)
	go func() {
		defer base.CheckPanic()
		completedCount, previousCount := 0, 0
		ticker := time.NewTicker(10 * time.Second)
		for {
			select {
			case _, ok := <-completed:
				if !ok {
					return
				}
				completedCount++
			case <-ticker.C:
				throughput := completedCount - previousCount
				previousCount = completedCount
				if throughput > 0 {
					log.Logger().Info("building index",
						zap.Int("n_indexed_vectors", completedCount),
						zap.Int("n_vectors", len(h.vectors)),
						zap.Int("throughput", throughput))
				}
			}
		}
	}()

	h.bottomNeighbors = make([]*heap.PriorityQueue, len(h.vectors))
	h.nodeMutexes = make([]sync.RWMutex, len(h.vectors))
	_ = parallel.Parallel(len(h.vectors), h.numJobs, func(_, jobId int) error {
		h.insert(int32(jobId))
		completed <- struct{}{}
		return nil
	})
	close(completed)
}

// insert i-th vector into the vector index.
func (h *HNSW) insert(q int32) {
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

	h.globalMutex.RLock()
	var (
		w           *heap.PriorityQueue                               // list for the currently found nearest elements
		enterPoints = h.distance(h.vectors[q], []int32{h.enterPoint}) // get enter point for hnsw
		l           = int(math32.Floor(-math32.Log(rand.Float32()) * h.levelFactor))
		topLayer    = len(h.upperNeighbors)
	)
	h.globalMutex.RUnlock()
	if l > topLayer {
		h.globalMutex.Lock()
		defer h.globalMutex.Unlock()
	}

	for currentLayer := topLayer; currentLayer >= l+1; currentLayer-- {
		w = h.searchLayer(h.vectors[q], enterPoints, 1, currentLayer)
		enterPoints = h.selectNeighbors(h.vectors[q], w, 1)
	}

	h.nodeMutexes[q].Lock()
	for currentLayer := mathutil.Min(topLayer, l); currentLayer >= 0; currentLayer-- {
		w = h.searchLayer(h.vectors[q], enterPoints, h.efConstruction, currentLayer)
		neighbors := h.selectNeighbors(h.vectors[q], w, h.maxConnection)
		// add bidirectional connections from upperNeighbors to q at layer l_c
		h.setNeighbourhood(q, currentLayer, neighbors)
		for _, e := range neighbors.Elems() {
			h.nodeMutexes[e.Value].Lock()
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
			h.nodeMutexes[e.Value].Unlock()
		}
		enterPoints = w
	}
	h.nodeMutexes[q].Unlock()

	if l > topLayer {
		// set enter point for hnsw to q
		h.enterPoint = q
		h.upperNeighbors = append(h.upperNeighbors, sync.Map{})
		h.setNeighbourhood(q, topLayer+1, heap.NewPriorityQueue(false))
	}
}

func (h *HNSW) searchLayer(q Vector, enterPoints *heap.PriorityQueue, ef, currentLayer int) *heap.PriorityQueue {
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
		h.nodeMutexes[c].RLock()
		neighbors := h.getNeighbourhood(c, currentLayer).Values()
		h.nodeMutexes[c].RUnlock()
		for _, e := range neighbors {
			if !v.Contains(e) {
				v.Add(e)
				// get the furthest element from w to q
				_, fq = w.Peek()
				if eq := h.vectors[e].Euclidean(q); eq < fq || w.Len() < ef {
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

func (h *HNSW) setNeighbourhood(e int32, currentLayer int, connections *heap.PriorityQueue) {
	if currentLayer == 0 {
		h.bottomNeighbors[e] = connections
	} else {
		h.upperNeighbors[currentLayer-1].Store(e, connections)
	}
}

func (h *HNSW) getNeighbourhood(e int32, currentLayer int) *heap.PriorityQueue {
	if currentLayer == 0 {
		return h.bottomNeighbors[e]
	} else {
		temp, _ := h.upperNeighbors[currentLayer-1].Load(e)
		return temp.(*heap.PriorityQueue)
	}
}

func (h *HNSW) selectNeighbors(_ Vector, candidates *heap.PriorityQueue, m int) *heap.PriorityQueue {
	pq := candidates.Reverse()
	for pq.Len() > m {
		pq.Pop()
	}
	return pq.Reverse()
}

func (h *HNSW) distance(q Vector, points []int32) *heap.PriorityQueue {
	pq := heap.NewPriorityQueue(false)
	for _, point := range points {
		pq.Push(point, h.vectors[point].Euclidean(q))
	}
	return pq
}
