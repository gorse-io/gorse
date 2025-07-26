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
	"weak"

	"github.com/chewxy/math32"
)

type op interface {
	String() string
	forward(inputs ...*Tensor) *Tensor
	backward(dy *Tensor) []*Tensor
	inputsAndOutput() ([]*Tensor, *Tensor)
	setInputs(inputs ...*Tensor)
	setOutput(y *Tensor)
	generation() int
	setGeneration(gen int)
}

type base struct {
	inputs []*Tensor
	output weak.Pointer[Tensor]
	gen    int
}

func (b *base) inputsAndOutput() ([]*Tensor, *Tensor) {
	return b.inputs, b.output.Value()
}

func (b *base) setInputs(inputs ...*Tensor) {
	b.inputs = inputs
}

func (b *base) setOutput(y *Tensor) {
	b.output = weak.Make(y)
}

func (b *base) generation() int {
	return b.gen
}

func (b *base) setGeneration(gen int) {
	b.gen = gen
}

func apply[T op](f T, inputs ...*Tensor) *Tensor {
	y := f.forward(inputs...)
	f.setInputs(inputs...)
	f.setOutput(y)
	y.op = f

	// Set generation
	gen := 0
	for _, x := range inputs {
		gen = max(gen, x.generation())
	}
	f.setGeneration(gen + 1)
	return y
}

type neg struct {
	base
}

func (n *neg) String() string {
	return "Neg"
}

func (n *neg) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.neg()
	return y
}

