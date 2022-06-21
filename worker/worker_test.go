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
	"github.com/thoas/go-funk"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestPullUsers(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	// create user index
	err := w.DataClient.BatchInsertUsers([]data.User{
		{UserId: "1"},
		{UserId: "2"},
		{UserId: "3"},
		{UserId: "4"},
		{UserId: "5"},
		{UserId: "6"},
		{UserId: "7"},
		{UserId: "8"},
	})
	assert.NoError(t, err)
	// create nodes
	nodes := []string{"a", "b", "c"}

	users, err := w.pullUsers(nodes, "b")
	assert.NoError(t, err)
	assert.Equal(t, []data.User{{UserId: "1"}, {UserId: "3"}, {UserId: "6"}}, users)

	_, err = w.pullUsers(nodes, "d")
	assert.Error(t, err)
}

func TestCheckRecommendCacheTimeout(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)

	// empty cache
	assert.True(t, w.checkRecommendCacheTimeout("0", nil))
	err := w.CacheClient.SetSorted(cache.Key(cache.OfflineRecommend, "0"), []cache.Scored{{"0", 0}})
	assert.NoError(t, err)

	// digest mismatch
	assert.True(t, w.checkRecommendCacheTimeout("0", nil))
	err = w.CacheClient.Set(cache.String(cache.Key(cache.OfflineRecommendDigest, "0"), w.Config.OfflineRecommendDigest()))
	assert.NoError(t, err)

	err = w.CacheClient.Set(cache.Time(cache.Key(cache.LastModifyUserTime, "0"), time.Now().Add(-time.Hour)))
	assert.NoError(t, err)
	assert.True(t, w.checkRecommendCacheTimeout("0", nil))
	err = w.CacheClient.Set(cache.Time(cache.Key(cache.LastUpdateUserRecommendTime, "0"), time.Now().Add(-time.Hour*100)))
	assert.NoError(t, err)
	assert.True(t, w.checkRecommendCacheTimeout("0", nil))
	err = w.CacheClient.Set(cache.Time(cache.Key(cache.LastUpdateUserRecommendTime, "0"), time.Now().Add(time.Hour*100)))
	assert.NoError(t, err)
	assert.False(t, w.checkRecommendCacheTimeout("0", nil))
	err = w.CacheClient.SetSorted(cache.Key(cache.OfflineRecommend, "0"), nil)
	assert.NoError(t, err)
	assert.True(t, w.checkRecommendCacheTimeout("0", nil))
}

type mockMatrixFactorizationForRecommend struct {
	ranking.BaseMatrixFactorization
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

func (m *mockMatrixFactorizationForRecommend) GetUserFactor(_ int32) []float32 {
	return []float32{1}
}

func (m *mockMatrixFactorizationForRecommend) GetItemFactor(itemId int32) []float32 {
	return []float32{float32(itemId)}
}

func (m *mockMatrixFactorizationForRecommend) Invalid() bool {
	panic("implement me")
}

func (m *mockMatrixFactorizationForRecommend) Fit(_, _ *ranking.DataSet, _ *ranking.FitConfig) ranking.Score {
	panic("implement me")
}

func (m *mockMatrixFactorizationForRecommend) Predict(_, itemId string) float32 {
	itemIndex, err := strconv.Atoi(itemId)
	if err != nil {
		panic(err)
	}
	return float32(itemIndex)
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
	w.Settings = config.NewSettings()
	w.DataClient, err = data.Open("redis://" + w.dataStoreServer.Addr())
	assert.NoError(t, err)
	w.CacheClient, err = cache.Open("redis://" + w.cacheStoreServer.Addr())
	assert.NoError(t, err)
	// configuration
	w.Config = config.GetDefaultConfig()
	w.jobs = 1
	return w
}

func (w *mockWorker) Close(t *testing.T) {
	err := w.DataClient.Close()
	assert.NoError(t, err)
	err = w.CacheClient.Close()
	assert.NoError(t, err)
	w.dataStoreServer.Close()
	w.cacheStoreServer.Close()
}

func TestRecommendMatrixFactorizationBruteForce(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.Config.Recommend.Offline.EnableColRecommend = true
	w.Config.Recommend.Collaborative.EnableIndex = false
	// insert feedbacks
	now := time.Now()
	err := w.DataClient.BatchInsertFeedback([]data.Feedback{
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
	err = w.DataClient.BatchInsertItems([]data.Item{
		{ItemId: "10", IsHidden: true},
		{ItemId: "11", IsHidden: true},
		{ItemId: "3", Categories: []string{"*"}},
		{ItemId: "1", Categories: []string{"*"}},
	})
	assert.NoError(t, err)

	// create mock model
	w.RankingModel = newMockMatrixFactorizationForRecommend(1, 12)
	w.Recommend([]data.User{{UserId: "0"}})

	recommends, err := w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0"), 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{
		{"3", 3},
		{"2", 2},
		{"1", 1},
		{"0", 0},
	}, recommends)
	recommends, err = w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0", "*"), 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{
		{"3", 3},
		{"1", 1},
	}, recommends)

	readCache, err := w.CacheClient.GetSorted(cache.Key(cache.IgnoreItems, "0"), 0, -1)
	read := cache.RemoveScores(readCache)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"0", "1", "2", "3"}, read)
	for _, v := range readCache {
		assert.Greater(t, v.Score, float64(time.Now().Unix()))
	}
}

