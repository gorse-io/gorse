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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/c-bata/goptuna"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/logics"
	"github.com/gorse-io/gorse/model"
	"github.com/gorse-io/gorse/model/cf"
	"github.com/gorse-io/gorse/model/ctr"
	"github.com/gorse-io/gorse/protocol"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

type WorkerTestSuite struct {
	suite.Suite
	Worker
}

func (suite *WorkerTestSuite) SetupSuite() {
	// open database
	var err error
	suite.tracer = monitor.NewTracer("test")
	suite.Settings = config.NewSettings()
	suite.DataClient, err = data.Open(fmt.Sprintf("sqlite://%s/data.db", suite.T().TempDir()), "")
	suite.NoError(err)
	suite.CacheClient, err = cache.Open(fmt.Sprintf("sqlite://%s/cache.db", suite.T().TempDir()), "")
	suite.NoError(err)
	// init database
	err = suite.DataClient.Init()
	suite.NoError(err)
	err = suite.CacheClient.Init()
	suite.NoError(err)
}

func (suite *WorkerTestSuite) TearDownSuite() {
	err := suite.DataClient.Close()
	suite.NoError(err)
	err = suite.CacheClient.Close()
	suite.NoError(err)
}

func (suite *WorkerTestSuite) SetupTest() {
	err := suite.DataClient.Purge()
	suite.NoError(err)
	err = suite.CacheClient.Purge()
	suite.NoError(err)
	// configuration
	suite.Config = config.GetDefaultConfig()
	suite.jobs = 1
	// reset random generator
	suite.randGenerator = rand.New(rand.NewSource(0))
	// reset index
	suite.matrixFactorizationItems = nil
}

func (suite *WorkerTestSuite) TestPullUsers() {
	ctx := context.Background()
	// create user index
	err := suite.DataClient.BatchInsertUsers(ctx, []data.User{
		{UserId: "1"},
		{UserId: "2"},
		{UserId: "3"},
		{UserId: "4"},
		{UserId: "5"},
		{UserId: "6"},
		{UserId: "7"},
		{UserId: "8"},
	})
	suite.NoError(err)
	// create nodes
	nodes := []string{"a", "b", "c"}

	users, err := suite.pullUsers(nodes, "b")
	suite.NoError(err)
	suite.Equal([]data.User{{UserId: "1"}, {UserId: "3"}, {UserId: "6"}}, users)

	_, err = suite.pullUsers(nodes, "d")
	suite.Error(err)
}

func (suite *WorkerTestSuite) TestCheckRecommendCacheTimeout() {
	ctx := context.Background()

	// empty cache
	suite.True(suite.checkRecommendCacheOutOfDate(ctx, "0"))
	err := suite.CacheClient.AddScores(ctx, cache.OfflineRecommend, "0", []cache.Score{{Id: "0", Score: 0, Categories: []string{""}}})
	suite.NoError(err)

	// digest mismatch
	suite.True(suite.checkRecommendCacheOutOfDate(ctx, "0"))
	err = suite.CacheClient.Set(ctx, cache.String(cache.Key(cache.OfflineRecommendDigest, "0"), suite.Config.OfflineRecommendDigest()))
	suite.NoError(err)

	err = suite.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, "0"), time.Now().Add(-time.Hour)))
	suite.NoError(err)
	suite.True(suite.checkRecommendCacheOutOfDate(ctx, "0"))
	err = suite.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastUpdateUserRecommendTime, "0"), time.Now().Add(-time.Hour*100)))
	suite.NoError(err)
	suite.True(suite.checkRecommendCacheOutOfDate(ctx, "0"))
	err = suite.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastUpdateUserRecommendTime, "0"), time.Now().Add(time.Hour*100)))
	suite.NoError(err)
	suite.False(suite.checkRecommendCacheOutOfDate(ctx, "0"))
	err = suite.CacheClient.DeleteScores(ctx, []string{cache.OfflineRecommend}, cache.ScoreCondition{Subset: proto.String("0")})
	suite.NoError(err)
	suite.True(suite.checkRecommendCacheOutOfDate(ctx, "0"))
}

