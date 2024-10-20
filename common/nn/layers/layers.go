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

package layers

import "github.com/zhenghaoz/gorse/common/nn"

var _ layer = &Linear{}

type layer interface {
	Parameters() []*nn.Tensor
	Forward(x *nn.Tensor) *nn.Tensor
}

type Linear struct {
	w *nn.Tensor
	b *nn.Tensor
}

func NewLinear(in, out int) *Linear {
	return &Linear{
		w: nn.RandN(in, out),
		b: nn.RandN(out),
	}
}

func (l *Linear) Forward(x *nn.Tensor) *nn.Tensor {
	return nn.Add(nn.MatMul(x, l.w), l.b)
}

func (l *Linear) Parameters() []*nn.Tensor {
	return []*nn.Tensor{l.w, l.b}
}

type Embedding struct {
	w *nn.Tensor
}

func NewEmbedding(n int, shape ...int) *Embedding {
	wShape := append([]int{n}, shape...)
	return &Embedding{
		w: nn.RandN(wShape...),
	}
}

func (e *Embedding) Parameters() []*nn.Tensor {
	return []*nn.Tensor{e.w}
}

func (e *Embedding) Forward(x *nn.Tensor) *nn.Tensor {
	return nn.Embedding(e.w, x)
}
