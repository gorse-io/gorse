// Copyright 2026 gorse Project Authors
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptimizers(t *testing.T) {
	for _, test := range []struct {
		name string
		new  func([]*Tensor, float32) Optimizer
	}{
		{"SGD", NewSGD},
		{"Adam", NewAdam},
	} {
		t.Run(test.name, func(t *testing.T) {
			full := Ones(2)
			half := full.ToFloat16()
			assert.NotPanics(t, func() { test.new([]*Tensor{full}, 0.01) })
			assert.PanicsWithValue(t, "optimizers only support FP32 parameters", func() {
				test.new([]*Tensor{half}, 0.01)
			})
			assert.PanicsWithValue(t, "optimizers only support FP32 parameters", func() {
				test.new([]*Tensor{full, half}, 0.01)
			})
		})
	}
}
