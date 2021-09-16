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
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"strconv"
	"testing"
	"time"
)

func TestMaster_RunFindItemNeighborsTask(t *testing.T) {
	// create mock master
	m := newMockMaster(t)
	defer m.Close()
	// create config
	m.GorseConfig = &config.Config{}
	m.GorseConfig.Database.CacheSize = 3
	m.GorseConfig.Master.NumJobs = 4
	// collect similar
	items := []data.Item{
		{"0", time.Now(), []string{"a", "b", "c", "d"}, ""},
		{"1", time.Now(), []string{"b", "c", "d"}, ""},
		{"2", time.Now(), []string{"b", "c"}, ""},
		{"3", time.Now(), []string{"c"}, ""},
		{"4", time.Now(), []string{}, ""},
		{"5", time.Now(), []string{}, ""},
		{"6", time.Now(), []string{}, ""},
		{"7", time.Now(), []string{}, ""},
		{"8", time.Now(), []string{"a", "b", "c", "d", "e"}, ""},
		{"9", time.Now(), []string{}, ""},
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
	err = m.DataClient.BatchInsertItems(items)
	assert.NoError(t, err)
	err = m.DataClient.BatchInsertFeedback(feedbacks, true, true)
	assert.NoError(t, err)
	dataset, err := ranking.LoadDataFromDatabase(m.DataClient, []string{"FeedbackType"}, 0, 0)
	assert.NoError(t, err)

	// similar items (common users)
	m.GorseConfig.Recommend.ItemNeighborType = config.NeighborTypeRelated
	m.runFindItemNeighborsTask(dataset)
	similar, err := m.CacheClient.GetScores(cache.ItemNeighbors, "9", 0, 100)
	assert.NoError(t, err)
	assert.Equal(t, []string{"8", "7", "6"}, cache.RemoveScores(similar))
	assert.Equal(t, 10, m.taskMonitor.Tasks[TaskFindItemNeighbors].Done)
	assert.Equal(t, TaskStatusComplete, m.taskMonitor.Tasks[TaskFindItemNeighbors].Status)

	// similar items (common labels)
	m.GorseConfig.Recommend.ItemNeighborType = config.NeighborTypeSimilar
	m.runFindItemNeighborsTask(dataset)
	similar, err = m.CacheClient.GetScores(cache.ItemNeighbors, "8", 0, 100)
	assert.NoError(t, err)
	assert.Equal(t, []string{"0", "1", "2"}, cache.RemoveScores(similar))
	assert.Equal(t, 10, m.taskMonitor.Tasks[TaskFindItemNeighbors].Done)
	assert.Equal(t, TaskStatusComplete, m.taskMonitor.Tasks[TaskFindItemNeighbors].Status)

	// similar items (auto)
	m.GorseConfig.Recommend.ItemNeighborType = config.NeighborTypeAuto
	m.runFindItemNeighborsTask(dataset)
	similar, err = m.CacheClient.GetScores(cache.ItemNeighbors, "8", 0, 100)
	assert.NoError(t, err)
	assert.Equal(t, []string{"0", "1", "2"}, cache.RemoveScores(similar))
	similar, err = m.CacheClient.GetScores(cache.ItemNeighbors, "9", 0, 100)
	assert.NoError(t, err)
	assert.Equal(t, []string{"8", "7", "6"}, cache.RemoveScores(similar))
	assert.Equal(t, 10, m.taskMonitor.Tasks[TaskFindItemNeighbors].Done)
	assert.Equal(t, TaskStatusComplete, m.taskMonitor.Tasks[TaskFindItemNeighbors].Status)
}

func TestMaster_RunFindUserNeighborsTask(t *testing.T) {
	// create mock master
	m := newMockMaster(t)
	defer m.Close()
	// create config
	m.GorseConfig = &config.Config{}
	m.GorseConfig.Database.CacheSize = 3
	m.GorseConfig.Master.NumJobs = 4
	// collect similar
	users := []data.User{
		{"0", []string{"a", "b", "c", "d"}, nil, ""},
		{"1", []string{"b", "c", "d"}, nil, ""},
		{"2", []string{"b", "c"}, nil, ""},
		{"3", []string{"c"}, nil, ""},
		{"4", []string{}, nil, ""},
		{"5", []string{}, nil, ""},
		{"6", []string{}, nil, ""},
		{"7", []string{}, nil, ""},
		{"8", []string{"a", "b", "c", "d", "e"}, nil, ""},
		{"9", []string{}, nil, ""},
	}
	feedbacks := make([]data.Feedback, 0)
	for i := 0; i < 10; i++ {
		for j := 0; j <= i; j++ {
			feedbacks = append(feedbacks, data.Feedback{
				FeedbackKey: data.FeedbackKey{
					ItemId:       strconv.Itoa(j),
					UserId:       strconv.Itoa(i),
					FeedbackType: "FeedbackType",
				},
				Timestamp: time.Now(),
			})
		}
	}
	var err error
	err = m.DataClient.BatchInsertUsers(users)
	assert.NoError(t, err)
	err = m.DataClient.BatchInsertFeedback(feedbacks, true, true)
	assert.NoError(t, err)
	dataset, err := ranking.LoadDataFromDatabase(m.DataClient, []string{"FeedbackType"}, 0, 0)
	assert.NoError(t, err)

	// similar items (common users)
	m.GorseConfig.Recommend.UserNeighborType = config.NeighborTypeRelated
	m.runFindUserNeighborsTask(dataset)
	similar, err := m.CacheClient.GetScores(cache.UserNeighbors, "9", 0, 100)
	assert.NoError(t, err)
	assert.Equal(t, []string{"8", "7", "6"}, cache.RemoveScores(similar))
	assert.Equal(t, 10, m.taskMonitor.Tasks[TaskFindUserNeighbors].Done)
	assert.Equal(t, TaskStatusComplete, m.taskMonitor.Tasks[TaskFindUserNeighbors].Status)

	// similar items (common labels)
	m.GorseConfig.Recommend.UserNeighborType = config.NeighborTypeSimilar
	m.runFindUserNeighborsTask(dataset)
	similar, err = m.CacheClient.GetScores(cache.UserNeighbors, "8", 0, 100)
	assert.NoError(t, err)
	assert.Equal(t, []string{"0", "1", "2"}, cache.RemoveScores(similar))
	assert.Equal(t, 10, m.taskMonitor.Tasks[TaskFindUserNeighbors].Done)
	assert.Equal(t, TaskStatusComplete, m.taskMonitor.Tasks[TaskFindUserNeighbors].Status)

	// similar items (auto)
	m.GorseConfig.Recommend.UserNeighborType = config.NeighborTypeAuto
	m.runFindUserNeighborsTask(dataset)
	similar, err = m.CacheClient.GetScores(cache.UserNeighbors, "8", 0, 100)
	assert.NoError(t, err)
	assert.Equal(t, []string{"0", "1", "2"}, cache.RemoveScores(similar))
	similar, err = m.CacheClient.GetScores(cache.UserNeighbors, "9", 0, 100)
	assert.NoError(t, err)
	assert.Equal(t, []string{"8", "7", "6"}, cache.RemoveScores(similar))
	assert.Equal(t, 10, m.taskMonitor.Tasks[TaskFindUserNeighbors].Done)
	assert.Equal(t, TaskStatusComplete, m.taskMonitor.Tasks[TaskFindUserNeighbors].Status)
}
