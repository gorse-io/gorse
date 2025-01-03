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
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLinearRegression(t *testing.T) {
	x := RandN(100, 1)
	y := Add(RandN(100, 1), NewScalar(5), Mul(NewScalar(2), x))

	w := Zeros(1, 1)
	b := Zeros(1)
	predict := func(x *Tensor) *Tensor { return Add(MatMul(x, w), b) }

	for i := 0; i < 100; i++ {
		yPred := predict(x)
		loss := MeanSquareError(y, yPred)

		w.grad = nil
		b.grad = nil
		loss.Backward()

		w.sub(w.grad.mul(NewScalar(0.01)))
		b.sub(b.grad.mul(NewScalar(0.01)))
	}

	assert.Equal(t, []int{1, 1}, w.shape)
	assert.InDelta(t, float64(2), w.data[0], 0.7)
	assert.Equal(t, []int{1}, b.shape)
	assert.InDelta(t, float64(5), b.data[0], 0.7)
}