func (n *neg) backward(dy *Tensor) []*Tensor {
	dx := dy.clone()
	dx.neg()
	return []*Tensor{dx}
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
		dx1.data[i%wSize] += dy.data[i] * p.output.Value().data[i] * math32.Log(p.inputs[0].data[i])
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
	dx := dy.clone()
	dx.div(l.inputs[0])
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

func (s *sum) backward(dy *Tensor) []*Tensor {
	dx := Zeros(s.inputs[0].shape...)
	for i := range dx.data {
		dx.data[i] = dy.data[0]
	}
	return []*Tensor{dx}
}

type partialSum struct {
	base
	along int64
}

func (p *partialSum) String() string {
	return "Sum"
}

func (p *partialSum) forward(inputs ...*Tensor) *Tensor {
	x := inputs[0]
	// Squash the shape.
	s1, s2, s3 := 1, 1, 1
	for i := 0; i < len(x.shape); i++ {
		if int64(i) == p.along {
			s2 = x.shape[i]
		} else if int64(i) < p.along {
			s1 *= x.shape[i]
		} else {
			s3 *= x.shape[i]
		}
	}
	// Calculate the output size and shape.
	outputSize := s1 * s3
	outputShape := make([]int, 0)
	for i := 0; i < len(x.shape); i++ {
		if int64(i) != p.along {
			outputShape = append(outputShape, x.shape[i])
		}
	}
	// Calculate the output.
	y := NewTensor(make([]float32, outputSize), outputShape...)
	for i := 0; i < s1; i++ {
		for j := 0; j < s2; j++ {
			for k := 0; k < s3; k++ {
				y.data[i*s3+k] += x.data[i*s2*s3+j*s3+k]
			}
		}
	}
	return y
}

func (p *partialSum) backward(dy *Tensor) []*Tensor {
	x := p.inputs[0]
	// Squash the shape.
	s1, s2, s3 := 1, 1, 1
	for i := 0; i < len(x.shape); i++ {
		if int64(i) == p.along {
			s2 = x.shape[i]
		} else if int64(i) < p.along {
			s1 *= x.shape[i]
		} else {
			s3 *= x.shape[i]
		}
	}
	// Calculate the output.
	dx := Zeros(x.shape...)
	for i := 0; i < s1; i++ {
		for j := 0; j < s2; j++ {
			for k := 0; k < s3; k++ {
				dx.data[i*s2*s3+j*s3+k] = dy.data[i*s3+k]
			}
		}
	}
	return []*Tensor{dx}
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

func (m *mean) backward(dy *Tensor) []*Tensor {
	dx := Zeros(m.inputs[0].shape...)
	for i := range dx.data {
		dx.data[i] = dy.data[0] / float32(len(dx.data))
	}
	return []*Tensor{dx}
}

type matMul struct {
	base
	transpose1 bool
	transpose2 bool
	jobs       int
}

func (m *matMul) String() string {
	return "MatMul"
}

func (m *matMul) forward(inputs ...*Tensor) *Tensor {
	return inputs[0].matMul(inputs[1], m.transpose1, m.transpose2, m.jobs)
}

func (m *matMul) backward(dy *Tensor) []*Tensor {
	var dx0, dx1 *Tensor
	if !m.transpose1 && !m.transpose2 { // y = x0 * x1
		// dx0 = dy * x1^T
		dx0 = dy.matMul(m.inputs[1], false, true, m.jobs)
		// dx1 = x0^T * dy
		dx1 = m.inputs[0].matMul(dy, true, false, m.jobs)
	} else if m.transpose1 && !m.transpose2 { // y = x0^T * x1
		// dx0 = dy * x1^T
		dx0 = m.inputs[1].matMul(dy, false, true, m.jobs)
		// dx1 = dy^T * x0
		dx1 = m.inputs[0].matMul(dy, false, false, m.jobs)
	} else if !m.transpose1 && m.transpose2 { // y = x0 * x1^T
		// dx0 = dy * x1
		dx0 = dy.matMul(m.inputs[1], false, false, m.jobs)
		// dx1 = dy^T * x0
		dx1 = dy.matMul(m.inputs[0], true, false, m.jobs)
	} else { // y = x0^T * x1^T
		// dx0 = x1 * dy^T
		dx0 = m.inputs[1].matMul(dy, true, true, m.jobs)
		// dx1 = dy * x0^T
		dx1 = dy.matMul(m.inputs[0], true, true, m.jobs)
	}
	return []*Tensor{dx0, dx1}
}

type batchMatMul struct {
	base
	transpose1 bool
	transpose2 bool
	jobs       int
}

func (b *batchMatMul) String() string {
	return "BatchMatMul"
}

func (b *batchMatMul) forward(inputs ...*Tensor) *Tensor {
	return inputs[0].batchMatMul(inputs[1], b.transpose1, b.transpose2, b.jobs)
}

func (b *batchMatMul) backward(dy *Tensor) []*Tensor {
	var dx0, dx1 *Tensor
	if !b.transpose1 && !b.transpose2 { // y = x0 * x1
		// dx0 = dy * x1^T
		dx0 = dy.batchMatMul(b.inputs[1], false, true, b.jobs)
		// dx1 = x0^T * dy
		dx1 = b.inputs[0].batchMatMul(dy, true, false, b.jobs)
	} else if b.transpose1 && !b.transpose2 { // y = x0^T * x1
		// dx0 = dy * x1^T
		dx0 = b.inputs[1].batchMatMul(dy, false, true, b.jobs)
		// dx1 = dy^T * x0
		dx1 = b.inputs[0].batchMatMul(dy, false, false, b.jobs)
	} else if !b.transpose1 && b.transpose2 { // y = x0 * x1^T
		// dx0 = dy * x1
		dx0 = dy.batchMatMul(b.inputs[1], false, false, b.jobs)
		// dx1 = dy^T * x0
		dx1 = dy.batchMatMul(b.inputs[0], true, false, b.jobs)
	} else { // y = x0^T * x1^T
		// dx0 = x1 * dy^T
		dx0 = b.inputs[1].batchMatMul(dy, true, true, b.jobs)
		// dx1 = dy * x0^T
		dx1 = dy.batchMatMul(b.inputs[0], true, true, b.jobs)
	}
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

type reshape struct {
	base
	shape []int
}

func (r *reshape) String() string {
	return "Reshape"
}

func (r *reshape) forward(inputs ...*Tensor) *Tensor {
	return NewTensor(inputs[0].data, r.shape...)
}

func (r *reshape) backward(dy *Tensor) []*Tensor {
	return []*Tensor{NewTensor(dy.data, r.inputs[0].shape...)}
}

type embedding struct {
	base
}

func (e *embedding) String() string {
	return "Embedding"
}

func (e *embedding) forward(inputs ...*Tensor) *Tensor {
	w, x := inputs[0], inputs[1]
	// Calculate embedding size
	dim := 1
	for i := 1; i < len(w.shape); i++ {
		dim *= w.shape[i]
	}
	// Calculate shape
	shape := make([]int, len(x.shape), len(x.shape)+1)
	copy(shape, x.shape)
	shape = append(shape, w.shape[1:]...)
	// Calculate data size
	size := 1
	for _, s := range shape {
		size *= s
	}
	// Create output tensor
	data := make([]float32, size)
	for i := 0; i < len(x.data); i++ {
		index := int(x.data[i])
		copy(data[i*dim:(i+1)*dim], w.data[index*dim:(index+1)*dim])
	}
	return NewTensor(data, shape...)
}

func (e *embedding) backward(dy *Tensor) []*Tensor {
	w, x := e.inputs[0], e.inputs[1]
	dim := 1
	for i := 1; i < len(w.shape); i++ {
		dim *= w.shape[i]
	}
	dw := Zeros(w.shape...)
	for i := 0; i < len(x.data); i++ {
		index := int(x.data[i])
		for j := 0; j < dim; j++ {
			dw.data[index*dim+j] += dy.data[i*dim+j]
		}
	}
	return []*Tensor{dw}
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
	dx := s.output.Value().clone()
	dx.neg()
	dx.add(NewScalar(1))
	dx.mul(s.output.Value())
	dx.mul(dy)
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
	x := r.inputs[0]
	dx := x.clone().gt(NewScalar(0)).mul(dy)
	return []*Tensor{dx}
}

type softmax struct {
	base
	axis int
}

func (s *softmax) String() string {
	return "Softmax"
}

func (s *softmax) forward(inputs ...*Tensor) *Tensor {
	x := inputs[0]
	y := x.clone()
	y.sub(x.max(s.axis, true))
	y.exp()
	y.div(y.sum(s.axis, true))
	return y
}

func (s *softmax) backward(dy *Tensor) []*Tensor {
	y := s.output.Value()
	gx := y.clone()
	gx.mul(dy)
	sumdx := gx.sum(s.axis, true)
	y.mul(sumdx)
	gx.sub(y)
	return []*Tensor{gx}
}

type softmaxCrossEntropy struct {
	base
}

func (c *softmaxCrossEntropy) String() string {
	return "SoftmaxCrossEntropy"
}

func (c *softmaxCrossEntropy) forward(inputs ...*Tensor) *Tensor {
	x, t := inputs[0], inputs[1]
	m := x.max(1, true)
	s := x.clone().bSub(m)    // x - m
	s = s.exp()               // exp(x - m)
	s = s.sum(1, true)        // sum(exp(x - m))
	s.log()                   // log(sum(exp(x - m)))
	m.add(s)                  // m + log(sum(exp(x - m)))
	logP := x.clone().bSub(m) // x - (m + log(sum(exp(x - m))))
	var crossEntropy float32
	for i := 0; i < len(t.data); i++ {
		crossEntropy -= logP.Get(i, int(t.data[i]))
	}
	crossEntropy /= float32(len(t.data))
	return NewScalar(crossEntropy)
}

func (c *softmaxCrossEntropy) backward(dy *Tensor) []*Tensor {
	x, t := c.inputs[0], c.inputs[1]
	// gy *= 1/N
	gy := dy.clone().mul(NewScalar(1 / float32(len(t.data))))
	// y = softmax(x)
	y := x.clone()
	y.bSub(x.max(1, true))
	y.exp()
	y.bDiv(y.sum(1, true))
	// convert to one-hot
	oneHot := Zeros(x.shape...)
	for i := 0; i < len(t.data); i++ {
		oneHot.data[i*x.shape[1]+int(t.data[i])] = 1
	}
	// y = (y - t_onehot) * gy
	y = y.sub(oneHot).mul(gy)
	return []*Tensor{y, Zeros(t.shape...)}
}

type opHeap []op

func (h opHeap) Len() int {
	return len(h)
}

func (h opHeap) Less(i, j int) bool {
	return h[i].generation() > h[j].generation()
}

func (h opHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *opHeap) Push(o any) {
	*h = append(*h, o.(op))
}

func (h *opHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
