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
		require.Empty(t, pb.Data)
		require.Len(t, pb.Data16, len(values)*2)
		for i, v := range values {
			require.Equal(t, uint16(v), binary.LittleEndian.Uint16(pb.Data16[2*i:]))
		}
		encoded, err := proto.Marshal(pb)
		require.NoError(t, err)
		decoded := new(protocol.Tensor)
		require.NoError(t, proto.Unmarshal(encoded, decoded))
		y := Ones(2)
		y.grad = Ones(2)
		y.op = &neg{}
		require.NoError(t, y.fromPB(decoded))
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

func TestTensorLegacySerialization(t *testing.T) {
	// Original protobuf fields only: shape=[2], data=[1,2], no dtype.
	wire := []byte{0x12, 1, 2, 0x1a, 8, 0, 0, 0x80, 0x3f, 0, 0, 0, 0x40}
	pb := new(protocol.Tensor)
	require.NoError(t, proto.Unmarshal(wire, pb))
	x := Ones(2).ToFloat16()
	require.NoError(t, x.fromPB(pb))
	require.Equal(t, Float32, x.DType())
	require.Equal(t, []float32{1, 2}, x.Data())
	require.Nil(t, x.data16)
	require.Equal(t, wire, func() []byte { b, err := proto.Marshal(x.toPB()); require.NoError(t, err); return b }())
}

func TestTensorInvalidSerializationIsAtomic(t *testing.T) {
	cases := []*protocol.Tensor{
		{Dtype: protocol.TensorDType(2), Shape: []int32{0}},
		{Shape: []int32{1}, Data: []float32{1}, Data16: []byte{0, 0}},
		{Dtype: protocol.TensorDType_FLOAT16, Shape: []int32{1}, Data: []float32{1}, Data16: []byte{0, 0}},
		{Dtype: protocol.TensorDType_FLOAT16, Shape: []int32{1}, Data16: []byte{0}},
		{Dtype: protocol.TensorDType_FLOAT16, Shape: []int32{2}, Data16: []byte{0, 0}},
		{Shape: []int32{-1, 0}}, {Shape: []int32{2147483647, 2147483647, 2147483647}},
		{Shape: []int32{2}, Data: []float32{1}}, {Data: nil},
	}
	for _, pb := range cases {
		t.Run("invalid", func(t *testing.T) {
			x := NewTensor([]float32{3}, 1)
			x.grad = Ones(1)
			x.op = &neg{}
			old := *x
			require.Error(t, x.fromPB(pb))
			require.Equal(t, old, *x)
			var buf bytes.Buffer
			_, err := pbutil.WriteDelimited(&buf, pb)
			require.NoError(t, err)
			require.Error(t, Load(x, &buf))
			require.Equal(t, old, *x)
		})
	}
}
