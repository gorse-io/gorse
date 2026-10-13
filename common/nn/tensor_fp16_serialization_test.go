// Copyright 2026 gorse Project Authors
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
	"math"
	"testing"

	"github.com/gorse-io/gorse/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

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
		require.Equal(t, values, y.Uint16s())
		require.Equal(t, x.Shape(), y.Shape())
		require.Nil(t, y.grad)
		require.Nil(t, y.op)
		require.Nil(t, y.data)
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
		require.Equal(t, values, result.W.Uint16s())
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
		require.Nil(t, y.data16)
		require.Len(t, y.Floats(), len(bits))
		for i, b := range bits {
			require.Equal(t, b, math.Float32bits(y.Floats()[i]))
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
			x.Uint16s()[0] = 0
			require.Zero(t, binary.LittleEndian.Uint16(pb.Data))
			binary.LittleEndian.PutUint16(pb.Data, 0x4000)
		} else {
			x.Floats()[0] = 0
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
			y.Uint16s()[0] = 0x3c00
			require.Equal(t, uint16(0x3c00), binary.LittleEndian.Uint16(pb.Data))
		} else {
			y.Floats()[0] = 1
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
	require.Equal(t, []float32{1, 2}, x.Floats())
	require.Nil(t, x.data16)
	require.Equal(t, wire, func() []byte { b, err := proto.Marshal(x.toPB()); require.NoError(t, err); return b }())
}
