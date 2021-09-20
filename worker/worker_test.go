// Copyright 2020 gorse Project Authors
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
package worker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"google.golang.org/grpc"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestSplit(t *testing.T) {
	// create user index
	userIndex := base.NewMapIndex()
	userIndex.Add("1")
	userIndex.Add("2")
	userIndex.Add("3")
	userIndex.Add("4")
	userIndex.Add("5")
	userIndex.Add("6")
	userIndex.Add("7")
	userIndex.Add("8")
	// create nodes
	nodes := []string{"a", "b", "c"}

	users, err := split(userIndex, nodes, "b")
	assert.NoError(t, err)
	assert.Equal(t, []string{"2", "5", "8"}, users)

	users, err = split(userIndex, nodes, "d")
	assert.Error(t, err)
}

func TestCheckRecommendCacheTimeout(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	// insert cache
	err := w.cacheClient.SetScores(cache.CTRRecommend, "0", []cache.Scored{{"0", 0}})
	assert.NoError(t, err)
	assert.True(t, w.checkRecommendCacheTimeout("0"))
	err = w.cacheClient.SetTime(cache.LastModifyUserTime, "0", time.Now().Add(-time.Hour))
	assert.True(t, w.checkRecommendCacheTimeout("0"))
	err = w.cacheClient.SetTime(cache.LastUpdateUserRecommendTime, "0", time.Now().Add(-time.Hour*100))
	assert.True(t, w.checkRecommendCacheTimeout("0"))
	err = w.cacheClient.SetTime(cache.LastUpdateUserRecommendTime, "0", time.Now().Add(time.Hour*100))
	assert.False(t, w.checkRecommendCacheTimeout("0"))
	err = w.cacheClient.ClearList(cache.CTRRecommend, "0")
	assert.True(t, w.checkRecommendCacheTimeout("0"))
}

type mockMatrixFactorizationForRecommend struct {
	ranking.BaseMatrixFactorization
}

func (m *mockMatrixFactorizationForRecommend) Invalid() bool {
	panic("implement me")
}

func (m *mockMatrixFactorizationForRecommend) Fit(_, _ *ranking.DataSet, _ *ranking.FitConfig) ranking.Score {
	panic("implement me")
}

func (m *mockMatrixFactorizationForRecommend) Predict(_, _ string) float32 {
	panic("implement me")
}

func newMockMatrixFactorizationForRecommend(numUsers, numItems int) *mockMatrixFactorizationForRecommend {
	m := new(mockMatrixFactorizationForRecommend)
	m.UserIndex = base.NewMapIndex()
	m.ItemIndex = base.NewMapIndex()
	for i := 0; i < numUsers; i++ {
		m.UserIndex.Add(strconv.Itoa(i))
	}
	for i := 0; i < numItems; i++ {
		m.ItemIndex.Add(strconv.Itoa(i))
	}
	return m
}

func (m *mockMatrixFactorizationForRecommend) InternalPredict(_, itemId int32) float32 {
	return float32(itemId)
}

func (m *mockMatrixFactorizationForRecommend) Clear() {
	// do nothing
}

func (m *mockMatrixFactorizationForRecommend) GetParamsGrid() model.ParamsGrid {
	panic("don't call me")
}

type mockWorker struct {
	dataStoreServer  *miniredis.Miniredis
	cacheStoreServer *miniredis.Miniredis
	Worker
}

func newMockWorker(t *testing.T) *mockWorker {
	w := new(mockWorker)
	// create mock redis server
	var err error
	w.dataStoreServer, err = miniredis.Run()
	assert.NoError(t, err)
	w.cacheStoreServer, err = miniredis.Run()
	assert.NoError(t, err)
	// open database
	w.dataClient, err = data.Open("redis://" + w.dataStoreServer.Addr())
	assert.NoError(t, err)
	w.cacheClient, err = cache.Open("redis://" + w.cacheStoreServer.Addr())
	assert.NoError(t, err)
	// configuration
	w.cfg = (*config.Config)(nil).LoadDefaultIfNil()
	w.jobs = 1
	return w
}

func (w *mockWorker) Close(t *testing.T) {
	err := w.dataClient.Close()
	assert.NoError(t, err)
	err = w.cacheClient.Close()
	assert.NoError(t, err)
	w.dataStoreServer.Close()
	w.cacheStoreServer.Close()
}

