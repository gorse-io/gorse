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
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
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
	assert.Nil(t, err)
	assert.Equal(t, []string{"2", "5", "8"}, users)

	users, err = split(userIndex, nodes, "d")
	assert.Error(t, err)
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
	return m
}

func (m *mockMatrixFactorizationForRecommend) InternalPredict(userId, itemId int) float32 {
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
	assert.Nil(t, err)
	w.cacheStoreServer, err = miniredis.Run()
	assert.Nil(t, err)
	// open database
	w.dataClient, err = data.Open("redis://" + w.dataStoreServer.Addr())
	assert.Nil(t, err)
	w.cacheClient, err = cache.Open("redis://" + w.cacheStoreServer.Addr())
	assert.Nil(t, err)
	// configuration
	w.cfg = (*config.Config)(nil).LoadDefaultIfNil()
	w.jobs = 1
	return w
}

func (w *mockWorker) Close(t *testing.T) {
	err := w.dataClient.Close()
	assert.Nil(t, err)
	err = w.cacheClient.Close()
	assert.Nil(t, err)
	w.dataStoreServer.Close()
	w.cacheStoreServer.Close()
}

func TestRecommendMatrixFactorization(t *testing.T) {
	// create mock worker
	w := newMockWorker(t)
	defer w.Close(t)
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
	assert.Nil(t, err)
	// create mock model
	m := newMockMatrixFactorizationForRecommend(1, 10)
	w.Recommend(m, []string{"0"})

	recommends, err := w.cacheClient.GetScores(cache.CollaborativeItems, "0", 0, -1)
	assert.Nil(t, err)
	assert.Equal(t, []cache.ScoredItem{
		{"3", 3},
		{"2", 2},
		{"1", 1},
		{"0", 0},
	}, recommends)

	read, err := w.cacheClient.GetList(cache.IgnoreItems, "0")
	assert.Nil(t, err)
	assert.Equal(t, []string{"4", "6", "8"}, read)
}