func (suite *WorkerTestSuite) TestRecommendMatrixFactorization() {
	ctx := context.Background()
	suite.Config.Recommend.Offline.EnableColRecommend = true
	suite.Config.Recommend.Offline.EnableClickThroughPrediction = true
	// insert feedbacks
	now := time.Now()
	err := suite.DataClient.BatchInsertFeedback(ctx, []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "9"}, Timestamp: now.Add(-time.Hour)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "8"}, Timestamp: now.Add(-time.Hour)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "7"}, Timestamp: now.Add(-time.Hour)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "6"}, Timestamp: now.Add(-time.Hour)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "5"}, Timestamp: now.Add(-time.Hour)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "4"}, Timestamp: now.Add(-time.Hour)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "3"}, Timestamp: now.Add(time.Hour)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "2"}, Timestamp: now.Add(time.Hour)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "1"}, Timestamp: now.Add(time.Hour)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "0"}, Timestamp: now.Add(time.Hour)},
	}, true, true, true)
	suite.NoError(err)

	// insert hidden items and categorized items
	err = suite.DataClient.BatchInsertItems(ctx, []data.Item{
		{ItemId: "10", IsHidden: true},
		{ItemId: "11", IsHidden: true},
		{ItemId: "3", Categories: []string{"*"}},
		{ItemId: "1", Categories: []string{"*"}},
	})
	suite.NoError(err)

	// create mock model
	suite.matrixFactorizationItems = logics.NewMatrixFactorizationItems(time.Time{})
	for i := 0; i < 10; i++ {
		suite.matrixFactorizationItems.Add(strconv.Itoa(i), []float32{float32(i)})
	}
	suite.matrixFactorizationUsers = logics.NewMatrixFactorizationUsers()
	suite.matrixFactorizationUsers.Add("0", []float32{1})
	suite.rankers = []ctr.FactorizationMachines{&mockFactorizationMachine{}}
	suite.Recommend([]data.User{{UserId: "0"}})

	// read recommend time
	recommendTime, err := suite.CacheClient.Get(ctx, cache.Key(cache.LastUpdateUserRecommendTime, "0")).Time()
	suite.NoError(err)

	recommends, err := suite.CacheClient.SearchScores(ctx, cache.OfflineRecommend, "0", []string{""}, 0, -1)
	suite.NoError(err)
	suite.Equal([]cache.Score{
		{Id: "3", Score: 3, Categories: []string{"", "*"}, Timestamp: recommendTime},
		{Id: "2", Score: 2, Categories: []string{""}, Timestamp: recommendTime},
		{Id: "1", Score: 1, Categories: []string{"", "*"}, Timestamp: recommendTime},
		{Id: "0", Score: 0, Categories: []string{""}, Timestamp: recommendTime},
	}, recommends)
}

