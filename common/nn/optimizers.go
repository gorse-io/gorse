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
	"github.com/zhenghaoz/gorse/common/floats"
)

type Optimizer interface {
	SetWeightDecay(rate float32)
	ZeroGrad()
	Step()
}

type baseOptimizer struct {
	params []*Tensor
	wd     float32
}

func (o *baseOptimizer) ZeroGrad() {
	for _, p := range o.params {
		p.grad = nil
	}
}

func (o *baseOptimizer) SetWeightDecay(wd float32) {
	o.wd = wd
}

type SGD struct {
	baseOptimizer
	lr float32
	b  []float32
}

func NewSGD(params []*Tensor, lr float32) Optimizer {
	return &SGD{
		baseOptimizer: baseOptimizer{params: params},
		lr:            lr,
	}
}

func (s *SGD) Step() {
	for _, p := range s.params {
		if len(s.b) < len(p.data) {
			s.b = make([]float32, len(p.data))
		}
		b := s.b[:len(p.data)]
		floats.MulConstTo(p.data, s.wd, b)
		floats.Add(b, p.grad.data)
		floats.MulConstAddTo(b, -s.lr, p.data)
	}
}

type Adam struct {
	baseOptimizer
	alpha float32
	beta1 float32
	beta2 float32
	eps   float32
	ms    map[*Tensor]*Tensor
	vs    map[*Tensor]*Tensor
	t     float32
	b1    []float32
	b2    []float32
}

func NewAdam(params []*Tensor, alpha float32) Optimizer {
	return &Adam{
		baseOptimizer: baseOptimizer{params: params},
		alpha:         alpha,
		beta1:         0.9,
		beta2:         0.999,
		eps:           1e-8,
		ms:            make(map[*Tensor]*Tensor),
		vs:            make(map[*Tensor]*Tensor),
	}
}

func (a *Adam) Step() {
	a.t++

	fix1 := 1 - math32.Pow(a.beta1, a.t)
	fix2 := 1 - math32.Pow(a.beta2, a.t)
	lr := a.alpha * math32.Sqrt(fix2) / fix1

	for _, p := range a.params {
		if _, ok := a.ms[p]; !ok {
			a.ms[p] = Zeros(p.shape...)
			a.vs[p] = Zeros(p.shape...)
			if len(p.data) > len(a.b1) {
				a.b1 = make([]float32, len(p.data))
				a.b2 = make([]float32, len(p.data))
			}
		}
		m, v := a.ms[p], a.vs[p]
		b1, b2 := a.b1[:len(p.data)], a.b2[:len(p.data)]

		// grad = grad + wd * param.data
		floats.MulConstTo(p.data, a.wd, b1)
		floats.Add(b1, p.grad.data)
		// m += (1 - beta1) * (grad - m)
		floats.SubTo(b1, m.data, b2)
		floats.MulConstAddTo(b2, 1-a.beta1, m.data)
		// v += (1 - beta2) * (grad * grad - v)
		floats.MulTo(b1, b1, b2)
		floats.Sub(b2, v.data)
		floats.MulConstAddTo(b2, 1-a.beta2, v.data)
		// param.data -= self.lr * m / (xp.sqrt(v) + eps)
		floats.SqrtTo(v.data, b2)
		floats.AddConst(b2, a.eps)
		floats.DivTo(m.data, b2, b1)
		floats.MulConstAddTo(b1, -lr, p.data)
	}
}
