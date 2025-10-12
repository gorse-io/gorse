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

package jsonutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshal(t *testing.T) {
	var a []int
	err := Unmarshal([]byte("[1,2,3]"), &a)
	assert.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, a)

	err = Unmarshal([]byte(""), &a)
	assert.NoError(t, err)
	assert.Empty(t, a)
}

func TestMarshal(t *testing.T) {
	data, err := Marshal(nil)
	assert.NoError(t, err)
	assert.Equal(t, "null", string(data))
}

func TestMustMarshal(t *testing.T) {
	assert.Panics(t, func() {
		MustMarshal(make(chan int))
	})
}