func TestRecommendMatrixFactorization(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.cfg.Recommend.EnableColRecommend = true
	// insert feedbacks
	now := time.Now()
	err := w.dataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "9"}, Timestamp: now.Add(-time.Hour)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "8"}, Timestamp: now.Add(time.Hour)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "7"}, Timestamp: now.Add(-time.Hour)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "6"}, Timestamp: now.Add(time.Hour)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "5"}, Timestamp: now.Add(-time.Hour)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "4"}, Timestamp: now.Add(time.Hour)},
	}, true, true)
	assert.NoError(t, err)
	// create mock model
	m := newMockMatrixFactorizationForRecommend(1, 10)
	w.Recommend(m, []string{"0"})

	recommends, err := w.cacheClient.GetScores(cache.CTRRecommend, "0", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{
		{"3", 0},
		{"2", 0},
		{"1", 0},
		{"0", 0},
	}, recommends)

	read, err := w.cacheClient.GetList(cache.IgnoreItems, "0")
	assert.NoError(t, err)
	assert.Equal(t, []string{"4", "6", "8"}, read)
}

func TestRecommend_ItemBased(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.cfg.Recommend.EnableColRecommend = false
	w.cfg.Recommend.EnableItemBasedRecommend = true
	// insert feedback
	err := w.dataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "21"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "22"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "23"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "24"}},
	}, true, true)
	assert.NoError(t, err)
	// insert similar items
	err = w.cacheClient.SetScores(cache.ItemNeighbors, "21", []cache.Scored{
		{"22", 100000},
		{"29", 1},
	})
	assert.NoError(t, err)
	err = w.cacheClient.SetScores(cache.ItemNeighbors, "22", []cache.Scored{
		{"23", 100000},
		{"28", 1},
		{"29", 1},
	})
	assert.NoError(t, err)
	err = w.cacheClient.SetScores(cache.ItemNeighbors, "23", []cache.Scored{
		{"24", 100000},
		{"27", 1},
		{"28", 1},
		{"29", 1},
	})
	assert.NoError(t, err)
	err = w.cacheClient.SetScores(cache.ItemNeighbors, "24", []cache.Scored{
		{"21", 100000},
		{"26", 1},
		{"27", 1},
		{"28", 1},
		{"29", 1},
	})
	assert.NoError(t, err)
	m := newMockMatrixFactorizationForRecommend(1, 10)
	w.Recommend(m, []string{"0"})
	recommends, err := w.cacheClient.GetScores(cache.CTRRecommend, "0", 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"29", 0}, {"28", 0}, {"27", 0}}, recommends)
}

func TestRecommend_UserBased(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.cfg.Recommend.EnableColRecommend = false
	w.cfg.Recommend.EnableUserBasedRecommend = true
	// insert similar users
	err := w.cacheClient.SetScores(cache.UserNeighbors, "0", []cache.Scored{
		{"1", 2},
		{"2", 1.5},
		{"3", 1},
	})
	assert.NoError(t, err)
	// insert feedback
	err = w.dataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "1", ItemId: "11"}},
	}, true, true)
	assert.NoError(t, err)
	err = w.dataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "12"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "48"}},
	}, true, true)
	assert.NoError(t, err)
	err = w.dataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "13"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "48"}},
	}, true, true)
	assert.NoError(t, err)
	m := newMockMatrixFactorizationForRecommend(1, 10)
	w.Recommend(m, []string{"0"})
	recommends, err := w.cacheClient.GetScores(cache.CTRRecommend, "0", 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"48", 0}, {"11", 0}, {"12", 0}}, recommends)
}

func TestRecommend_Popular(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.cfg.Recommend.EnableColRecommend = false
	w.cfg.Recommend.EnablePopularRecommend = true
	// insert popular items
	err := w.cacheClient.SetScores(cache.PopularItems, "", []cache.Scored{{"10", 10}, {"9", 9}, {"8", 8}})
	assert.NoError(t, err)
	m := newMockMatrixFactorizationForRecommend(1, 10)
	w.Recommend(m, []string{"0"})
	recommends, err := w.cacheClient.GetScores(cache.CTRRecommend, "0", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"10", 0}, {"9", 0}, {"8", 0}}, recommends)
}

func TestRecommend_Latest(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.cfg.Recommend.EnableColRecommend = false
	w.cfg.Recommend.EnableLatestRecommend = true
	// insert popular items
	err := w.cacheClient.SetScores(cache.LatestItems, "", []cache.Scored{{"10", 10}, {"9", 9}, {"8", 8}})
	assert.NoError(t, err)
	m := newMockMatrixFactorizationForRecommend(1, 10)
	w.Recommend(m, []string{"0"})
	recommends, err := w.cacheClient.GetScores(cache.CTRRecommend, "0", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"10", 0}, {"9", 0}, {"8", 0}}, recommends)
}

func marshal(t *testing.T, v interface{}) string {
	s, err := json.Marshal(v)
	assert.NoError(t, err)
	return string(s)
}

func newRankingDataset() (*ranking.DataSet, *ranking.DataSet) {
	dataset := &ranking.DataSet{
		UserIndex: base.NewMapIndex(),
		ItemIndex: base.NewMapIndex(),
	}
	return dataset, dataset
}

