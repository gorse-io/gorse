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
	"github.com/zhenghaoz/gorse/base/heap"
)

var _ VectorIndex = &Bruteforce{}

// Bruteforce is a naive implementation of vector index.
type Bruteforce struct {
	vectors []Vector
}

// NewBruteforce creates a Bruteforce vector index.
func NewBruteforce(vectors []Vector) VectorIndex {
	return &Bruteforce{vectors: vectors}
}

// Search top-k similar vectors.
func (b *Bruteforce) Search(q Vector, n int) (values []int32, scores []float32) {
	pq := heap.NewPriorityQueue(true)
	for i, vec := range b.vectors {
		pq.Push(int32(i), q.Distance(vec))
		if pq.Len() > n {
			pq.Pop()
		}
	}
	pq = pq.Reverse()
	for pq.Len() > 0 {
		value, score := pq.Pop()
		values = append(values, value)
		scores = append(scores, score)
	}
	return
}

// SearchConditional searches top-k similar vectors with condition.
func (b *Bruteforce) SearchConditional(q Vector, condition string, n int) (values []int32, scores []float32) {
	pq := heap.NewPriorityQueue(true)
	for i, vec := range b.vectors {
		if vec.Match(condition) {
			pq.Push(int32(i), q.Distance(vec))
			if pq.Len() > n {
				pq.Pop()
			}
		}
	}
	pq = pq.Reverse()
	for pq.Len() > 0 {
		value, score := pq.Pop()
		values = append(values, value)
		scores = append(scores, score)
	}
	return
}
