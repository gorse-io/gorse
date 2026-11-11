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
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/gorse-io/gorse/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestTensor_Slice(t *testing.T) {
	x := Rand(nil, 3, 4, 5)
	y := x.Slice(1, 3)
	assert.Equal(t, []int{2, 4, 5}, y.Shape())
	for i := range 2 {
		for j := range 4 {
			for k := range 5 {
				assert.Equal(t, x.Get(i+1, j, k), y.Get(i, j, k))
			}
		}
	}
}

func TestTensor_SliceIndices(t *testing.T) {
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 3, 2)
	y := x.SliceIndices(2, 0)
	assert.Equal(t, []int{2, 2}, y.Shape())
	assert.Equal(t, []float32{5, 6, 1, 2}, y.Data[float32]())
}

func TestTensor_Max(t *testing.T) {
	x := NewTensor([]float32{3, 2, 5, 6, 0, 0}, 6)
	y := x.max(0, false)
	assert.Len(t, y.shape, 0)
	assert.Equal(t, []float32{6}, y.Data[float32]())

	assert.Panics(t, func() { x.max(-1, false) })
	assert.Panics(t, func() { x.max(2, false) })

	x = NewTensor([]float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, 3, 2, 2)
	y = x.max(1, false)
	assert.Equal(t, []int{3, 2}, y.shape)
	assert.Equal(t, []float32{3, 4, 7, 8, 11, 12}, y.Data[float32]())
	y = x.max(1, true)
	assert.Equal(t, []int{3, 1, 2}, y.shape)
	assert.Equal(t, []float32{3, 4, 7, 8, 11, 12}, y.Data[float32]())
}

func TestTensor_Sum(t *testing.T) {
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 6)
	y := x.sum(0, false)
	assert.Len(t, y.shape, 0)
	assert.Equal(t, []float32{21}, y.Data[float32]())

	assert.Panics(t, func() { x.sum(-1, false) })
	assert.Panics(t, func() { x.sum(2, false) })

	x = NewTensor([]float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, 3, 2, 2)
	y = x.sum(1, false)
	assert.Equal(t, []int{3, 2}, y.shape)
	assert.Equal(t, []float32{4, 6, 12, 14, 20, 22}, y.Data[float32]())
	y = x.sum(1, true)
	assert.Equal(t, []int{3, 1, 2}, y.shape)
	assert.Equal(t, []float32{4, 6, 12, 14, 20, 22}, y.Data[float32]())
}