func TestRecommendMatrixFactorizationHNSW(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.Config.Recommend.Offline.EnableColRecommend = true
	w.Config.Recommend.Collaborative.EnableIndex = true
	// insert feedbacks
	now := time.Now()
	err := w.DataClient.BatchInsertFeedback([]data.Feedback{
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
	err = w.DataClient.BatchInsertItems([]data.Item{
		{ItemId: "10", IsHidden: true},
		{ItemId: "11", IsHidden: true},
		{ItemId: "3", Categories: []string{"*"}},
		{ItemId: "1", Categories: []string{"*"}},
	})
	assert.NoError(t, err)

	// create mock model
	w.RankingModel = newMockMatrixFactorizationForRecommend(1, 12)
	w.Recommend([]data.User{{UserId: "0"}})

	recommends, err := w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0"), 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{
		{"3", 3},
		{"2", 2},
		{"1", 1},
		{"0", 0},
	}, recommends)
	recommends, err = w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0", "*"), 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{
		{"3", 3},
		{"1", 1},
	}, recommends)

	readCache, err := w.CacheClient.GetSorted(cache.Key(cache.IgnoreItems, "0"), 0, -1)
	read := cache.RemoveScores(readCache)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"0", "1", "2", "3"}, read)
	for _, v := range readCache {
		assert.Greater(t, v.Score, float64(time.Now().Unix()))
	}
}

func TestRecommend_ItemBased(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.Config.Recommend.Offline.EnableColRecommend = false
	w.Config.Recommend.Offline.EnableItemBasedRecommend = true
	// insert feedback
	err := w.DataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "21"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "22"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "23"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "24"}},
	}, true, true, true)
	assert.NoError(t, err)

	// insert similar items
	err = w.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "21"), []cache.Scored{
		{"22", 100000},
		{"25", 1000000},
		{"29", 1},
	})
	assert.NoError(t, err)
	err = w.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "22"), []cache.Scored{
		{"23", 100000},
		{"25", 1000000},
		{"28", 1},
		{"29", 1},
	})
	assert.NoError(t, err)
	err = w.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "23"), []cache.Scored{
		{"24", 100000},
		{"25", 1000000},
		{"27", 1},
		{"28", 1},
		{"29", 1},
	})
	assert.NoError(t, err)
	err = w.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "24"), []cache.Scored{
		{"21", 100000},
		{"25", 1000000},
		{"26", 1},
		{"27", 1},
		{"28", 1},
		{"29", 1},
	})
	assert.NoError(t, err)

	// insert similar items in category
	err = w.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "21", "*"), []cache.Scored{
		{"22", 100000},
	})
	assert.NoError(t, err)
	err = w.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "22", "*"), []cache.Scored{
		{"28", 1},
	})
	assert.NoError(t, err)
	err = w.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "23", "*"), []cache.Scored{
		{"24", 100000},
		{"28", 1},
	})
	assert.NoError(t, err)
	err = w.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "24", "*"), []cache.Scored{
		{"26", 1},
		{"28", 1},
	})
	assert.NoError(t, err)

	// insert items
	err = w.DataClient.BatchInsertItems([]data.Item{{ItemId: "21"}, {ItemId: "22"}, {ItemId: "23"}, {ItemId: "24"},
		{ItemId: "25"}, {ItemId: "26"}, {ItemId: "27"}, {ItemId: "28"}, {ItemId: "29"}})
	assert.NoError(t, err)
	// insert hidden items
	err = w.DataClient.BatchInsertItems([]data.Item{{ItemId: "25", IsHidden: true}})
	assert.NoError(t, err)
	// insert categorized items
	err = w.DataClient.BatchInsertItems([]data.Item{{ItemId: "26", Categories: []string{"*"}}, {ItemId: "28", Categories: []string{"*"}}})
	assert.NoError(t, err)
	w.RankingModel = newMockMatrixFactorizationForRecommend(1, 10)
	w.Recommend([]data.User{{UserId: "0"}})
	recommends, err := w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0"), 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"29", 29}, {"28", 28}, {"27", 27}}, recommends)
	recommends, err = w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0", "*"), 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"28", 28}, {"26", 26}}, recommends)
}

