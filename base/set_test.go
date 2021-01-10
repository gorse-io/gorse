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
	"strconv"
	"testing"
)

func TestSet(t *testing.T) {
	nums := make([]int, 100)
	for i := range nums {
		nums[i] = i
	}
	s := NewSet(nums[:10]...)
	s.Add(nums[10:]...)
	assert.Equal(t, len(nums), s.Len())
	for i := range nums {
		assert.True(t, s.Contain(i))
	}
	assert.False(t, s.Contain(s.Len()))
}

func TestStringSet(t *testing.T) {
	nums := make([]string, 100)
	for i := range nums {
		nums[i] = strconv.Itoa(i)
	}
	s := NewStringSet(nums[:10]...)
	s.Add(nums[10:]...)
	assert.Equal(t, len(nums), s.Len())
	for i := range nums {
		assert.True(t, s.Contain(strconv.Itoa(i)))
	}
	assert.False(t, s.Contain(strconv.Itoa(s.Len())))
}
