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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDenseVector_Euclidean(t *testing.T) {
	a := NewDenseVector([]float32{1, 2, 3})
	b := NewDenseVector([]float32{1, 4, 9})
	assert.Equal(t, float32(40), a.Euclidean(b))

	a = NewDenseVector([]float32{1, 2, 3, 4})
	b = NewDenseVector([]float32{1, 2, 3, 4, 5})
	assert.Panics(t, func() { a.Euclidean(b) })
}

func TestDenseVector_Dot(t *testing.T) {
	a := NewDenseVector([]float32{1, 2, 3})
	b := NewDenseVector([]float32{1, 4, 9})
	assert.Equal(t, float32(36), a.Dot(b))

	a = NewDenseVector([]float32{1, 2, 3, 4})
	b = NewDenseVector([]float32{1, 2, 3, 4, 5})
	assert.Panics(t, func() { a.Dot(b) })
}

func TestSparseVector_Euclidean(t *testing.T) {
	a := NewSparseVector([]int32{1, 2, 3}, []float32{1, 2, 3})
	b := NewSparseVector([]int32{1, 2, 3}, []float32{1, 4, 9})
	assert.Equal(t, float32(40), a.Euclidean(b))

	a = NewSparseVector([]int32{1, 2, 4, 5}, []float32{1, 2, 3, 4})
	b = NewSparseVector([]int32{1, 2, 3, 5, 6}, []float32{1, 2, 3, 4, 5})
	assert.Equal(t, float32(43), a.Euclidean(b))

	a = NewSparseVector([]int32{1, 2, 3, 5, 6}, []float32{1, 2, 3, 4, 5})
	b = NewSparseVector([]int32{1, 2, 4, 5}, []float32{1, 2, 3, 4})
	assert.Equal(t, float32(43), a.Euclidean(b))
}

func TestSparseVector_Dot(t *testing.T) {
	a := NewSparseVector([]int32{1, 2, 3}, []float32{1, 2, 3})
	b := NewSparseVector([]int32{1, 2, 3}, []float32{1, 4, 9})
	assert.Equal(t, float32(36), a.Dot(b))

	a = NewSparseVector([]int32{1, 2, 4, 5}, []float32{1, 2, 3, 4})
	b = NewSparseVector([]int32{1, 2, 3, 5, 6}, []float32{1, 2, 3, 4, 5})
	assert.Equal(t, float32(21), a.Dot(b))

	a = NewSparseVector([]int32{1, 2, 3, 5, 6}, []float32{1, 2, 3, 4, 5})
	b = NewSparseVector([]int32{1, 2, 4, 5}, []float32{1, 2, 3, 4})
	assert.Equal(t, float32(21), a.Dot(b))
}
