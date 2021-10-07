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
	"github.com/ReneKroon/ttlcache/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/server"
	"google.golang.org/grpc"
	"net"
	"testing"
	"time"
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
	fm.Fit(train, test, nil)
	// create ranking model
	trainSet, testSet := newRankingDataset()
	bpr := ranking.NewBPR(model.Params{model.NEpochs: 0})
	bpr.Fit(trainSet, testSet, nil)
	// create user index
	userIndex := base.NewMapIndex()
	return &mockMasterRPC{
		Master: Master{
			taskMonitor:         NewTaskMonitor(),
			nodesInfo:           make(map[string]*Node),
			rankingModelName:    "bpr",
			rankingModelVersion: 123,
			rankingModel:        bpr,
			clickModelVersion:   456,
			clickModel:          fm,
			userIndexVersion:    789,
			userIndex:           userIndex,
			RestServer: server.RestServer{
				GorseConfig: (*config.Config)(nil).LoadDefaultIfNil(),
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
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	assert.NoError(t, err)
	client := protocol.NewMasterClient(conn)
	ctx := context.Background()

	_, err = client.StartTask(ctx, &protocol.StartTaskRequest{Name: "a", Total: 12})
	assert.NoError(t, err)
	assert.Equal(t, 12, rpcServer.taskMonitor.Tasks["a"].Total)
	assert.Equal(t, 0, rpcServer.taskMonitor.Tasks["a"].Done)
	assert.Equal(t, TaskStatusRunning, rpcServer.taskMonitor.Tasks["a"].Status)

	_, err = client.UpdateTask(ctx, &protocol.UpdateTaskRequest{Name: "a", Done: 10})
	assert.NoError(t, err)
	assert.Equal(t, 12, rpcServer.taskMonitor.Tasks["a"].Total)
	assert.Equal(t, 10, rpcServer.taskMonitor.Tasks["a"].Done)
	assert.Equal(t, TaskStatusRunning, rpcServer.taskMonitor.Tasks["a"].Status)

	_, err = client.FinishTask(ctx, &protocol.FinishTaskRequest{Name: "a"})
	assert.NoError(t, err)
	assert.Equal(t, 12, rpcServer.taskMonitor.Tasks["a"].Total)
	assert.Equal(t, 12, rpcServer.taskMonitor.Tasks["a"].Done)
	assert.Equal(t, TaskStatusComplete, rpcServer.taskMonitor.Tasks["a"].Status)

	// test get click model
	clickModelReceiver, err := client.GetClickModel(ctx, &protocol.VersionInfo{Version: 456})
	assert.NoError(t, err)
	clickModel, err := protocol.UnmarshalClickModel(clickModelReceiver)
	assert.NoError(t, err)
	assert.Equal(t, rpcServer.clickModel, clickModel)

	// test get ranking model
	rankingModelReceiver, err := client.GetRankingModel(ctx, &protocol.VersionInfo{Version: 123})
	assert.NoError(t, err)
	rankingModel, err := protocol.UnmarshalRankingModel(rankingModelReceiver)
	assert.NoError(t, err)
	rpcServer.rankingModel.SetParams(rpcServer.rankingModel.GetParams())
	assert.Equal(t, rpcServer.rankingModel, rankingModel)

	// test get user index
	userIndexReceiver, err := client.GetUserIndex(ctx, &protocol.VersionInfo{Version: 789})
	assert.NoError(t, err)
	userIndex, err := protocol.UnmarshalIndex(userIndexReceiver)
	assert.NoError(t, err)
	assert.Equal(t, rpcServer.userIndex, userIndex)

	// test get meta
	_, err = client.GetMeta(ctx,
		&protocol.NodeInfo{NodeType: protocol.NodeType_ServerNode, NodeName: "server1", HttpPort: 1234})
	assert.NoError(t, err)
	metaResp, err := client.GetMeta(ctx,
		&protocol.NodeInfo{NodeType: protocol.NodeType_WorkerNode, NodeName: "worker1", HttpPort: 1234})
	assert.NoError(t, err)
	assert.Equal(t, int64(123), metaResp.RankingModelVersion)
	assert.Equal(t, int64(456), metaResp.ClickModelVersion)
	assert.Equal(t, int64(789), metaResp.UserIndexVersion)
	assert.Equal(t, "worker1", metaResp.Me)
	assert.Equal(t, []string{"server1"}, metaResp.Servers)
	assert.Equal(t, []string{"worker1"}, metaResp.Workers)
	var cfg config.Config
	err = json.Unmarshal([]byte(metaResp.Config), &cfg)
	assert.NoError(t, err)
	assert.Equal(t, *rpcServer.GorseConfig, cfg)

	time.Sleep(time.Second * 2)
	metaResp, err = client.GetMeta(ctx,
		&protocol.NodeInfo{NodeType: protocol.NodeType_WorkerNode, NodeName: "worker2", HttpPort: 1234})
	assert.NoError(t, err)
	assert.Equal(t, []string{"worker2"}, metaResp.Workers)

	rpcServer.Stop()
}
