// Copyright 2020 gorse Project Authors
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
package base

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMaxHeap(t *testing.T) {
	// Test a adjacent vec
	a := NewMaxHeap(3)
	a.Add(10, 2)
	a.Add(20, 8)
	a.Add(30, 1)
	elem, scores := a.ToSorted()
	assert.Equal(t, []interface{}{20, 10, 30}, elem)
	assert.Equal(t, []float64{8, 2, 1}, scores)
	// Test a full adjacent vec
	a.Add(40, 2)
	a.Add(50, 5)
	a.Add(12, 10)
	a.Add(67, 7)
	a.Add(32, 9)
	elem, scores = a.ToSorted()
	assert.Equal(t, []interface{}{12, 32, 20}, elem)
	assert.Equal(t, []float64{10, 9, 8}, scores)
}
