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
	"github.com/gorse-io/gorse/base/heap"
	"github.com/juju/errors"
	"github.com/samber/lo"
)

// Bruteforce is a naive implementation of vector index.
type Bruteforce[T any] struct {
	distanceFunc func(a, b T) float32
	vectors      []T
}

func NewBruteforce[T any](distanceFunc func(a, b T) float32) *Bruteforce[T] {
	return &Bruteforce[T]{distanceFunc: distanceFunc}
}

func (b *Bruteforce[T]) Add(v T) int {
	// Add vector
	b.vectors = append(b.vectors, v)
	return len(b.vectors)
}

func (b *Bruteforce[T]) SearchIndex(q, k int, prune0 bool) ([]lo.Tuple2[int, float32], error) {
	// Check index
	if q < 0 || q >= len(b.vectors) {
		return nil, errors.Errorf("index out of range: %v", q)
	}
	// Search
	pq := heap.NewPriorityQueue(true)
	for i, vec := range b.vectors {
		if i != q {
			pq.Push(int32(i), b.distanceFunc(b.vectors[q], vec))
			if pq.Len() > k {
				pq.Pop()
			}
		}
	}
	pq = pq.Reverse()
	scores := make([]lo.Tuple2[int, float32], 0)
	for pq.Len() > 0 {
		value, score := pq.Pop()
		if !prune0 || score > 0 {
			scores = append(scores, lo.Tuple2[int, float32]{A: int(value), B: score})
		}
	}
	return scores, nil
}

func (b *Bruteforce[T]) SearchVector(q T, k int, prune0 bool) []lo.Tuple2[int, float32] {
	// Search
	pq := heap.NewPriorityQueue(true)
	for i, vec := range b.vectors {
		pq.Push(int32(i), b.distanceFunc(q, vec))
		if pq.Len() > k {
			pq.Pop()
		}
	}
	pq = pq.Reverse()
	scores := make([]lo.Tuple2[int, float32], 0)
	for pq.Len() > 0 {
		value, score := pq.Pop()
		if !prune0 || score > 0 {
			scores = append(scores, lo.Tuple2[int, float32]{A: int(value), B: score})
		}
	}
	return scores
}
