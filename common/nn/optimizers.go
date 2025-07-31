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
	"sync"

	"github.com/chewxy/math32"
	"github.com/gorse-io/gorse/common/floats"
	"github.com/samber/lo"
)

type Optimizer interface {
	SetWeightDecay(rate float32)
	SetJobs(jobs int)
	ZeroGrad()
	Step()
}

type baseOptimizer struct {
	params []*Tensor
	wd     float32
	jobs   int
}

func (o *baseOptimizer) ZeroGrad() {
	for _, p := range o.params {
		p.grad = nil
	}
}

func (o *baseOptimizer) SetWeightDecay(wd float32) {
	o.wd = wd
}

func (o *baseOptimizer) SetJobs(jobs int) {
	o.jobs = jobs
}

type SGD struct {
	baseOptimizer
	lr float32
	b  []float32
}

func NewSGD(params []*Tensor, lr float32) Optimizer {
	bufSize := 0
	for _, p := range params {
		bufSize = max(bufSize, len(p.data))
	}
	return &SGD{
		baseOptimizer: baseOptimizer{params: params},
		lr:            lr,
		b:             make([]float32, bufSize),
	}
}

func (s *SGD) Step() {
	for _, p := range s.params {
		b := s.b[:len(p.data)]
		parts := partitionAligned(len(p.data), s.jobs, 32)
		var wg sync.WaitGroup
		// TODO: Replace with wg.Go in Go 1.25
		wg.Add(len(parts))
		for _, part := range parts {
			go func(i, j int) {
				defer wg.Done()
				floats.MulConstAddTo(p.data[i:j], s.wd, p.grad.data[i:j], b[i:j])
				floats.MulConstAdd(b[i:j], -s.lr, p.data[i:j])
			}(part.A, part.B)
		}
		wg.Wait()
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

	b1 []float32
	b2 []float32
}

func NewAdam(params []*Tensor, alpha float32) Optimizer {
	bufSize := 0
	for _, p := range params {
		bufSize = max(bufSize, len(p.data))
	}
	return &Adam{
		baseOptimizer: baseOptimizer{params: params},
		alpha:         alpha,
		beta1:         0.9,
		beta2:         0.999,
		eps:           1e-8,
		ms:            make(map[*Tensor]*Tensor),
		vs:            make(map[*Tensor]*Tensor),
		b1:            make([]float32, bufSize),
		b2:            make([]float32, bufSize),
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
		}
		m, v := a.ms[p], a.vs[p]
		b1, b2 := a.b1[:len(p.data)], a.b2[:len(p.data)]

		parts := partitionAligned(len(p.data), a.jobs, 32)
		var wg sync.WaitGroup
		// TODO: Replace with wg.Go in Go 1.25
		wg.Add(len(parts))
		for _, part := range parts {
			go func(i, j int) {
				defer wg.Done()
				// grad = grad + wd * param.data
				floats.MulConstAddTo(p.data[i:j], a.wd, p.grad.data[i:j], b1[i:j])
				// m += (1 - beta1) * (grad - m)
				floats.SubTo(b1[i:j], m.data[i:j], b2[i:j])
				floats.MulConstAdd(b2[i:j], 1-a.beta1, m.data[i:j])
				// v += (1 - beta2) * (grad * grad - v)
				floats.MulTo(b1[i:j], b1[i:j], b2[i:j])
				floats.Sub(b2[i:j], v.data[i:j])
				floats.MulConstAdd(b2[i:j], 1-a.beta2, v.data[i:j])
				// param.data -= self.lr * m / (xp.sqrt(v) + eps)
				floats.SqrtTo(v.data[i:j], b2[i:j])
				floats.AddConst(b2[i:j], a.eps)
				floats.DivTo(m.data[i:j], b2[i:j], b1[i:j])
				floats.MulConstAdd(b1[i:j], -lr, p.data[i:j])
			}(part.A, part.B)
		}
		wg.Wait()
	}
}

// partitionAligned partitions n-size slice into m parts. Each part is aligned to k elements except the last part. For example:
// split(10, 3, 2) = [(0, 4), (4, 8), (8, 10)]
func partitionAligned(n, m, k int) []lo.Tuple2[int, int] {
	if n <= 0 {
		return nil
	}
	if m <= 0 {
		return []lo.Tuple2[int, int]{{0, n}}
	}
	if k <= 0 {
		k = 1
	}
	// calculate the size of each part
	partSize := n / m
	if partSize%k != 0 {
		partSize += k - partSize%k
	}
	if partSize == 0 {
		partSize = k
	}
	// split the slice into m parts
	parts := make([]lo.Tuple2[int, int], 0, m)
	start := 0
	for start < n {
		end := start + partSize
		if end > n {
			end = n
		}
		parts = append(parts, lo.Tuple2[int, int]{A: start, B: end})
		start = end
	}
	return parts
}
