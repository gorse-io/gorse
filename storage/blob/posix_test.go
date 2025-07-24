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

package blob

import (
	"github.com/stretchr/testify/assert"
	"path"
	"testing"
)

func TestPOSIX(t *testing.T) {
	// create client
	client := NewPOSIX(path.Join(t.TempDir(), "blob"))

	// write a temp file
	w, done, err := client.Create("test")
	assert.NoError(t, err)
	_, err = w.Write([]byte("hello world"))
	assert.NoError(t, err)
	assert.NoError(t, w.Close())
	<-done

	// read the file
	r, err := client.Open("test")
	assert.NoError(t, err)
	content := make([]byte, 11)
	_, err = r.Read(content)
	assert.NoError(t, err)
	assert.Equal(t, "hello world", string(content))
	assert.NoError(t, r.Close())
}