func (suite *WorkerTestSuite) TestRecommendItemBased() {
	ctx := context.Background()
	suite.Config.Recommend.Offline.EnableColRecommend = false
	suite.Config.Recommend.Offline.EnableItemBasedRecommend = true
	suite.Config.Recommend.Offline.EnableClickThroughPrediction = true
	suite.Config.Recommend.ItemToItem = []config.ItemToItemConfig{{Name: "default"}}
	// insert feedback
	err := suite.DataClient.BatchInsertFeedback(ctx, []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "21"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "22"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "23"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "24"}},
	}, true, true, true)
	suite.NoError(err)

	// insert similar items
	err = suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "21"), []cache.Score{
		{Id: "22", Score: 100000, Categories: []string{"*"}},
		{Id: "25", Score: 1000000},
		{Id: "29", Score: 1},
	})
	suite.NoError(err)
	err = suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "22"), []cache.Score{
		{Id: "23", Score: 100000, Categories: []string{"*"}},
		{Id: "25", Score: 1000000},
		{Id: "28", Score: 1, Categories: []string{"*"}},
		{Id: "29", Score: 1},
	})
	suite.NoError(err)
	err = suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "23"), []cache.Score{
		{Id: "24", Score: 100000, Categories: []string{"*"}},
		{Id: "25", Score: 1000000},
		{Id: "27", Score: 1},
		{Id: "28", Score: 1, Categories: []string{"*"}},
		{Id: "29", Score: 1},
	})
	suite.NoError(err)
	err = suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "24"), []cache.Score{
		{Id: "21", Score: 100000},
		{Id: "25", Score: 1000000},
		{Id: "26", Score: 1, Categories: []string{"*"}},
		{Id: "27", Score: 1},
		{Id: "28", Score: 1, Categories: []string{"*"}},
		{Id: "29", Score: 1},
	})
	suite.NoError(err)

	// insert items
	err = suite.DataClient.BatchInsertItems(ctx, []data.Item{{ItemId: "21"}, {ItemId: "22"}, {ItemId: "23"}, {ItemId: "24"},
		{ItemId: "25"}, {ItemId: "26"}, {ItemId: "27"}, {ItemId: "28"}, {ItemId: "29"}})
	suite.NoError(err)
	// insert hidden items
	err = suite.DataClient.BatchInsertItems(ctx, []data.Item{{ItemId: "25", IsHidden: true}})
	suite.NoError(err)
	// insert categorized items
	err = suite.DataClient.BatchInsertItems(ctx, []data.Item{{ItemId: "26", Categories: []string{"*"}}, {ItemId: "28", Categories: []string{"*"}}})
	suite.NoError(err)
	suite.rankers = []ctr.FactorizationMachines{&mockFactorizationMachine{}}
	suite.Recommend([]data.User{{UserId: "0"}})
	// read recommend time
	recommendTime, err := suite.CacheClient.Get(ctx, cache.Key(cache.LastUpdateUserRecommendTime, "0")).Time()
	suite.NoError(err)
	// read recommend result
	recommends, err := suite.CacheClient.SearchScores(ctx, cache.OfflineRecommend, "0", []string{""}, 0, 3)
	suite.NoError(err)
	suite.Equal([]cache.Score{
		{Id: "29", Score: 29, Categories: []string{""}, Timestamp: recommendTime},
		{Id: "28", Score: 28, Categories: []string{"", "*"}, Timestamp: recommendTime},
		{Id: "27", Score: 27, Categories: []string{""}, Timestamp: recommendTime},
	}, recommends)
	recommends, err = suite.CacheClient.SearchScores(ctx, cache.OfflineRecommend, "0", []string{"*"}, 0, 3)
	suite.NoError(err)
	suite.Equal([]cache.Score{
		{Id: "28", Score: 28, Categories: []string{"", "*"}, Timestamp: recommendTime},
		{Id: "26", Score: 26, Categories: []string{"", "*"}, Timestamp: recommendTime},
	}, recommends)
}

