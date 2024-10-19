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
)

type op interface {
	String() string
	forward(inputs ...*Tensor) *Tensor
	backward(dy *Tensor) []*Tensor
	inputsAndOutput() ([]*Tensor, *Tensor)
	setInputs(inputs ...*Tensor)
	setOutput(y *Tensor)
}

type base struct {
	inputs []*Tensor
	output *Tensor
}

func (b *base) inputsAndOutput() ([]*Tensor, *Tensor) {
	return b.inputs, b.output
}

func (b *base) setInputs(inputs ...*Tensor) {
	b.inputs = inputs
}

func (b *base) setOutput(y *Tensor) {
	b.output = y
}

func apply[T op](f T, inputs ...*Tensor) *Tensor {
	y := f.forward(inputs...)
	f.setInputs(inputs...)
	f.setOutput(y)
	y.op = f
	return y
}

type add struct {
	base
}

func (a *add) String() string {
	return "Add"
}

func (a *add) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.add(inputs[1])
	return y
}

func (a *add) backward(dy *Tensor) []*Tensor {
	gx0 := dy.clone()
	gx1 := Zeros(a.inputs[1].shape...)
	wSize := 1
	for i := range gx1.shape {
		wSize *= gx1.shape[i]
	}
	for i := range dy.data {
		gx1.data[i%wSize] += dy.data[i]
	}
	return []*Tensor{gx0, gx1}
}

type sub struct {
	base
}

func (s *sub) String() string {
	return "Sub"
}

func (s *sub) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.sub(inputs[1])
	return y
}

func (s *sub) backward(dy *Tensor) []*Tensor {
	gx0 := dy.clone()
	gx1 := Zeros(s.inputs[1].shape...)
	wSize := 1
	for i := range gx1.shape {
		wSize *= gx1.shape[i]
	}
	for i := range dy.data {
		gx1.data[i%wSize] -= dy.data[i]
	}
	return []*Tensor{gx0, gx1}
}

type mul struct {
	base
}

func (m *mul) String() string {
	return "Mul"
}

func (m *mul) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.mul(inputs[1])
	return y
}

func (m *mul) backward(dy *Tensor) []*Tensor {
	gx0 := dy.clone()
	gx0.mul(m.inputs[1])
	gx1 := Zeros(m.inputs[1].shape...)
	wSize := 1
	for i := range gx1.shape {
		wSize *= gx1.shape[i]
	}
	for i := range dy.data {
		gx1.data[i%wSize] += dy.data[i] * m.inputs[0].data[i]
	}
	return []*Tensor{gx0, gx1}
}

type div struct {
	base
}

func (d *div) String() string {
	return "Div"
}

func (d *div) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.div(inputs[1])
	return y
}

func (d *div) backward(dy *Tensor) []*Tensor {
	wSize := 1
	for i := range d.inputs[1].shape {
		wSize *= d.inputs[1].shape[i]
	}
	gx0 := Zeros(d.inputs[0].shape...)
	for i := range dy.data {
		gx0.data[i] = dy.data[i] / d.inputs[1].data[i%wSize]
	}
	gx1 := Zeros(d.inputs[1].shape...)
	for i := range dy.data {
		gx1.data[i%wSize] -= dy.data[i] * d.inputs[0].data[i] / d.inputs[1].data[i%wSize] / d.inputs[1].data[i%wSize]
	}
	return []*Tensor{gx0, gx1}
}

type sin struct {
	base
}

func (s *sin) String() string {
	return "Sin"
}

func (s *sin) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.sin()
	return y
}

func (s *sin) backward(dy *Tensor) []*Tensor {
	dx := s.inputs[0].clone()
	dx.cos()
	dx.mul(dy)
	return []*Tensor{dx}
}

type cos struct {
	base
}

func (c *cos) String() string {
	return "Cos"
}

func (c *cos) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.cos()
	return y
}

func (c *cos) backward(dy *Tensor) []*Tensor {
	dx := c.inputs[0].clone()
	dx.sin()
	dx.neg()
	dx.mul(dy)
	return []*Tensor{dx}
}

type square struct {
	base
}

func (s *square) String() string {
	return "Square"
}

func (s *square) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.square()
	return y
}

func (s *square) backward(dy *Tensor) []*Tensor {
	dx := s.inputs[0].clone()
	dx.mul(dy)
	for i := range dx.data {
		dx.data[i] *= 2
	}
	return []*Tensor{dx}
}

type pow struct {
	base
}

func (p *pow) String() string {
	return "Pow"
}

func (p *pow) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.pow(inputs[1])
	return y
}