func TestTensor_Transpose(t *testing.T) {
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 3, 2)
	y := x.transpose()
	assert.Equal(t, []int{2, 3}, y.Shape())
	assert.Equal(t, []float32{1, 3, 5, 2, 4, 6}, y.Data[float32]())
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
		for i := range m {
			for j := range p {
				for k := range n {
					result[i*p+j] += t.Data[float32]()[i*n+k] * other.Data[float32]()[k*p+j]
				}
			}
		}
		return &Tensor{
			data:  toBytes(result),
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
		for i := range m {
			for j := range p {
				for k := range n {
					result[i*p+j] += t.Data[float32]()[k*m+i] * other.Data[float32]()[k*p+j]
				}
			}
		}
		return &Tensor{
			data:  toBytes(result),
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
		for i := range m {
			for j := range p {
				for k := range n {
					result[i*p+j] += t.Data[float32]()[i*n+k] * other.Data[float32]()[j*n+k]
				}
			}
		}
		return &Tensor{
			data:  toBytes(result),
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
		for i := range m {
			for j := range p {
				for k := range n {
					result[i*p+j] += t.Data[float32]()[k*m+i] * other.Data[float32]()[j*n+k]
				}
			}
		}
		return &Tensor{
			data:  toBytes(result),
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
		for i := range m {
			for j := range n {
				for k := range p {
					for l := 0; l < t.shape[2]; l++ {
						result[i*n*p+j*p+k] += t.Data[float32]()[i*n*t.shape[2]+j*t.shape[2]+l] * other.Data[float32]()[i*other.shape[1]*other.shape[2]+l*other.shape[2]+k]
					}
				}
			}
		}
		return &Tensor{
			data:  toBytes(result),
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
		for i := range m {
			for j := range n {
				for k := range p {
					for l := 0; l < t.shape[1]; l++ {
						result[i*n*p+j*p+k] += t.Data[float32]()[i*t.shape[1]*t.shape[2]+l*t.shape[2]+j] * other.Data[float32]()[i*other.shape[1]*other.shape[2]+l*other.shape[2]+k]
					}
				}
			}
		}
		return &Tensor{
			data:  toBytes(result),
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
		for i := range m {
			for j := range n {
				for k := range p {
					for l := 0; l < t.shape[2]; l++ {
						result[i*n*p+j*p+k] += t.Data[float32]()[i*n*t.shape[2]+j*t.shape[2]+l] * other.Data[float32]()[i*other.shape[1]*other.shape[2]+k*other.shape[2]+l]
					}
				}
			}
		}
		return &Tensor{
			data:  toBytes(result),
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
		for i := range m {
			for j := range n {
				for k := range p {
					for l := 0; l < t.shape[0]; l++ {
						result[i*n*p+j*p+k] += t.Data[float32]()[l*t.shape[1]*t.shape[2]+i*t.shape[2]+j] * other.Data[float32]()[l*other.shape[1]*other.shape[2]+j*other.shape[2]+k]
					}
				}
			}
		}
		return &Tensor{
			data:  toBytes(result),
			shape: []int{m, n, p},
		}
	}
}

func BenchmarkMatMulLegacy64(b *testing.B) {
	x := Rand(nil, 64, 64)
	y := Rand(nil, 64, 64)
	for t1 := range 2 {
		for t2 := range 2 {
			b.Run(fmt.Sprintf("(%d,%d)", t1, t2), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					x.matMulLegacy(y, t1 == 1, t2 == 1)
				}
			})
		}
	}
}

func BenchmarkMatMul64(b *testing.B) {
	x := Rand(nil, 64, 64)
	y := Rand(nil, 64, 64)
	for t1 := range 2 {
		for t2 := range 2 {
			b.Run(fmt.Sprintf("(%d,%d)", t1, t2), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					x.matMul(y, t1 == 1, t2 == 1, 0)
				}
			})
		}
	}
}

func BenchmarkBatchMatMulLegacy64(b *testing.B) {
	x := Rand(nil, 64, 64, 64)
	y := Rand(nil, 64, 64, 64)
	for t1 := range 2 {
		for t2 := range 2 {
			b.Run(fmt.Sprintf("(%d,%d)", t1, t2), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					x.batchMatMulLegacy(y, t1 == 1, t2 == 1)
				}
			})
		}
	}
}

func BenchmarkBatchMatMul64(b *testing.B) {
	x := Rand(nil, 64, 64, 64)
	y := Rand(nil, 64, 64, 64)
	for t1 := range 2 {
		for t2 := range 2 {
			b.Run(fmt.Sprintf("(%d,%d)", t1, t2), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					x.batchMatMul(y, t1 == 1, t2 == 1, 0)
				}
			})
		}
	}
}

func TestTensorData(t *testing.T) {
	fp32 := []float32{1, 2}
	x := NewTensor(fp32, 2)
	require.Equal(t, fp32, x.Data[float32]())
	x.Data[float32]()[0] = 3
	require.Equal(t, float32(3), fp32[0])
	fp32[1] = 4
	require.Equal(t, float32(4), x.Data[float32]()[1])

	fp16 := []uint16{0x3c00, 0x7e55}
	y := NewTensor(fp16, 2)
	require.Equal(t, fp16, y.Data[uint16]())
	y.Data[uint16]()[0] = 0x4000
	require.Equal(t, uint16(0x4000), fp16[0])
	fp16[1] = 0xfe01
	require.Equal(t, uint16(0xfe01), y.Data[uint16]()[1])

	require.Empty(t, NewTensor[float32](nil, 0).Data[float32]())
	require.Empty(t, NewTensor[uint16](nil, 0).Data[uint16]())
}

func TestTensorFloat32ViewAliases(t *testing.T) {
	x := NewTensor([]float32{1, 2, 3, 4}, 2, 2)
	for _, view := range []*Tensor{x.Slice(0, 2), Flatten(x), Reshape(x, 4)} {
		view.Data[float32]()[0]++
		require.Equal(t, view.Data[float32]()[0], x.Data[float32]()[0])
		x.Data[float32]()[1]++
		require.Equal(t, x.Data[float32]()[1], view.Data[float32]()[1])
	}
}

