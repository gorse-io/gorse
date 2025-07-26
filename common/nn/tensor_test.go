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

package nn

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTensor_Slice(t *testing.T) {
	x := Rand(3, 4, 5)
	y := x.Slice(1, 3)
	assert.Equal(t, []int{2, 4, 5}, y.Shape())
	for i := 0; i < 2; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 5; k++ {
				assert.Equal(t, x.Get(i+1, j, k), y.Get(i, j, k))
			}
		}
	}
}

func TestTensor_SliceIndices(t *testing.T) {
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 3, 2)
	y := x.SliceIndices(2, 0)
	assert.Equal(t, []int{2, 2}, y.Shape())
	assert.Equal(t, []float32{5, 6, 1, 2}, y.Data())
}

func TestTensor_Max(t *testing.T) {
	x := NewTensor([]float32{3, 2, 5, 6, 0, 0}, 6)
	y := x.max(0, false)
	assert.Len(t, y.shape, 0)
	assert.Equal(t, []float32{6}, y.data)

	assert.Panics(t, func() { x.max(-1, false) })
	assert.Panics(t, func() { x.max(2, false) })

	x = NewTensor([]float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, 3, 2, 2)
	y = x.max(1, false)
	assert.Equal(t, []int{3, 2}, y.shape)
	assert.Equal(t, []float32{3, 4, 7, 8, 11, 12}, y.data)
	y = x.max(1, true)
	assert.Equal(t, []int{3, 1, 2}, y.shape)
	assert.Equal(t, []float32{3, 4, 7, 8, 11, 12}, y.data)
}

func TestTensor_Sum(t *testing.T) {
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 6)
	y := x.sum(0, false)
	assert.Len(t, y.shape, 0)
	assert.Equal(t, []float32{21}, y.data)

	assert.Panics(t, func() { x.sum(-1, false) })
	assert.Panics(t, func() { x.sum(2, false) })

	x = NewTensor([]float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, 3, 2, 2)
	y = x.sum(1, false)
	assert.Equal(t, []int{3, 2}, y.shape)
	assert.Equal(t, []float32{4, 6, 12, 14, 20, 22}, y.data)
	y = x.sum(1, true)
	assert.Equal(t, []int{3, 1, 2}, y.shape)
	assert.Equal(t, []float32{4, 6, 12, 14, 20, 22}, y.data)
}

func TestTensor_Transpose(t *testing.T) {
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 3, 2)
	y := x.transpose()
	assert.Equal(t, []int{2, 3}, y.Shape())
	assert.Equal(t, []float32{1, 3, 5, 2, 4, 6}, y.Data())
}

func (t *Tensor) matMulLegacy(other *Tensor, transpose1, transpose2 bool) *Tensor {
	if !transpose1 && !transpose2 {
		if len(t.shape) != 2 || len(other.shape) != 2 {
			panic("matMul requires 2-D tensors")
		}
		if t.shape[1] != other.shape[0] {
			panic("matMul requires the shapes of tensors are compatible")
		}
		m, n, p := t.shape[0], t.shape[1], other.shape[1]
		result := make([]float32, m*p)
		for i := 0; i < m; i++ {
			for j := 0; j < p; j++ {
				for k := 0; k < n; k++ {
					result[i*p+j] += t.data[i*n+k] * other.data[k*p+j]
				}
			}
		}
		return &Tensor{
			data:  result,
			shape: []int{m, p},
		}
	} else if transpose1 && !transpose2 {
		if len(t.shape) != 2 || len(other.shape) != 2 {
			panic("matMul requires 2-D tensors")
		}
		if t.shape[0] != other.shape[0] {
			panic("matMul requires the shapes of tensors are compatible")
		}
		m, n, p := t.shape[1], t.shape[0], other.shape[1]
		result := make([]float32, m*p)
		for i := 0; i < m; i++ {
			for j := 0; j < p; j++ {
				for k := 0; k < n; k++ {
					result[i*p+j] += t.data[k*m+i] * other.data[k*p+j]
				}
			}
		}
		return &Tensor{
			data:  result,
			shape: []int{m, p},
		}
	} else if !transpose1 && transpose2 {
		if len(t.shape) != 2 || len(other.shape) != 2 {
			panic("matMul requires 2-D tensors")
		}
		if t.shape[1] != other.shape[1] {
			panic("matMul requires the shapes of tensors are compatible")
		}
		m, n, p := t.shape[0], t.shape[1], other.shape[0]
		result := make([]float32, m*p)
		for i := 0; i < m; i++ {
			for j := 0; j < p; j++ {
				for k := 0; k < n; k++ {
					result[i*p+j] += t.data[i*n+k] * other.data[j*n+k]
				}
			}
		}
		return &Tensor{
			data:  result,
			shape: []int{m, p},
		}
	} else {
		if len(t.shape) != 2 || len(other.shape) != 2 {
			panic("matMul requires 2-D tensors")
		}
		if t.shape[0] != other.shape[0] {
			panic("matMul requires the shapes of tensors are compatible")
		}
		m, n, p := t.shape[1], t.shape[0], other.shape[1]
		result := make([]float32, m*p)
		for i := 0; i < m; i++ {
			for j := 0; j < p; j++ {
				for k := 0; k < n; k++ {
					result[i*p+j] += t.data[k*m+i] * other.data[j*n+k]
				}
			}
		}
		return &Tensor{
			data:  result,
			shape: []int{m, p},
		}
	}
}

