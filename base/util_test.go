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
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

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

func TestWriteMatrix(t *testing.T) {
	a := [][]float32{{1, 2}, {3, 4}}
	buf := bytes.NewBuffer(nil)
	err := WriteMatrix(buf, a)
	assert.NoError(t, err)
	b := [][]float32{{0, 0}, {0, 0}}
	err = ReadMatrix(buf, b)
	assert.NoError(t, err)
	assert.Equal(t, a, b)
}

func TestWriteString(t *testing.T) {
	a := "abc"
	buf := bytes.NewBuffer(nil)
	err := WriteString(buf, a)
	assert.NoError(t, err)
	var b string
	b, err = ReadString(buf)
	assert.NoError(t, err)
	assert.Equal(t, a, b)
}

func TestWriteGob(t *testing.T) {
	a := "abc"
	buf := bytes.NewBuffer(nil)
	err := WriteGob(buf, a)
	assert.NoError(t, err)
	var b string
	err = ReadGob(buf, &b)
	assert.NoError(t, err)
	assert.Equal(t, a, b)
}

func TestFloat32(t *testing.T) {
	a := FormatFloat32(1.23)
	assert.Equal(t, "1.23", a)
	b := ParseFloat32("1.23")
	assert.Equal(t, float32(1.23), b)
}

func TestSetDevelopmentLogger(t *testing.T) {
	temp, err := os.MkdirTemp("", "test_gorse")
	assert.NoError(t, err)
	// set existed path
	SetDevelopmentLogger(temp + "/gorse.log")
	_, err = os.Stat(temp + "/gorse.log")
	assert.NoError(t, err)
	// set non-existed path
	SetDevelopmentLogger(temp + "/gorse/gorse.log")
	_, err = os.Stat(temp + "/gorse/gorse.log")
	assert.NoError(t, err)
	// permission denied
	assert.Panics(t, func() {
		SetDevelopmentLogger("/gorse.log")
	})
	assert.Panics(t, func() {
		SetDevelopmentLogger("/gorse/gorse.log")
	})
}

func TestSetProductionLogger(t *testing.T) {
	temp, err := os.MkdirTemp("", "test_gorse")
	assert.NoError(t, err)
	// set existed path
	SetProductionLogger(temp + "/gorse.log")
	_, err = os.Stat(temp + "/gorse.log")
	assert.NoError(t, err)
	// set non-existed path
	SetProductionLogger(temp + "/gorse/gorse.log")
	_, err = os.Stat(temp + "/gorse/gorse.log")
	assert.NoError(t, err)
	// permission denied
	assert.Panics(t, func() {
		SetProductionLogger("/gorse.log")
	})
	assert.Panics(t, func() {
		SetProductionLogger("/gorse/gorse.log")
	})
}

func TestToFloat64s(t *testing.T) {
	a := ToFloat64s([]float32{1, 2, 3})
	assert.Equal(t, []float64{1, 2, 3}, a)
}
