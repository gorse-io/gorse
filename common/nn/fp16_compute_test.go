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
	"math/rand"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFloat16PrimitiveComputation(t *testing.T) {
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
			x32 := x.ToFloat32()
			reference := tt.run(x32)
			y := tt.run(x)
			require.Equal(t, Float16, y.DType())
			require.Equal(t, reference.Shape(), y.Shape())
			require.Equal(t, reference.ToFloat16().Data16(), y.Data16())
			runtime.GC() // The FP32 output used by backward must outlive forward.
			reference.Backward()
			y.Backward()
			require.NotNil(t, x.Grad())
			require.Equal(t, Float32, x.Grad().DType())
			require.InDeltaSlice(t, x32.Grad().Data(), x.Grad().Data(), 1e-6)
		})
	}
}

func TestFloat16MixedOperands(t *testing.T) {
	half := NewTensor([]float32{1, 2}, 2).ToFloat16()
	full := NewTensor([]float32{3, 4}, 2)
	y := Mul(full, half)
	require.Equal(t, Float32, y.DType())
	require.Equal(t, []float32{3, 8}, y.Data())
	y.Backward()
	require.Equal(t, []float32{3, 4}, half.Grad().Data())
	require.Equal(t, []float32{1, 2}, full.Grad().Data())
	require.Equal(t, Float16, Mul(half, full).DType())
	require.Same(t, half, Add(half))
}

func TestFloat16SharedGraph(t *testing.T) {
	x := NewTensor([]float32{0.5, 1.5}, 2).ToFloat16()
	exponent := NewScalar(1.5)
	p := Pow(x, exponent)
	s := Sigmoid(p)
	y := Sum(Add(Square(s), s))
	runtime.GC()
	y.Backward()
	// Compare with an FP32 graph with each intermediate rounded like FP16 forward.
	xr := x.ToFloat32()
	pr := Pow(xr, NewScalar(1.5))
	sr := Sigmoid(pr.ToFloat16().ToFloat32())
	upstream := Add(Mul(sr.ToFloat16().ToFloat32(), NewScalar(2)), NewScalar(1))
	sg := sr.op.backward(upstream)[0]
	expected := pr.op.backward(sg)
	require.InDeltaSlice(t, expected[0].Data(), x.Grad().Data(), 1e-6)
	require.Equal(t, Float32, x.Grad().DType())
	require.InDeltaSlice(t, expected[1].Data(), exponent.Grad().Data(), 1e-6)
}

func TestFloat16InferenceAndConversions(t *testing.T) {
	SetInferenceMode(true)
	defer SetInferenceMode(false)
	x := NewTensor([]float32{-1, 2}, 2).ToFloat16()
	y := Square(x)
	require.Equal(t, []float32{1, 4}, y.ToFloat32().Data())
	require.Nil(t, y.op)
	require.Nil(t, x.Grad())
	SetInferenceMode(false)
	y = Square(x)
	require.NotNil(t, y.op)
	require.Nil(t, y.ToFloat16().op)
	require.Nil(t, y.ToFloat32().op)
}

func TestFloat16NormalInit(t *testing.T) {
	full := Zeros(8)
	half := full.ToFloat16()
	backing := half.Data16()
	NormalInit(rand.New(rand.NewSource(123)), full, 0.2, 0.5)
	NormalInit(rand.New(rand.NewSource(123)), half, 0.2, 0.5)
	require.Equal(t, full.ToFloat16().Data16(), half.Data16())
	require.Equal(t, backing, half.Data16())
}

func TestFloat16Optimizers(t *testing.T) {
	for _, name := range []string{"SGD", "Adam"} {
		for _, wd := range []float32{0, 0.1} {
			t.Run(name+"/"+NewScalar(wd).String(), func(t *testing.T) {
				half := LinSpace(-1, 1, 65).ToFloat16()
				alias := half.Slice(0, 65)
				full := NewTensor([]float32{2, 3}, 2)
				reference := half.ToFloat32()
				fullReference := full.clone()
				newOptimizer := NewSGD
				if name == "Adam" {
					newOptimizer = NewAdam
				}
				opt := newOptimizer([]*Tensor{half, full}, 0.01)
				ref := newOptimizer([]*Tensor{reference, fullReference}, 0.01)
				opt.SetJobs(3)
				ref.SetJobs(3)
				opt.SetWeightDecay(wd)
				ref.SetWeightDecay(wd)
				for step := 0; step < 3; step++ {
					half.grad = LinSpace(-0.3, 0.7, 65)
					reference.grad = half.grad.clone()
					full.grad = NewTensor([]float32{0.2, -0.4}, 2)
					fullReference.grad = full.grad.clone()
					opt.Step()
					ref.Step()
					require.Equal(t, reference.ToFloat16().Data16(), half.Data16())
					require.Equal(t, half.Data16(), alias.Data16())
					require.Equal(t, fullReference.Data(), full.Data())
					// There is deliberately no persistent FP32 master parameter.
					copy(reference.data, reference.ToFloat16().ToFloat32().data)
				}
				if adam, ok := opt.(*Adam); ok {
					require.Equal(t, Float32, adam.ms[half].DType())
					require.Equal(t, Float32, adam.vs[half].DType())
				}
				opt.ZeroGrad()
				require.Nil(t, half.Grad())
				require.Nil(t, full.Grad())
			})
		}
	}
}