func TestTensorByteStorage(t *testing.T) {
	for _, x := range []*Tensor{
		NewTensor([]float32{1, math.Float32frombits(0x7f800055), math.Float32frombits(0x80000000)}, 3),
		NewTensor([]uint16{0x3c00, 0x7c55, 0x8000}, 3),
	} {
		width := 4
		if x.DType() == Float16 {
			width = 2
		}
		require.Len(t, x.data, 3*width)
		original := append([]byte(nil), x.data...)
		for _, copied := range []*Tensor{x.clone(), x.SliceIndices(0, 1, 2)} {
			require.Equal(t, original, copied.data)
			clear(copied.data)
			require.Equal(t, original, x.data)
		}
		sliced := x.Slice(1, 3)
		require.Equal(t, original[width:], sliced.data)
		sliced.data[0] ^= 1
		require.Equal(t, sliced.data[0], x.data[width])
		pb := x.toPB()
		require.Equal(t, x.data, pb.Data)
		pb.Data[0] ^= 1
		require.Equal(t, pb.Data[0], x.data[0])
		y := new(Tensor)
		y.fromPB(pb)
		y.data[0] ^= 1
		require.Equal(t, y.data[0], pb.Data[0])
		if x.DType() == Float16 {
			x.Data[uint16]()[0] = 0x4000
			require.Equal(t, uint16(0x4000), binary.LittleEndian.Uint16(x.data))
		} else {
			x.Data[float32]()[0] = 2
			require.Equal(t, math.Float32bits(2), binary.LittleEndian.Uint32(x.data))
		}
		require.Equal(t, float32(2), y.Get(0))
	}
}

func TestFloat16Serialization(t *testing.T) {
	for _, values := range [][]uint16{{0, 0x8000, 1, 0x3c00, 0x7c00, 0xfc00, 0x7c01, 0x7e55, 0xfe01}, {0x3c00}, nil} {
		shape := []int{len(values)}
		if len(values) == 1 {
			shape = []int{}
		}
		x := NewTensor(values, shape...)
		pb := x.toPB()
		require.Equal(t, protocol.TensorDType_FLOAT16, pb.Dtype)
		require.Len(t, pb.Data, len(values)*2)
		for i, v := range values {
			require.Equal(t, v, binary.LittleEndian.Uint16(pb.Data[2*i:]))
		}
		encoded, err := proto.Marshal(pb)
		require.NoError(t, err)
		decoded := new(protocol.Tensor)
		require.NoError(t, proto.Unmarshal(encoded, decoded))
		y := Ones(2)
		y.grad = Ones(2)
		y.op = &neg{}
		y.fromPB(decoded)
		require.Equal(t, Float16, y.DType())
		require.Equal(t, values, y.Data[uint16]())
		require.Equal(t, x.Shape(), y.Shape())
		require.Nil(t, y.grad)
		require.Nil(t, y.op)
		var buf bytes.Buffer
		model := struct {
			W *Tensor
			B *Tensor
		}{x, NewScalar(2)}
		require.NoError(t, Save(model, &buf))
		result := struct {
			W *Tensor
			B *Tensor
		}{new(Tensor), new(Tensor)}
		require.NoError(t, Load(result, &buf))
		require.Equal(t, values, result.W.Data[uint16]())
		require.Equal(t, Float32, result.B.DType())
		require.Equal(t, float32(2), result.B.Get())
	}
}

func TestFloat32SerializationRawBits(t *testing.T) {
	for _, bits := range [][]uint32{{0, 0x80000000, 1, 0x3f800000, 0x7f800000, 0xff800000, 0x7f800001, 0x7fc00055}, {0x3f800000}, nil} {
		values := make([]float32, len(bits))
		for i, b := range bits {
			values[i] = math.Float32frombits(b)
		}
		shape := []int{len(values)}
		if len(values) == 1 {
			shape = []int{}
		}
		x := NewTensor(values, shape...)
		pb := x.toPB()
		require.Equal(t, protocol.TensorDType_FLOAT32, pb.Dtype)
		require.Len(t, pb.Data, len(bits)*4)
		for i, b := range bits {
			require.Equal(t, b, binary.LittleEndian.Uint32(pb.Data[4*i:]))
		}
		var buf bytes.Buffer
		require.NoError(t, Save(x, &buf))
		y := Ones(1).ToFloat16()
		require.NoError(t, Load(y, &buf))
		require.Equal(t, Float32, y.DType())
		require.Equal(t, shape, y.Shape())
		require.Len(t, y.Data[float32](), len(bits))
		for i, b := range bits {
			require.Equal(t, b, math.Float32bits(y.Data[float32]()[i]))
		}
	}
}

