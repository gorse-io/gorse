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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madflojo/testcerts"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/progress"
	"github.com/zhenghaoz/gorse/common/util"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/dataset"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/cf"
	"github.com/zhenghaoz/gorse/model/ctr"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/server"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"github.com/zhenghaoz/gorse/storage/meta"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func newRankingDataset() (*dataset.Dataset, *dataset.Dataset) {
	return dataset.NewDataset(time.Now(), 0, 0), dataset.NewDataset(time.Now(), 0, 0)
}

func newClickDataset() (*ctr.Dataset, *ctr.Dataset) {
	dataset := &ctr.Dataset{
		Index: base.NewUnifiedMapIndexBuilder().Build(),
	}
	return dataset, dataset
}

type mockMasterRPC struct {
	Master
	addr       chan string
	grpcServer *grpc.Server
}

func newMockMasterRPC(t *testing.T) *mockMasterRPC {
	// create meta store
	metaStore, err := meta.Open(fmt.Sprintf("sqlite://%s/meta.db", t.TempDir()), time.Second)
	assert.NoError(t, err)
	err = metaStore.Init()
	assert.NoError(t, err)
	// create click model
	train, test := newClickDataset()
	fm := ctr.NewFM(model.Params{model.NEpochs: 0})
	fm.Fit(context.Background(), train, test, nil)
	// create ranking model
	trainSet, testSet := newRankingDataset()
	bpr := cf.NewBPR(model.Params{model.NEpochs: 0})
	bpr.Fit(context.Background(), trainSet, testSet, cf.NewFitConfig())
	return &mockMasterRPC{
		Master: Master{
			RestServer: server.RestServer{
				Settings: &config.Settings{
					Config:                        config.GetDefaultConfig(),
					CacheClient:                   cache.NoDatabase{},
					DataClient:                    data.NoDatabase{},
					CollaborativeFilteringModel:   bpr,
					ClickModel:                    fm,
					CollaborativeFilteringModelId: 123,
					ClickThroughRateModelId:       456,
				},
			},
			metaStore: metaStore,
		},
		addr: make(chan string),
	}
}

func (m *mockMasterRPC) Start(t *testing.T) {
	listen, err := net.Listen("tcp", ":0")
	assert.NoError(t, err)
	m.addr <- listen.Addr().String()
	var opts []grpc.ServerOption
	m.grpcServer = grpc.NewServer(opts...)
	protocol.RegisterMasterServer(m.grpcServer, m)
	err = m.grpcServer.Serve(listen)
	assert.NoError(t, err)
}

func (m *mockMasterRPC) StartTLS(t *testing.T, o *util.TLSConfig) {
	listen, err := net.Listen("tcp", ":0")
	assert.NoError(t, err)
	m.addr <- listen.Addr().String()
	creds, err := util.NewServerCreds(&util.TLSConfig{
		SSLCA:   o.SSLCA,
		SSLCert: o.SSLCert,
		SSLKey:  o.SSLKey,
	})
	assert.NoError(t, err)
	m.grpcServer = grpc.NewServer(grpc.Creds(creds))
	protocol.RegisterMasterServer(m.grpcServer, m)
	err = m.grpcServer.Serve(listen)
	assert.NoError(t, err)
}

func (m *mockMasterRPC) Stop() {
	_ = m.metaStore.Close()
	m.grpcServer.Stop()
}

func TestRPC(t *testing.T) {
	rpcServer := newMockMasterRPC(t)
	go rpcServer.Start(t)
	defer rpcServer.Stop()
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

	// test get meta
	_, err = client.GetMeta(ctx,
		&protocol.NodeInfo{NodeType: protocol.NodeType_Server, Uuid: "server1", Hostname: "yoga"})
	assert.NoError(t, err)
	metaResp, err := client.GetMeta(ctx,
		&protocol.NodeInfo{NodeType: protocol.NodeType_Worker, Uuid: "worker1", Hostname: "yoga"})
	assert.NoError(t, err)
	assert.Equal(t, int64(123), metaResp.CollaborativeFilteringModelId)
	assert.Equal(t, int64(456), metaResp.ClickThroughRateModelId)
	assert.Equal(t, "worker1", metaResp.Me)
	assert.Equal(t, []string{"server1"}, metaResp.Servers)
	assert.Equal(t, []string{"worker1"}, metaResp.Workers)
	var cfg config.Config
	err = json.Unmarshal([]byte(metaResp.Config), &cfg)
	assert.NoError(t, err)
	assert.Equal(t, rpcServer.Config, &cfg)

	time.Sleep(time.Second * 2)
	metaResp, err = client.GetMeta(ctx,
		&protocol.NodeInfo{NodeType: protocol.NodeType_Worker, Uuid: "worker2", Hostname: "yoga"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"worker2"}, metaResp.Workers)

	rpcServer.Stop()
}

func generateToTempFile(t *testing.T) (string, string, string) {
	// Generate Certificate Authority
	ca := testcerts.NewCA()
	// Create a signed Certificate and Key
	certs, err := ca.NewKeyPair()
	assert.NoError(t, err)
	// Write certificates to a file
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	certFile := filepath.Join(t.TempDir(), "cert.pem")
	keyFile := filepath.Join(t.TempDir(), "key.pem")
	pem := ca.PublicKey()
	err = os.WriteFile(caFile, pem, 0640)
	assert.NoError(t, err)
	err = certs.ToFile(certFile, keyFile)
	assert.NoError(t, err)
	return caFile, certFile, keyFile
}

func TestSSL(t *testing.T) {
	caFile, certFile, keyFile := generateToTempFile(t)
	o := &util.TLSConfig{
		SSLCA:   caFile,
		SSLCert: certFile,
		SSLKey:  keyFile,
	}
	rpcServer := newMockMasterRPC(t)
	go rpcServer.StartTLS(t, o)
	defer rpcServer.Stop()
	address := <-rpcServer.addr

	// success
	c, err := util.NewClientCreds(o)
	assert.NoError(t, err)
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(c))
	assert.NoError(t, err)
	client := protocol.NewMasterClient(conn)
	_, err = client.GetMeta(context.Background(), &protocol.NodeInfo{NodeType: protocol.NodeType_Server, Uuid: "server1", Hostname: "yoga"})
	assert.NoError(t, err)

	// insecure
	conn, err = grpc.Dial(address, grpc.WithInsecure())
	assert.NoError(t, err)
	client = protocol.NewMasterClient(conn)
	_, err = client.GetMeta(context.Background(), &protocol.NodeInfo{NodeType: protocol.NodeType_Server, Uuid: "server1", Hostname: "yoga"})
	assert.Error(t, err)

	// certificate mismatch
	caFile2, certFile2, keyFile2 := generateToTempFile(t)
	o2 := &util.TLSConfig{
		SSLCA:   caFile2,
		SSLCert: certFile2,
		SSLKey:  keyFile2,
	}
	c, err = util.NewClientCreds(o2)
	assert.NoError(t, err)
	conn, err = grpc.Dial(address, grpc.WithTransportCredentials(c))
	assert.NoError(t, err)
	client = protocol.NewMasterClient(conn)
	_, err = client.GetMeta(context.Background(), &protocol.NodeInfo{NodeType: protocol.NodeType_Server, Uuid: "server1", Hostname: "yoga"})
	assert.Error(t, err)
}
