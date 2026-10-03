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
	"container/heap"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/chewxy/math32"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/common/floats"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/protocol"
	"github.com/samber/lo"
	"github.com/x448/float16"
	"golang.org/x/exp/slices"
)

// inferenceMode disables gradient computation when enabled, improving performance during model evaluation.
var inferenceMode = atomic.Bool{}

// SetInferenceMode enables or disables inference mode, which disables gradient computation to improve performance during model evaluation.
func SetInferenceMode(enabled bool) {
	inferenceMode.Store(enabled)
}

// DType identifies a tensor's storage format.
type DType uint8

const (
	// Float32 stores IEEE 754 single-precision values and supports computation.
	Float32 DType = iota
	// Float16 stores IEEE 754 half-precision values; convert to Float32 for computation.
	Float16
)

// Tensor stores dense values and, for Float32 tensors, an optional computation graph.
type Tensor struct {
	data   []float32
	data16 []uint16
	dtype  DType
	shape  []int
	grad   *Tensor
	op     op
}

// DType returns the tensor's storage format.
func (t *Tensor) DType() DType { return t.dtype }

// Data16 returns the shared IEEE 754 half-precision bit patterns.
func (t *Tensor) Data16() []uint16 {
	return t.data16
}

// ToFloat16 returns an independent, graph-free copy in half precision.
// Float32 values are rounded to nearest, with ties to even.
func (t *Tensor) ToFloat16() *Tensor {
	if t.dtype == Float16 {
		return t.clone()
	}
	data := make([]uint16, len(t.data))
	for i, value := range t.data {
		data[i] = uint16(float16.Fromfloat32(value))
	}
	return NewTensor(data, slices.Clone(t.shape)...)
}

// ToFloat32 returns an independent, graph-free copy in single precision.
func (t *Tensor) ToFloat32() *Tensor {
	if t.dtype == Float32 {
		return t.clone()
	}
	data := make([]float32, len(t.data16))
	for i, value := range t.data16 {
		data[i] = float16.Frombits(value).Float32()
	}
	return NewTensor(data, slices.Clone(t.shape)...)
}

