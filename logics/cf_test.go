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

package logics

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMatrixFactorizationUsers(t *testing.T) {
	users := NewMatrixFactorizationUsers()
	users.Add("1", []float32{1, 1, 1})
	users.Add("2", []float32{2, 2, 2})
	users.Add("3", []float32{3, 3, 3})

	path := filepath.Join(t.TempDir(), "users")
	f, err := os.Create(path)
	assert.NoError(t, err)
	defer f.Close()
	err = users.Marshal(f)
	assert.NoError(t, err)

	f, err = os.Open(path)
	assert.NoError(t, err)
	defer f.Close()
	users2 := NewMatrixFactorizationUsers()
	err = users2.Unmarshal(f)
	assert.NoError(t, err)
	assert.Equal(t, users.embeddings, users2.embeddings)
}
