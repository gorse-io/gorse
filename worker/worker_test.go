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
	"github.com/alicebob/miniredis/v2"
	"github.com/bits-and-blooms/bitset"
	"github.com/scylladb/go-set/strset"
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
	"io"
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
	err := w.cacheClient.SetScores(cache.OfflineRecommend, "0", []cache.Scored{{"0", 0}})
	assert.NoError(t, err)
	assert.True(t, w.checkRecommendCacheTimeout("0", nil))
	err = w.cacheClient.SetTime(cache.LastModifyUserTime, "0", time.Now().Add(-time.Hour))
	assert.True(t, w.checkRecommendCacheTimeout("0", nil))
	err = w.cacheClient.SetTime(cache.LastUpdateUserRecommendTime, "0", time.Now().Add(-time.Hour*100))
	assert.True(t, w.checkRecommendCacheTimeout("0", nil))
	err = w.cacheClient.SetTime(cache.LastUpdateUserRecommendTime, "0", time.Now().Add(time.Hour*100))
	assert.False(t, w.checkRecommendCacheTimeout("0", nil))
	err = w.cacheClient.ClearScores(cache.OfflineRecommend, "0")
	assert.True(t, w.checkRecommendCacheTimeout("0", nil))
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
	m.UserPredictable = bitset.New(uint(numUsers)).Complement()
	m.ItemPredictable = bitset.New(uint(numItems)).Complement()
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
	assert.NoError(t, err)

	// insert hidden items and categorized items
	err = w.dataClient.BatchInsertItems([]data.Item{
		{ItemId: "10", IsHidden: true},
		{ItemId: "11", IsHidden: true},
		{ItemId: "3", Categories: []string{"*"}},
		{ItemId: "1", Categories: []string{"*"}},
	})
	assert.NoError(t, err)

	// create mock model
	w.rankingModel = newMockMatrixFactorizationForRecommend(1, 12)
	w.userIndex = w.rankingModel.GetUserIndex()
	w.Recommend([]string{"0"})

	recommends, err := w.cacheClient.GetScores(cache.OfflineRecommend, "0", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{
		{"3", 0},
		{"2", 0},
		{"1", 0},
		{"0", 0},
	}, recommends)
	recommends, err = w.cacheClient.GetCategoryScores(cache.OfflineRecommend, "0", "*", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{
		{"3", 0},
		{"1", 0},
	}, recommends)

	readCache, err := w.cacheClient.GetScores(cache.IgnoreItems, "0", 0, -1)
	read := cache.RemoveScores(readCache)
	assert.NoError(t, err)
	assert.Equal(t, []string{"0", "1", "2", "3"}, read)
	for _, v := range readCache {
		assert.Greater(t, v.Score, float32(time.Now().Unix()))
	}
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
	}, true, true, true)
	assert.NoError(t, err)

	// insert similar items
	err = w.cacheClient.SetScores(cache.ItemNeighbors, "21", []cache.Scored{
		{"22", 100000},
		{"25", 1000000},
		{"29", 1},
	})
	assert.NoError(t, err)
	err = w.cacheClient.SetScores(cache.ItemNeighbors, "22", []cache.Scored{
		{"23", 100000},
		{"25", 1000000},
		{"28", 1},
		{"29", 1},
	})
	assert.NoError(t, err)
	err = w.cacheClient.SetScores(cache.ItemNeighbors, "23", []cache.Scored{
		{"24", 100000},
		{"25", 1000000},
		{"27", 1},
		{"28", 1},
		{"29", 1},
	})
	assert.NoError(t, err)
	err = w.cacheClient.SetScores(cache.ItemNeighbors, "24", []cache.Scored{
		{"21", 100000},
		{"25", 1000000},
		{"26", 1},
		{"27", 1},
		{"28", 1},
		{"29", 1},
	})
	assert.NoError(t, err)

	// insert similar items in category
	err = w.cacheClient.SetCategoryScores(cache.ItemNeighbors, "21", "*", []cache.Scored{
		{"22", 100000},
	})
	assert.NoError(t, err)
	err = w.cacheClient.SetCategoryScores(cache.ItemNeighbors, "22", "*", []cache.Scored{
		{"28", 1},
	})
	assert.NoError(t, err)
	err = w.cacheClient.SetCategoryScores(cache.ItemNeighbors, "23", "*", []cache.Scored{
		{"24", 100000},
		{"28", 1},
	})
	assert.NoError(t, err)
	err = w.cacheClient.SetCategoryScores(cache.ItemNeighbors, "24", "*", []cache.Scored{
		{"26", 1},
		{"28", 1},
	})
	assert.NoError(t, err)

	// insert items
	err = w.dataClient.BatchInsertItems([]data.Item{{ItemId: "21"}, {ItemId: "22"}, {ItemId: "23"}, {ItemId: "24"},
		{ItemId: "25"}, {ItemId: "26"}, {ItemId: "27"}, {ItemId: "28"}, {ItemId: "29"}})
	// insert hidden items
	err = w.dataClient.BatchInsertItems([]data.Item{{ItemId: "25", IsHidden: true}})
	assert.NoError(t, err)
	// insert categorized items
	err = w.dataClient.BatchInsertItems([]data.Item{{ItemId: "26", Categories: []string{"*"}}, {ItemId: "28", Categories: []string{"*"}}})
	assert.NoError(t, err)
	w.rankingModel = newMockMatrixFactorizationForRecommend(1, 10)
	w.userIndex = w.rankingModel.GetUserIndex()
	w.Recommend([]string{"0"})
	recommends, err := w.cacheClient.GetScores(cache.OfflineRecommend, "0", 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"29", 0}, {"28", 0}, {"27", 0}}, recommends)
	recommends, err = w.cacheClient.GetCategoryScores(cache.OfflineRecommend, "0", "*", 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"28", 0}, {"26", 0}}, recommends)
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
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "1", ItemId: "10"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "1", ItemId: "11"}},
	}, true, true, true)
	assert.NoError(t, err)
	err = w.dataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "10"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "12"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "48"}},
	}, true, true, true)
	assert.NoError(t, err)
	err = w.dataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "10"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "13"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "48"}},
	}, true, true, true)
	assert.NoError(t, err)
	// insert hidden items
	err = w.dataClient.BatchInsertItems([]data.Item{{ItemId: "10", IsHidden: true}})
	assert.NoError(t, err)
	// insert categorized items
	err = w.dataClient.BatchInsertItems([]data.Item{
		{ItemId: "12", Categories: []string{"*"}},
		{ItemId: "48", Categories: []string{"*"}},
	})
	assert.NoError(t, err)
	w.rankingModel = newMockMatrixFactorizationForRecommend(1, 10)
	w.userIndex = w.rankingModel.GetUserIndex()
	w.Recommend([]string{"0"})
	recommends, err := w.cacheClient.GetScores(cache.OfflineRecommend, "0", 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"48", 0}, {"11", 0}, {"12", 0}}, recommends)
	recommends, err = w.cacheClient.GetCategoryScores(cache.OfflineRecommend, "0", "*", 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"48", 0}, {"12", 0}}, recommends)
}

