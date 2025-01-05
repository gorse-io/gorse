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
	alpha float32
	beta1 float32
	beta2 float32
	eps   float32
	ms    map[*Tensor]*Tensor
	vs    map[*Tensor]*Tensor
	t     float32
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
	for _, p := range a.params {
		if _, ok := a.ms[p]; !ok {
			a.ms[p] = Zeros(p.shape...)
			a.vs[p] = Zeros(p.shape...)
		}

		m, v := a.ms[p], a.vs[p]
		grad := p.grad.data

		for i := range m.data {
			// m_t = beta1 * m + (1 - beta1) * grad
			m.data[i] = a.beta1*m.data[i] + (1-a.beta1)*grad[i]
			// v_t = beta2 * v + (1 - beta2) * grad^2
			v.data[i] = a.beta2*v.data[i] + (1-a.beta2)*grad[i]*grad[i]
			// \hat{m} = m / (1 - beta1^t)
			hatM := m.data[i] / (1 - math32.Pow(a.beta1, a.t))
			// \hat{v} = v / (1 - beta2^t)
			hatV := v.data[i] / (1 - math32.Pow(a.beta2, a.t))
			// p_t = p - alpha * \hat{m} / (\sqrt{\hat{v}} + eps)
			p.data[i] -= a.alpha * hatM / (math32.Sqrt(hatV) + a.eps)
		}
	}
}
