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

package search

import (
	"github.com/zhenghaoz/gorse/base/heap"
)

var _ VectorIndex = &Bruteforce{}

// Bruteforce is a naive implementation of vector index.
type Bruteforce struct {
	vectors []Vector
}

// Build a vector index on data.
func (b *Bruteforce) Build() {}

// NewBruteforce creates a Bruteforce vector index.
func NewBruteforce(vectors []Vector) *Bruteforce {
	return &Bruteforce{
		vectors: vectors,
	}
}

// Search top-k similar vectors.
func (b *Bruteforce) Search(q Vector, n int, prune0 bool) (values []int32, scores []float32) {
	pq := heap.NewPriorityQueue(true)
	for i, vec := range b.vectors {
		if vec != q {
			pq.Push(int32(i), q.Distance(vec))
			if pq.Len() > n {
				pq.Pop()
			}
		}
	}
	pq = pq.Reverse()
	for pq.Len() > 0 {
		value, score := pq.Pop()
		if !prune0 || score < 0 {
			values = append(values, value)
			scores = append(scores, score)
		}
	}
	return
}

func (b *Bruteforce) MultiSearch(q Vector, terms []string, n int, prune0 bool) (values map[string][]int32, scores map[string][]float32) {
	// create priority queues
	queues := make(map[string]*heap.PriorityQueue)
	queues[""] = heap.NewPriorityQueue(true)
	for _, term := range terms {
		queues[term] = heap.NewPriorityQueue(true)
	}

	// search with terms
	for i, vec := range b.vectors {
		if vec != q {
			queues[""].Push(int32(i), q.Distance(vec))
			if queues[""].Len() > n {
				queues[""].Pop()
			}
			for _, term := range vec.Terms() {
				if _, match := queues[term]; match {
					queues[term].Push(int32(i), q.Distance(vec))
					if queues[term].Len() > n {
						queues[term].Pop()
					}
				}
			}
		}
	}

	// retrieve results
	values = make(map[string][]int32)
	scores = make(map[string][]float32)
	for term, pq := range queues {
		pq = pq.Reverse()
		for pq.Len() > 0 {
			value, score := pq.Pop()
			if !prune0 || score < 0 {
				values[term] = append(values[term], value)
				scores[term] = append(scores[term], score)
			}
		}
	}
	return
}