func TestRecommend_Popular(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.cfg.Recommend.EnableColRecommend = false
	w.cfg.Recommend.EnablePopularRecommend = true
	// insert popular items
	err := w.cacheClient.SetSorted(cache.PopularItems, []cache.Scored{{"11", 11}, {"10", 10}, {"9", 9}, {"8", 8}})
	assert.NoError(t, err)
	// insert popular items with category *
	err = w.cacheClient.SetSorted(cache.Key(cache.PopularItems, "*"), []cache.Scored{{"20", 20}, {"19", 19}, {"18", 18}})
	assert.NoError(t, err)
	// insert items
	err = w.dataClient.BatchInsertItems([]data.Item{
		{ItemId: "11"}, {ItemId: "10"}, {ItemId: "9"}, {ItemId: "8"},
		{ItemId: "20", Categories: []string{"*"}},
		{ItemId: "19", Categories: []string{"*"}},
		{ItemId: "18", Categories: []string{"*"}},
	})
	assert.NoError(t, err)
	// insert hidden items
	err = w.dataClient.BatchInsertItems([]data.Item{{ItemId: "11", IsHidden: true}})
	assert.NoError(t, err)
	w.rankingModel = newMockMatrixFactorizationForRecommend(1, 10)
	w.userIndex = w.rankingModel.GetUserIndex()
	w.Recommend([]string{"0"})
	recommends, err := w.cacheClient.GetScores(cache.OfflineRecommend, "0", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"10", 0}, {"9", 0}, {"8", 0}}, recommends)
	recommends, err = w.cacheClient.GetCategoryScores(cache.OfflineRecommend, "0", "*", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"20", 0}, {"19", 0}, {"18", 0}}, recommends)
}

