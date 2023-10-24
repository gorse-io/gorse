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

package master

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/ReneKroon/ttlcache/v2"
	"github.com/stretchr/testify/assert"
	"github.com/Neura-Studios/gorse/base/progress"
	"github.com/Neura-Studios/gorse/config"
	"github.com/Neura-Studios/gorse/model"
	"github.com/Neura-Studios/gorse/model/click"
	"github.com/Neura-Studios/gorse/model/ranking"
	"github.com/Neura-Studios/gorse/protocol"
	"github.com/Neura-Studios/gorse/server"
	"github.com/Neura-Studios/gorse/storage/cache"
	"github.com/Neura-Studios/gorse/storage/data"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type mockMasterRPC struct {
	Master
	addr       chan string
	grpcServer *grpc.Server
}

func newMockMasterRPC(_ *testing.T) *mockMasterRPC {
	// create click model
	train, test := newClickDataset()
	fm := click.NewFM(click.FMClassification, model.Params{model.NEpochs: 0})
	fm.Fit(context.Background(), train, test, nil)
	// create ranking model
	trainSet, testSet := newRankingDataset()
	bpr := ranking.NewBPR(model.Params{model.NEpochs: 0})
	bpr.Fit(context.Background(), trainSet, testSet, nil)
	return &mockMasterRPC{
		Master: Master{
			nodesInfo:        make(map[string]*Node),
			rankingModelName: "bpr",
			RestServer: server.RestServer{
				Settings: &config.Settings{
					Config:              config.GetDefaultConfig(),
					CacheClient:         cache.NoDatabase{},
					DataClient:          data.NoDatabase{},
					RankingModel:        bpr,
					ClickModel:          fm,
					RankingModelVersion: 123,
					ClickModelVersion:   456,
				},
			},
		},
		addr: make(chan string),
	}
}

func (m *mockMasterRPC) Start(t *testing.T) {
	m.ttlCache = ttlcache.NewCache()
	m.ttlCache.SetExpirationCallback(m.nodeDown)
	m.ttlCache.SetNewItemCallback(m.nodeUp)
	err := m.ttlCache.SetTTL(time.Second)
	assert.NoError(t, err)

	listen, err := net.Listen("tcp", ":0")
	assert.NoError(t, err)
	m.addr <- listen.Addr().String()
	var opts []grpc.ServerOption
	m.grpcServer = grpc.NewServer(opts...)
	protocol.RegisterMasterServer(m.grpcServer, m)
	err = m.grpcServer.Serve(listen)
	assert.NoError(t, err)
}

func (m *mockMasterRPC) Stop() {
	m.grpcServer.Stop()
}

func TestRPC(t *testing.T) {
	rpcServer := newMockMasterRPC(t)
	go rpcServer.Start(t)
	address := <-rpcServer.addr
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	client := protocol.NewMasterClient(conn)
	ctx := context.Background()

	progressList := []progress.Progress{{
		Tracer:     "tracer",
		Name:       "a",
		Status:     progress.StatusRunning,
		Total:      12,
		Count:      6,
		StartTime:  time.Date(2018, time.January, 1, 0, 0, 0, 0, time.Local),
		FinishTime: time.Date(2018, time.January, 2, 0, 0, 0, 0, time.Local),
	}}
	_, err = client.PushProgress(ctx, protocol.EncodeProgress(progressList))
	assert.NoError(t, err)
	i, ok := rpcServer.remoteProgress.Load("tracer")
	assert.True(t, ok)
	remoteProgressList := i.([]progress.Progress)
	assert.Equal(t, progressList, remoteProgressList)

	// test get click model
	clickModelReceiver, err := client.GetClickModel(ctx, &protocol.VersionInfo{Version: 456})
	assert.NoError(t, err)
	clickModel, err := protocol.UnmarshalClickModel(clickModelReceiver)
	assert.NoError(t, err)
	assert.Equal(t, rpcServer.ClickModel, clickModel)

	// test get ranking model
	rankingModelReceiver, err := client.GetRankingModel(ctx, &protocol.VersionInfo{Version: 123})
	assert.NoError(t, err)
	rankingModel, err := protocol.UnmarshalRankingModel(rankingModelReceiver)
	assert.NoError(t, err)
	rpcServer.RankingModel.SetParams(rpcServer.RankingModel.GetParams())
	assert.Equal(t, rpcServer.RankingModel, rankingModel)

	// test get meta
	_, err = client.GetMeta(ctx,
		&protocol.NodeInfo{NodeType: protocol.NodeType_ServerNode, NodeName: "server1", HttpPort: 1234})
	assert.NoError(t, err)
	metaResp, err := client.GetMeta(ctx,
		&protocol.NodeInfo{NodeType: protocol.NodeType_WorkerNode, NodeName: "worker1", HttpPort: 1234})
	assert.NoError(t, err)
	assert.Equal(t, int64(123), metaResp.RankingModelVersion)
	assert.Equal(t, int64(456), metaResp.ClickModelVersion)
	assert.Equal(t, "worker1", metaResp.Me)
	assert.Equal(t, []string{"server1"}, metaResp.Servers)
	assert.Equal(t, []string{"worker1"}, metaResp.Workers)
	var cfg config.Config
	err = json.Unmarshal([]byte(metaResp.Config), &cfg)
	assert.NoError(t, err)
	assert.Equal(t, rpcServer.Config, &cfg)

	time.Sleep(time.Second * 2)
	metaResp, err = client.GetMeta(ctx,
		&protocol.NodeInfo{NodeType: protocol.NodeType_WorkerNode, NodeName: "worker2", HttpPort: 1234})
	assert.NoError(t, err)
	assert.Equal(t, []string{"worker2"}, metaResp.Workers)

	rpcServer.Stop()
}
