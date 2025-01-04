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

import "github.com/chewxy/math32"

type Layer interface {
	Parameters() []*Tensor
	Forward(x *Tensor) *Tensor
}

type Model Layer

type LinearLayer struct {
	W *Tensor
	B *Tensor
}

func NewLinear(in, out int) Layer {
	return &LinearLayer{
		W: Normal(0, 1.0/math32.Sqrt(float32(in)), in, out).RequireGrad(),
		B: Zeros(out).RequireGrad(),
	}
}

func (l *LinearLayer) Forward(x *Tensor) *Tensor {
	return Add(MatMul(x, l.W), l.B)
}

func (l *LinearLayer) Parameters() []*Tensor {
	return []*Tensor{l.W, l.B}
}

type flattenLayer struct{}

func NewFlatten() Layer {
	return &flattenLayer{}
}

func (f *flattenLayer) Parameters() []*Tensor {
	return nil
}

func (f *flattenLayer) Forward(x *Tensor) *Tensor {
	return Flatten(x)
}

type EmbeddingLayer struct {
	W *Tensor
}

func NewEmbedding(n int, shape ...int) Layer {
	wShape := append([]int{n}, shape...)
	return &EmbeddingLayer{
		W: Rand(wShape...),
	}
}

func (e *EmbeddingLayer) Parameters() []*Tensor {
	return []*Tensor{e.W}
}

func (e *EmbeddingLayer) Forward(x *Tensor) *Tensor {
	return Embedding(e.W, x)
}

type sigmoidLayer struct{}

func NewSigmoid() Layer {
	return &sigmoidLayer{}
}

func (s *sigmoidLayer) Parameters() []*Tensor {
	return nil
}

func (s *sigmoidLayer) Forward(x *Tensor) *Tensor {
	return Sigmoid(x)
}

type reluLayer struct{}

func NewReLU() Layer {
	return &reluLayer{}
}

func (r *reluLayer) Parameters() []*Tensor {
	return nil
}

func (r *reluLayer) Forward(x *Tensor) *Tensor {
	return ReLu(x)
}

type Sequential struct {
	Layers []Layer
}

func NewSequential(layers ...Layer) Model {
	return &Sequential{Layers: layers}
}

func (s *Sequential) Parameters() []*Tensor {
	var params []*Tensor
	for _, l := range s.Layers {
		params = append(params, l.Parameters()...)
	}
	return params
}

func (s *Sequential) Forward(x *Tensor) *Tensor {
	for _, l := range s.Layers {
		x = l.Forward(x)
	}
	return x
}