func (p *pow) backward(dy *Tensor) []*Tensor {
	dx0 := p.inputs[0].clone()
	dx0.pow(p.inputs[1])
	dx0.mul(p.inputs[1])
	dx0.div(p.inputs[0])
	dx0.mul(dy)
	wSize := 1
	for i := range p.inputs[1].shape {
		wSize *= p.inputs[1].shape[i]
	}
	dx1 := Zeros(p.inputs[1].shape...)
	for i := range dy.data {
		dx1.data[i%wSize] += dy.data[i] * p.output.data[i] * math32.Log(p.inputs[0].data[i])
	}
	return []*Tensor{dx0, dx1}
}

type exp struct {
	base
}

func (e *exp) String() string {
	return "Exp"
}

func (e *exp) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.exp()
	return y
}

func (e *exp) backward(dy *Tensor) []*Tensor {
	dx := e.inputs[0].clone()
	dx.exp()
	dx.mul(dy)
	return []*Tensor{dx}
}

type log struct {
	base
}

func (l *log) String() string {
	return "Log"
}

func (l *log) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.log()
	return y
}

func (l *log) backward(dy *Tensor) []*Tensor {
	dx := l.inputs[0].clone()
	dx.div(l.inputs[0])
	dx.mul(dy)
	return []*Tensor{dx}
}

type sum struct {
	base
}

func (s *sum) String() string {
	return "Sum"
}

func (s *sum) forward(inputs ...*Tensor) *Tensor {
	x := inputs[0]
	y := NewTensor([]float32{0})
	for i := range x.data {
		y.data[0] += x.data[i]
	}
	return y
}

func (s *sum) backward(*Tensor) []*Tensor {
	return []*Tensor{Ones(s.inputs[0].shape...)}
}

type mean struct {
	base
}

func (m *mean) String() string {
	return "Mean"
}

func (m *mean) forward(inputs ...*Tensor) *Tensor {
	x := inputs[0]
	y := NewTensor([]float32{0})
	for i := range x.data {
		y.data[0] += x.data[i]
	}
	y.data[0] /= float32(len(x.data))
	return y
}

func (m *mean) backward(*Tensor) []*Tensor {
	dx := Zeros(m.inputs[0].shape...)
	for i := range dx.data {
		dx.data[i] = 1 / float32(len(dx.data))
	}
	return []*Tensor{dx}
}

type matMul struct {
	base
}

func (m *matMul) String() string {
	return "MatMul"
}

func (m *matMul) forward(inputs ...*Tensor) *Tensor {
	return inputs[0].matMul(inputs[1], false, false)
}

func (m *matMul) backward(dy *Tensor) []*Tensor {
	dx0 := dy.matMul(m.inputs[1], false, true)
	dx1 := m.inputs[0].matMul(dy, true, false)
	return []*Tensor{dx0, dx1}
}

type broadcast struct {
	base
	shape []int
}

func (b *broadcast) String() string {
	return "Broadcast"
}

func (b *broadcast) forward(inputs ...*Tensor) *Tensor {
	x := inputs[0]
	// Concatenate the shape
	shape := make([]int, len(x.shape))
	copy(shape, x.shape)
	shape = append(shape, b.shape...)
	size := 1
	for i := range shape {
		size *= shape[i]
	}
	// Create a new tensor with the new shape
	y := NewTensor(make([]float32, size), shape...)
	wSize := 1
	for i := range b.shape {
		wSize *= b.shape[i]
	}
	for i := range x.data {
		for j := i * wSize; j < (i+1)*wSize; j++ {
			y.data[j] = x.data[i]
		}
	}
	return y
}

func (b *broadcast) backward(dy *Tensor) []*Tensor {
	gx := Zeros(b.inputs[0].shape...)
	wSize := 1
	for i := range b.shape {
		wSize *= b.shape[i]
	}
	for i := range gx.data {
		for j := i * wSize; j < (i+1)*wSize; j++ {
			gx.data[i] += dy.data[j]
		}
	}
	return []*Tensor{gx}
}

type flatten struct {
	base
}

func (f *flatten) String() string {
	return "Flatten"
}

func (f *flatten) forward(inputs ...*Tensor) *Tensor {
	return NewTensor(inputs[0].data, len(inputs[0].data))
}

func (f *flatten) backward(dy *Tensor) []*Tensor {
	return []*Tensor{NewTensor(dy.data, f.inputs[0].shape...)}
}

type sigmoid struct {
	base
}

func (s *sigmoid) String() string {
	return "Sigmoid"
}

func (s *sigmoid) forward(inputs ...*Tensor) *Tensor {
	// y = tanh(x * 0.5) * 0.5 + 0.5
	y := inputs[0].clone()
	y.mul(NewScalar(0.5))
	y.tanh()
	y.mul(NewScalar(0.5))
	y.add(NewScalar(0.5))
	return y
}

func (s *sigmoid) backward(dy *Tensor) []*Tensor {
	// dx = dy * y * (1 - y)
	dx := dy.clone()
	dx.mul(s.output)
	dx.mul(Sub(NewScalar(1), s.output))
	return []*Tensor{dx}
}

