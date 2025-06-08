// Copyright 2022 gorse Project Authors
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

package heap

import (
	"sort"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"modernc.org/sortutil"
)

func TestPriorityQueue(t *testing.T) {
	pq := NewPriorityQueue(false)
	elements := []int32{5, 3, 7, 8, 6, 2, 9}
	for _, e := range elements {
		pq.Push(e, float32(e))
	}
	assert.Equal(t, len(elements), pq.Len())
	assert.ElementsMatch(t, elements, pq.Values())
	assert.Equal(t, len(elements), len(pq.Elems()))

	// test clone
	cp := pq.Clone()
	assert.Equal(t, len(elements), cp.Len())

	// test peek pop
	sort.Sort(sortutil.Int32Slice(elements))
	for _, e := range elements {
		value, weight := pq.Peek()
		assert.Equal(t, e, value)
		assert.Equal(t, e, int32(weight))
		value, weight = pq.Pop()
		assert.Equal(t, e, value)
		assert.Equal(t, e, int32(weight))
	}

	// test reverse
	r := cp.Reverse()
	lo.Reverse(elements)
	for _, e := range elements {
		value, weight := r.Pop()
		assert.Equal(t, e, value)
		assert.Equal(t, e, int32(weight))
	}
}
