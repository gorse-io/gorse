// Copyright 2025 gorse Project Authors
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

package sizeof

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCyclic(t *testing.T) {
	type V struct {
		Z int
		E *V
	}

	v := &V{Z: 25}
	want := DeepSize(v)
	v.E = v // induce a cycle
	got := DeepSize(v)
	if got != want {
		t.Errorf("Cyclic size: got %d, want %d", got, want)
	}
}

func TestDeepSize(t *testing.T) {
	// matrix
	a := [][]int64{{1}, {2}, {3}, {4}}
	assert.Equal(t, 5*24+4*8, DeepSize(a))
	b := [][]int32{{1}, {2}, {3}, {4}}
	assert.Equal(t, 5*24+4*4, DeepSize(b))
	c := [][]int16{{1}, {2}, {3}, {4}}
	assert.Equal(t, 5*24+4*2, DeepSize(c))
	d := [][]int8{{1}, {2}, {3}, {4}}
	assert.Equal(t, 5*24+4, DeepSize(d))

	// strings
	e := []string{"abc", "de", "f"}
	assert.Equal(t, 24+16*3+6, DeepSize(e))
	f := []string{"♥♥♥", "♥♥", "♥"}
	assert.Equal(t, 24+16*3+18, DeepSize(f))

	// slice
	g := []int64{1, 2, 3, 4}
	assert.Equal(t, 7*8, DeepSize(g))
	h := []int32{1, 2, 3, 4}
	assert.Equal(t, 3*8+4*4, DeepSize(h))
	i := []int16{1, 2, 3, 4}
	assert.Equal(t, 3*8+2*4, DeepSize(i))
	j := []int8{1, 2, 3, 4}
	assert.Equal(t, 3*8+4, DeepSize(j))
}
