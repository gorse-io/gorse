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
	"os"
	"testing"

	"github.com/gorse-io/gorse/config"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
)

var (
	endpoint        = os.Getenv("S3_ENDPOINT")
	accessKeyID     = os.Getenv("S3_ACCESS_KEY_ID")
	secretAccessKey = os.Getenv("S3_SECRET_ACCESS_KEY")
)

func TestS3(t *testing.T) {
	if endpoint == "" || accessKeyID == "" || secretAccessKey == "" {
		t.Skip("S3 environment variables are not set, skipping S3 tests")
	}

	// create client
	client, err := NewS3(config.S3Config{
		Endpoint:        endpoint,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		Bucket:          "gorse-test",
		Prefix:          "blob",
	})
	assert.NoError(t, err)

	// create bucket if not exists
	err = client.Client.MakeBucket(context.Background(), client.bucket, minio.MakeBucketOptions{})
	assert.NoError(t, err)

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
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, "hello world", string(content))
}
