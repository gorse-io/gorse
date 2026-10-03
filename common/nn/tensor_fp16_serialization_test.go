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
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/stretchr/testify/require"
	"github.com/x448/float16"
	"google.golang.org/protobuf/proto"
)

func TestFloat16Serialization(t *testing.T) {
	for _, values := range [][]float16.Float16{{0, 0x8000, 1, 0x3c00, 0x7c00, 0xfc00, 0x7c01, 0x7e55, 0xfe01}, {0x3c00}, nil} {
		shape := []int{len(values)}
		if len(values) == 1 {
			shape = []int{}
		}
		x := NewTensor16(values, shape...)
		pb := x.toPB()
		require.Equal(t, protocol.TensorDType_FLOAT16, pb.Dtype)
		require.Len(t, pb.Data, len(values)*2)
		for i, v := range values {
			require.Equal(t, uint16(v), binary.LittleEndian.Uint16(pb.Data[2*i:]))
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
		require.Equal(t, values, y.Data16())
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
		require.Equal(t, values, result.W.Data16())
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
		require.Len(t, y.Data(), len(bits))
		for i, b := range bits {
			require.Equal(t, b, math.Float32bits(y.Data()[i]))
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
	require.Equal(t, []float32{1, 2}, x.Data())
	require.Nil(t, x.data16)
	require.Equal(t, wire, func() []byte { b, err := proto.Marshal(x.toPB()); require.NoError(t, err); return b }())
}

func TestTensorInvalidSerializationIsAtomic(t *testing.T) {
	cases := []*protocol.Tensor{
		{Dtype: protocol.TensorDType(2), Shape: []int32{0}},
		{Shape: []int32{1}, Data: []byte{0, 0}},
		{Shape: []int32{1}, Data: []byte{0, 0, 0, 0, 0}},
		{Dtype: protocol.TensorDType_FLOAT16, Shape: []int32{1}, Data: []byte{0}},
		{Dtype: protocol.TensorDType_FLOAT16, Shape: []int32{2}, Data: []byte{0, 0}},
		{Shape: []int32{-1, 0}}, {Shape: []int32{2147483647, 2147483647, 2147483647}},
		{Shape: []int32{2}, Data: []byte{0, 0, 0, 0}}, {Data: nil},
	}
	for _, pb := range cases {
		t.Run("invalid", func(t *testing.T) {
			x := NewTensor([]float32{3}, 1)
			x.grad = Ones(1)
			x.op = &neg{}
			old := *x
			require.Panics(t, func() { x.fromPB(pb) })
			require.Equal(t, old, *x)
			var buf bytes.Buffer
			_, err := pbutil.WriteDelimited(&buf, pb)
			require.NoError(t, err)
			require.Panics(t, func() { _ = Load(x, &buf) })
			require.Equal(t, old, *x)
		})
	}
}
