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
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/protocol"
	"google.golang.org/grpc"
	"net"
	"testing"
)

type mockMaster struct {
	protocol.UnimplementedMasterServer
	addr       chan string
	grpcServer *grpc.Server
	meta       *protocol.Meta
	cacheStore *miniredis.Miniredis
	dataStore  *miniredis.Miniredis
}

func newMockMaster(t *testing.T) *mockMaster {
	cacheStore, err := miniredis.Run()
	assert.NoError(t, err)
	dataStore, err := miniredis.Run()
	assert.NoError(t, err)
	cfg := (*config.Config)(nil).LoadDefaultIfNil()
	cfg.Database.DataStore = "redis://" + dataStore.Addr()
	cfg.Database.CacheStore = "redis://" + cacheStore.Addr()
	return &mockMaster{
		addr:       make(chan string),
		meta:       &protocol.Meta{Config: marshal(t, cfg)},
		cacheStore: cacheStore,
		dataStore:  dataStore,
	}
}

func (m *mockMaster) GetMeta(_ context.Context, _ *protocol.NodeInfo) (*protocol.Meta, error) {
	return m.meta, nil
}

func (m *mockMaster) GetRankingModel(_ *protocol.VersionInfo, _ protocol.Master_GetRankingModelServer) error {
	panic("not implement")
}

func (m *mockMaster) GetClickModel(_ *protocol.VersionInfo, _ protocol.Master_GetClickModelServer) error {
	panic("not implement")
}

func (m *mockMaster) GetUserIndex(_ *protocol.VersionInfo, _ protocol.Master_GetUserIndexServer) error {
	panic("not implement")
}

func (m *mockMaster) Start(t *testing.T) {
	listen, err := net.Listen("tcp", ":0")
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
	m.dataStore.Close()
	m.cacheStore.Close()
}

func TestServer_Sync(t *testing.T) {
	master := newMockMaster(t)
	go master.Start(t)
	address := <-master.addr
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	assert.NoError(t, err)
	serv := &Server{
		testMode:     true,
		masterClient: protocol.NewMasterClient(conn),
		RestServer: RestServer{
			GorseConfig: (*config.Config)(nil).LoadDefaultIfNil(),
		},
	}
	serv.Sync()
	assert.Equal(t, "redis://"+master.dataStore.Addr(), serv.dataPath)
	assert.Equal(t, "redis://"+master.cacheStore.Addr(), serv.cachePath)
	master.Stop()
}
