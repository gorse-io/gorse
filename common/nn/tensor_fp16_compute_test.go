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

func TestFloat16RejectCompute(t *testing.T) {
	h := Ones(2, 2).ToFloat16()
	f := Ones(2, 2)
	unary := map[string]func(*Tensor){
		"Neg": func(x *Tensor) { Neg(x) }, "AddIdentity": func(x *Tensor) { Add(x) },
		"Square": func(x *Tensor) { Square(x) }, "Exp": func(x *Tensor) { Exp(x) }, "Log": func(x *Tensor) { Log(x) },
		"Sin": func(x *Tensor) { Sin(x) }, "Cos": func(x *Tensor) { Cos(x) }, "Abs": func(x *Tensor) { Abs(x) },
		"Sum": func(x *Tensor) { Sum(x) }, "PartialSum": func(x *Tensor) { Sum(x, 1) }, "Mean": func(x *Tensor) { Mean(x) },
		"Flatten": func(x *Tensor) { Flatten(x) }, "Reshape": func(x *Tensor) { Reshape(x, 4) }, "Broadcast": func(x *Tensor) { Broadcast(x, 2) },
		"Sigmoid": func(x *Tensor) { Sigmoid(x) }, "ReLU": func(x *Tensor) { ReLu(x) }, "Softmax": func(x *Tensor) { Softmax(x, 1) },
		"Backward": func(x *Tensor) { x.Backward() }, "NormalInit": func(x *Tensor) { NormalInit(nil, x, 0, 1) },
		"neg": func(x *Tensor) { x.neg() }, "square": func(x *Tensor) { x.square() }, "exp": func(x *Tensor) { x.exp() },
		"log": func(x *Tensor) { x.log() }, "sin": func(x *Tensor) { x.sin() }, "cos": func(x *Tensor) { x.cos() }, "tanh": func(x *Tensor) { x.tanh() },
		"transpose": func(x *Tensor) { x.transpose() }, "max": func(x *Tensor) { x.max(1, true) }, "sum": func(x *Tensor) { x.sum(1, true) }, "argmax": func(x *Tensor) { x.argmax() },
	}
	binary := map[string]func(*Tensor, *Tensor){
		"Add": func(x, y *Tensor) { Add(x, y) }, "Sub": func(x, y *Tensor) { Sub(x, y) }, "Mul": func(x, y *Tensor) { Mul(x, y) },
		"Div": func(x, y *Tensor) { Div(x, y) }, "Pow": func(x, y *Tensor) { Pow(x, y) }, "MatMul": func(x, y *Tensor) { MatMul(x, y, false, false, 1) },
		"Embedding": func(x, y *Tensor) { Embedding(x, y) },
		"add":       func(x, y *Tensor) { x.add(y) }, "sub": func(x, y *Tensor) { x.sub(y) }, "bSub": func(x, y *Tensor) { x.bSub(y) },
		"mul": func(x, y *Tensor) { x.mul(y) }, "div": func(x, y *Tensor) { x.div(y) }, "bDiv": func(x, y *Tensor) { x.bDiv(y) },
		"pow": func(x, y *Tensor) { x.pow(y) }, "maximum": func(x, y *Tensor) { x.maximum(y) }, "gt": func(x, y *Tensor) { x.gt(y) },
		"matMul": func(x, y *Tensor) { x.matMul(y, false, false, 1) },
	}
	for _, inference := range []bool{false, true} {
		SetInferenceMode(inference)
		for name, fn := range unary {
			t.Run(name, func(t *testing.T) {
				require.PanicsWithValue(t, "computation requires Float32 tensors; convert Float16 storage with ToFloat32", func() { fn(h) })
			})
		}
		for name, fn := range binary {
			t.Run(name, func(t *testing.T) {
				require.PanicsWithValue(t, "computation requires Float32 tensors; convert Float16 storage with ToFloat32", func() { fn(h, f) })
				require.PanicsWithValue(t, "computation requires Float32 tensors; convert Float16 storage with ToFloat32", func() { fn(f, h) })
			})
		}
	}
	SetInferenceMode(false)
	require.Nil(t, h.grad)
	require.Nil(t, h.op)
	require.Equal(t, []float32{1, 1, 1, 1}, f.Data())
	require.Equal(t, []float32{1, 1, 1, 1}, h.ToFloat32().Data())
	b := Ones(1, 2, 2).ToFloat16()
	require.Panics(t, func() { BMM(b, b, false, false, 1) })
	require.Panics(t, func() { b.batchMatMul(b, false, false, 1) })
	require.Panics(t, func() { SoftmaxCrossEntropy(h, Ones(2)) })
	require.Panics(t, func() { MeanSquareError(h, f) })
	require.Panics(t, func() { BCEWithLogits(h, f, nil) })
}

func TestFloat16RejectOptimizerWithoutMutation(t *testing.T) {
	for _, constructor := range []func([]*Tensor, float32) Optimizer{NewSGD, NewAdam} {
		t.Run("optimizer", func(t *testing.T) {
			h := Ones(1).ToFloat16()
			require.Panics(t, func() { constructor([]*Tensor{Ones(1), h}, 0.1) })
			for _, badGrad := range []bool{false, true} {
				p, q := Ones(1), Ones(1)
				p.grad = Ones(1)
				q.grad = Ones(1)
				opt := constructor([]*Tensor{p, q}, 0.1)
				if badGrad {
					q.grad = h
				} else {
					*q = *h
				}
				require.Panics(t, func() { opt.Step() })
				require.Equal(t, []float32{1}, p.Data())
				switch o := opt.(type) {
				case *Adam:
					require.Zero(t, o.t)
					require.Empty(t, o.ms)
					require.Empty(t, o.vs)
					require.Equal(t, []float32{0}, o.b1)
					require.Equal(t, []float32{0}, o.b2)
				case *SGD:
					require.Equal(t, []float32{0}, o.b)
				}
			}
		})
	}
}

func TestFloat16RejectZeroGradWithoutMutation(t *testing.T) {
	for _, constructor := range []func([]*Tensor, float32) Optimizer{NewSGD, NewAdam} {
		for _, badGrad := range []bool{false, true} {
			p, q := Ones(1), Ones(1)
			p.grad, q.grad = Ones(1), Ones(1)
			opt := constructor([]*Tensor{p, q}, 0.1)
			if badGrad {
				q.grad = Ones(1).ToFloat16()
			} else {
				*q = *q.ToFloat16()
			}
			oldGrad := p.grad
			require.PanicsWithValue(t, "computation requires Float32 tensors; convert Float16 storage with ToFloat32", func() { opt.ZeroGrad() })
			require.Same(t, oldGrad, p.grad)
		}
	}
}

func TestFloat16BackwardGraphPreflight(t *testing.T) {
	x := Ones(2)
	y := Sum(Square(x))
	oldGrad := Ones(2)
	x.grad = oldGrad
	*x = *x.ToFloat16()
	x.grad = oldGrad
	require.Panics(t, func() { y.Backward() })
	require.Nil(t, y.grad)
	require.Same(t, oldGrad, x.grad)
}
