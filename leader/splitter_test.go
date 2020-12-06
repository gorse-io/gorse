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
package leader

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSplitter_Split(t *testing.T) {
	s := NewSplitter()
	parts := s.Split(5)
	assert.Equal(t, 5, len(parts))
	count := 0
	for _, p := range parts {
		count += len(p)
	}
	assert.Equal(t, 26*2+10, count)
}
