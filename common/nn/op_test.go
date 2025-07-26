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
	"github.com/chewxy/math32"
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	eps  = 1e-4
	rtol = 1e-2
	atol = 5e-3
)

func numericalDiff(f func(*Tensor) *Tensor, x *Tensor) *Tensor {
	x0, x1 := x.clone(), x.clone()
	dx := make([]float32, len(x.data))
	for i, v := range x.data {
		x0.data[i] = v - eps
		x1.data[i] = v + eps
		y0 := f(x0)
		y1 := f(x1)
		for j := range y0.data {
			dx[i] += (y1.data[j] - y0.data[j]) / (2 * eps)
		}
		x0.data[i] = v
		x1.data[i] = v
	}
	return NewTensor(dx, x.shape...)
}

func allClose(t *testing.T, a, b *Tensor) {
	if !assert.Equal(t, a.shape, b.shape) {
		return
	}
	for i := range a.data {
		if math32.Abs(a.data[i]-b.data[i]) > atol+rtol*math32.Abs(b.data[i]) {
			t.Fatalf("a.data[%d] = %f, b.data[%d] = %f\n", i, a.data[i], i, b.data[i])
			return
		}
	}
}

func TestAdd(t *testing.T) {
	// (2,3) + (2,3) -> (2,3)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := NewTensor([]float32{2, 3, 4, 5, 6, 7}, 2, 3)
	z := Add(x, y)
	assert.Equal(t, []float32{3, 5, 7, 9, 11, 13}, z.data)

	// Test gradient
	x = Rand(2, 3)
	y = Rand(2, 3)
	z = Add(x, y)
	z.Backward()
	dx := numericalDiff(func(x *Tensor) *Tensor { return Add(x, y) }, x)
	allClose(t, x.grad, dx)
	dy := numericalDiff(func(y *Tensor) *Tensor { return Add(x, y) }, y)
	allClose(t, y.grad, dy)

	// (2,3) + () -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2})
	z = Add(x, y)
	assert.Equal(t, []float32{3, 4, 5, 6, 7, 8}, z.data)

	// Test gradient
	z.Backward()
	assert.Equal(t, []float32{1, 1, 1, 1, 1, 1}, x.grad.data)
	assert.Equal(t, []float32{6}, y.grad.data)

	// (2,3) + (3) -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2, 3, 4}, 3)
	z = Add(x, y)
	assert.Equal(t, []float32{3, 5, 7, 6, 8, 10}, z.data)

	// Test gradient
	z.Backward()
	assert.Equal(t, []float32{1, 1, 1, 1, 1, 1}, x.grad.data)
	assert.Equal(t, []float32{2, 2, 2}, y.grad.data)
}

func TestSub(t *testing.T) {
	// (2,3) - (2,3) -> (2,3)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := NewTensor([]float32{2, 3, 4, 5, 6, 7}, 2, 3)
	z := Sub(x, y)
	assert.Equal(t, []float32{-1, -1, -1, -1, -1, -1}, z.data)

	// Test gradient
	x = Rand(2, 3)
	y = Rand(2, 3)
	z = Sub(x, y)
	z.Backward()
	dx := numericalDiff(func(x *Tensor) *Tensor { return Sub(x, y) }, x)
	allClose(t, x.grad, dx)
	dy := numericalDiff(func(y *Tensor) *Tensor { return Sub(x, y) }, y)
	allClose(t, y.grad, dy)

	// (2,3) - () -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2})
	z = Sub(x, y)
	assert.Equal(t, []float32{-1, 0, 1, 2, 3, 4}, z.data)

	// Test gradient
	z.Backward()
	assert.Equal(t, []float32{1, 1, 1, 1, 1, 1}, x.grad.data)
	assert.Equal(t, []float32{-6}, y.grad.data)

	// (2,3) - (3) -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2, 3, 4}, 3)
	z = Sub(x, y)
	assert.Equal(t, []float32{-1, -1, -1, 2, 2, 2}, z.data)

	// Test gradient
	z.Backward()
	assert.Equal(t, []float32{1, 1, 1, 1, 1, 1}, x.grad.data)
	assert.Equal(t, []float32{-2, -2, -2}, y.grad.data)
}

