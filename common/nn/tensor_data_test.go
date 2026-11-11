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
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

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