// NewTensor creates a tensor sharing data and shape with its inputs.
// Float32 values support computation; uint16 values are storage-only IEEE 754
// half-precision bit patterns, not integer-valued tensor elements.
func NewTensor[T float32 | uint16](data []T, shape ...int) *Tensor {
	size := 1
	for i := range shape {
		size *= shape[i]
	}
	if len(data) != size {
		panic(fmt.Sprintf("shape %v does not match data size %v", shape, len(data)))
	}
	t := &Tensor{shape: shape}
	switch values := any(data).(type) {
	case []float32:
		t.data = values
	case []uint16:
		t.data16 = values
		t.dtype = Float16
	}
	return t
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

func Rand(rng *rand.Rand, shape ...int) *Tensor {
	random := rand.Float32
	if rng != nil {
		random = rng.Float32
	}
	n := 1
	for _, s := range shape {
		n *= s
	}
	data := make([]float32, n)
	for i := range data {
		data[i] = random()
	}
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

func Uniform(low, high float32, shape ...int) *Tensor {
	n := 1
	for _, s := range shape {
		n *= s
	}
	data := make([]float32, n)
	for i := range data {
		data[i] = rand.Float32()*(high-low) + low
	}
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

func Normal(mean, std float32, shape ...int) *Tensor {
	n := 1
	for _, s := range shape {
		n *= s
	}
	data := make([]float32, n)
	for i := range data {
		data[i] = float32(rand.NormFloat64())*std + mean
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

func (t *Tensor) generation() int {
	if t.op != nil {
		return t.op.generation()
	}
	return 0
}

func (t *Tensor) IsScalar() bool {
	return len(t.shape) == 0
}

// NoGrad convert a node tensor to a leaf tensor.
func (t *Tensor) NoGrad() *Tensor {
	if t.op != nil {
		t.op = nil
	}
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
	if t.dtype == Float16 {
		return NewTensor(t.data16[start*subSize:end*subSize], append([]int{end - start}, t.shape[1:]...)...)
	}
	return &Tensor{
		data:  t.data[start*subSize : end*subSize],
		shape: append([]int{end - start}, t.shape[1:]...),
	}
}

func (t *Tensor) SliceIndices(indices ...int) *Tensor {
	shape := []int{len(indices)}
	subSize := 1
	for i := range t.shape[1:] {
		shape = append(shape, t.shape[i+1])
		subSize *= t.shape[i+1]
	}
	if t.dtype == Float16 {
		data := make([]uint16, len(indices)*subSize)
		for i, index := range indices {
			copy(data[i*subSize:(i+1)*subSize], t.data16[index*subSize:(index+1)*subSize])
		}
		return NewTensor(data, shape...)
	}
	data := make([]float32, len(indices)*subSize)
	for i, index := range indices {
		copy(data[i*subSize:(i+1)*subSize], t.data[index*subSize:(index+1)*subSize])
	}
	return &Tensor{
		data:  data,
		shape: shape,
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
	if t.dtype == Float16 {
		return float16.Frombits(t.data16[index]).Float32()
	}
	return t.data[index]
}

func (t *Tensor) String() string {
	size := len(t.data)
	value := func(i int) float32 { return t.data[i] }
	if t.dtype == Float16 {
		size = len(t.data16)
		value = func(i int) float32 { return float16.Frombits(t.data16[i]).Float32() }
	}
	// Print scalar value
	if len(t.shape) == 0 {
		return fmt.Sprint(value(0))
	}

	builder := strings.Builder{}
	builder.WriteString("[")
	if size <= 10 {
		for i := 0; i < size; i++ {
			fmt.Fprint(&builder, value(i))
			if i != size-1 {
				builder.WriteString(", ")
			}
		}
	} else {
		for i := range 5 {
			fmt.Fprint(&builder, value(i))
			builder.WriteString(", ")
		}
		builder.WriteString("..., ")
		for i := size - 5; i < size; i++ {
			fmt.Fprint(&builder, value(i))
			if i != size-1 {
				builder.WriteString(", ")
			}
		}
	}
	builder.WriteString("]")
	return builder.String()
}

func (t *Tensor) Backward() {
	t.grad = Ones(t.shape...)
	ops := &opHeap{t.op}
	seen := mapset.NewSet[op](t.op)
	for ops.Len() > 0 {
		op := heap.Pop(ops).(op)
		inputs, output := op.inputsAndOutput()
		grads := op.backward(output.grad)
		for i := range grads {
			if !slices.Equal(inputs[i].shape, grads[i].shape) {
				panic(fmt.Sprintf("%s: shape %v does not match shape %v", op.String(), inputs[i].shape, grads[i].shape))
			}
			if inputs[i].grad == nil {
				inputs[i].grad = grads[i]
			} else {
				inputs[i].grad.add(grads[i])
			}
			if inputs[i].op != nil && !seen.Contains(inputs[i].op) {
				heap.Push(ops, inputs[i].op)
				seen.Add(inputs[i].op)
			}
		}
		output.grad = nil
	}
}

func (t *Tensor) Grad() *Tensor {
	return t.grad
}

// Data returns the shared single-precision storage.
func (t *Tensor) Data() []float32 {
	return t.data
}

func (t *Tensor) clone() *Tensor {
	if t.dtype == Float16 {
		return NewTensor(append([]uint16(nil), t.data16...), slices.Clone(t.shape)...)
	}
	newData := make([]float32, len(t.data))
	copy(newData, t.data)
	return &Tensor{
		data:  newData,
		shape: slices.Clone(t.shape),
	}
}

func (t *Tensor) add(other *Tensor) *Tensor {
	wSize := 1
	for i := range other.shape {
		wSize *= other.shape[i]
	}
	if wSize == 1 {
		floats.AddConst(t.data, other.data[0])
	} else {
		for i := 0; i < len(t.data); i += wSize {
			floats.Add(t.data[i:i+wSize], other.data)
		}
	}
	return t
}

// sub returns the element-wise addition of two tensors. The shape
// of the second tensor must be a suffix sequence of the shape of
// the first tensor: (...,m,n) - (m,n) = (...,m,n).
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

// bSub returns the element-wise addition of two tensors. The shape
// of the second tensor must be a prefix sequence of the shape of
// the first tensor: (m,n,...) - (m,n) = (m,n,...).
func (t *Tensor) bSub(other *Tensor) *Tensor {
	bSize := 1
	for i := range t.shape {
		bSize *= t.shape[i]
	}
	for i := range other.shape {
		bSize /= other.shape[i]
	}
	for i := range t.data {
		t.data[i] -= other.data[i/bSize]
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

// div returns the element-wise division of two tensors. The shape
// of the second tensor must be a suffix sequence of the shape of
// the first tensor: (...,m,n) / (m,n) = (...,m,n).
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

// bDiv returns the element-wise division of two tensors. The shape
// of the second tensor must be a prefix sequence of the shape of
// the first tensor: (m,n,...) / (m,n) = (m,n,...).
func (t *Tensor) bDiv(other *Tensor) *Tensor {
	bSize := 1
	for i := range t.shape {
		bSize *= t.shape[i]
	}
	for i := range other.shape {
		bSize /= other.shape[i]
	}
	for i := range t.data {
		t.data[i] /= other.data[i/bSize]
	}
	return t
}

func (t *Tensor) square() *Tensor {
	floats.MulTo(t.data, t.data, t.data)
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
		t.data[i] = float32(math.Exp(float64(t.data[i])))
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

func partition(n, p int) []lo.Tuple2[int, int] {
	// If n is less than or equal to 0, return nil.
	if n <= 0 {
		return nil
	}

	// If p is less than or equal to 1, return a single part covering the whole range.
	if p <= 1 {
		return []lo.Tuple2[int, int]{{A: 0, B: n}}
	}

	// If n is less than or equal to p, return each index as a separate part.
	if n <= p {
		return lo.Map(lo.Range(n), func(i int, _ int) lo.Tuple2[int, int] {
			return lo.Tuple2[int, int]{A: i, B: i + 1}
		})
	}

	// Otherwise, split n into p parts as evenly as possible.
	minPartSize := n / p
	maxPartCount := n % p
	parts := make([]lo.Tuple2[int, int], 0, p)
	for i := 0; i < n; {
		partSize := minPartSize
		if maxPartCount > 0 {
			partSize++
			maxPartCount--
		}
		end := min(i+partSize, n)
		parts = append(parts, lo.Tuple2[int, int]{A: i, B: end})
		i = end
	}
	return parts
}

func (t *Tensor) matMul(other *Tensor, transpose1, transpose2 bool, jobs int) *Tensor {
	if len(t.shape) != 2 || len(other.shape) != 2 {
		panic("matMul requires 2-D tensors")
	}
	var m, n, k int
	var result []float32
	var wg sync.WaitGroup
	if !transpose1 && !transpose2 {
		if t.shape[1] != other.shape[0] {
			panic(fmt.Sprintf("matMul requires the shapes of tensors are compatible, but got %v and %v", t.shape, other.shape))
		}
		m, n, k = t.shape[0], other.shape[1], t.shape[1]
		result = make([]float32, m*n)
		for _, p := range partition(m, jobs) {
			wg.Go(func() {
				floats.MM(transpose1, transpose2, p.B-p.A, n, k, t.data[p.A*k:], k, other.data, n, result[p.A*n:], n)
			})
		}
	} else if transpose1 && !transpose2 {
		if t.shape[0] != other.shape[0] {
			panic(fmt.Sprintf("matMul requires the shapes of tensors are compatible, but got %v and %v", t.shape, other.shape))
		}
		m, n, k = t.shape[1], other.shape[1], t.shape[0]
		result = make([]float32, m*n)
		for _, p := range partition(m, jobs) {
			wg.Go(func() {
				floats.MM(transpose1, transpose2, p.B-p.A, n, k, t.data[p.A:], m, other.data, n, result[p.A*n:], n)
			})
		}
	} else if !transpose1 && transpose2 {
		if t.shape[1] != other.shape[1] {
			panic(fmt.Sprintf("matMul requires the shapes of tensors are compatible, but got %v and %v", t.shape, other.shape))
		}
		m, n, k = t.shape[0], other.shape[0], t.shape[1]
		result = make([]float32, m*n)
		for _, p := range partition(m, jobs) {
			wg.Go(func() {
				floats.MM(transpose1, transpose2, p.B-p.A, n, k, t.data[p.A*k:], k, other.data, k, result[p.A*n:], n)
			})
		}
	} else {
		if t.shape[0] != other.shape[1] {
			panic(fmt.Sprintf("matMul requires the shapes of tensors are compatible, but got %v and %v", t.shape, other.shape))
		}
		m, n, k = t.shape[1], other.shape[0], t.shape[0]
		result = make([]float32, m*n)
		for _, p := range partition(m, jobs) {
			wg.Go(func() {
				floats.MM(transpose1, transpose2, p.B-p.A, n, k, t.data[p.A:], m, other.data, k, result[p.A*n:], n)
			})
		}
	}
	wg.Wait()
	return &Tensor{
		data:  result,
		shape: []int{m, n},
	}
}

func (t *Tensor) batchMatMul(other *Tensor, transpose1, transpose2 bool, jobs int) *Tensor {
	if len(t.shape) != 3 || len(other.shape) != 3 {
		panic("BatchMatMul requires 3-D tensors")
	}
	var b, m, n, k int
	var result []float32
	if !transpose1 && !transpose2 {
		if t.shape[0] != other.shape[0] || t.shape[2] != other.shape[1] {
			panic("BatchMatMul requires the shapes of tensors are compatible")
		}
		b, m, n, k = t.shape[0], t.shape[1], other.shape[2], t.shape[2]
		result = make([]float32, b*m*n)
		_ = parallel.For(context.Background(), b, jobs, func(i int) {
			floats.MM(transpose1, transpose2, m, n, k, t.data[i*m*k:(i+1)*m*k], k, other.data[i*n*k:(i+1)*n*k], n, result[i*m*n:(i+1)*m*n], n)
		})
	} else if transpose1 && !transpose2 {
		if t.shape[0] != other.shape[0] || t.shape[1] != other.shape[1] {
			panic("batchMatMul requires the shapes of tensors are compatible")
		}
		b, m, n, k = t.shape[0], t.shape[2], other.shape[2], t.shape[1]
		result = make([]float32, b*m*n)
		_ = parallel.For(context.Background(), b, jobs, func(i int) {
			floats.MM(transpose1, transpose2, m, n, k, t.data[i*m*k:(i+1)*m*k], m, other.data[i*n*k:(i+1)*n*k], n, result[i*m*n:(i+1)*m*n], n)
		})
	} else if !transpose1 && transpose2 {
		if t.shape[0] != other.shape[0] || t.shape[2] != other.shape[2] {
			panic("batchMatMul requires the shapes of tensors are compatible")
		}
		b, m, n, k = t.shape[0], t.shape[1], other.shape[1], t.shape[2]
		result = make([]float32, b*m*n)
		_ = parallel.For(context.Background(), b, jobs, func(i int) {
			floats.MM(transpose1, transpose2, m, n, k, t.data[i*m*k:(i+1)*m*k], k, other.data[i*n*k:(i+1)*n*k], k, result[i*m*n:(i+1)*m*n], n)
		})
	} else {
		if t.shape[0] != other.shape[0] || t.shape[1] != other.shape[2] {
			panic("batchMatMul requires the shapes of tensors are compatible")
		}
		b, m, n, k = t.shape[0], t.shape[2], other.shape[1], t.shape[1]
		result = make([]float32, b*m*n)
		_ = parallel.For(context.Background(), b, jobs, func(i int) {
			floats.MM(transpose1, transpose2, m, n, k, t.data[i*m*k:(i+1)*m*k], m, other.data[i*n*k:(i+1)*n*k], k, result[i*m*n:(i+1)*m*n], n)
		})
	}
	return &Tensor{
		data:  result,
		shape: []int{b, m, n},
	}
}

func (t *Tensor) maximum(other *Tensor) {
	if other.IsScalar() {
		for i := range t.data {
			t.data[i] = max(t.data[i], other.data[0])
		}
	} else {
		for i := range t.data {
			t.data[i] = max(t.data[i], other.data[i])
		}
	}
}

func (t *Tensor) gt(other *Tensor) *Tensor {
	if other.IsScalar() {
		for i := range t.data {
			if t.data[i] > other.data[0] {
				t.data[i] = 1
			} else {
				t.data[i] = 0
			}
		}
	} else {
		for i := range t.data {
			if t.data[i] > other.data[i] {
				t.data[i] = 1
			} else {
				t.data[i] = 0
			}
		}
	}
	return t
}

func (t *Tensor) transpose() *Tensor {
	if len(t.shape) < 2 {
		panic("transpose requires at least 2-D tensor")
	}
	shape := make([]int, 0, len(t.shape))
	batchSize := 1
	for i := 0; i < len(t.shape)-2; i++ {
		batchSize *= t.shape[i]
		shape = append(shape, t.shape[i])
	}
	m, n := t.shape[len(t.shape)-2], t.shape[len(t.shape)-1]
	shape = append(shape, n, m)
	data := make([]float32, batchSize*m*n)
	for b := 0; b < batchSize; b++ {
		for i := range m {
			for j := range n {
				data[b*m*n+j*m+i] = t.data[b*m*n+i*n+j]
			}
		}
	}
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

func (t *Tensor) max(axis int, keepDim bool) *Tensor {
	if axis < 0 || axis >= len(t.shape) {
		panic("axis out of range")
	}
	if len(t.shape) == 1 {
		return NewScalar(lo.Max(t.data))
	}
	shape := make([]int, 0, len(t.shape)-1)
	a, b, c := 1, 1, 1
	for i := 0; i < len(t.shape); i++ {
		if i < axis {
			shape = append(shape, t.shape[i])
			a *= t.shape[i]
		} else if i == axis {
			if keepDim {
				shape = append(shape, 1)
			}
			b = t.shape[i]
		} else {
			shape = append(shape, t.shape[i])
			c *= t.shape[i]
		}
	}
	data := make([]float32, a*c)
	for i := 0; i < a; i++ {
		for j := 0; j < c; j++ {
			maxValue := t.data[i*b*c+j]
			for k := 1; k < b; k++ {
				maxValue = max(maxValue, t.data[i*b*c+j+k*c])
			}
			data[i*c+j] = maxValue
		}
	}
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

func (t *Tensor) sum(axis int, keepDim bool) *Tensor {
	if axis < 0 || axis >= len(t.shape) {
		panic("axis out of range")
	}
	if len(t.shape) == 1 {
		return NewScalar(lo.Sum(t.data))
	}
	shape := make([]int, 0, len(t.shape)-1)
	a, b, c := 1, 1, 1
	for i := 0; i < len(t.shape); i++ {
		if i < axis {
			shape = append(shape, t.shape[i])
			a *= t.shape[i]
		} else if i == axis {
			if keepDim {
				shape = append(shape, 1)
			}
			b = t.shape[i]
		} else {
			shape = append(shape, t.shape[i])
			c *= t.shape[i]
		}
	}
	data := make([]float32, a*c)
	for i := 0; i < a; i++ {
		for j := 0; j < c; j++ {
			sumValue := t.data[i*b*c+j]
			for k := 1; k < b; k++ {
				sumValue += t.data[i*b*c+j+k*c]
			}
			data[i*c+j] = sumValue
		}
	}
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

func (t *Tensor) argmax() []int {
	if len(t.data) == 0 {
		return nil
	}
	maxValue := t.data[0]
	maxIndex := 0
	for i := 1; i < len(t.data); i++ {
		if t.data[i] > maxValue {
			maxValue = t.data[i]
			maxIndex = i
		}
	}
	indices := make([]int, len(t.shape))
	for i := len(t.shape) - 1; i >= 0; i-- {
		indices[i] = maxIndex % t.shape[i]
		maxIndex /= t.shape[i]
	}
	return indices
}

func (t *Tensor) toPB() *protocol.Tensor {
	pb := &protocol.Tensor{
		Shape: lo.Map(t.shape, func(i, _ int) int32 { return int32(i) }),
	}
	if t.dtype == Float16 {
		pb.Dtype = protocol.TensorDType_FLOAT16
		pb.Data = make([]byte, 2*len(t.data16))
		for i, value := range t.data16 {
			binary.LittleEndian.PutUint16(pb.Data[2*i:], value)
		}
	} else {
		pb.Data = make([]byte, 4*len(t.data))
		for i, value := range t.data {
			binary.LittleEndian.PutUint32(pb.Data[4*i:], math.Float32bits(value))
		}
	}
	return pb
}

func (t *Tensor) fromPB(pb *protocol.Tensor) {
	shape := make([]int, len(pb.Shape))
	for i, dim := range pb.Shape {
		shape[i] = int(dim)
	}
	decoded := Tensor{shape: shape}
	if pb.Dtype == protocol.TensorDType_FLOAT16 {
		decoded.dtype = Float16
		if len(pb.Data) > 0 {
			decoded.data16 = make([]uint16, len(pb.Data)/2)
			for i := range decoded.data16 {
				decoded.data16[i] = binary.LittleEndian.Uint16(pb.Data[2*i:])
			}
		}
	} else {
		decoded.data = make([]float32, len(pb.Data)/4)
		for i := range decoded.data {
			decoded.data[i] = math.Float32frombits(binary.LittleEndian.Uint32(pb.Data[4*i:]))
		}
	}
	*t = decoded
}

func NormalInit(rng *rand.Rand, t *Tensor, mean, std float32) {
	random := rand.NormFloat64
	if rng != nil {
		random = rng.NormFloat64
	}
	for i := range t.data {
		t.data[i] = float32(random())*(std) + (mean)
	}
}