func TestTensorSerializationSharesStorage(t *testing.T) {
	for _, x := range []*Tensor{
		NewTensor([]uint16{0x3c00, 0x7e55}, 2),
		NewTensor([]float32{1, math.Float32frombits(0x7fc00055)}, 2),
	} {
		pb := x.toPB()
		if x.DType() == Float16 {
			x.Data[uint16]()[0] = 0
			require.Zero(t, binary.LittleEndian.Uint16(pb.Data))
			binary.LittleEndian.PutUint16(pb.Data, 0x4000)
		} else {
			x.Data[float32]()[0] = 0
			require.Zero(t, binary.LittleEndian.Uint32(pb.Data))
			binary.LittleEndian.PutUint32(pb.Data, math.Float32bits(2))
		}
		require.Equal(t, float32(2), x.Get(0))

		// Decode from an intentionally unaligned byte slice.
		pb.Data = append([]byte{0}, pb.Data...)[1:]
		y := new(Tensor)
		y.fromPB(pb)
		require.Equal(t, float32(2), y.Get(0))
		clear(pb.Data)
		require.Equal(t, float32(0), y.Get(0))
		if y.DType() == Float16 {
			y.Data[uint16]()[0] = 0x3c00
			require.Equal(t, uint16(0x3c00), binary.LittleEndian.Uint16(pb.Data))
		} else {
			y.Data[float32]()[0] = 1
			require.Equal(t, math.Float32bits(1), binary.LittleEndian.Uint32(pb.Data))
		}
	}
}

func TestTensorLegacySerialization(t *testing.T) {
	// Original protobuf fields only: shape=[2], data=[1,2], no dtype.
	wire := []byte{0x12, 1, 2, 0x1a, 8, 0, 0, 0x80, 0x3f, 0, 0, 0, 0x40}
	pb := new(protocol.Tensor)
	require.NoError(t, proto.Unmarshal(wire, pb))
	x := Ones(2).ToFloat16()
	x.fromPB(pb)
	require.Equal(t, Float32, x.DType())
	require.Equal(t, []float32{1, 2}, x.Data[float32]())
	require.Equal(t, wire, func() []byte { b, err := proto.Marshal(x.toPB()); require.NoError(t, err); return b }())
}

func TestFloat16Storage(t *testing.T) {
	data := []uint16{0x3c00, 0x4000, 0x4200, 0x4400}
	x := NewTensor(data, 2, 2)
	require.Equal(t, Float16, x.DType())
	require.Equal(t, float32(4), x.Get(1, 1))
	require.Equal(t, "[1, 2, 3, 4]", x.String())
	data[0] = uint16(0x4500)
	require.Equal(t, float32(5), x.Get(0, 0))
	x.Slice(1, 2).Data[uint16]()[0] = uint16(0x4600)
	require.Equal(t, float32(6), x.Get(1, 0))
	for _, copy := range []*Tensor{x.SliceIndices(1, 0), x.clone(), x.ToFloat16()} {
		require.Equal(t, Float16, copy.DType())
		copy.Data[uint16]()[0] = uint16(0x4880)
		require.Equal(t, float32(5), x.Get(0, 0))
		require.Nil(t, copy.Grad())
		require.Nil(t, copy.op)
	}
	y := x.ToFloat32()
	require.Equal(t, Float32, y.DType())
	require.Equal(t, []float32{5, 2, 6, 4}, y.Data[float32]())
	y.Data[float32]()[0] = 10
	require.Equal(t, float32(5), x.Get(0, 0))
	z := y.ToFloat32()
	z.Data[float32]()[0] = 11
	require.Equal(t, float32(10), y.Get(0, 0))
	z.Shape()[0] = 1
	require.Equal(t, []int{2, 2}, y.Shape())
	require.Equal(t, "[]", NewTensor[uint16](nil, 0).String())
	require.Equal(t, "1", NewTensor([]uint16{0x3c00}).String())
	require.Panics(t, func() { NewTensor(data, 3) })
}

func TestFloat16Conversion(t *testing.T) {
	values := []float32{1.00048828125, 1.00146484375, 0x1p-24, 0x1p-25, float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())}
	x := NewTensor(values, len(values))
	x.grad = Ones(len(values))
	x.op = &neg{}
	y := x.ToFloat16()
	require.Equal(t, []uint16{0x3c00, 0x3c02, 1, 0, 0x7c00, 0xfc00}, y.Data[uint16]()[:6])
	require.True(t, math.IsNaN(float64(y.Get(6))))
	require.Nil(t, y.grad)
	require.Nil(t, y.op)
	require.Equal(t, float32(0x1p-24), y.ToFloat32().Get(2))
	require.Equal(t, Float32, NewTensor([]float32{1}, 1).DType())
}