func TestMul(t *testing.T) {
	// (2,3) * (2,3) -> (2,3)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := NewTensor([]float32{2, 3, 4, 5, 6, 7}, 2, 3)
	z := Mul(x, y)
	assert.Equal(t, []float32{2, 6, 12, 20, 30, 42}, z.data)

	// Test gradient
	x = Rand(2, 3)
	y = Rand(2, 3)
	z = Mul(x, y)
	z.Backward()
	dx := numericalDiff(func(x *Tensor) *Tensor { return Mul(x, y) }, x)
	allClose(t, x.grad, dx)
	dy := numericalDiff(func(y *Tensor) *Tensor { return Mul(x, y) }, y)
	allClose(t, y.grad, dy)

	// (2,3) * () -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2})
	z = Mul(x, y)
	assert.Equal(t, []float32{2, 4, 6, 8, 10, 12}, z.data)

	// Test gradient
	z.Backward()
	assert.Equal(t, []float32{2, 2, 2, 2, 2, 2}, x.grad.data)
	assert.Equal(t, []float32{21}, y.grad.data)

	// (2,3) * (3) -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2, 3, 4}, 3)
	z = Mul(x, y)
	assert.Equal(t, []float32{2, 6, 12, 8, 15, 24}, z.data)

	// Test gradient
	z.Backward()
	assert.Equal(t, []float32{2, 3, 4, 2, 3, 4}, x.grad.data)
	assert.Equal(t, []float32{5, 7, 9}, y.grad.data)
}

func TestDiv(t *testing.T) {
	// (2,3) / (2,3) -> (2,3)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := NewTensor([]float32{2, 3, 4, 5, 6, 7}, 2, 3)
	z := Div(x, y)
	assert.InDeltaSlice(t, []float32{0.5, 2.0 / 3.0, 0.75, 4.0 / 5.0, 5.0 / 6.0, 6.0 / 7.0}, z.data, 1e-6)

	// Test gradient
	z.Backward()
	dx := numericalDiff(func(x *Tensor) *Tensor { return Div(x, y) }, x)
	allClose(t, x.grad, dx)
	dy := numericalDiff(func(y *Tensor) *Tensor { return Div(x, y) }, y)
	allClose(t, y.grad, dy)

	// (2,3) / () -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2})
	z = Div(x, y)
	assert.InDeltaSlice(t, []float32{0.5, 1, 1.5, 2, 2.5, 3}, z.data, 1e-6)

	// Test gradient
	z.Backward()
	assert.InDeltaSlice(t, []float32{0.5, 0.5, 0.5, 0.5, 0.5, 0.5}, x.grad.data, 1e-6)
	assert.InDeltaSlice(t, []float32{-21.0 / 4.0}, y.grad.data, 1e-6)

	// (2,3) / (3) -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2, 3, 4}, 3)
	z = Div(x, y)
	assert.InDeltaSlice(t, []float32{0.5, 2.0 / 3.0, 3.0 / 4.0, 2, 5.0 / 3.0, 1.5}, z.data, 1e-6)

	// Test gradient
	z.Backward()
	assert.InDeltaSlice(t, []float32{1.0 / 2, 1.0 / 3, 1.0 / 4, 1.0 / 2, 1.0 / 3, 1.0 / 4}, x.grad.data, 1e-6)
	assert.InDeltaSlice(t, []float32{-5.0 / 4.0, -7.0 / 9.0, -9.0 / 16.0}, y.grad.data, 1e-6)
}

func TestSquare(t *testing.T) {
	// (2,3) -> (2,3)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := Square(x)
	assert.Equal(t, []float32{1, 4, 9, 16, 25, 36}, y.data)

	// Test gradient
	x = Rand(2, 3)
	y = Square(x)
	y.Backward()
	dx := numericalDiff(Square, x)
	allClose(t, x.grad, dx)
}

