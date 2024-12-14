// Copyright 2024 gorse Project Authors
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
	"github.com/zhenghaoz/gorse/protocol"
	"google.golang.org/grpc"
	"net"
	"os"
	"path"
	"testing"
)

func TestBlob(t *testing.T) {
	// start server
	lis, err := net.Listen("tcp", "localhost:0")
	assert.NoError(t, err)
	grpcServer := grpc.NewServer()
	protocol.RegisterBlobStoreServer(grpcServer, NewMasterStoreServer(path.Join(t.TempDir(), "blob")))
	go func() {
		err = grpcServer.Serve(lis)
		assert.NoError(t, err)
	}()
	defer grpcServer.Stop()

	// create client
	clientConn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	assert.NoError(t, err)
	client := NewMasterStoreClient(clientConn)

	// create a temp file
	tempFilePath := path.Join(t.TempDir(), "test.txt")
	err = os.WriteFile(tempFilePath, []byte("hello world"), 0644)
	assert.NoError(t, err)
	info, err := os.Stat(tempFilePath)
	assert.NoError(t, err)

	// upload blob
	err = client.UploadBlob("test", tempFilePath)
	assert.NoError(t, err)

	// fetch blob
	modTime, err := client.FetchBlob("test")
	assert.NoError(t, err)
	assert.Equal(t, info.ModTime().UTC(), modTime)

	// download blob
	downloadFilePath := path.Join(t.TempDir(), "download.txt")
	err = client.DownloadBlob("test", downloadFilePath)
	assert.NoError(t, err)
	downloadContent, err := os.ReadFile(downloadFilePath)
	assert.NoError(t, err)
	assert.Equal(t, "hello world", string(downloadContent))
}