func TestRecommend_Latest(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.cfg.Recommend.EnableColRecommend = false
	w.cfg.Recommend.EnableLatestRecommend = true
	// insert latest items
	err := w.cacheClient.SetSorted(cache.LatestItems, []cache.Scored{{"11", 11}, {"10", 10}, {"9", 9}, {"8", 8}})
	assert.NoError(t, err)
	// insert the latest items with category *
	err = w.cacheClient.SetSorted(cache.Key(cache.LatestItems, "*"), []cache.Scored{{"20", 10}, {"19", 9}, {"18", 8}})
	assert.NoError(t, err)
	// insert items
	err = w.dataClient.BatchInsertItems([]data.Item{
		{ItemId: "11"}, {ItemId: "10"}, {ItemId: "9"}, {ItemId: "8"},
		{ItemId: "20", Categories: []string{"*"}},
		{ItemId: "19", Categories: []string{"*"}},
		{ItemId: "18", Categories: []string{"*"}},
	})
	assert.NoError(t, err)
	// insert hidden items
	err = w.dataClient.BatchInsertItems([]data.Item{{ItemId: "11", IsHidden: true}})
	assert.NoError(t, err)
	w.rankingModel = newMockMatrixFactorizationForRecommend(1, 10)
	w.userIndex = w.rankingModel.GetUserIndex()
	w.Recommend([]string{"0"})
	recommends, err := w.cacheClient.GetScores(cache.OfflineRecommend, "0", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"10", 0}, {"9", 0}, {"8", 0}}, recommends)
	recommends, err = w.cacheClient.GetCategoryScores(cache.OfflineRecommend, "0", "*", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"20", 0}, {"19", 0}, {"18", 0}}, recommends)
}

func TestRecommend_ColdStart(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.cfg.Recommend.EnableColRecommend = true
	w.cfg.Recommend.EnableLatestRecommend = true
	// insert latest items
	err := w.cacheClient.SetSorted(cache.LatestItems, []cache.Scored{{"11", 11}, {"10", 10}, {"9", 9}, {"8", 8}})
	assert.NoError(t, err)
	// insert the latest items with category *
	err = w.cacheClient.SetSorted(cache.Key(cache.LatestItems, "*"), []cache.Scored{{"20", 10}, {"19", 9}, {"18", 8}})
	assert.NoError(t, err)
	// insert items
	err = w.dataClient.BatchInsertItems([]data.Item{
		{ItemId: "11"}, {ItemId: "10"}, {ItemId: "9"}, {ItemId: "8"},
		{ItemId: "20", Categories: []string{"*"}},
		{ItemId: "19", Categories: []string{"*"}},
		{ItemId: "18", Categories: []string{"*"}},
	})
	assert.NoError(t, err)
	// insert hidden items
	err = w.dataClient.BatchInsertItems([]data.Item{{ItemId: "11", IsHidden: true}})
	assert.NoError(t, err)

	// ranking model not exist
	m := newMockMatrixFactorizationForRecommend(10, 100)
	w.userIndex = m.GetUserIndex()
	w.Recommend([]string{"0"})
	recommends, err := w.cacheClient.GetScores(cache.OfflineRecommend, "0", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"10", 0}, {"9", 0}, {"8", 0}}, recommends)
	recommends, err = w.cacheClient.GetCategoryScores(cache.OfflineRecommend, "0", "*", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"20", 0}, {"19", 0}, {"18", 0}}, recommends)

	// user not predictable
	w.rankingModel = m
	w.Recommend([]string{"100"})
	recommends, err = w.cacheClient.GetScores(cache.OfflineRecommend, "100", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"10", 0}, {"9", 0}, {"8", 0}}, recommends)
	recommends, err = w.cacheClient.GetCategoryScores(cache.OfflineRecommend, "100", "*", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"20", 0}, {"19", 0}, {"18", 0}}, recommends)
}

func TestMergeAndShuffle(t *testing.T) {
	scores := mergeAndShuffle([][]string{{"1", "2", "3"}, {"1", "3", "5"}})
	assert.ElementsMatch(t, []string{"1", "2", "3", "5"}, cache.RemoveScores(scores))
}

