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

package datautil

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLoadIris(t *testing.T) {
	data, target, err := LoadIris()
	assert.NoError(t, err)
	assert.Len(t, data, 150)
	assert.Len(t, data[0], 4)
	assert.Len(t, target, 150)
}