func TestPow(t *testing.T) {
	// (2,3) ** (2,3) -> (2,3)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := NewTensor([]float32{2, 3, 4, 5, 6, 7}, 2, 3)
	z := Pow(x, y)
	assert.InDeltaSlice(t, []float32{1, 8, 81, 1024, 15625, 279936}, z.data, 1e-6)

	// Test gradient
	x = Uniform(0.5, 1, 2, 3)
	y = Uniform(0.5, 1, 2, 3)
	z = Pow(x, y)
	z.Backward()
	dx := numericalDiff(func(x *Tensor) *Tensor { return Pow(x, y) }, x)
	allClose(t, x.grad, dx)
	dy := numericalDiff(func(y *Tensor) *Tensor { return Pow(x, y) }, y)
	allClose(t, y.grad, dy)

	// (2,3) ** () -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2})
	z = Pow(x, y)
	assert.InDeltaSlice(t, []float32{1, 4, 9, 16, 25, 36}, z.data, 1e-6)

	// Test gradient
	z.Backward()
	assert.InDeltaSlice(t, []float32{2, 4, 6, 8, 10, 12}, x.grad.data, 1e-6)
	assert.InDeltaSlice(t, []float32{
		math32.Pow(1, 2)*math32.Log(1) +
			math32.Pow(2, 2)*math32.Log(2) +
			math32.Pow(3, 2)*math32.Log(3) +
			math32.Pow(4, 2)*math32.Log(4) +
			math32.Pow(5, 2)*math32.Log(5) +
			math32.Pow(6, 2)*math32.Log(6),
	}, y.grad.data, 1e-6)
}

func TestExp(t *testing.T) {
	// (2,3) -> (2,3)
	x := NewTensor([]float32{0, 1, 2, 3, 4, 5}, 2, 3)
	y := Exp(x)
	assert.InDeltaSlice(t, []float32{1, math32.Exp(1), math32.Exp(2), math32.Exp(3), math32.Exp(4), math32.Exp(5)}, y.data, 1e-5)

	// Test gradient
	x = Rand(2, 3)
	y = Exp(x)
	y.Backward()
	dx := numericalDiff(Exp, x)
	allClose(t, x.grad, dx)
}

func TestLog(t *testing.T) {
	// (2,3) -> (2,3)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := Log(x)
	assert.InDeltaSlice(t, []float32{0, math32.Log(2), math32.Log(3), math32.Log(4), math32.Log(5), math32.Log(6)}, y.data, 1e-6)

	// Test gradient
	x = Uniform(1, 2, 2, 3)
	y = Log(x)
	y.Backward()
	dx := numericalDiff(Log, x)
	allClose(t, x.grad, dx)
}

