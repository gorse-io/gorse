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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMatrix32(t *testing.T) {
	a := NewMatrix32(3, 4)
	assert.Equal(t, 3, len(a))
	assert.Equal(t, 4, len(a[0]))
	assert.Equal(t, 4, len(a[0]))
	assert.Equal(t, 4, len(a[0]))
}

func TestRangeInt(t *testing.T) {
	a := RangeInt(7)
	assert.Equal(t, 7, len(a))
	for i := range a {
		assert.Equal(t, i, a[i])
	}
}

func TestRepeatFloat32s(t *testing.T) {
	a := RepeatFloat32s(3, 0.1)
	assert.Equal(t, []float32{0.1, 0.1, 0.1}, a)
}

func TestNewMatrixInt(t *testing.T) {
	m := NewMatrixInt(4, 3)
	assert.Equal(t, 4, len(m))
	for _, v := range m {
		assert.Equal(t, 3, len(v))
	}
}

func TestNewTensor32(t *testing.T) {
	a := NewTensor32(3, 4, 5)
	assert.Equal(t, 3, len(a))
	assert.Equal(t, 4, len(a[0]))
	assert.Equal(t, 5, len(a[0][0]))
}

func TestValidateId(t *testing.T) {
	assert.NotNil(t, ValidateId(""))
	assert.NotNil(t, ValidateId("/"))
	assert.Nil(t, ValidateId("abc"))
}
