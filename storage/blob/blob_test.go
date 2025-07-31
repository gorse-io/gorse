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
	"net"
	"path"
	"testing"

	"github.com/gorse-io/gorse/protocol"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
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
}
