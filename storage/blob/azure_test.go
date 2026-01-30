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
	"errors"
	"io"
	"os"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/gorse-io/gorse/config"
	"github.com/stretchr/testify/assert"
)

func TestAzureBlobEmulator(t *testing.T) {
	connectionString := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
	if connectionString == "" {
		t.Skip("AZURE_STORAGE_CONNECTION_STRING is not set, skipping Azure Blob emulator test")
	}

	client, err := NewAzureBlob(config.AzureBlobConfig{ConnectionString: connectionString}, "gorse-test", "blob")
	assert.NoError(t, err)

	ctx := context.Background()
	_, err = client.client.CreateContainer(ctx, client.container, nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if !errors.As(err, &respErr) || respErr.ErrorCode != string(bloberror.ContainerAlreadyExists) {
			assert.NoError(t, err)
		}
	}

	w, done, err := client.Create("test.txt")
	assert.NoError(t, err)
	_, err = w.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.NoError(t, w.Close())
	<-done

	r, err := client.Open("test.txt")
	assert.NoError(t, err)
	data, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, "hello", string(data))
	assert.NoError(t, r.Close())

	names, err := client.List()
	assert.NoError(t, err)
	assert.Contains(t, names, "test.txt")

	err = client.Remove("test.txt")
	assert.NoError(t, err)
	names, err = client.List()
	assert.NoError(t, err)
	assert.NotContains(t, names, "test.txt")
}
