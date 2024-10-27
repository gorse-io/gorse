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

func TestTensor_Slice(t *testing.T) {
	x := RandN(3, 4, 5)
	y := x.Slice(1, 3)
	assert.Equal(t, []int{2, 4, 5}, y.Shape())
	for i := 0; i < 2; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 5; k++ {
				assert.Equal(t, x.Get(i+1, j, k), y.Get(i, j, k))
			}
		}
	}
}
