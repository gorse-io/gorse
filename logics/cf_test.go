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
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMatrixFactorizationItems(t *testing.T) {
	ts := time.Now()
	items := NewMatrixFactorizationItems(ts)
	items.Add("1", []float32{1, 1, 1})
	items.Add("2", []float32{2, 2, 2})
	items.Add("3", []float32{3, 3, 3})
	items.Add("4", []float32{4, 4, 4})
	items.Add("5", []float32{5, 5, 5})

	path := filepath.Join(t.TempDir(), "items")
	f, err := os.Create(path)
	assert.NoError(t, err)
	defer f.Close()
	err = items.Marshal(f)
	assert.NoError(t, err)

	f, err = os.Open(path)
	assert.NoError(t, err)
	defer f.Close()
	items2 := NewMatrixFactorizationItems(time.Time{})
	err = items2.Unmarshal(f)
	assert.NoError(t, err)

	assert.Equal(t, items.timestamp.UnixNano(), items2.timestamp.UnixNano())
	assert.Equal(t, items.dimension, items2.dimension)
	assert.Equal(t, items.items, items2.items)
	scores := items2.Search([]float32{1, 1, 1}, 3)
	assert.Equal(t, []cache.Score{
		{Id: "5", Score: 15, Timestamp: items2.timestamp},
		{Id: "4", Score: 12, Timestamp: items2.timestamp},
		{Id: "3", Score: 9, Timestamp: items2.timestamp},
	}, scores)
}

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
