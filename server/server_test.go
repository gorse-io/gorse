// Copyright 2021 gorse Project Authors
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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"

	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/protocol"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type mockMaster struct {
	protocol.UnimplementedMasterServer
	addr          chan string
	grpcServer    *grpc.Server
	meta          *protocol.Meta
	cacheTempFile string
	dataTempFile  string
}

func newMockMaster(t *testing.T) *mockMaster {
	cfg := config.GetDefaultConfig()
	cfg.Database.DataStore = fmt.Sprintf("sqlite://%s/data.db", t.TempDir())
	cfg.Database.CacheStore = fmt.Sprintf("sqlite://%s/cache.db", t.TempDir())
	bytes, err := json.Marshal(cfg)
	assert.NoError(t, err)
	return &mockMaster{
		addr:          make(chan string),
		meta:          &protocol.Meta{Config: string(bytes)},
		dataTempFile:  cfg.Database.DataStore,
		cacheTempFile: cfg.Database.CacheStore,
	}
}

func (m *mockMaster) GetMeta(_ context.Context, _ *protocol.NodeInfo) (*protocol.Meta, error) {
	return m.meta, nil
}

func (m *mockMaster) Start(t *testing.T) {
	listen, err := net.Listen("tcp", "localhost:0")
	assert.NoError(t, err)
	m.addr <- listen.Addr().String()
	var opts []grpc.ServerOption
	m.grpcServer = grpc.NewServer(opts...)
	protocol.RegisterMasterServer(m.grpcServer, m)
	err = m.grpcServer.Serve(listen)
	assert.NoError(t, err)
}

func (m *mockMaster) Stop() {
	m.grpcServer.Stop()
}

func TestServer_Sync(t *testing.T) {
	master := newMockMaster(t)
	go master.Start(t)
	address := <-master.addr
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	serv := &Server{
		testMode:     true,
		masterClient: protocol.NewMasterClient(conn),
		RestServer: RestServer{
			Settings: config.NewSettings(),
		},
	}
	serv.Sync()
	assert.Equal(t, master.dataTempFile, serv.dataPath)
	assert.Equal(t, master.cacheTempFile, serv.cachePath)
	assert.NoError(t, serv.DataClient.Close())
	assert.NoError(t, serv.CacheClient.Close())
	master.Stop()
}