type relu struct {
	base
}

func (r *relu) String() string {
	return "ReLU"
}

func (r *relu) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.maximum(NewScalar(0))
	return y
}

func (r *relu) backward(dy *Tensor) []*Tensor {
	dx := dy.clone()
	dx.maximum(NewScalar(0))
	return []*Tensor{dx}
}

// Add returns the element-wise sum of two tensors. The shape of the second tensor must be a suffix sequence of the shape of the first tensor.
func Add(x0, x1 *Tensor) *Tensor {
	if len(x0.shape) < len(x1.shape) {
		x0, x1 = x1, x0
	}
	for i := 0; i < len(x1.shape); i++ {
		if x0.shape[len(x0.shape)-len(x1.shape)+i] != x1.shape[i] {
			panic("the shape of the second tensor must be a suffix sequence of the shape of the first tensor")
		}
	}
	return apply(&add{}, x0, x1)
}

// Sub returns the element-wise difference of two tensors. The shape of the second tensor must be a suffix sequence of the shape of the first tensor.
func Sub(x0, x1 *Tensor) *Tensor {
	if len(x0.shape) < len(x1.shape) {
		x0, x1 = x1, x0
	}
	for i := 0; i < len(x1.shape); i++ {
		if x0.shape[len(x0.shape)-len(x1.shape)+i] != x1.shape[i] {
			panic("the shape of the second tensor must be a suffix sequence of the shape of the first tensor")
		}
	}
	return apply(&sub{}, x0, x1)
}

// Mul returns the element-wise product of two tensors. The shape of the second tensor must be a suffix sequence of the shape of the first tensor.
func Mul(x0, x1 *Tensor) *Tensor {
	if len(x0.shape) < len(x1.shape) {
		x0, x1 = x1, x0
	}
	for i := 0; i < len(x1.shape); i++ {
		if x0.shape[len(x0.shape)-len(x1.shape)+i] != x1.shape[i] {
			panic("the shape of the second tensor must be a suffix sequence of the shape of the first tensor")
		}
	}
	return apply(&mul{}, x0, x1)
}

// Div returns the element-wise division of two tensors. The shape of the second tensor must be a suffix sequence of the shape of the first tensor.
func Div(x0, x1 *Tensor) *Tensor {
	if len(x0.shape) < len(x1.shape) {
		x0, x1 = x1, x0
	}
	for i := 0; i < len(x1.shape); i++ {
		if x0.shape[len(x0.shape)-len(x1.shape)+i] != x1.shape[i] {
			panic("the shape of the second tensor must be a suffix sequence of the shape of the first tensor")
		}
	}
	return apply(&div{}, x0, x1)
}

// Square returns the element-wise square of a tensor.
func Square(x *Tensor) *Tensor {
	return apply(&square{}, x)
}

// Pow returns the element-wise power of a tensor. The shape of the second tensor must be a suffix sequence of the shape of the first tensor.
func Pow(x *Tensor, n *Tensor) *Tensor {
	if len(x.shape) < len(x.shape) {
		panic("the shape of the second tensor must be a suffix sequence of the shape of the first tensor")
	}
	for i := 0; i < len(x.shape); i++ {
		if x.shape[len(x.shape)-len(x.shape)+i] != x.shape[i] {
			panic("the shape of the second tensor must be a suffix sequence of the shape of the first tensor")
		}
	}
	return apply(&pow{}, x, n)
}

// Exp returns the element-wise exponential of a tensor.
func Exp(x *Tensor) *Tensor {
	return apply(&exp{}, x)
}

// Log returns the element-wise natural logarithm of a tensor.
func Log(x *Tensor) *Tensor {
	return apply(&log{}, x)
}

// Sin returns the element-wise sine of a tensor.
func Sin(x *Tensor) *Tensor {
	return apply(&sin{}, x)
}

func Cos(x *Tensor) *Tensor {
	return apply(&cos{}, x)
}

// Sum returns the sum of all elements in a tensor.
func Sum(x *Tensor) *Tensor {
	return apply(&sum{}, x)
}

// Mean returns the mean of all elements in a tensor.
func Mean(x *Tensor) *Tensor {
	return apply(&mean{}, x)
}

func MatMul(x, y *Tensor) *Tensor {
	return apply(&matMul{}, x, y)
}

func Broadcast(x *Tensor, shape ...int) *Tensor {
	return apply(&broadcast{shape: shape}, x)
}

func Flatten(x *Tensor) *Tensor {
	return apply(&flatten{}, x)
}

func Sigmoid(x *Tensor) *Tensor {
	return apply(&sigmoid{}, x)
}

func ReLu(x *Tensor) *Tensor {
	return apply(&relu{}, x)
}
