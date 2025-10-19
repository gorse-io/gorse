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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopKFilter(t *testing.T) {
	// Test a adjacent vec
	a := NewTopKFilter[int32, float32](3)
	a.Push(10, 2)
	a.Push(20, 8)
	a.Push(30, 1)
	values := a.PopAllValues()
	assert.Equal(t, []int32{20, 10, 30}, values)
	// Test a full adjacent vec
	a = NewTopKFilter[int32, float32](3)
	a.Push(10, 2)
	a.Push(20, 8)
	a.Push(30, 1)
	a.Push(40, 2)
	a.Push(50, 5)
	a.Push(12, 10)
	a.Push(67, 7)
	a.Push(32, 9)
	elems := a.PopAll()
	assert.Equal(t, []Elem[int32, float32]{
		{Value: 12, Weight: 10},
		{Value: 32, Weight: 9},
		{Value: 20, Weight: 8},
	}, elems)
}

func TestTopKStringFilter(t *testing.T) {
	// Test a adjacent vec
	a := NewTopKFilter[string, float64](3)
	a.Push("10", 2)
	a.Push("20", 8)
	a.Push("30", 1)
	elems := a.PopAll()
	assert.Equal(t, []Elem[string, float64]{
		{Value: "20", Weight: 8},
		{Value: "10", Weight: 2},
		{Value: "30", Weight: 1},
	}, elems)
	// Test a full adjacent vec
	a = NewTopKFilter[string, float64](3)
	a.Push("10", 2)
	a.Push("20", 8)
	a.Push("30", 1)
	a.Push("40", 2)
	a.Push("50", 5)
	a.Push("12", 10)
	a.Push("67", 7)
	a.Push("32", 9)
	elems = a.PopAll()
	assert.Equal(t, []Elem[string, float64]{
		{Value: "12", Weight: 10},
		{Value: "32", Weight: 9},
		{Value: "20", Weight: 8},
	}, elems)
}