func TestRecommend_UserBased(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.Config.Recommend.Offline.EnableColRecommend = false
	w.Config.Recommend.Offline.EnableUserBasedRecommend = true
	// insert similar users
	err := w.CacheClient.SetSorted(cache.Key(cache.UserNeighbors, "0"), []cache.Scored{
		{"1", 2},
		{"2", 1.5},
		{"3", 1},
	})
	assert.NoError(t, err)
	// insert feedback
	err = w.DataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "1", ItemId: "10"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "1", ItemId: "11"}},
	}, true, true, true)
	assert.NoError(t, err)
	err = w.DataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "10"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "12"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "48"}},
	}, true, true, true)
	assert.NoError(t, err)
	err = w.DataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "10"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "13"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "48"}},
	}, true, true, true)
	assert.NoError(t, err)
	// insert hidden items
	err = w.DataClient.BatchInsertItems([]data.Item{{ItemId: "10", IsHidden: true}})
	assert.NoError(t, err)
	// insert categorized items
	err = w.DataClient.BatchInsertItems([]data.Item{
		{ItemId: "12", Categories: []string{"*"}},
		{ItemId: "48", Categories: []string{"*"}},
	})
	assert.NoError(t, err)
	w.RankingModel = newMockMatrixFactorizationForRecommend(1, 10)
	w.Recommend([]data.User{{UserId: "0"}})
	recommends, err := w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0"), 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"48", 48}, {"13", 13}, {"12", 12}}, recommends)
	recommends, err = w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0", "*"), 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"48", 48}, {"12", 12}}, recommends)
}

func TestRecommend_Popular(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.Config.Recommend.Offline.EnableColRecommend = false
	w.Config.Recommend.Offline.EnablePopularRecommend = true
	// insert popular items
	err := w.CacheClient.SetSorted(cache.PopularItems, []cache.Scored{{"11", 11}, {"10", 10}, {"9", 9}, {"8", 8}})
	assert.NoError(t, err)
	// insert popular items with category *
	err = w.CacheClient.SetSorted(cache.Key(cache.PopularItems, "*"), []cache.Scored{{"20", 20}, {"19", 19}, {"18", 18}})
	assert.NoError(t, err)
	// insert items
	err = w.DataClient.BatchInsertItems([]data.Item{
		{ItemId: "11"}, {ItemId: "10"}, {ItemId: "9"}, {ItemId: "8"},
		{ItemId: "20", Categories: []string{"*"}},
		{ItemId: "19", Categories: []string{"*"}},
		{ItemId: "18", Categories: []string{"*"}},
	})
	assert.NoError(t, err)
	// insert hidden items
	err = w.DataClient.BatchInsertItems([]data.Item{{ItemId: "11", IsHidden: true}})
	assert.NoError(t, err)
	w.RankingModel = newMockMatrixFactorizationForRecommend(1, 10)
	w.Recommend([]data.User{{UserId: "0"}})
	recommends, err := w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0"), 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"10", 10}, {"9", 9}, {"8", 8}}, recommends)
	recommends, err = w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0", "*"), 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"20", 20}, {"19", 19}, {"18", 18}}, recommends)
}