func TestSum(t *testing.T) {
	// (2,3) -> ()
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := Sum(x)
	assert.Equal(t, []float32{21}, y.data)

	// Test gradient
	x = Rand(2, 3)
	y = Sum(x)
	y.Backward()
	assert.Equal(t, []float32{1, 1, 1, 1, 1, 1}, x.grad.data)

	// (2,3,2) -> (2,2)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6, 1, 2, 3, 4, 5, 6}, 2, 3, 2)
	y = Sum(x, 1)
	assert.Equal(t, []int{2, 2}, y.shape)
	assert.Equal(t, []float32{9, 12, 9, 12}, y.data)

	// Test gradient
	x = Rand(2, 3, 2)
	y = Sum(x, 1)
	y.Backward()
	assert.Equal(t, []int{2, 3, 2}, x.grad.shape)
	assert.Equal(t, []float32{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, x.grad.data)
}

func TestMean(t *testing.T) {
	// (2,3) -> ()
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := Mean(x)
	assert.Equal(t, []float32{3.5}, y.data)

	// Test gradient
	x = Rand(2, 3)
	y = Mean(x)
	y.Backward()
	assert.Equal(t, []float32{1.0 / 6, 1.0 / 6, 1.0 / 6, 1.0 / 6, 1.0 / 6, 1.0 / 6}, x.grad.data)
}

func TestCos(t *testing.T) {
	// (2,3) -> (2,3)
	x := NewTensor([]float32{0, 0.1, 0.2, 0.3, 0.4, 0.5}, 2, 3)
	y := Cos(x)
	assert.InDeltaSlice(t, []float32{1, 0.9950041652780258, 0.9800665778412416, 0.955336489125606, 0.9210609940028851, 0.8775825618903728}, y.data, 1e-6)

	// Test gradient
	x = Rand(2, 3)
	y = Cos(x)
	y.Backward()
	dx := numericalDiff(Cos, x)
	allClose(t, x.grad, dx)
}

func TestSin(t *testing.T) {
	// (2,3) -> (2,3)
	x := NewTensor([]float32{0, 1, 2, 3, 4, 5}, 2, 3)
	y := Sin(x)
	assert.InDeltaSlice(t, []float32{0, 0.8414709848078965, 0.9092974268256817, 0.1411200080598672, -0.7568024953079282, -0.9589242746631385}, y.data, 1e-6)

	// Test gradient
	x = Rand(2, 3)
	y = Sin(x)
	y.Backward()
	dx := numericalDiff(Sin, x)
	allClose(t, x.grad, dx)
}

func TestMatMul(t *testing.T) {
	// (2,3) * (3,4) -> (2,4)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := NewTensor([]float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, 3, 4)
	z := MatMul(x, y, false, false, 0)
	assert.Equal(t, []int{2, 4}, z.shape)
	assert.Equal(t, []float32{38, 44, 50, 56, 83, 98, 113, 128}, z.data)

	// Test gradient
	z.Backward()
	assert.Equal(t, []int{2, 3}, x.grad.shape)
	assert.Equal(t, []float32{10, 26, 42, 10, 26, 42}, x.grad.data)
	assert.Equal(t, []int{3, 4}, y.grad.shape)
	assert.Equal(t, []float32{5, 5, 5, 5, 7, 7, 7, 7, 9, 9, 9, 9}, y.grad.data)

	// (3,2).T * (3,4) -> (2,4)
	x = Rand(3, 2)
	y = Rand(3, 4)
	z = MatMul(x, y, true, false, 0)
	assert.Equal(t, []int{2, 4}, z.shape)
	z.Backward()
	assert.Equal(t, []int{3, 2}, x.grad.shape)
	assert.Equal(t, []int{3, 4}, y.grad.shape)

	// (2,3) * (4,3).T -> (2,4)
	x = Rand(2, 3)
	y = Rand(4, 3)
	z = MatMul(x, y, false, true, 0)
	assert.Equal(t, []int{2, 4}, z.shape)
	z.Backward()
	assert.Equal(t, []int{2, 3}, x.grad.shape)
	assert.Equal(t, []int{4, 3}, y.grad.shape)

	// (3,2).T * (4,3).T -> (2,4)
	x = Rand(3, 2)
	y = Rand(4, 3)
	z = MatMul(x, y, true, true, 0)
	assert.Equal(t, []int{2, 4}, z.shape)
	z.Backward()
	assert.Equal(t, []int{3, 2}, x.grad.shape)
}

func TestBMM(t *testing.T) {
	// (2,2,3) * (2,3,4) -> (2,2,4)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6, 1, 2, 3, 4, 5, 6}, 2, 2, 3)
	y := NewTensor([]float32{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
	}, 2, 3, 4)
	z := BMM(x, y, false, false, 0)
	assert.Equal(t, []int{2, 2, 4}, z.shape)
	assert.Equal(t, []float32{
		38, 44, 50, 56, 83, 98, 113, 128,
		38, 44, 50, 56, 83, 98, 113, 128,
	}, z.data)

	// Test gradient
	z.Backward()
	assert.Equal(t, []int{2, 2, 3}, x.grad.shape)
	assert.Equal(t, []float32{
		10, 26, 42, 10, 26, 42,
		10, 26, 42, 10, 26, 42,
	}, x.grad.data)
	assert.Equal(t, []int{2, 3, 4}, y.grad.shape)
	assert.Equal(t, []float32{
		5, 5, 5, 5, 7, 7, 7, 7, 9, 9, 9, 9,
		5, 5, 5, 5, 7, 7, 7, 7, 9, 9, 9, 9,
	}, y.grad.data)

	// (2,3,2).T * (2,3,4) -> (2,2,4)
	x = Rand(2, 3, 2)
	y = Rand(2, 3, 4)
	z = BMM(x, y, true, false, 0)
	assert.Equal(t, []int{2, 2, 4}, z.shape)
	z.Backward()
	assert.Equal(t, []int{2, 3, 2}, x.grad.shape)

	// (2,2,3) * (2,4,3).T -> (2,2,4)
	x = Rand(2, 2, 3)
	y = Rand(2, 4, 3)
	z = BMM(x, y, false, true, 0)
	assert.Equal(t, []int{2, 2, 4}, z.shape)
	z.Backward()
	assert.Equal(t, []int{2, 2, 3}, x.grad.shape)

	// (2,3,2).T * (2,43).T -> (2,2,4)
	x = Rand(2, 3, 2)
	y = Rand(2, 4, 3)
	z = BMM(x, y, true, true, 0)
	assert.Equal(t, []int{2, 2, 4}, z.shape)
	z.Backward()
	assert.Equal(t, []int{2, 3, 2}, x.grad.shape)
}

func TestBroadcast(t *testing.T) {
	// (2) -> (2,3)
	x := NewTensor([]float32{1, 2}, 2)
	y := Broadcast(x, 3)
	assert.Equal(t, []float32{1, 1, 1, 2, 2, 2}, y.data)

	// Test gradient
	y.Backward()
	assert.Equal(t, []float32{3, 3}, x.grad.data)
}

func TestEmbedding(t *testing.T) {
	// (2,3) -> (2,3,2)
	x := NewTensor([]float32{0, 1, 0, 3, 0, 5}, 2, 3)
	w := NewTensor([]float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}, 6, 2)
	y := Embedding(w, x)
	assert.Equal(t, []int{2, 3, 2}, y.shape)
	assert.Equal(t, []float32{0, 1, 2, 3, 0, 1, 6, 7, 0, 1, 10, 11}, y.data)

	// Test gradient
	y.Backward()
	assert.Nil(t, x.grad)
	assert.Equal(t, []float32{3, 3, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1}, w.grad.data)

	// (2,3) -> (2,3,1,2)
	x = NewTensor([]float32{0, 1, 0, 3, 0, 5}, 2, 3)
	w = NewTensor([]float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}, 6, 1, 2)
	y = Embedding(w, x)
	assert.Equal(t, []int{2, 3, 1, 2}, y.shape)
	assert.Equal(t, []float32{0, 1, 2, 3, 0, 1, 6, 7, 0, 1, 10, 11}, y.data)

	// Test gradient
	y.Backward()
	assert.Nil(t, x.grad)
	assert.Equal(t, []float32{3, 3, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1}, w.grad.data)
}

