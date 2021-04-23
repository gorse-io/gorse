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
package master

import (
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/pr"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

type mockMaster struct {
	Master
	dataStoreServer  *miniredis.Miniredis
	cacheStoreServer *miniredis.Miniredis
}

func (m *mockMaster) Close() {
	m.dataStoreServer.Close()
	m.cacheStoreServer.Close()
}

func newMockMaster(t *testing.T) *mockMaster {
	s := new(mockMaster)
	// create mock database
	var err error
	s.dataStoreServer, err = miniredis.Run()
	assert.Nil(t, err)
	s.cacheStoreServer, err = miniredis.Run()
	assert.Nil(t, err)
	// open database
	s.dataStore, err = data.Open("redis://" + s.dataStoreServer.Addr())
	assert.Nil(t, err)
	s.cacheStore, err = cache.Open("redis://" + s.cacheStoreServer.Addr())
	assert.Nil(t, err)
	return s
}

func TestMaster_CollectLatest(t *testing.T) {
	// create mock master
	m := newMockMaster(t)
	defer m.Close()
	// create config
	m.cfg = &config.Config{}
	m.cfg.Database.CacheSize = 3
	// collect latest
	items := []data.Item{
		{"0", time.Date(2000, 1, 1, 1, 1, 0, 0, time.UTC), []string{"even"}, ""},
		{"1", time.Date(2001, 1, 1, 1, 1, 0, 0, time.UTC), []string{"odd"}, ""},
		{"2", time.Date(2002, 1, 1, 1, 1, 0, 0, time.UTC), []string{"even"}, ""},
		{"3", time.Date(2003, 1, 1, 1, 1, 0, 0, time.UTC), []string{"odd"}, ""},
		{"4", time.Date(2004, 1, 1, 1, 1, 0, 0, time.UTC), []string{"even"}, ""},
		{"5", time.Date(2005, 1, 1, 1, 1, 0, 0, time.UTC), []string{"odd"}, ""},
		{"6", time.Date(2006, 1, 1, 1, 1, 0, 0, time.UTC), []string{"even"}, ""},
		{"7", time.Date(2007, 1, 1, 1, 1, 0, 0, time.UTC), []string{"odd"}, ""},
		{"8", time.Date(2008, 1, 1, 1, 1, 0, 0, time.UTC), []string{"even"}, ""},
		{"9", time.Date(2009, 1, 1, 1, 1, 0, 0, time.UTC), []string{"odd"}, ""},
	}
	m.latest(items)
	// check latest items
	latest, err := m.cacheStore.GetList(cache.LatestItems, "", 0, 100)
	assert.Nil(t, err)
	assert.Equal(t, []string{"9", "8", "7"}, latest)
	latest, err = m.cacheStore.GetList(cache.LatestItems, "even", 0, 100)
	assert.Nil(t, err)
	assert.Equal(t, []string{"8", "6", "4"}, latest)
	latest, err = m.cacheStore.GetList(cache.LatestItems, "odd", 0, 100)
	assert.Nil(t, err)
	assert.Equal(t, []string{"9", "7", "5"}, latest)
}

func TestMaster_CollectPopItem(t *testing.T) {
	// create mock master
	m := newMockMaster(t)
	defer m.Close()
	// create config
	m.cfg = &config.Config{}
	m.cfg.Database.CacheSize = 3
	m.cfg.Recommend.PopularWindow = 365
	// collect latest
	items := []data.Item{
		{"0", time.Now(), []string{"even"}, ""},
		{"1", time.Now(), []string{"odd"}, ""},
		{"2", time.Now(), []string{"even"}, ""},
		{"3", time.Now(), []string{"odd"}, ""},
		{"4", time.Now(), []string{"even"}, ""},
		{"5", time.Now(), []string{"odd"}, ""},
		{"6", time.Now(), []string{"even"}, ""},
		{"7", time.Now(), []string{"odd"}, ""},
		{"8", time.Now(), []string{"even"}, ""},
		{"9", time.Now(), []string{"odd"}, ""},
	}
	feedbacks := make([]data.Feedback, 0)
	for i := 0; i < 10; i++ {
		for j := 0; j <= i; j++ {
			feedbacks = append(feedbacks, data.Feedback{
				FeedbackKey: data.FeedbackKey{
					ItemId: strconv.Itoa(i),
					UserId: strconv.Itoa(rand.Int()),
				},
				Timestamp: time.Now(),
			})
		}
	}
	for i := 0; i < 100; i++ {
		feedbacks = append(feedbacks, data.Feedback{
			FeedbackKey: data.FeedbackKey{
				ItemId: "0",
				UserId: strconv.Itoa(rand.Int()),
			},
			Timestamp: time.Now().AddDate(-100, 0, 0),
		})
	}
	m.popItem(items, feedbacks)
	// check popular items
	popular, err := m.cacheStore.GetList(cache.PopularItems, "", 0, 100)
	assert.Nil(t, err)
	assert.Equal(t, []string{"9", "8", "7"}, popular)
	popular, err = m.cacheStore.GetList(cache.PopularItems, "even", 0, 100)
	assert.Nil(t, err)
	assert.Equal(t, []string{"8", "6", "4"}, popular)
	popular, err = m.cacheStore.GetList(cache.PopularItems, "odd", 0, 100)
	assert.Nil(t, err)
	assert.Equal(t, []string{"9", "7", "5"}, popular)
}

func TestMaster_FitCFModel(t *testing.T) {
	// create mock master
	m := newMockMaster(t)
	defer m.Close()
	// create config
	m.cfg = &config.Config{}
	m.cfg.Database.CacheSize = 3
	m.cfg.Master.FitJobs = 4
	// collect similar
	items := []data.Item{
		{"0", time.Now(), []string{"even"}, ""},
		{"1", time.Now(), []string{"odd"}, ""},
		{"2", time.Now(), []string{"even"}, ""},
		{"3", time.Now(), []string{"odd"}, ""},
		{"4", time.Now(), []string{"even"}, ""},
		{"5", time.Now(), []string{"odd"}, ""},
		{"6", time.Now(), []string{"even"}, ""},
		{"7", time.Now(), []string{"odd"}, ""},
		{"8", time.Now(), []string{"even"}, ""},
		{"9", time.Now(), []string{"odd"}, ""},
	}
	feedbacks := make([]data.Feedback, 0)
	for i := 0; i < 10; i++ {
		for j := 0; j <= i; j++ {
			feedbacks = append(feedbacks, data.Feedback{
				FeedbackKey: data.FeedbackKey{
					ItemId:       strconv.Itoa(i),
					UserId:       strconv.Itoa(j),
					FeedbackType: "FeedbackType",
				},
				Timestamp: time.Now(),
			})
		}
	}
	var err error
	err = m.dataStore.BatchInsertItem(items)
	assert.Nil(t, err)
	err = m.dataStore.BatchInsertFeedback(feedbacks, true, true)
	assert.Nil(t, err)
	dataset, _, _, err := pr.LoadDataFromDatabase(m.dataStore, []string{"FeedbackType"})
	assert.Nil(t, err)
	// similar items (common users)
	m.similar(items, dataset, model.SimilarityDot)
	similar, err := m.cacheStore.GetList(cache.SimilarItems, "9", 0, 100)
	assert.Nil(t, err)
	assert.Equal(t, []string{"8", "7", "6"}, similar)
}