func TestRecommend_Latest(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.Config.Recommend.Offline.EnableColRecommend = false
	w.Config.Recommend.Offline.EnableLatestRecommend = true
	// insert latest items
	err := w.CacheClient.SetSorted(cache.LatestItems, []cache.Scored{{"11", 11}, {"10", 10}, {"9", 9}, {"8", 8}})
	assert.NoError(t, err)
	// insert the latest items with category *
	err = w.CacheClient.SetSorted(cache.Key(cache.LatestItems, "*"), []cache.Scored{{"20", 10}, {"19", 9}, {"18", 8}})
	assert.NoError(t, err)
	// insert items
	err = w.DataClient.BatchInsertItems([]data.Item{
		{ItemId: "11"}, {ItemId: "10"}, {ItemId: "9"}, {ItemId: "8"},
		{ItemId: "20", Categories: []string{"*"}},
		{ItemId: "19", Categories: []string{"*"}},
		{ItemId: "18", Categories: []string{"*"}},
	})
	assert.NoError(t, err)
	// insert hidden items
	err = w.DataClient.BatchInsertItems([]data.Item{{ItemId: "11", IsHidden: true}})
	assert.NoError(t, err)
	w.RankingModel = newMockMatrixFactorizationForRecommend(1, 10)
	w.Recommend([]data.User{{UserId: "0"}})
	recommends, err := w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0"), 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"10", 10}, {"9", 9}, {"8", 8}}, recommends)
	recommends, err = w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0", "*"), 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"20", 20}, {"19", 19}, {"18", 18}}, recommends)
}

func TestRecommend_ColdStart(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.Config.Recommend.Offline.EnableColRecommend = true
	w.Config.Recommend.Offline.EnableLatestRecommend = true
	// insert latest items
	err := w.CacheClient.SetSorted(cache.LatestItems, []cache.Scored{{"11", 11}, {"10", 10}, {"9", 9}, {"8", 8}})
	assert.NoError(t, err)
	// insert the latest items with category *
	err = w.CacheClient.SetSorted(cache.Key(cache.LatestItems, "*"), []cache.Scored{{"20", 10}, {"19", 9}, {"18", 8}})
	assert.NoError(t, err)
	// insert items
	err = w.DataClient.BatchInsertItems([]data.Item{
		{ItemId: "11"}, {ItemId: "10"}, {ItemId: "9"}, {ItemId: "8"},
		{ItemId: "20", Categories: []string{"*"}},
		{ItemId: "19", Categories: []string{"*"}},
		{ItemId: "18", Categories: []string{"*"}},
	})
	assert.NoError(t, err)
	// insert hidden items
	err = w.DataClient.BatchInsertItems([]data.Item{{ItemId: "11", IsHidden: true}})
	assert.NoError(t, err)

	// ranking model not exist
	m := newMockMatrixFactorizationForRecommend(10, 100)
	w.Recommend([]data.User{{UserId: "0"}})
	recommends, err := w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0"), 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []string{"10", "9", "8"}, cache.RemoveScores(recommends))
	recommends, err = w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0", "*"), 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []string{"20", "19", "18"}, cache.RemoveScores(recommends))

	// user not predictable
	w.RankingModel = m
	w.Recommend([]data.User{{UserId: "100"}})
	recommends, err = w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "100"), 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []string{"10", "9", "8"}, cache.RemoveScores(recommends))
	recommends, err = w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "100", "*"), 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []string{"20", "19", "18"}, cache.RemoveScores(recommends))
}

func TestMergeAndShuffle(t *testing.T) {
	scores := mergeAndShuffle([][]string{{"1", "2", "3"}, {"1", "3", "5"}})
	assert.ElementsMatch(t, []string{"1", "2", "3", "5"}, cache.RemoveScores(scores))
}