func TestExploreRecommend(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.cfg.Recommend.ExploreRecommend = map[string]float64{"popular": 0.3, "latest": 0.3}
	// insert popular items
	err := w.cacheClient.SetSorted(cache.PopularItems, []cache.Scored{{"popular", 0}})
	assert.NoError(t, err)
	// insert latest items
	err = w.cacheClient.SetSorted(cache.LatestItems, []cache.Scored{{"latest", 0}})
	assert.NoError(t, err)

	recommend, err := w.exploreRecommend(cache.CreateScoredItems(
		[]string{"1", "2", "3", "4", "5", "6", "7", "8"},
		[]float32{0, 0, 0, 0, 0, 0, 0, 0}), strset.New(), "")
	assert.NoError(t, err)
	items := cache.RemoveScores(recommend)
	assert.Contains(t, items, "latest")
	assert.Contains(t, items, "popular")
	assert.Equal(t, 8, len(recommend))
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
	rankingModel []byte
	clickModel   []byte
	userIndex    []byte
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
	clickModelBuffer := bytes.NewBuffer(nil)
	err = click.MarshalModel(clickModelBuffer, fm)
	assert.NoError(t, err)

	// create ranking model
	trainSet, testSet := newRankingDataset()
	bpr := ranking.NewBPR(model.Params{model.NEpochs: 0})
	bpr.Fit(trainSet, testSet, nil)
	rankingModelBuffer := bytes.NewBuffer(nil)
	err = ranking.MarshalModel(rankingModelBuffer, bpr)
	assert.NoError(t, err)

	// create user index
	userIndexBuffer := bytes.NewBuffer(nil)
	err = base.MarshalIndex(userIndexBuffer, base.NewMapIndex())
	assert.NoError(t, err)

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
		userIndex:    userIndexBuffer.Bytes(),
		clickModel:   clickModelBuffer.Bytes(),
		rankingModel: rankingModelBuffer.Bytes(),
	}
}

func (m *mockMaster) GetMeta(_ context.Context, _ *protocol.NodeInfo) (*protocol.Meta, error) {
	return m.meta, nil
}

func (m *mockMaster) GetRankingModel(_ *protocol.VersionInfo, sender protocol.Master_GetRankingModelServer) error {
	return sender.Send(&protocol.Fragment{Data: m.rankingModel})
}

func (m *mockMaster) GetClickModel(_ *protocol.VersionInfo, sender protocol.Master_GetClickModelServer) error {
	return sender.Send(&protocol.Fragment{Data: m.clickModel})
}

func (m *mockMaster) GetUserIndex(_ *protocol.VersionInfo, sender protocol.Master_GetUserIndexServer) error {
	return sender.Send(&protocol.Fragment{Data: m.userIndex})
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

	// This clause is used to test race condition.
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				p, _ := serv.cfg.Recommend.GetExploreRecommend("popular")
				assert.Zero(t, p)
			}
		}
	}()

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
	done <- struct{}{}
}

type mockFactorizationMachine struct {
	click.BaseFactorizationMachine
}

func (m mockFactorizationMachine) GetParamsGrid() model.ParamsGrid {
	panic("implement me")
}

func (m mockFactorizationMachine) Clear() {
	panic("implement me")
}

func (m mockFactorizationMachine) Invalid() bool {
	panic("implement me")
}

func (m mockFactorizationMachine) Predict(_, itemId string, _, _ []string) float32 {
	score, err := strconv.Atoi(itemId)
	if err != nil {
		panic(err)
	}
	return float32(score)
}

func (m mockFactorizationMachine) InternalPredict(_ []int32, _ []float32) float32 {
	panic("implement me")
}

func (m mockFactorizationMachine) Fit(_, _ *click.Dataset, _ *click.FitConfig) click.Score {
	panic("implement me")
}

func (m mockFactorizationMachine) Marshal(_ io.Writer) error {
	panic("implement me")
}

func TestRankByClickTroughRate(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	// insert a user
	err := w.dataClient.BatchInsertUsers([]data.User{{UserId: "1"}})
	assert.NoError(t, err)
	// insert items
	itemCache := make(map[string]data.Item)
	for i := 1; i <= 5; i++ {
		itemCache[strconv.Itoa(i)] = data.Item{ItemId: strconv.Itoa(i)}
	}
	// rank items
	w.clickModel = new(mockFactorizationMachine)
	result, err := w.rankByClickTroughRate("1", [][]string{{"1", "2", "3", "4", "5"}}, itemCache)
	assert.NoError(t, err)
	assert.Equal(t, []string{"5", "4", "3", "2", "1"}, cache.RemoveScores(result))
}
