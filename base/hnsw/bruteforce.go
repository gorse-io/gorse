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
	"context"

	"github.com/chewxy/math32"
	"github.com/zhenghaoz/gorse/base/heap"
)

type Bruteforce struct {
	vectors []Vector
	distFn  Distance
}

func NewBruteforce(dist Distance) *Bruteforce {
	return &Bruteforce{
		distFn: dist,
	}
}

func (b *Bruteforce) Add(ctx context.Context, vectors ...Vector) {
	b.vectors = append(b.vectors, vectors...)
}

func (b *Bruteforce) Evaluate(n int) float64 {
	return 1
}

// Search returns the nearest neighbors of q.
func (b *Bruteforce) Search(q Vector, n int) []Result {
	if q == nil {
		return nil
	}
	pq := heap.NewPriorityQueue(true)
	for i, vec := range b.vectors {
		if vec != nil && vec != q {
			d := b.distFn(q, vec)
			if math32.IsInf(d, 1) {
				continue
			}
			pq.Push(int32(i), d)
			if pq.Len() > n {
				pq.Pop()
			}
		}
	}
	pq = pq.Reverse()
	var results []Result
	for pq.Len() > 0 {
		value, score := pq.Pop()
		results = append(results, Result{
			Index:    value,
			Distance: score,
		})
	}
	return results
}