func TestSigmoid(t *testing.T) {
	// (2,3) -> (2,3)
	x := NewTensor([]float32{0, 1, 2, 3, 4, 5}, 2, 3)
	y := Sigmoid(x)
	assert.InDeltaSlice(t, []float32{0.5, 0.7310585786300049, 0.8807970779778823, 0.9525741268224334, 0.9820137900379085, 0.9933071490757153}, y.data, 1e-6)

	// Test gradient
	x = Rand(2, 3)
	y = Sigmoid(x)
	y.Backward()
	dx := numericalDiff(Sigmoid, x)
	allClose(t, x.grad, dx)
}

func TestReLu(t *testing.T) {
	// (2,3) -> (2,3)
	x := NewTensor([]float32{-1, 0, 1, 2, 3, 4}, 2, 3)
	y := ReLu(x)
	assert.Equal(t, []float32{0, 0, 1, 2, 3, 4}, y.data)

	// Test gradient
	x = Rand(2, 3)
	y = ReLu(x)
	y.Backward()
	dx := numericalDiff(ReLu, x)
	allClose(t, x.grad, dx)
}

func TestSoftmax(t *testing.T) {
	// (1,3) -> (1,3)
	x := NewTensor([]float32{3.0, 1.0, 0.2}, 1, 3)
	y := Softmax(x, 1)
	assert.Equal(t, []int{1, 3}, y.shape)
	assert.InDeltaSlice(t, []float32{0.8360188027814407, 0.11314284146556013, 0.05083835575299916}, y.data, 1e-6)

	// Test gradient
	y.Backward()
	dx := numericalDiff(func(x *Tensor) *Tensor { return Softmax(x, 1) }, x)
	allClose(t, x.grad, dx)
}

func TestFlatten(t *testing.T) {
	// (2,3) -> (6)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := Flatten(x)
	assert.Equal(t, []float32{1, 2, 3, 4, 5, 6}, y.data)

	// Test gradient
	y.Backward()
	assert.Equal(t, []float32{1, 1, 1, 1, 1, 1}, x.grad.data)
}

