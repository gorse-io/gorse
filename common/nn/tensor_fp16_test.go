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
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/x448/float16"
)

func TestFloat16Storage(t *testing.T) {
	data := []float16.Float16{float16.Fromfloat32(1), float16.Fromfloat32(2), float16.Fromfloat32(3), float16.Fromfloat32(4)}
	x := NewTensor16(data, 2, 2)
	require.Equal(t, Float16, x.DType())
	require.Equal(t, float32(4), x.Get(1, 1))
	require.Equal(t, "[1, 2, 3, 4]", x.String())
	require.Panics(t, func() { x.Data() })
	require.Panics(t, func() { NewScalar(1).Data16() })
	data[0] = float16.Fromfloat32(5)
	require.Equal(t, float32(5), x.Get(0, 0))
	x.Slice(1, 2).Data16()[0] = float16.Fromfloat32(6)
	require.Equal(t, float32(6), x.Get(1, 0))
	for _, copy := range []*Tensor{x.SliceIndices(1, 0), x.clone(), x.ToFloat16()} {
		require.Equal(t, Float16, copy.DType())
		copy.Data16()[0] = float16.Fromfloat32(9)
		require.Equal(t, float32(5), x.Get(0, 0))
		require.Nil(t, copy.Grad())
		require.Nil(t, copy.op)
	}
	y := x.ToFloat32()
	require.Equal(t, Float32, y.DType())
	require.Equal(t, []float32{5, 2, 6, 4}, y.Data())
	y.Data()[0] = 10
	require.Equal(t, float32(5), x.Get(0, 0))
	z := y.ToFloat32()
	z.Data()[0] = 11
	require.Equal(t, float32(10), y.Get(0, 0))
	z.Shape()[0] = 1
	require.Equal(t, []int{2, 2}, y.Shape())
	require.Equal(t, "[]", NewTensor16(nil, 0).String())
	require.Equal(t, "1", NewTensor16([]float16.Float16{float16.Fromfloat32(1)}).String())
	require.Panics(t, func() { NewTensor16(data, 3) })
	require.Panics(t, func() { NewTensor16(nil, -1, 0) })
}

func TestFloat16Conversion(t *testing.T) {
	values := []float32{1.00048828125, 1.00146484375, 0x1p-24, 0x1p-25, float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())}
	x := NewTensor(values, len(values))
	x.grad = Ones(len(values))
	x.op = &neg{}
	y := x.ToFloat16()
	require.Equal(t, []float16.Float16{0x3c00, 0x3c02, 1, 0, 0x7c00, 0xfc00}, y.Data16()[:6])
	require.True(t, math.IsNaN(float64(y.Get(6))))
	require.Nil(t, y.grad)
	require.Nil(t, y.op)
	require.Equal(t, float32(0x1p-24), y.ToFloat32().Get(2))
	require.Equal(t, Float32, NewTensor([]float32{1}, 1).DType())
}
