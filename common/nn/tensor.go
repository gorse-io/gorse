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
	"github.com/chewxy/math32"
	"github.com/google/uuid"
	"math/rand"
	"strings"
)

type Tensor struct {
	data  []float32
	shape []int
	grad  *Tensor
	op    op

	requireGrad bool
	id          uuid.UUID // Only assigned if requireGrad is true
}

func NewTensor(data []float32, shape ...int) *Tensor {
	size := 1
	for i := range shape {
		size *= shape[i]
	}
	if len(data) != size {
		panic(fmt.Sprintf("shape %v does not match data size %v", shape, len(data)))
	}
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

func NewScalar(data float32) *Tensor {
	return &Tensor{
		data:  []float32{data},
		shape: []int{},
	}
}

func LinSpace(start, end float32, shape ...int) *Tensor {
	n := 1
	for _, s := range shape {
		n *= s
	}
	data := make([]float32, n)
	delta := (end - start) / float32(n-1)
	for i := range data {
		data[i] = start + delta*float32(i)
	}
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

func RandN(shape ...int) *Tensor {
	n := 1
	for _, s := range shape {
		n *= s
	}
	data := make([]float32, n)
	for i := range data {
		data[i] = rand.Float32()
	}
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

// Ones creates a tensor filled with ones.
func Ones(shape ...int) *Tensor {
	n := 1
	for _, s := range shape {
		n *= s
	}
	data := make([]float32, n)
	for i := range data {
		data[i] = 1
	}
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

// Zeros creates a tensor filled with zeros.
func Zeros(shape ...int) *Tensor {
	n := 1
	for _, s := range shape {
		n *= s
	}
	data := make([]float32, n)
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

func (t *Tensor) IsScalar() bool {
	return len(t.shape) == 0
}

// NoGrad creates a tensor does not require gradient.
func (t *Tensor) NoGrad() *Tensor {
	if t.op != nil {
		t.op = nil
	}
	return t
}

func (t *Tensor) RequireGrad() *Tensor {
	t.requireGrad = true
	t.id = uuid.New()
	return t
}

func (t *Tensor) Shape() []int {
	return t.shape
}

// Slice returns a slice of the tensor.
func (t *Tensor) Slice(start, end int) *Tensor {
	if len(t.shape) < 1 {
		panic("slice requires at least 1-D tensor")
	}
	if start < 0 || end > t.shape[0] {
		panic("slice out of range")
	}
	subSize := 1
	for i := 1; i < len(t.shape); i++ {
		subSize *= t.shape[i]
	}
	return &Tensor{
		data:  t.data[start*subSize : end*subSize],
		shape: append([]int{end - start}, t.shape[1:]...),
	}
}

// Get returns the value of the tensor at the given indices.
func (t *Tensor) Get(indices ...int) float32 {
	if len(indices) != len(t.shape) {
		panic("the number of indices does not match the shape of the tensor")
	}
	index := 0
	for i := range indices {
		if indices[i] < 0 || indices[i] >= t.shape[i] {
			panic("index out of range")
		}
		index = index*t.shape[i] + indices[i]
	}
	return t.data[index]
}

func (t *Tensor) String() string {
	// Print scalar value
	if len(t.shape) == 0 {
		return fmt.Sprint(t.data[0])
	}

	builder := strings.Builder{}
	builder.WriteString("[")
	if len(t.data) <= 10 {
		for i := 0; i < len(t.data); i++ {
			builder.WriteString(fmt.Sprint(t.data[i]))
			if i != len(t.data)-1 {
				builder.WriteString(", ")
			}
		}
	} else {
		for i := 0; i < 5; i++ {
			builder.WriteString(fmt.Sprint(t.data[i]))
			builder.WriteString(", ")
		}
		builder.WriteString("..., ")
		for i := len(t.data) - 5; i < len(t.data); i++ {
			builder.WriteString(fmt.Sprint(t.data[i]))
			if i != len(t.data)-1 {
				builder.WriteString(", ")
			}
		}
	}
	builder.WriteString("]")
	return builder.String()
}

func (t *Tensor) Backward() {
	t.grad = Ones(t.shape...)
	ops := []op{t.op}
	for len(ops) > 0 {
		op := ops[0]
		ops = ops[1:]
		inputs, output := op.inputsAndOutput()
		grads := op.backward(output.grad)
		// Clear gradient of non-leaf tensor
		output.grad = nil
		for i := range grads {
			if inputs[i].grad == nil {
				inputs[i].grad = grads[i]
			} else {
				inputs[i].grad.add(grads[i])
			}
			if inputs[i].op != nil {
				ops = append(ops, inputs[i].op)
			} else if !inputs[i].requireGrad {
				// Clear gradient if the leaf tensor does not require gradient
				inputs[i].grad = nil
			}
		}
	}
}

func (t *Tensor) Grad() *Tensor {
	return t.grad
}

func (t *Tensor) Data() []float32 {
	return t.data
}

func (t *Tensor) clone() *Tensor {
	newData := make([]float32, len(t.data))
	copy(newData, t.data)
	return &Tensor{
		data:  newData,
		shape: t.shape,
	}
}

func (t *Tensor) add(other *Tensor) *Tensor {
	wSize := 1
	for i := range other.shape {
		wSize *= other.shape[i]
	}
	for i := range t.data {
		t.data[i] += other.data[i%wSize]
	}
	return t
}

func (t *Tensor) sub(other *Tensor) *Tensor {
	wSize := 1
	for i := range other.shape {
		wSize *= other.shape[i]
	}
	for i := range t.data {
		t.data[i] -= other.data[i%wSize]
	}
	return t
}

func (t *Tensor) mul(other *Tensor) *Tensor {
	wSize := 1
	for i := range other.shape {
		wSize *= other.shape[i]
	}
	for i := range t.data {
		t.data[i] *= other.data[i%wSize]
	}
	return t
}

func (t *Tensor) div(other *Tensor) *Tensor {
	wSize := 1
	for i := range other.shape {
		wSize *= other.shape[i]
	}
	for i := range t.data {
		t.data[i] /= other.data[i%wSize]
	}
	return t
}

func (t *Tensor) square() *Tensor {
	for i := range t.data {
		t.data[i] = t.data[i] * t.data[i]
	}
	return t
}

func (t *Tensor) pow(other *Tensor) *Tensor {
	wSize := 1
	for i := range other.shape {
		wSize *= other.shape[i]
	}
	for i := range t.data {
		t.data[i] = math32.Pow(t.data[i], other.data[i%wSize])
	}
	return t
}

func (t *Tensor) exp() *Tensor {
	for i := range t.data {
		t.data[i] = math32.Exp(t.data[i])
	}
	return t
}

func (t *Tensor) log() *Tensor {
	for i := range t.data {
		t.data[i] = math32.Log(t.data[i])
	}
	return t
}

func (t *Tensor) sin() *Tensor {
	for i := range t.data {
		t.data[i] = math32.Sin(t.data[i])
	}
	return t
}

func (t *Tensor) cos() *Tensor {
	for i := range t.data {
		t.data[i] = math32.Cos(t.data[i])
	}
	return t
}

func (t *Tensor) tanh() *Tensor {
	for i := range t.data {
		t.data[i] = math32.Tanh(t.data[i])
	}
	return t
}

func (t *Tensor) neg() *Tensor {
	for i := range t.data {
		t.data[i] = -t.data[i]
	}
	return t
}

func (t *Tensor) matMul(other *Tensor, transpose1, transpose2 bool) *Tensor {
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

func (t *Tensor) batchMatMul(other *Tensor, transpose1, transpose2 bool) *Tensor {
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

func (t *Tensor) maximum(other *Tensor) {
	if other.IsScalar() {
		for i := range t.data {
			t.data[i] = math32.Max(t.data[i], other.data[0])
		}
	} else {
		for i := range t.data {
			t.data[i] = math32.Max(t.data[i], other.data[i])
		}
	}
}
