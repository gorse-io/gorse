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

type layer interface {
	Parameters() []*Tensor
}

type Linear struct {
	w *Tensor
	b *Tensor
}

func NewLinear(in, out int) *Linear {
	return &Linear{
		w: RandN(in, out),
		b: RandN(out),
	}
}

func (l *Linear) Forward(x *Tensor) *Tensor {
	return Add(MatMul(x, l.w), l.b)
}

func (l *Linear) Parameters() []*Tensor {
	return []*Tensor{l.w, l.b}
}