func TestReshape(t *testing.T) {
	// (2,3) -> (3,2)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := Reshape(x, 3, 2)
	assert.Equal(t, []float32{1, 2, 3, 4, 5, 6}, y.data)

	// Test gradient
	y.Backward()
	assert.Equal(t, []float32{1, 1, 1, 1, 1, 1}, x.grad.data)
}

func TestSoftmaxCrossEntropy(t *testing.T) {
	// (2,3) -> (2,3)
	x := NewTensor([]float32{0.3, 2.9, 4.0, 0.2, 1.0, 3.0}, 3, 2)
	y := NewTensor([]float32{1, 0, 1}, 3)
	z := SoftmaxCrossEntropy(x, y)
	assert.Empty(t, z.shape)
	assert.InDelta(t, float32(0.07356563982184072), z.data[0], 1e-4)

	// Test gradient
	z.Backward()
	dx := numericalDiff(func(x *Tensor) *Tensor { return SoftmaxCrossEntropy(x, y) }, x)
	allClose(t, x.grad, dx)
}

func TestReuseLeaf(t *testing.T) {
	// x + x
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := Add(x, x)
	assert.Equal(t, []float32{2, 4, 6, 8, 10, 12}, y.data)

	// Test gradient
	y.Backward()
	dx := numericalDiff(func(x *Tensor) *Tensor { return Add(x, x) }, x)
	allClose(t, x.grad, dx)
}

func TestReuseNode(t *testing.T) {
	// x^2 + x^2
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	temp := Pow(x, NewTensor([]float32{2}))
	y := Add(temp, temp)
	assert.Equal(t, []float32{2, 8, 18, 32, 50, 72}, y.data)

	// Test gradient
	y.Backward()
	dx := numericalDiff(func(x *Tensor) *Tensor {
		temp := Pow(x, NewTensor([]float32{2}))
		return Add(temp, temp)
	}, x)
	allClose(t, x.grad, dx)
}

func TestDependency(t *testing.T) {
	// x^2 + 2x^2
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	temp := Pow(x, NewTensor([]float32{2}))
	y := Add(temp, Mul(NewTensor([]float32{2}), temp))
	assert.Equal(t, []float32{3, 12, 27, 48, 75, 108}, y.data)

	// Test gradient
	y.Backward()
	dx := numericalDiff(func(x *Tensor) *Tensor {
		temp := Pow(x, NewTensor([]float32{2}))
		return Add(temp, Mul(NewTensor([]float32{2}), temp))
	}, x)
	allClose(t, x.grad, dx)
}

func TestSphere(t *testing.T) {
	// x^2 + y^2
	x := NewScalar(1)
	y := NewScalar(1)
	z := Add(Mul(x, x), Mul(y, y))
	assert.Equal(t, []float32{2}, z.data)

	// Test gradient
	z.Backward()
	dx := numericalDiff(func(x *Tensor) *Tensor { return Add(Mul(x, x), Mul(y, y)) }, x)
	dy := numericalDiff(func(y *Tensor) *Tensor { return Add(Mul(x, x), Mul(y, y)) }, y)
	allClose(t, x.grad, dx)
	allClose(t, y.grad, dy)
}

func TestMatyas(t *testing.T) {
	// 0.26 * (x^2 + y^2) - 0.48 * x * y
	x := NewScalar(1)
	y := NewScalar(1)
	z := Sub(Mul(NewScalar(0.26), Add(Mul(x, x), Mul(y, y))), Mul(NewScalar(0.48), Mul(x, y)))
	assert.InDeltaSlice(t, []float32{0.04}, z.data, 1e-6)

	// Test gradient
	z.Backward()
	dx := numericalDiff(func(x *Tensor) *Tensor {
		return Sub(Mul(NewScalar(0.26), Add(Mul(x, x), Mul(y, y))), Mul(NewScalar(0.48), Mul(x, y)))
	}, x)
	dy := numericalDiff(func(y *Tensor) *Tensor {
		return Sub(Mul(NewScalar(0.26), Add(Mul(x, x), Mul(y, y))), Mul(NewScalar(0.48), Mul(x, y)))
	}, y)
	allClose(t, x.grad, dx)
	allClose(t, y.grad, dy)
}

