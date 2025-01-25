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

package encoding

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHex(t *testing.T) {
	assert.Equal(t, fmt.Sprintf("%x", 325600), Hex(325600))
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