func (t *Tensor) batchMatMulLegacy(other *Tensor, transpose1, transpose2 bool) *Tensor {
	if !transpose1 && !transpose2 {
		if len(t.shape) != 3 || len(other.shape) != 3 {
			panic("BatchMatMul requires 3-D tensors")
		}
		if t.shape[0] != other.shape[0] || t.shape[2] != other.shape[1] {
			panic("BatchMatMul requires the shapes of tensors are compatible")
		}
		m, n, p := t.shape[0], t.shape[1], other.shape[2]
		result := make([]float32, m*n*p)
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				for k := 0; k < p; k++ {
					for l := 0; l < t.shape[2]; l++ {
						result[i*n*p+j*p+k] += t.data[i*n*t.shape[2]+j*t.shape[2]+l] * other.data[i*other.shape[1]*other.shape[2]+l*other.shape[2]+k]
					}
				}
			}
		}
		return &Tensor{
			data:  result,
			shape: []int{m, n, p},
		}
	} else if transpose1 && !transpose2 {
		if len(t.shape) != 3 || len(other.shape) != 3 {
			panic("batchMatMul requires 3-D tensors")
		}
		if t.shape[0] != other.shape[0] || t.shape[1] != other.shape[1] {
			panic("batchMatMul requires the shapes of tensors are compatible")
		}
		m, n, p := t.shape[0], t.shape[2], other.shape[2]
		result := make([]float32, m*n*p)
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				for k := 0; k < p; k++ {
					for l := 0; l < t.shape[1]; l++ {
						result[i*n*p+j*p+k] += t.data[i*t.shape[1]*t.shape[2]+l*t.shape[2]+j] * other.data[i*other.shape[1]*other.shape[2]+l*other.shape[2]+k]
					}
				}
			}
		}
		return &Tensor{
			data:  result,
			shape: []int{m, n, p},
		}
	} else if !transpose1 && transpose2 {
		if len(t.shape) != 3 || len(other.shape) != 3 {
			panic("batchMatMul requires 3-D tensors")
		}
		if t.shape[0] != other.shape[0] || t.shape[2] != other.shape[2] {
			panic("batchMatMul requires the shapes of tensors are compatible")
		}
		m, n, p := t.shape[0], t.shape[1], other.shape[1]
		result := make([]float32, m*n*p)
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				for k := 0; k < p; k++ {
					for l := 0; l < t.shape[2]; l++ {
						result[i*n*p+j*p+k] += t.data[i*n*t.shape[2]+j*t.shape[2]+l] * other.data[i*other.shape[1]*other.shape[2]+k*other.shape[2]+l]
					}
				}
			}
		}
		return &Tensor{
			data:  result,
			shape: []int{m, n, p},
		}
	} else {
		if len(t.shape) != 3 || len(other.shape) != 3 {
			panic("batchMatMul requires 3-D tensors")
		}
		if t.shape[0] != other.shape[0] || t.shape[2] != other.shape[2] {
			panic("batchMatMul requires the shapes of tensors are compatible")
		}
		m, n, p := t.shape[1], t.shape[2], other.shape[2]
		result := make([]float32, m*n*p)
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				for k := 0; k < p; k++ {
					for l := 0; l < t.shape[0]; l++ {
						result[i*n*p+j*p+k] += t.data[l*t.shape[1]*t.shape[2]+i*t.shape[2]+j] * other.data[l*other.shape[1]*other.shape[2]+j*other.shape[2]+k]
					}
				}
			}
		}
		return &Tensor{
			data:  result,
			shape: []int{m, n, p},
		}
	}
}

func BenchmarkMatMulLegacy64(b *testing.B) {
	x := Rand(64, 64)
	y := Rand(64, 64)
	for t1 := 0; t1 < 2; t1++ {
		for t2 := 0; t2 < 2; t2++ {
			b.Run(fmt.Sprintf("(%d,%d)", t1, t2), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					x.matMulLegacy(y, t1 == 1, t2 == 1)
				}
			})
		}
	}
}

func BenchmarkMatMul64(b *testing.B) {
	x := Rand(64, 64)
	y := Rand(64, 64)
	for t1 := 0; t1 < 2; t1++ {
		for t2 := 0; t2 < 2; t2++ {
			b.Run(fmt.Sprintf("(%d,%d)", t1, t2), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					x.matMul(y, t1 == 1, t2 == 1, 0)
				}
			})
		}
	}
}

func BenchmarkBatchMatMulLegacy64(b *testing.B) {
	x := Rand(64, 64, 64)
	y := Rand(64, 64, 64)
	for t1 := 0; t1 < 2; t1++ {
		for t2 := 0; t2 < 2; t2++ {
			b.Run(fmt.Sprintf("(%d,%d)", t1, t2), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					x.batchMatMulLegacy(y, t1 == 1, t2 == 1)
				}
			})
		}
	}
}

func BenchmarkBatchMatMul64(b *testing.B) {
	x := Rand(64, 64, 64)
	y := Rand(64, 64, 64)
	for t1 := 0; t1 < 2; t1++ {
		for t2 := 0; t2 < 2; t2++ {
			b.Run(fmt.Sprintf("(%d,%d)", t1, t2), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					x.batchMatMul(y, t1 == 1, t2 == 1, 0)
				}
			})
		}
	}
}
