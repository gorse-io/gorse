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
	"time"
)

func TestMax(t *testing.T) {
	assert.Equal(t, int32(10), Max(2, 4, 6, 8, 10))
}

func TestMin(t *testing.T) {
	assert.Equal(t, 4, Min(12, 4, 6, 8, 10))
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

func TestNow(t *testing.T) {
	s := Now()
	assert.Regexp(t, "\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}(\\+\\d{2}:\\d{2}|Z)", s)
}

func TestDateNow(t *testing.T) {
	date := DateNow()
	assert.Zero(t, date.Hour())
	assert.Zero(t, date.Minute())
	assert.Zero(t, date.Second())
	assert.Zero(t, date.Nanosecond())
	assert.Equal(t, time.UTC, date.Location())
}
