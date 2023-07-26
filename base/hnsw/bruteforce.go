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

import "github.com/zhenghaoz/gorse/base/heap"

type Bruteforce struct {
	vectors []Vector
}

func NewBruteforce() *Bruteforce {
	return &Bruteforce{}
}

func (b *Bruteforce) Add(vectors ...Vector) {
	b.vectors = append(b.vectors, vectors...)
}

// Search returns the nearest neighbors of q.
func (b *Bruteforce) Search(q Vector, n int) []Result {
	pq := heap.NewPriorityQueue(true)
	for i, vec := range b.vectors {
		if vec != q {
			pq.Push(int32(i), q.Euclidean(vec))
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
