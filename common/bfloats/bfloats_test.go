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

package bfloats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEuclidean(t *testing.T) {
	a := FromFloat32([]float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	b := FromFloat32([]float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20})
	assert.Equal(t, float32(19.621416), Euclidean(a, b))
	assert.Panics(t, func() { Euclidean([]uint16{1}, nil) })
}
