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
	"sort"
)

type Distance func(a, b Vector) float32

func Euclidean(a, b Vector) float32 {
	return a.Euclidean(b)
}

func Dot(a, b Vector) float32 {
	return a.Dot(b)
}

type Vector interface {
	Euclidean(vec Vector) float32
	Dot(vec Vector) float32
}

type VectorIndex interface {
	Add(ctx context.Context, vectors ...Vector)
	Evaluate(n int) float64
	Search(q Vector, n int) []Result
}

type Result struct {
	Index    int32
	Distance float32
}

type DenseVector struct {
	Data []float32
}

func NewDenseVector(data []float32) Vector {
	return &DenseVector{
		Data: data,
	}
}

func (v DenseVector) Euclidean(vec Vector) float32 {
	// check type
	dense, ok := vec.(*DenseVector)
	if !ok {
		panic("dense vector can only compare with dense vector")
	}
	// check length
	if len(v.Data) != len(dense.Data) {
		panic("dense vector must have the same length")
	}
	// calculate distance
	var sum float32
	for i := range v.Data {
		sum += (v.Data[i] - dense.Data[i]) * (v.Data[i] - dense.Data[i])
	}
	return sum
}

func (v DenseVector) Dot(vec Vector) float32 {
	// check type
	dense, ok := vec.(*DenseVector)
	if !ok {
		panic("dense vector can only compare with dense vector")
	}
	// check length
	if len(v.Data) != len(dense.Data) {
		panic("dense vector must have the same length")
	}
	// calculate distance
	var sum float32
	for i := range v.Data {
		sum += v.Data[i] * dense.Data[i]
	}
	return -sum
}

type SparseVector struct {
	indices []int32
	values  []float32
}

func NewSparseVector(indices []int32, values []float32) Vector {
	v := &SparseVector{
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
	sparse, ok := vec.(*SparseVector)
	if !ok {
		panic("sparse vector can only compare with sparse vector")
	}
	// calculate distance
	var sum float32
	i, j := 0, 0
	for i < len(v.indices) && j < len(sparse.indices) {
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
	for ; i < len(v.indices); i++ {
		sum += v.values[i] * v.values[i]
	}
	for ; j < len(sparse.indices); j++ {
		sum += sparse.values[j] * sparse.values[j]
	}
	return sum
}

func (v SparseVector) Dot(vec Vector) float32 {
	// check type
	sparse, ok := vec.(*SparseVector)
	if !ok {
		panic("sparse vector can only compare with sparse vector")
	}
	// calculate distance
	var sum float32
	i, j := 0, 0
	for i < len(v.indices) && j < len(sparse.indices) {
		if v.indices[i] == sparse.indices[j] {
			sum += v.values[i] * sparse.values[j]
			i++
			j++
		} else if v.indices[i] < sparse.indices[j] {
			i++
		} else {
			j++
		}
	}
	return -sum
}
