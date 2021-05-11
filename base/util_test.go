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
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMax(t *testing.T) {
	assert.Equal(t, 10, Max(2, 4, 6, 8, 10))
}

func TestNewMatrix32(t *testing.T) {
	a := NewMatrix32(3, 4)
	assert.Equal(t, 3, len(a))
	assert.Equal(t, 4, len(a[0]))
	assert.Equal(t, 4, len(a[0]))
	assert.Equal(t, 4, len(a[0]))
}

func TestHex(t *testing.T) {
	assert.Equal(t, fmt.Sprintf("%x", 325600), Hex(325600))
}

func TestRangeInt(t *testing.T) {
	a := RangeInt(7)
	assert.Equal(t, 7, len(a))
	for i := range a {
		assert.Equal(t, i, a[i])
	}
}

func TestNewMatrixInt(t *testing.T) {
	m := NewMatrixInt(4, 3)
	assert.Equal(t, 4, len(m))
	for _, v := range m {
		assert.Equal(t, 3, len(v))
	}
}
