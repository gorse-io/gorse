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

type Layer interface {
	Parameters() []*Tensor
	Forward(x *Tensor) *Tensor
}

type Model Layer

type linearLayer struct {
	w *Tensor
	b *Tensor
}

func NewLinear(in, out int) Layer {
	return &linearLayer{
		w: Rand(in, out).RequireGrad(),
		b: Zeros(out).RequireGrad(),
	}
}

func (l *linearLayer) Forward(x *Tensor) *Tensor {
	return Add(MatMul(x, l.w), l.b)
}

func (l *linearLayer) Parameters() []*Tensor {
	return []*Tensor{l.w, l.b}
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

type embeddingLayer struct {
	w *Tensor
}

func NewEmbedding(n int, shape ...int) Layer {
	wShape := append([]int{n}, shape...)
	return &embeddingLayer{
		w: Rand(wShape...),
	}
}

func (e *embeddingLayer) Parameters() []*Tensor {
	return []*Tensor{e.w}
}

func (e *embeddingLayer) Forward(x *Tensor) *Tensor {
	return Embedding(e.w, x)
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
	layers []Layer
}

func NewSequential(layers ...Layer) Model {
	return &Sequential{layers: layers}
}

func (s *Sequential) Parameters() []*Tensor {
	var params []*Tensor
	for _, l := range s.layers {
		params = append(params, l.Parameters()...)
	}
	return params
}

func (s *Sequential) Forward(x *Tensor) *Tensor {
	for _, l := range s.layers {
		x = l.Forward(x)
	}
	return x
}