func (suite *WorkerTestSuite) TestRecommendUserBased() {
	ctx := context.Background()
	suite.Config.Recommend.Offline.EnableColRecommend = false
	suite.Config.Recommend.Offline.EnableUserBasedRecommend = true
	suite.Config.Recommend.Offline.EnableClickThroughPrediction = true
	suite.Config.Recommend.UserToUser = []config.UserToUserConfig{{Name: "default"}}
	// insert similar users
	err := suite.CacheClient.AddScores(ctx, cache.UserToUser, cache.Key("default", "0"), []cache.Score{
		{Id: "1", Score: 2},
		{Id: "2", Score: 1.5},
		{Id: "3", Score: 1},
	})
	suite.NoError(err)
	// insert feedback
	err = suite.DataClient.BatchInsertFeedback(ctx, []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "1", ItemId: "10"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "1", ItemId: "11"}},
	}, true, true, true)
	suite.NoError(err)
	err = suite.DataClient.BatchInsertFeedback(ctx, []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "10"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "12"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "48"}},
	}, true, true, true)
	suite.NoError(err)
	err = suite.DataClient.BatchInsertFeedback(ctx, []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "10"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "13"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "48"}},
	}, true, true, true)
	suite.NoError(err)
	// insert hidden items
	err = suite.DataClient.BatchInsertItems(ctx, []data.Item{{ItemId: "10", IsHidden: true}})
	suite.NoError(err)
	// insert categorized items
	err = suite.DataClient.BatchInsertItems(ctx, []data.Item{
		{ItemId: "12", Categories: []string{"*"}},
		{ItemId: "48", Categories: []string{"*"}},
	})
	suite.NoError(err)
	suite.rankers = []ctr.FactorizationMachines{&mockFactorizationMachine{}}
	suite.Recommend([]data.User{{UserId: "0"}})
	// read recommend time
	recommendTime, err := suite.CacheClient.Get(ctx, cache.Key(cache.LastUpdateUserRecommendTime, "0")).Time()
	suite.NoError(err)
	// read recommend result
	recommends, err := suite.CacheClient.SearchScores(ctx, cache.OfflineRecommend, "0", []string{""}, 0, 3)
	suite.NoError(err)
	suite.Equal([]cache.Score{
		{Id: "48", Score: 48, Categories: []string{"", "*"}, Timestamp: recommendTime},
		{Id: "13", Score: 13, Categories: []string{""}, Timestamp: recommendTime},
		{Id: "12", Score: 12, Categories: []string{"", "*"}, Timestamp: recommendTime},
	}, recommends)
	recommends, err = suite.CacheClient.SearchScores(ctx, cache.OfflineRecommend, "0", []string{"*"}, 0, 3)
	suite.NoError(err)
	suite.Equal([]cache.Score{
		{Id: "48", Score: 48, Categories: []string{"", "*"}, Timestamp: recommendTime},
		{Id: "12", Score: 12, Categories: []string{"", "*"}, Timestamp: recommendTime},
	}, recommends)
}

func (suite *WorkerTestSuite) TestRecommendLatest() {
	// create mock worker
	ctx := context.Background()
	suite.Config.Recommend.Offline.EnableColRecommend = false
	suite.Config.Recommend.Offline.EnableLatestRecommend = true
	suite.Config.Recommend.Offline.EnableClickThroughPrediction = true
	// insert items
	err := suite.DataClient.BatchInsertItems(ctx, []data.Item{
		{ItemId: "21", Timestamp: time.Unix(21, 0)},
		{ItemId: "20", Timestamp: time.Unix(20, 0)},
		{ItemId: "19", Timestamp: time.Unix(19, 0)},
		{ItemId: "18", Timestamp: time.Unix(18, 0)},
		{ItemId: "10", Categories: []string{"*"}, Timestamp: time.Unix(10, 0)},
		{ItemId: "9", Categories: []string{"*"}, Timestamp: time.Unix(9, 0)},
		{ItemId: "8", Categories: []string{"*"}, Timestamp: time.Unix(8, 0)},
	})
	suite.NoError(err)
	// insert hidden items
	err = suite.DataClient.BatchInsertItems(ctx, []data.Item{{ItemId: "21", IsHidden: true}})
	suite.NoError(err)
	suite.rankers = []ctr.FactorizationMachines{&mockFactorizationMachine{}}
	suite.Recommend([]data.User{{UserId: "0"}})
	// read recommend time
	recommendTime, err := suite.CacheClient.Get(ctx, cache.Key(cache.LastUpdateUserRecommendTime, "0")).Time()
	suite.NoError(err)
	// read recommend result
	recommends, err := suite.CacheClient.SearchScores(ctx, cache.OfflineRecommend, "0", []string{""}, 0, 3)
	suite.NoError(err)
	suite.Equal([]cache.Score{
		{Id: "20", Score: 20, Categories: []string{""}, Timestamp: recommendTime},
		{Id: "19", Score: 19, Categories: []string{""}, Timestamp: recommendTime},
		{Id: "18", Score: 18, Categories: []string{""}, Timestamp: recommendTime},
	}, recommends)
	recommends, err = suite.CacheClient.SearchScores(ctx, cache.OfflineRecommend, "0", []string{"*"}, 0, -1)
	suite.NoError(err)
	suite.Equal([]cache.Score{
		{Id: "10", Score: 10, Categories: []string{"", "*"}, Timestamp: recommendTime},
		{Id: "9", Score: 9, Categories: []string{"", "*"}, Timestamp: recommendTime},
		{Id: "8", Score: 8, Categories: []string{"", "*"}, Timestamp: recommendTime},
	}, recommends)
}

