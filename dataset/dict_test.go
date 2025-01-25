// Copyright 2025 gorse Project Authors
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

package dataset

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFreqDict(t *testing.T) {
	dict := NewFreqDict()
	assert.Equal(t, 0, dict.Id("a"))
	assert.Equal(t, 1, dict.Id("b"))
	assert.Equal(t, 1, dict.Id("b"))
	assert.Equal(t, 2, dict.Id("c"))
	assert.Equal(t, 2, dict.Id("c"))
	assert.Equal(t, 2, dict.Id("c"))
	assert.Equal(t, 3, dict.Count())
	assert.Equal(t, 1, dict.Freq(0))
	assert.Equal(t, 2, dict.Freq(1))
	assert.Equal(t, 3, dict.Freq(2))
}
