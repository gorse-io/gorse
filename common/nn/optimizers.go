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

type Optimizer interface {
	ZeroGrad()
	Step()
}

type baseOptimizer struct {
	params []*Tensor
}

func (o *baseOptimizer) ZeroGrad() {
	for _, p := range o.params {
		p.grad = nil
	}
}

type SGD struct {
	baseOptimizer
	lr float32
}

func NewSGD(params []*Tensor, lr float32) Optimizer {
	return &SGD{
		baseOptimizer: baseOptimizer{params: params},
		lr:            lr,
	}
}

func (s *SGD) Step() {
	for _, p := range s.params {
		for i := range p.data {
			p.data[i] -= s.lr * p.grad.data[i]
		}
	}
}

type Adam struct {
	baseOptimizer
	lr float32
}

func NewAdam(params []*Tensor, lr float32) *Adam {
	return &Adam{
		baseOptimizer: baseOptimizer{params: params},
		lr:            lr,
	}
}

func (a *Adam) Step() {
	for _, p := range a.params {
		for i := range p.data {
			p.data[i] -= a.lr * p.grad.data[i]
		}
	}
}
