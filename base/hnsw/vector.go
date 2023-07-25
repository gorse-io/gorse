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

import "sort"

type Vector interface {
	Euclidean(vec Vector) float32
}

type SparseVector struct {
	indices []int32
	values  []float32
}

func NewSparseVector(indices []int32, values []float32) SparseVector {
	v := SparseVector{
		indices: indices,
		values:  values,
	}
	sort.Sort(v)
	return v
}

func (v SparseVector) Len() int {
	return len(v.indices)
}

func (v SparseVector) Less(i, j int) bool {
	return v.indices[i] < v.indices[j]
}

func (v SparseVector) Swap(i, j int) {
	v.indices[i], v.indices[j] = v.indices[j], v.indices[i]
	v.values[i], v.values[j] = v.values[j], v.values[i]
}

func (v SparseVector) Euclidean(vec Vector) float32 {
	// check type
	sparse, ok := vec.(SparseVector)
	if !ok {
		panic("sparse vector can only compare with sparse vector")
	}
	// calculate distance
	var sum float32
	for i, j := 0, 0; i < len(v.indices) && j < len(sparse.indices); {
		if v.indices[i] == sparse.indices[j] {
			sum += (v.values[i] - sparse.values[j]) * (v.values[i] - sparse.values[j])
			i++
			j++
		} else if v.indices[i] < sparse.indices[j] {
			sum += v.values[i] * v.values[i]
			i++
		} else {
			sum += sparse.values[j] * sparse.values[j]
			j++
		}
	}
	return sum
}
