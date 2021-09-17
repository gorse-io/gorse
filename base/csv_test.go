// Copyright 2021 gorse Project Authors
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
	"bufio"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestValidateId(t *testing.T) {
	assert.NotNil(t, ValidateId(""))
	assert.NotNil(t, ValidateId("/"))
	assert.Nil(t, ValidateId("abc"))
}

func TestValidateLabel(t *testing.T) {
	assert.NotNil(t, ValidateLabel(""))
	assert.NotNil(t, ValidateLabel("/"))
	assert.NotNil(t, ValidateLabel("|"))
	assert.Nil(t, ValidateLabel("abc"))
}

func TestEscape(t *testing.T) {
	assert.Equal(t, "123", Escape("123"))
	assert.Equal(t, "\"\"\"123\"\"\"", Escape("\"123\""))
	assert.Equal(t, "\"1,2,3\"", Escape("1,2,3"))
	assert.Equal(t, "\"\"\",\"\"\"", Escape("\",\""))
	assert.Equal(t, "\"1\r\n2\r\n3\"", Escape("1\r\n2\r\n3"))
}

func splitLines(t *testing.T, text string) [][]string {
	sc := bufio.NewScanner(strings.NewReader(text))
	lines := make([][]string, 0)
	err := ReadLines(sc, ",", func(i int, i2 []string) bool {
		lines = append(lines, i2)
		return i2[0] != "STOP"
	})
	assert.NoError(t, err)
	return lines
}

func TestReadLines(t *testing.T) {
	assert.Equal(t, [][]string{{"1", "2", "3"}, {"4", "5", "6"}},
		splitLines(t, "1,2,3\r\n4,5,6\r\n"))
	assert.Equal(t, [][]string{{"1,2", "3,4", "5,6"}, {"2,3", "4,6", "6,9"}},
		splitLines(t, "\"1,2\",\"3,4\",\"5,6\"\r\n\"2,3\",\"4,6\",\"6,9\""))
	assert.Equal(t, [][]string{{"\"1,2\",\"3,4\",\"5,6\""}, {"\"2,3\",\"4,6\",\"6,9\""}},
		splitLines(t, "\"\"\"1,2\"\",\"\"3,4\"\",\"\"5,6\"\"\"\r\n\"\"\"2,3\"\",\"\"4,6\"\",\"\"6,9\"\"\""))
	assert.Equal(t, [][]string{{"1\r\n2", "3\r\n4", "5\r\n6"}, {"2\r\n3", "4\r\n6", "6\r\n9"}},
		splitLines(t, "\"1\r\n2\",\"3\r\n4\",\"5\r\n6\"\r\n\"2\r\n3\",\"4\r\n6\",\"6\r\n9\""))
	assert.Equal(t, [][]string{{"1", "2", "3"}, {"4", "5", "6"}, {"STOP"}},
		splitLines(t, "1,2,3\r\n4,5,6\r\nSTOP\r\n7,8,9"))
}