func (suite *WorkerTestSuite) TestMergeAndShuffle() {
	scores := suite.mergeAndShuffle([][]string{{"1", "2", "3"}, {"1", "3", "5"}})
	suite.ElementsMatch([]string{"1", "2", "3", "5"}, lo.Map(scores, func(d cache.Score, _ int) string { return d.Id }))
}

func marshal(t *testing.T, v interface{}) string {
	s, err := json.Marshal(v)
	assert.NoError(t, err)
	return string(s)
}

func newRankingDataset() (*dataset.Dataset, *dataset.Dataset) {
	return dataset.NewDataset(time.Now(), 0, 0), dataset.NewDataset(time.Now(), 0, 0)
}

func newClickDataset() (*ctr.Dataset, *ctr.Dataset) {
	dataSet := &ctr.Dataset{
		Index: dataset.NewUnifiedMapIndexBuilder().Build(),
	}
	return dataSet, dataSet
}

type mockMaster struct {
	protocol.UnimplementedMasterServer
	addr          chan string
	grpcServer    *grpc.Server
	cacheFilePath string
	dataFilePath  string
	meta          *protocol.Meta
	rankingModel  []byte
	clickModel    []byte
	userIndex     []byte
}

func newMockMaster(t *testing.T) *mockMaster {
	cfg := config.GetDefaultConfig()
	cfg.Database.DataStore = fmt.Sprintf("sqlite://%s/data.db", t.TempDir())
	cfg.Database.CacheStore = fmt.Sprintf("sqlite://%s/cache.db", t.TempDir())

	// create click model
	train, test := newClickDataset()
	fm := ctr.NewFMV2(model.Params{model.NEpochs: 0})
	fm.Fit(context.Background(), train, test, &ctr.FitConfig{})
	clickModelBuffer := bytes.NewBuffer(nil)
	err := ctr.MarshalModel(clickModelBuffer, fm)
	assert.NoError(t, err)

	// create ranking model
	trainSet, testSet := newRankingDataset()
	bpr := cf.NewBPR(model.Params{model.NEpochs: 0})
	bpr.Fit(context.Background(), trainSet, testSet, cf.NewFitConfig())
	rankingModelBuffer := bytes.NewBuffer(nil)
	err = cf.MarshalModel(rankingModelBuffer, bpr)
	assert.NoError(t, err)

	// create user index
	userIndexBuffer := bytes.NewBuffer(nil)
	err = dataset.MarshalIndex(userIndexBuffer, dataset.NewMapIndex())
	assert.NoError(t, err)

	return &mockMaster{
		addr: make(chan string),
		meta: &protocol.Meta{
			Config:                        marshal(t, cfg),
			ClickThroughRateModelId:       1,
			CollaborativeFilteringModelId: 2,
		},
		cacheFilePath: cfg.Database.CacheStore,
		dataFilePath:  cfg.Database.DataStore,
		userIndex:     userIndexBuffer.Bytes(),
		clickModel:    clickModelBuffer.Bytes(),
		rankingModel:  rankingModelBuffer.Bytes(),
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

func TestWorker_Sync(t *testing.T) {
	master := newMockMaster(t)
	go master.Start(t)
	address := <-master.addr
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	serv := &Worker{
		Settings:     config.NewSettings(),
		testMode:     true,
		masterClient: protocol.NewMasterClient(conn),
		syncedChan:   parallel.NewConditionChannel(),
		ticker:       time.NewTicker(time.Minute),
	}

	// This clause is used to test race condition.
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				p, _ := serv.Config.Recommend.Offline.GetExploreRecommend("popular")
				assert.Zero(t, p)
			}
		}
	}()

	serv.Sync()
	assert.Equal(t, master.dataFilePath, serv.dataPath)
	assert.Equal(t, master.cacheFilePath, serv.cachePath)
	assert.NoError(t, serv.DataClient.Close())
	assert.NoError(t, serv.CacheClient.Close())
	assert.Equal(t, int64(1), serv.latestClickThroughRateModelId)
	assert.Equal(t, int64(2), serv.latestCollaborativeFilteringModelId)
	assert.Zero(t, serv.clickThroughRateModelId)
	assert.Zero(t, serv.collaborativeFilteringModelId)
	master.Stop()
	done <- struct{}{}
}

