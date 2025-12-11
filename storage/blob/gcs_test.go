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
	"context"
	"io"
	"testing"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/gorse-io/gorse/config"
	"github.com/stretchr/testify/assert"
)

func TestGCS(t *testing.T) {
	server, err := fakestorage.NewServerWithOptions(fakestorage.Options{
		Scheme:     "http",
		Port:       5050,
		PublicHost: "localhost:5050",
	})
	assert.NoError(t, err)
	defer server.Stop()
	t.Setenv("GCS_EMULATOR_ENDPOINT", "http://localhost:5050/storage/v1/")

	// create client
	client, err := NewGCS(config.GCSConfig{
		Bucket: "gorse-test",
		Prefix: "blob",
	})
	assert.NoError(t, err)

	// create bucket if not exists
	err = client.client.Bucket("gorse-test").Create(context.Background(), "test-project", nil)
	if err != nil {
		assert.ErrorContains(t, err, "A Cloud Storage bucket named 'gorse-test' already exists.")
	}

	// create file
	w, done, err := client.Create("test.txt")
	assert.NoError(t, err)
	_, err = w.Write([]byte("hello"))
	assert.NoError(t, err)
	err = w.Close()
	assert.NoError(t, err)
	<-done

	// list files
	names, err := client.List()
	assert.NoError(t, err)
	assert.Equal(t, []string{"test.txt"}, names)

	// read file
	r, err := client.Open("test.txt")
	assert.NoError(t, err)
	data, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, "hello", string(data))
	err = r.Close()
	assert.NoError(t, err)

	// remove file
	err = client.Remove("test.txt")
	assert.NoError(t, err)

	// list files again
	names, err = client.List()
	assert.NoError(t, err)
	assert.Empty(t, names)
}
