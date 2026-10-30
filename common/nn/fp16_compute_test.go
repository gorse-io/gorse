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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFloat16PrimitiveComputation(t *testing.T) {
	SetInferenceMode(true)
	t.Cleanup(func() { SetInferenceMode(false) })
	matrix := func() *Tensor { return NewTensor([]float32{0.5, 1, 1.5, 2}, 2, 2) }
	tests := []struct {
		name string
		run  func(*Tensor) *Tensor
	}{
		{"Neg", Neg}, {"Add", func(x *Tensor) *Tensor { return Add(x, NewScalar(0.25)) }},
		{"Sub", func(x *Tensor) *Tensor { return Sub(x, NewScalar(0.25)) }},
		{"Mul", func(x *Tensor) *Tensor { return Mul(x, NewScalar(0.25)) }},
		{"Div", func(x *Tensor) *Tensor { return Div(x, NewScalar(0.25)) }},
		{"Square", Square}, {"Pow", func(x *Tensor) *Tensor { return Pow(x, NewScalar(1.5)) }},
		{"Exp", Exp}, {"Log", Log}, {"Sin", Sin}, {"Cos", Cos}, {"Abs", Abs},
		{"Sum", func(x *Tensor) *Tensor { return Sum(x) }},
		{"PartialSum", func(x *Tensor) *Tensor { return Sum(x, 1) }}, {"Mean", Mean},
		{"MatMul", func(x *Tensor) *Tensor { return MatMul(x, matrix(), false, false, 2) }},
		{"MatMulHalf", func(x *Tensor) *Tensor { return MatMul(x, matrix().ToFloat16(), false, false, 2) }},
		{"MatMulTransposeLeft", func(x *Tensor) *Tensor { return MatMul(x, matrix(), true, false, 2) }},
		{"MatMulTransposeRight", func(x *Tensor) *Tensor { return MatMul(x, matrix(), false, true, 2) }},
		{"MatMulTransposeBoth", func(x *Tensor) *Tensor { return MatMul(x, matrix(), true, true, 2) }},
		{"BMM", func(x *Tensor) *Tensor { return BMM(Reshape(x, 1, 2, 2), Reshape(matrix(), 1, 2, 2), false, false, 2) }},
		{"BMMTransposeLeft", func(x *Tensor) *Tensor { return BMM(Reshape(x, 1, 2, 2), Reshape(matrix(), 1, 2, 2), true, false, 2) }},
		{"BMMTransposeRight", func(x *Tensor) *Tensor { return BMM(Reshape(x, 1, 2, 2), Reshape(matrix(), 1, 2, 2), false, true, 2) }},
		{"BMMTransposeBoth", func(x *Tensor) *Tensor { return BMM(Reshape(x, 1, 2, 2), Reshape(matrix(), 1, 2, 2), true, true, 2) }},
		{"Broadcast", func(x *Tensor) *Tensor { return Broadcast(x, 2) }}, {"Flatten", Flatten},
		{"Reshape", func(x *Tensor) *Tensor { return Reshape(x, 4) }},
		{"Embedding", func(x *Tensor) *Tensor { return Embedding(x, NewTensor([]float32{1, 0, 1}, 3)) }},
		{"Sigmoid", Sigmoid}, {"ReLu", ReLu}, {"Softmax", func(x *Tensor) *Tensor { return Softmax(x, 1) }},
		{"SoftmaxCrossEntropy", func(x *Tensor) *Tensor { return SoftmaxCrossEntropy(x, NewTensor([]float32{1, 0}, 2)) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := matrix().ToFloat16()
			reference := tt.run(x.ToFloat32())
			y := tt.run(x)
			require.Equal(t, Float16, y.DType())
			require.Equal(t, reference.Shape(), y.Shape())
			require.Equal(t, reference.ToFloat16().Data[uint16](), y.Data[uint16]())
			require.Nil(t, y.op)
			require.Nil(t, x.Grad())
		})
	}
}

func TestFloat16MixedOperands(t *testing.T) {
	SetInferenceMode(true)
	t.Cleanup(func() { SetInferenceMode(false) })
	half := NewTensor([]float32{1, 2}, 2).ToFloat16()
	full := NewTensor([]float32{3, 4}, 2)
	y := Mul(full, half)
	require.Equal(t, Float32, y.DType())
	require.Equal(t, []float32{3, 8}, y.Data[float32]())
	require.Nil(t, y.op)
	y = Mul(half, full)
	require.Equal(t, Float16, y.DType())
	require.Equal(t, []float32{3, 8}, y.ToFloat32().Data[float32]())
	require.Nil(t, y.op)
	require.Same(t, half, Add(half))
}

func TestFloat16ComputationRequiresInferenceMode(t *testing.T) {
	SetInferenceMode(false)
	half := NewTensor([]float32{1, 2}, 2).ToFloat16()
	require.PanicsWithValue(t, "FP16 computation requires inference mode", func() { Square(half) })
	require.PanicsWithValue(t, "FP16 computation requires inference mode", func() { Mul(Ones(2), half) })
	require.Nil(t, half.Grad())
}