func TestWorker_SyncRecommend(t *testing.T) {
	cfg := config.GetDefaultConfig()
	cfg.Recommend.Offline.ExploreRecommend = map[string]float64{"popular": 0.5}
	master := newMockMaster(t)
	master.meta.Config = marshal(t, cfg)
	go master.Start(t)
	address := <-master.addr
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	worker := &Worker{
		Settings:     config.NewSettings(),
		jobs:         1,
		testMode:     true,
		masterClient: protocol.NewMasterClient(conn),
		syncedChan:   parallel.NewConditionChannel(),
		ticker:       time.NewTicker(time.Minute),
	}
	worker.Sync()

	stopSync := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopSync:
				return
			default:
				worker.Sync()
			}
		}
	}()

	stopRecommend := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopRecommend:
				return
			default:
				worker.Settings.Config.OfflineRecommendDigest()
			}
		}
	}()

	time.Sleep(time.Second)
	stopSync <- struct{}{}
	stopRecommend <- struct{}{}
	master.Stop()
}

type mockFactorizationMachine struct {
	ctr.BaseFactorizationMachines
}

func (m mockFactorizationMachine) Complexity() int {
	panic("implement me")
}

func (m mockFactorizationMachine) SuggestParams(_ goptuna.Trial) model.Params {
	panic("implement me")
}

func (m mockFactorizationMachine) Clear() {
	panic("implement me")
}

func (m mockFactorizationMachine) Invalid() bool {
	return false
}

func (m mockFactorizationMachine) Predict(_, itemId string, _, _ []ctr.Label) float32 {
	score, err := strconv.Atoi(itemId)
	if err != nil {
		panic(err)
	}
	return float32(score)
}

func (m mockFactorizationMachine) InternalPredict(_ []int32, _ []float32) float32 {
	panic("implement me")
}

func (m mockFactorizationMachine) Fit(_ context.Context, _, _ dataset.CTRSplit, _ *ctr.FitConfig) ctr.Score {
	panic("implement me")
}

func (m mockFactorizationMachine) Marshal(_ io.Writer) error {
	panic("implement me")
}

func (suite *WorkerTestSuite) TestRankByClickTroughRate() {
	ctx := context.Background()
	// insert a user
	err := suite.DataClient.BatchInsertUsers(ctx, []data.User{{UserId: "1"}})
	suite.NoError(err)
	// insert items
	itemCache := NewItemCache()
	for i := 1; i <= 5; i++ {
		itemCache.Set(strconv.Itoa(i), data.Item{ItemId: strconv.Itoa(i)})
	}
	// rank items
	result, err := suite.rankByClickTroughRate(&data.User{UserId: "1"}, [][]string{{"1", "2", "3", "4", "5"}}, itemCache, new(mockFactorizationMachine))
	suite.NoError(err)
	suite.Equal([]string{"5", "4", "3", "2", "1"}, lo.Map(result, func(d cache.Score, _ int) string {
		return d.Id
	}))
	suite.IsDecreasing(lo.Map(result, func(d cache.Score, _ int) float64 {
		return d.Score
	}))
}