func TestExploreRecommend(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.Config.Recommend.Offline.ExploreRecommend = map[string]float64{"popular": 0.3, "latest": 0.3}
	// insert popular items
	err := w.CacheClient.SetSorted(cache.PopularItems, []cache.Scored{{"popular", 0}})
	assert.NoError(t, err)
	// insert latest items
	err = w.CacheClient.SetSorted(cache.LatestItems, []cache.Scored{{"latest", 0}})
	assert.NoError(t, err)

	recommend, err := w.exploreRecommend(cache.CreateScoredItems(
		funk.ReverseStrings([]string{"1", "2", "3", "4", "5", "6", "7", "8"}),
		funk.ReverseFloat64([]float64{1, 2, 3, 4, 5, 6, 7, 8})), strset.New(), "")
	assert.NoError(t, err)
	items := cache.RemoveScores(recommend)
	assert.Contains(t, items, "latest")
	assert.Contains(t, items, "popular")
	items = funk.FilterString(items, func(item string) bool {
		return item != "latest" && item != "popular"
	})
	assert.IsDecreasing(t, items)
	scores := cache.GetScores(recommend)
	assert.IsDecreasing(t, scores)
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
	cfg := config.GetDefaultConfig()
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
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	serv := &Worker{
		Settings:     config.NewSettings(),
		testMode:     true,
		masterClient: protocol.NewMasterClient(conn),
		syncedChan:   make(chan bool, 1024),
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
	assert.Equal(t, "redis://"+master.dataStore.Addr(), serv.dataPath)
	assert.Equal(t, "redis://"+master.cacheStore.Addr(), serv.cachePath)
	assert.Equal(t, int64(1), serv.latestClickModelVersion)
	assert.Equal(t, int64(2), serv.latestRankingModelVersion)
	assert.Zero(t, serv.ClickModelVersion)
	assert.Zero(t, serv.RankingModelVersion)
	serv.Pull()
	assert.Equal(t, int64(1), serv.ClickModelVersion)
	assert.Equal(t, int64(2), serv.RankingModelVersion)
	master.Stop()
	done <- struct{}{}
}

type mockFactorizationMachine struct {
	click.BaseFactorizationMachine
}

func (m mockFactorizationMachine) Bytes() int {
	panic("implement me")
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

func TestRankByCollaborativeFiltering(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	// insert a user
	err := w.DataClient.BatchInsertUsers([]data.User{{UserId: "1"}})
	assert.NoError(t, err)
	// insert items
	itemCache := make(map[string]data.Item)
	for i := 1; i <= 5; i++ {
		itemCache[strconv.Itoa(i)] = data.Item{ItemId: strconv.Itoa(i)}
	}
	// rank items
	w.RankingModel = newMockMatrixFactorizationForRecommend(10, 10)
	result, err := w.rankByCollaborativeFiltering("1", [][]string{{"1", "2", "3", "4", "5"}})
	assert.NoError(t, err)
	assert.Equal(t, []string{"5", "4", "3", "2", "1"}, cache.RemoveScores(result))
	assert.IsDecreasing(t, cache.GetScores(result))
}

func TestRankByClickTroughRate(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	// insert a user
	err := w.DataClient.BatchInsertUsers([]data.User{{UserId: "1"}})
	assert.NoError(t, err)
	// insert items
	itemCache := NewItemCache()
	for i := 1; i <= 5; i++ {
		itemCache.Set(strconv.Itoa(i), data.Item{ItemId: strconv.Itoa(i)})
	}
	// rank items
	w.ClickModel = new(mockFactorizationMachine)
	result, err := w.rankByClickTroughRate(&data.User{UserId: "1"}, [][]string{{"1", "2", "3", "4", "5"}}, itemCache)
	assert.NoError(t, err)
	assert.Equal(t, []string{"5", "4", "3", "2", "1"}, cache.RemoveScores(result))
	assert.IsDecreasing(t, cache.GetScores(result))
}

func TestReplacement_ClickThroughRate(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.Config.Recommend.DataSource.PositiveFeedbackTypes = []string{"p"}
	w.Config.Recommend.DataSource.ReadFeedbackTypes = []string{"n"}
	w.Config.Recommend.Offline.EnableColRecommend = false
	w.Config.Recommend.Offline.EnablePopularRecommend = true
	w.Config.Recommend.Replacement.EnableReplacement = true
	w.Config.Recommend.Offline.EnableClickThroughPrediction = true

	// 1. Insert historical items into empty recommendation.
	// insert items
	err := w.DataClient.BatchInsertItems([]data.Item{
		{ItemId: "10"}, {ItemId: "9"}, {ItemId: "8"}, {ItemId: "7"}, {ItemId: "6"}, {ItemId: "5"},
	})
	assert.NoError(t, err)
	// insert feedback
	err = w.DataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "p", UserId: "0", ItemId: "10"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "n", UserId: "0", ItemId: "9"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "i", UserId: "0", ItemId: "8"}},
	}, true, false, true)
	assert.NoError(t, err)
	w.ClickModel = new(mockFactorizationMachine)
	w.Recommend([]data.User{{UserId: "0"}})
	recommends, err := w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0"), 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"10", 10}, {"9", 9}}, recommends)

	// 2. Insert historical items into non-empty recommendation.
	err = w.CacheClient.Set(cache.Time(cache.Key(cache.LastUpdateUserRecommendTime, "0"), time.Now().AddDate(-1, 0, 0)))
	assert.NoError(t, err)
	// insert popular items
	err = w.CacheClient.SetSorted(cache.PopularItems, []cache.Scored{{"7", 10}, {"6", 9}, {"5", 8}})
	assert.NoError(t, err)
	// insert feedback
	err = w.DataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "p", UserId: "0", ItemId: "10"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "n", UserId: "0", ItemId: "9"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "i", UserId: "0", ItemId: "8"}},
	}, true, false, true)
	assert.NoError(t, err)
	w.Recommend([]data.User{{UserId: "0"}})
	recommends, err = w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0"), 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"10", 9}, {"9", 7.4}, {"7", 7}}, recommends)
}

