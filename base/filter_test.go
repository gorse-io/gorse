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

func TestTopKFilter(t *testing.T) {
	// Test a adjacent vec
	a := NewTopKFilter(3)
	a.Push(10, 2)
	a.Push(20, 8)
	a.Push(30, 1)
	elem, scores := a.PopAll()
	assert.Equal(t, []int32{20, 10, 30}, elem)
	assert.Equal(t, []float32{8, 2, 1}, scores)
	// Test a full adjacent vec
	a = NewTopKFilter(3)
	a.Push(10, 2)
	a.Push(20, 8)
	a.Push(30, 1)
	a.Push(40, 2)
	a.Push(50, 5)
	a.Push(12, 10)
	a.Push(67, 7)
	a.Push(32, 9)
	elem, scores = a.PopAll()
	assert.Equal(t, []int32{12, 32, 20}, elem)
	assert.Equal(t, []float32{10, 9, 8}, scores)
}

func TestTopKStringFilter(t *testing.T) {
	// Test a adjacent vec
	a := NewTopKStringFilter(3)
	a.Push("10", 2)
	a.Push("20", 8)
	a.Push("30", 1)
	elem, scores := a.PopAll()
	assert.Equal(t, []string{"20", "10", "30"}, elem)
	assert.Equal(t, []float32{8, 2, 1}, scores)
	// Test a full adjacent vec
	a = NewTopKStringFilter(3)
	a.Push("10", 2)
	a.Push("20", 8)
	a.Push("30", 1)
	a.Push("40", 2)
	a.Push("50", 5)
	a.Push("12", 10)
	a.Push("67", 7)
	a.Push("32", 9)
	elem, scores = a.PopAll()
	assert.Equal(t, []string{"12", "32", "20"}, elem)
	assert.Equal(t, []float32{10, 9, 8}, scores)
}