func (suite *WorkerTestSuite) TestUserActivity() {
	ctx := context.Background()
	err := suite.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, "0"), time.Now().AddDate(0, 0, -1)))
	suite.NoError(err)
	err = suite.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, "1"), time.Now().AddDate(0, 0, -10)))
	suite.NoError(err)
	err = suite.CacheClient.AddScores(ctx, cache.OfflineRecommend, "0", []cache.Score{{Id: "0", Score: 1, Categories: []string{""}}})
	suite.NoError(err)
	err = suite.CacheClient.AddScores(ctx, cache.OfflineRecommend, "1", []cache.Score{{Id: "1", Score: 1, Categories: []string{""}}})
	suite.NoError(err)
	err = suite.CacheClient.AddScores(ctx, cache.OfflineRecommend, "2", []cache.Score{{Id: "2", Score: 1, Categories: []string{""}}})
	suite.NoError(err)

	suite.True(suite.checkUserActiveTime(ctx, "0"))
	suite.True(suite.checkUserActiveTime(ctx, "1"))
	suite.True(suite.checkUserActiveTime(ctx, "2"))
	docs, err := suite.CacheClient.SearchScores(ctx, cache.OfflineRecommend, "0", []string{""}, 0, 1)
	suite.NoError(err)
	suite.NotEmpty(docs)
	docs, err = suite.CacheClient.SearchScores(ctx, cache.OfflineRecommend, "1", []string{""}, 0, 1)
	suite.NoError(err)
	suite.NotEmpty(docs)
	docs, err = suite.CacheClient.SearchScores(ctx, cache.OfflineRecommend, "2", []string{""}, 0, 1)
	suite.NoError(err)
	suite.NotEmpty(docs)

	suite.Config.Recommend.ActiveUserTTL = 5
	suite.True(suite.checkUserActiveTime(ctx, "0"))
	suite.False(suite.checkUserActiveTime(ctx, "1"))
	suite.True(suite.checkUserActiveTime(ctx, "2"))
	docs, err = suite.CacheClient.SearchScores(ctx, cache.OfflineRecommend, "0", []string{""}, 0, 1)
	suite.NoError(err)
	suite.NotEmpty(docs)
	docs, err = suite.CacheClient.SearchScores(ctx, cache.OfflineRecommend, "1", []string{""}, 0, 1)
	suite.NoError(err)
	suite.Empty(docs)
	docs, err = suite.CacheClient.SearchScores(ctx, cache.OfflineRecommend, "2", []string{""}, 0, 1)
	suite.NoError(err)
	suite.NotEmpty(docs)
}

func (suite *WorkerTestSuite) TestHealth() {
	// ready
	req := httptest.NewRequest("GET", "https://example.com/", nil)
	w := httptest.NewRecorder()
	suite.checkLive(w, req)
	suite.Equal(http.StatusOK, w.Code)
	suite.Equal(marshal(suite.T(), HealthStatus{
		DataStoreError:      nil,
		CacheStoreError:     nil,
		DataStoreConnected:  true,
		CacheStoreConnected: true,
	}), w.Body.String())

	// not ready
	dataClient, cacheClient := suite.DataClient, suite.CacheClient
	suite.DataClient, suite.CacheClient = data.NoDatabase{}, cache.NoDatabase{}
	w = httptest.NewRecorder()
	suite.checkLive(w, req)
	suite.Equal(http.StatusOK, w.Code)
	suite.Equal(marshal(suite.T(), HealthStatus{
		DataStoreError:      data.ErrNoDatabase,
		CacheStoreError:     cache.ErrNoDatabase,
		DataStoreConnected:  false,
		CacheStoreConnected: false,
	}), w.Body.String())
	suite.DataClient, suite.CacheClient = dataClient, cacheClient
}

func TestWorker(t *testing.T) {
	suite.Run(t, new(WorkerTestSuite))
}