func TestReplacement_CollaborativeFiltering(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
	w.Config.Recommend.DataSource.PositiveFeedbackTypes = []string{"p"}
	w.Config.Recommend.DataSource.ReadFeedbackTypes = []string{"n"}
	w.Config.Recommend.Offline.EnableColRecommend = false
	w.Config.Recommend.Offline.EnablePopularRecommend = true
	w.Config.Recommend.Replacement.EnableReplacement = true

	// 1. Insert historical items into empty recommendation.
	// insert items
	err := w.DataClient.BatchInsertItems([]data.Item{
		{ItemId: "10"}, {ItemId: "9"}, {ItemId: "8"}, {ItemId: "7"}, {ItemId: "6"}, {ItemId: "5"},
	})
	assert.NoError(t, err)
	// insert feedback
	err = w.DataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "p", UserId: "0", ItemId: "10"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "n", UserId: "0", ItemId: "9"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "i", UserId: "0", ItemId: "8"}},
	}, true, false, true)
	assert.NoError(t, err)
	w.RankingModel = newMockMatrixFactorizationForRecommend(1, 10)
	w.Recommend([]data.User{{UserId: "0"}})
	recommends, err := w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0"), 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"10", 10}, {"9", 9}}, recommends)

	// 2. Insert historical items into non-empty recommendation.
	err = w.CacheClient.Set(cache.Time(cache.Key(cache.LastUpdateUserRecommendTime, "0"), time.Now().AddDate(-1, 0, 0)))
	assert.NoError(t, err)
	// insert popular items
	err = w.CacheClient.SetSorted(cache.PopularItems, []cache.Scored{{"7", 10}, {"6", 9}, {"5", 8}})
	assert.NoError(t, err)
	// insert feedback
	err = w.DataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "p", UserId: "0", ItemId: "10"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "n", UserId: "0", ItemId: "9"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "i", UserId: "0", ItemId: "8"}},
	}, true, false, true)
	assert.NoError(t, err)
	w.Recommend([]data.User{{UserId: "0"}})
	recommends, err = w.CacheClient.GetSorted(cache.Key(cache.OfflineRecommend, "0"), 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []cache.Scored{{"10", 9}, {"9", 7.4}, {"7", 7}}, recommends)
}