func TestGoldsteinPrice(t *testing.T) {
	// (1 + (x + y + 1)^2 * (19 - 14x + 3x^2 - 14y + 6xy + 3y^2)) * (30 + (2x - 3y)^2 * (18 - 32x + 12x^2 + 48y - 36xy + 27y^2))
	x := NewScalar(1)
	y := NewScalar(1)
	z := Mul(
		Add(NewScalar(1), Mul(
			Pow(Add(x, y, NewScalar(1)), NewScalar(2)), // (x + y + 1)^2
			Add(
				NewScalar(19),                              // 19
				Mul(NewScalar(-14), x),                     // -14x
				Mul(NewScalar(3), Pow(x, NewScalar(2))),    // 3x^2
				Mul(NewScalar(-14), y),                     // -14y
				Mul(NewScalar(6), Mul(x, y)),               // 6xy
				Mul(NewScalar(3), Pow(y, NewScalar(2)))))), // 3y^2
		Add(NewScalar(30), Mul(
			Pow(Sub(Mul(NewScalar(2), x), Mul(NewScalar(3), y)), NewScalar(2)), // (2x - 3y)^2
			Add(
				NewScalar(18),                               // 18
				Mul(NewScalar(-32), x),                      // -32x
				Mul(NewScalar(12), Pow(x, NewScalar(2))),    // 12x^2
				Mul(NewScalar(48), y),                       // 48y
				Mul(NewScalar(-36), Mul(x, y)),              // -36xy
				Mul(NewScalar(27), Pow(y, NewScalar(2))))))) // 27y^2
	assert.InDeltaSlice(t, []float32{1876}, z.data, 1e-6)

	// Test gradient
	z.Backward()
	dx := numericalDiff(func(x *Tensor) *Tensor {
		return Mul(
			Add(NewScalar(1), Mul(
				Pow(Add(x, y, NewScalar(1)), NewScalar(2)), // (x + y + 1)^2
				Add(
					NewScalar(19),                              // 19
					Mul(NewScalar(-14), x),                     // -14x
					Mul(NewScalar(3), Pow(x, NewScalar(2))),    // 3x^2
					Mul(NewScalar(-14), y),                     // -14y
					Mul(NewScalar(6), Mul(x, y)),               // 6xy
					Mul(NewScalar(3), Pow(y, NewScalar(2)))))), // 3y^2
			Add(NewScalar(30), Mul(
				Pow(Sub(Mul(NewScalar(2), x), Mul(NewScalar(3), y)), NewScalar(2)), // (2x - 3y)^2
				Add(
					NewScalar(18),                               // 18
					Mul(NewScalar(-32), x),                      // -32x
					Mul(NewScalar(12), Pow(x, NewScalar(2))),    // 12x^2
					Mul(NewScalar(48), y),                       // 48y
					Mul(NewScalar(-36), Mul(x, y)),              // -36xy
					Mul(NewScalar(27), Pow(y, NewScalar(2))))))) // 27y^2
	}, x)
	dy := numericalDiff(func(y *Tensor) *Tensor {
		return Mul(
			Add(NewScalar(1), Mul(
				Pow(Add(x, y, NewScalar(1)), NewScalar(2)), // (x + y + 1)^2
				Add(
					NewScalar(19),                              // 19
					Mul(NewScalar(-14), x),                     // -14x
					Mul(NewScalar(3), Pow(x, NewScalar(2))),    // 3x^2
					Mul(NewScalar(-14), y),                     // -14y
					Mul(NewScalar(6), Mul(x, y)),               // 6xy
					Mul(NewScalar(3), Pow(y, NewScalar(2)))))), // 3y^2
			Add(NewScalar(30), Mul(
				Pow(Sub(Mul(NewScalar(2), x), Mul(NewScalar(3), y)), NewScalar(2)), // (2x - 3y)^2
				Add(
					NewScalar(18),                               // 18
					Mul(NewScalar(-32), x),                      // -32x
					Mul(NewScalar(12), Pow(x, NewScalar(2))),    // 12x^2
					Mul(NewScalar(48), y),                       // 48y
					Mul(NewScalar(-36), Mul(x, y)),              // -36xy
					Mul(NewScalar(27), Pow(y, NewScalar(2))))))) // 27y^2
	}, y)
	allClose(t, x.grad, dx)
	allClose(t, y.grad, dy)
}