func newClickDataset() (*click.Dataset, *click.Dataset) {
	dataset := &click.Dataset{
		Index: click.NewUnifiedMapIndexBuilder().Build(),
	}
	return dataset, dataset
}

type mockMaster struct {
	protocol.UnimplementedMasterServer
	addr         chan string
	grpcServer   *grpc.Server
	cacheStore   *miniredis.Miniredis
	dataStore    *miniredis.Miniredis
	meta         *protocol.Meta
	rankingModel *protocol.Model
	clickModel   *protocol.Model
	userIndex    *protocol.UserIndex
}

func newMockMaster(t *testing.T) *mockMaster {
	cacheStore, err := miniredis.Run()
	assert.NoError(t, err)
	dataStore, err := miniredis.Run()
	assert.NoError(t, err)
	cfg := (*config.Config)(nil).LoadDefaultIfNil()
	cfg.Database.DataStore = "redis://" + dataStore.Addr()
	cfg.Database.CacheStore = "redis://" + cacheStore.Addr()

	// create click model
	train, test := newClickDataset()
	fm := click.NewFM(click.FMClassification, model.Params{model.NEpochs: 0})
	fm.Fit(train, test, nil)
	clickModelPB := &protocol.Model{}
	clickModelPB.Model, err = click.EncodeModel(fm)
	assert.NoError(t, err)
	clickModelPB.Version = 1

	// create ranking model
	trainSet, testSet := newRankingDataset()
	bpr := ranking.NewBPR(model.Params{model.NEpochs: 0})
	bpr.Fit(trainSet, testSet, nil)
	rankingModelPB := &protocol.Model{}
	rankingModelPB.Model, err = ranking.EncodeModel(bpr)
	assert.NoError(t, err)
	rankingModelPB.Name = "bpr"
	rankingModelPB.Version = 2

	// create user index
	buf := bytes.NewBuffer(nil)
	writer := bufio.NewWriter(buf)
	encoder := gob.NewEncoder(writer)
	err = encoder.Encode(base.NewMapIndex())
	assert.NoError(t, err)
	err = writer.Flush()
	assert.NoError(t, err)
	userIndexPB := &protocol.UserIndex{}
	userIndexPB.Version = 3
	userIndexPB.UserIndex = buf.Bytes()

	return &mockMaster{
		addr: make(chan string),
		meta: &protocol.Meta{
			Config:              marshal(t, cfg),
			ClickModelVersion:   1,
			RankingModelVersion: 2,
			UserIndexVersion:    3,
		},
		cacheStore:   cacheStore,
		dataStore:    dataStore,
		userIndex:    userIndexPB,
		clickModel:   clickModelPB,
		rankingModel: rankingModelPB,
	}
}

func (m *mockMaster) GetMeta(_ context.Context, _ *protocol.NodeInfo) (*protocol.Meta, error) {
	return m.meta, nil
}

func (m *mockMaster) GetRankingModel(context.Context, *protocol.NodeInfo) (*protocol.Model, error) {
	return m.rankingModel, nil
}

func (m *mockMaster) GetClickModel(context.Context, *protocol.NodeInfo) (*protocol.Model, error) {
	return m.clickModel, nil
}

func (m *mockMaster) GetUserIndex(context.Context, *protocol.NodeInfo) (*protocol.UserIndex, error) {
	return m.userIndex, nil
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

func TestWorker_Sync(t *testing.T) {
	master := newMockMaster(t)
	go master.Start(t)
	address := <-master.addr
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	assert.NoError(t, err)
	serv := &Worker{
		testMode:     true,
		masterClient: protocol.NewMasterClient(conn),
		cfg:          (*config.Config)(nil).LoadDefaultIfNil(),
		syncedChan:   make(chan bool, 1024),
	}
	serv.Sync()
	assert.Equal(t, "redis://"+master.dataStore.Addr(), serv.dataPath)
	assert.Equal(t, "redis://"+master.cacheStore.Addr(), serv.cachePath)
	assert.Equal(t, int64(1), serv.latestClickModelVersion)
	assert.Equal(t, int64(2), serv.latestRankingModelVersion)
	assert.Equal(t, int64(3), serv.latestUserIndexVersion)
	assert.Zero(t, serv.currentClickModelVersion)
	assert.Zero(t, serv.currentRankingModelVersion)
	assert.Zero(t, serv.currentUserIndexVersion)
	serv.Pull()
	assert.Equal(t, int64(1), serv.currentClickModelVersion)
	assert.Equal(t, int64(2), serv.currentRankingModelVersion)
	assert.Equal(t, int64(3), serv.currentUserIndexVersion)
	master.Stop()
}
