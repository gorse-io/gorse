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
	"context"
	"strconv"
	"time"

	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base/task"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
)

func (s *MasterTestSuite) TestFindItemNeighborsBruteForce() {
	ctx := context.Background()
	// create config
	s.Config = &config.Config{}
	s.Config.Recommend.CacheSize = 3
	s.Config.Master.NumJobs = 4
	// collect similar
	items := []data.Item{
		{"0", false, []string{"*"}, time.Now(), []string{"a", "b", "c", "d"}, ""},
		{"1", false, []string{"*"}, time.Now(), []string{}, ""},
		{"2", false, []string{"*"}, time.Now(), []string{"b", "c", "d"}, ""},
		{"3", false, nil, time.Now(), []string{}, ""},
		{"4", false, nil, time.Now(), []string{"b", "c"}, ""},
		{"5", false, []string{"*"}, time.Now(), []string{}, ""},
		{"6", false, []string{"*"}, time.Now(), []string{"c"}, ""},
		{"7", false, []string{"*"}, time.Now(), []string{}, ""},
		{"8", false, []string{"*"}, time.Now(), []string{"a", "b", "c", "d", "e"}, ""},
		{"9", false, nil, time.Now(), []string{}, ""},
	}
	feedbacks := make([]data.Feedback, 0)
	for i := 0; i < 10; i++ {
		for j := 0; j <= i; j++ {
			if i%2 == 1 {
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
	}
	var err error
	err = s.DataClient.BatchInsertItems(ctx, items)
	s.NoError(err)
	err = s.DataClient.BatchInsertFeedback(ctx, feedbacks, true, true, true)
	s.NoError(err)

	// insert hidden item
	err = s.DataClient.BatchInsertItems(ctx, []data.Item{{
		ItemId:   "10",
		Labels:   []string{"a", "b", "c", "d", "e"},
		IsHidden: true,
	}})
	s.NoError(err)
	for i := 0; i <= 10; i++ {
		err = s.DataClient.BatchInsertFeedback(ctx, []data.Feedback{{
			FeedbackKey: data.FeedbackKey{UserId: strconv.Itoa(i), ItemId: "10", FeedbackType: "FeedbackType"},
		}}, true, true, true)
		s.NoError(err)
	}

	// load mock dataset
	dataset, _, _, _, err := s.LoadDataFromDatabase(s.DataClient, []string{"FeedbackType"}, nil, 0, 0, NewOnlineEvaluator())
	s.NoError(err)
	s.rankingTrainSet = dataset

	// similar items (common users)
	s.Config.Recommend.ItemNeighbors.NeighborType = config.NeighborTypeRelated
	neighborTask := NewFindItemNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err := s.CacheClient.SearchDocuments(ctx, cache.ItemNeighbors, "9", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5", "3"}, cache.ConvertDocumentsToValues(similar))
	s.Equal(s.estimateFindItemNeighborsComplexity(dataset), s.taskMonitor.Tasks[TaskFindItemNeighbors].Done)
	s.Equal(task.StatusComplete, s.taskMonitor.Tasks[TaskFindItemNeighbors].Status)
	// similar items in category (common users)
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.ItemNeighbors, "9", []string{"*"}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5", "1"}, cache.ConvertDocumentsToValues(similar))

	// similar items (common labels)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyItemTime, "8"), time.Now()))
	s.NoError(err)
	s.Config.Recommend.ItemNeighbors.NeighborType = config.NeighborTypeSimilar
	neighborTask = NewFindItemNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.ItemNeighbors, "8", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2", "4"}, cache.ConvertDocumentsToValues(similar))
	s.Equal(s.estimateFindItemNeighborsComplexity(dataset), s.taskMonitor.Tasks[TaskFindItemNeighbors].Done)
	s.Equal(task.StatusComplete, s.taskMonitor.Tasks[TaskFindItemNeighbors].Status)
	// similar items in category (common labels)
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.ItemNeighbors, "8", []string{"*"}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2", "6"}, cache.ConvertDocumentsToValues(similar))

	// similar items (auto)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyItemTime, "8"), time.Now()))
	s.NoError(err)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyItemTime, "9"), time.Now()))
	s.NoError(err)
	s.Config.Recommend.ItemNeighbors.NeighborType = config.NeighborTypeAuto
	neighborTask = NewFindItemNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.ItemNeighbors, "8", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2", "4"}, cache.ConvertDocumentsToValues(similar))
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.ItemNeighbors, "9", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5", "3"}, cache.ConvertDocumentsToValues(similar))
	s.Equal(s.estimateFindItemNeighborsComplexity(dataset), s.taskMonitor.Tasks[TaskFindItemNeighbors].Done)
	s.Equal(task.StatusComplete, s.taskMonitor.Tasks[TaskFindItemNeighbors].Status)
}

func (s *MasterTestSuite) TestFindItemNeighborsIVF() {
	// create mock master
	ctx := context.Background()
	// create config
	s.Config = &config.Config{}
	s.Config.Recommend.CacheSize = 3
	s.Config.Master.NumJobs = 4
	s.Config.Recommend.ItemNeighbors.EnableIndex = true
	s.Config.Recommend.ItemNeighbors.IndexRecall = 1
	s.Config.Recommend.ItemNeighbors.IndexFitEpoch = 10
	// collect similar
	items := []data.Item{
		{"0", false, []string{"*"}, time.Now(), []string{"a", "b", "c", "d"}, ""},
		{"1", false, []string{"*"}, time.Now(), []string{}, ""},
		{"2", false, []string{"*"}, time.Now(), []string{"b", "c", "d"}, ""},
		{"3", false, nil, time.Now(), []string{}, ""},
		{"4", false, nil, time.Now(), []string{"b", "c"}, ""},
		{"5", false, []string{"*"}, time.Now(), []string{}, ""},
		{"6", false, []string{"*"}, time.Now(), []string{"c"}, ""},
		{"7", false, []string{"*"}, time.Now(), []string{}, ""},
		{"8", false, []string{"*"}, time.Now(), []string{"a", "b", "c", "d", "e"}, ""},
		{"9", false, nil, time.Now(), []string{}, ""},
	}
	feedbacks := make([]data.Feedback, 0)
	for i := 0; i < 10; i++ {
		for j := 0; j <= i; j++ {
			if i%2 == 1 {
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
	}
	var err error
	err = s.DataClient.BatchInsertItems(ctx, items)
	s.NoError(err)
	err = s.DataClient.BatchInsertFeedback(ctx, feedbacks, true, true, true)
	s.NoError(err)

	// insert hidden item
	err = s.DataClient.BatchInsertItems(ctx, []data.Item{{
		ItemId:   "10",
		Labels:   []string{"a", "b", "c", "d", "e"},
		IsHidden: true,
	}})
	s.NoError(err)
	for i := 0; i <= 10; i++ {
		err = s.DataClient.BatchInsertFeedback(ctx, []data.Feedback{{
			FeedbackKey: data.FeedbackKey{UserId: strconv.Itoa(i), ItemId: "10", FeedbackType: "FeedbackType"},
		}}, true, true, true)
		s.NoError(err)
	}

	// load mock dataset
	dataset, _, _, _, err := s.LoadDataFromDatabase(s.DataClient, []string{"FeedbackType"}, nil, 0, 0, NewOnlineEvaluator())
	s.NoError(err)
	s.rankingTrainSet = dataset

	// similar items (common users)
	s.Config.Recommend.ItemNeighbors.NeighborType = config.NeighborTypeRelated
	neighborTask := NewFindItemNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err := s.CacheClient.SearchDocuments(ctx, cache.ItemNeighbors, "9", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5", "3"}, cache.ConvertDocumentsToValues(similar))
	s.Equal(s.estimateFindItemNeighborsComplexity(dataset), s.taskMonitor.Tasks[TaskFindItemNeighbors].Done)
	s.Equal(task.StatusComplete, s.taskMonitor.Tasks[TaskFindItemNeighbors].Status)
	// similar items in category (common users)
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.ItemNeighbors, "9", []string{"*"}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5", "1"}, cache.ConvertDocumentsToValues(similar))

	// similar items (common labels)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyItemTime, "8"), time.Now()))
	s.NoError(err)
	s.Config.Recommend.ItemNeighbors.NeighborType = config.NeighborTypeSimilar
	neighborTask = NewFindItemNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.ItemNeighbors, "8", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2", "4"}, cache.ConvertDocumentsToValues(similar))
	s.Equal(s.estimateFindItemNeighborsComplexity(dataset), s.taskMonitor.Tasks[TaskFindItemNeighbors].Done)
	s.Equal(task.StatusComplete, s.taskMonitor.Tasks[TaskFindItemNeighbors].Status)
	// similar items in category (common labels)
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.ItemNeighbors, "8", []string{"*"}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2", "6"}, cache.ConvertDocumentsToValues(similar))

	// similar items (auto)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyItemTime, "8"), time.Now()))
	s.NoError(err)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyItemTime, "9"), time.Now()))
	s.NoError(err)
	s.Config.Recommend.ItemNeighbors.NeighborType = config.NeighborTypeAuto
	neighborTask = NewFindItemNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.ItemNeighbors, "8", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2", "4"}, cache.ConvertDocumentsToValues(similar))
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.ItemNeighbors, "9", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5", "3"}, cache.ConvertDocumentsToValues(similar))
	s.Equal(s.estimateFindItemNeighborsComplexity(dataset), s.taskMonitor.Tasks[TaskFindItemNeighbors].Done)
	s.Equal(task.StatusComplete, s.taskMonitor.Tasks[TaskFindItemNeighbors].Status)
}

func (s *MasterTestSuite) TestFindItemNeighborsIVF_ZeroIDF() {
	ctx := context.Background()
	// create config
	s.Config = &config.Config{}
	s.Config.Recommend.CacheSize = 3
	s.Config.Master.NumJobs = 4
	s.Config.Recommend.ItemNeighbors.EnableIndex = true
	s.Config.Recommend.ItemNeighbors.IndexRecall = 1
	s.Config.Recommend.ItemNeighbors.IndexFitEpoch = 10

	// create dataset
	err := s.DataClient.BatchInsertItems(ctx, []data.Item{
		{"0", false, []string{"*"}, time.Now(), []string{"a", "a"}, ""},
		{"1", false, []string{"*"}, time.Now(), []string{"a", "a"}, ""},
	})
	s.NoError(err)
	err = s.DataClient.BatchInsertFeedback(ctx, []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "FeedbackType", UserId: "0", ItemId: "0"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "FeedbackType", UserId: "0", ItemId: "1"}},
	}, true, true, true)
	s.NoError(err)
	dataset, _, _, _, err := s.LoadDataFromDatabase(s.DataClient, []string{"FeedbackType"}, nil, 0, 0, NewOnlineEvaluator())
	s.NoError(err)
	s.rankingTrainSet = dataset

	// similar items (common users)
	s.Config.Recommend.ItemNeighbors.NeighborType = config.NeighborTypeRelated
	neighborTask := NewFindItemNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err := s.CacheClient.SearchDocuments(ctx, cache.ItemNeighbors, "0", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"1"}, cache.ConvertDocumentsToValues(similar))

	// similar items (common labels)
	s.Config.Recommend.ItemNeighbors.NeighborType = config.NeighborTypeSimilar
	neighborTask = NewFindItemNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.ItemNeighbors, "0", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"1"}, cache.ConvertDocumentsToValues(similar))
}

func (s *MasterTestSuite) TestFindUserNeighborsBruteForce() {
	ctx := context.Background()
	// create config
	s.Config = &config.Config{}
	s.Config.Recommend.CacheSize = 3
	s.Config.Master.NumJobs = 4
	// collect similar
	users := []data.User{
		{"0", []string{"a", "b", "c", "d"}, nil, ""},
		{"1", []string{}, nil, ""},
		{"2", []string{"b", "c", "d"}, nil, ""},
		{"3", []string{}, nil, ""},
		{"4", []string{"b", "c"}, nil, ""},
		{"5", []string{}, nil, ""},
		{"6", []string{"c"}, nil, ""},
		{"7", []string{}, nil, ""},
		{"8", []string{"a", "b", "c", "d", "e"}, nil, ""},
		{"9", []string{}, nil, ""},
	}
	feedbacks := make([]data.Feedback, 0)
	for i := 0; i < 10; i++ {
		for j := 0; j <= i; j++ {
			if i%2 == 1 {
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
	}
	var err error
	err = s.DataClient.BatchInsertUsers(ctx, users)
	s.NoError(err)
	err = s.DataClient.BatchInsertFeedback(ctx, feedbacks, true, true, true)
	s.NoError(err)
	dataset, _, _, _, err := s.LoadDataFromDatabase(s.DataClient, []string{"FeedbackType"}, nil, 0, 0, NewOnlineEvaluator())
	s.NoError(err)
	s.rankingTrainSet = dataset

	// similar items (common users)
	s.Config.Recommend.UserNeighbors.NeighborType = config.NeighborTypeRelated
	neighborTask := NewFindUserNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err := s.CacheClient.SearchDocuments(ctx, cache.UserNeighbors, "9", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5", "3"}, cache.ConvertDocumentsToValues(similar))
	s.Equal(s.estimateFindUserNeighborsComplexity(dataset), s.taskMonitor.Tasks[TaskFindUserNeighbors].Done)
	s.Equal(task.StatusComplete, s.taskMonitor.Tasks[TaskFindUserNeighbors].Status)

	// similar items (common labels)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, "8"), time.Now()))
	s.NoError(err)
	s.Config.Recommend.UserNeighbors.NeighborType = config.NeighborTypeSimilar
	neighborTask = NewFindUserNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.UserNeighbors, "8", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2", "4"}, cache.ConvertDocumentsToValues(similar))
	s.Equal(s.estimateFindUserNeighborsComplexity(dataset), s.taskMonitor.Tasks[TaskFindUserNeighbors].Done)
	s.Equal(task.StatusComplete, s.taskMonitor.Tasks[TaskFindUserNeighbors].Status)

	// similar items (auto)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, "8"), time.Now()))
	s.NoError(err)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, "9"), time.Now()))
	s.NoError(err)
	s.Config.Recommend.UserNeighbors.NeighborType = config.NeighborTypeAuto
	neighborTask = NewFindUserNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.UserNeighbors, "8", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2", "4"}, cache.ConvertDocumentsToValues(similar))
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.UserNeighbors, "9", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5", "3"}, cache.ConvertDocumentsToValues(similar))
	s.Equal(s.estimateFindUserNeighborsComplexity(dataset), s.taskMonitor.Tasks[TaskFindUserNeighbors].Done)
	s.Equal(task.StatusComplete, s.taskMonitor.Tasks[TaskFindUserNeighbors].Status)
}

func (s *MasterTestSuite) TestFindUserNeighborsIVF() {
	ctx := context.Background()
	// create config
	s.Config = &config.Config{}
	s.Config.Recommend.CacheSize = 3
	s.Config.Master.NumJobs = 4
	s.Config.Recommend.UserNeighbors.EnableIndex = true
	s.Config.Recommend.UserNeighbors.IndexRecall = 1
	s.Config.Recommend.UserNeighbors.IndexFitEpoch = 10
	// collect similar
	users := []data.User{
		{"0", []string{"a", "b", "c", "d"}, nil, ""},
		{"1", []string{}, nil, ""},
		{"2", []string{"b", "c", "d"}, nil, ""},
		{"3", []string{}, nil, ""},
		{"4", []string{"b", "c"}, nil, ""},
		{"5", []string{}, nil, ""},
		{"6", []string{"c"}, nil, ""},
		{"7", []string{}, nil, ""},
		{"8", []string{"a", "b", "c", "d", "e"}, nil, ""},
		{"9", []string{}, nil, ""},
	}
	feedbacks := make([]data.Feedback, 0)
	for i := 0; i < 10; i++ {
		for j := 0; j <= i; j++ {
			if i%2 == 1 {
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
	}
	var err error
	err = s.DataClient.BatchInsertUsers(ctx, users)
	s.NoError(err)
	err = s.DataClient.BatchInsertFeedback(ctx, feedbacks, true, true, true)
	s.NoError(err)
	dataset, _, _, _, err := s.LoadDataFromDatabase(s.DataClient, []string{"FeedbackType"}, nil, 0, 0, NewOnlineEvaluator())
	s.NoError(err)
	s.rankingTrainSet = dataset

	// similar items (common users)
	s.Config.Recommend.UserNeighbors.NeighborType = config.NeighborTypeRelated
	neighborTask := NewFindUserNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err := s.CacheClient.SearchDocuments(ctx, cache.UserNeighbors, "9", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5", "3"}, cache.ConvertDocumentsToValues(similar))
	s.Equal(s.estimateFindUserNeighborsComplexity(dataset), s.taskMonitor.Tasks[TaskFindUserNeighbors].Done)
	s.Equal(task.StatusComplete, s.taskMonitor.Tasks[TaskFindUserNeighbors].Status)

	// similar items (common labels)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, "8"), time.Now()))
	s.NoError(err)
	s.Config.Recommend.UserNeighbors.NeighborType = config.NeighborTypeSimilar
	neighborTask = NewFindUserNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.UserNeighbors, "8", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2", "4"}, cache.ConvertDocumentsToValues(similar))
	s.Equal(s.estimateFindUserNeighborsComplexity(dataset), s.taskMonitor.Tasks[TaskFindUserNeighbors].Done)
	s.Equal(task.StatusComplete, s.taskMonitor.Tasks[TaskFindUserNeighbors].Status)

	// similar items (auto)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, "8"), time.Now()))
	s.NoError(err)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, "9"), time.Now()))
	s.NoError(err)
	s.Config.Recommend.UserNeighbors.NeighborType = config.NeighborTypeAuto
	neighborTask = NewFindUserNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.UserNeighbors, "8", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2", "4"}, cache.ConvertDocumentsToValues(similar))
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.UserNeighbors, "9", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5", "3"}, cache.ConvertDocumentsToValues(similar))
	s.Equal(s.estimateFindUserNeighborsComplexity(dataset), s.taskMonitor.Tasks[TaskFindUserNeighbors].Done)
	s.Equal(task.StatusComplete, s.taskMonitor.Tasks[TaskFindUserNeighbors].Status)
}

func (s *MasterTestSuite) TestFindUserNeighborsIVF_ZeroIDF() {
	ctx := context.Background()
	// create config
	s.Config = &config.Config{}
	s.Config.Recommend.CacheSize = 3
	s.Config.Master.NumJobs = 4
	s.Config.Recommend.UserNeighbors.EnableIndex = true
	s.Config.Recommend.UserNeighbors.IndexRecall = 1
	s.Config.Recommend.UserNeighbors.IndexFitEpoch = 10

	// create dataset
	err := s.DataClient.BatchInsertUsers(ctx, []data.User{
		{"0", []string{"a", "a"}, nil, ""},
		{"1", []string{"a", "a"}, nil, ""},
	})
	s.NoError(err)
	err = s.DataClient.BatchInsertFeedback(ctx, []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "FeedbackType", UserId: "0", ItemId: "0"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "FeedbackType", UserId: "1", ItemId: "0"}},
	}, true, true, true)
	s.NoError(err)
	dataset, _, _, _, err := s.LoadDataFromDatabase(s.DataClient, []string{"FeedbackType"}, nil, 0, 0, NewOnlineEvaluator())
	s.NoError(err)
	s.rankingTrainSet = dataset

	// similar users (common items)
	s.Config.Recommend.UserNeighbors.NeighborType = config.NeighborTypeRelated
	neighborTask := NewFindUserNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err := s.CacheClient.SearchDocuments(ctx, cache.UserNeighbors, "0", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"1"}, cache.ConvertDocumentsToValues(similar))

	// similar users (common labels)
	s.Config.Recommend.UserNeighbors.NeighborType = config.NeighborTypeSimilar
	neighborTask = NewFindUserNeighborsTask(&s.Master)
	s.NoError(neighborTask.run(nil))
	similar, err = s.CacheClient.SearchDocuments(ctx, cache.UserNeighbors, "0", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"1"}, cache.ConvertDocumentsToValues(similar))
}

func (s *MasterTestSuite) TestLoadDataFromDatabase() {
	ctx := context.Background()
	// create config
	s.Config = &config.Config{}
	s.Config.Recommend.CacheSize = 3
	s.Config.Recommend.DataSource.PositiveFeedbackTypes = []string{"positive"}
	s.Config.Recommend.DataSource.ReadFeedbackTypes = []string{"negative"}

	// insert items
	var items []data.Item
	for i := 0; i < 9; i++ {
		items = append(items, data.Item{
			ItemId:     strconv.Itoa(i),
			Timestamp:  time.Date(2000+i, 1, 1, 1, 1, 0, 0, time.UTC),
			Labels:     []string{strconv.Itoa(i % 3), strconv.Itoa(i*10 + 10)},
			Categories: []string{strconv.Itoa(i % 3)},
		})
	}
	err := s.DataClient.BatchInsertItems(ctx, items)
	s.NoError(err)
	err = s.DataClient.BatchInsertItems(ctx, []data.Item{{
		ItemId:    "9",
		Timestamp: time.Date(2020, 1, 1, 1, 1, 0, 0, time.UTC),
		IsHidden:  true,
	}})
	s.NoError(err)

	// insert users
	var users []data.User
	for i := 0; i <= 10; i++ {
		users = append(users, data.User{
			UserId: strconv.Itoa(i),
			Labels: []string{strconv.Itoa(i % 5), strconv.Itoa(i*10 + 10)},
		})
	}
	err = s.DataClient.BatchInsertUsers(ctx, users)
	s.NoError(err)

	// insert feedback
	feedbacks := make([]data.Feedback, 0)
	for i := 0; i < 10; i++ {
		// positive feedback
		// item 0: user 0
		// ...
		// item 9: user 0 ... user 9
		for j := 0; j <= i; j++ {
			feedbacks = append(feedbacks, data.Feedback{
				FeedbackKey: data.FeedbackKey{
					ItemId:       strconv.Itoa(i),
					UserId:       strconv.Itoa(j),
					FeedbackType: "positive",
				},
				Timestamp: time.Now(),
			})
		}
		// negative feedback
		// item 0: user 1 .. user 10
		// ...
		// item 9: user 10
		for j := i + 1; j < 11; j++ {
			feedbacks = append(feedbacks, data.Feedback{
				FeedbackKey: data.FeedbackKey{
					ItemId:       strconv.Itoa(i),
					UserId:       strconv.Itoa(j),
					FeedbackType: "negative",
				},
				Timestamp: time.Now(),
			})
		}
	}
	err = s.DataClient.BatchInsertFeedback(ctx, feedbacks, false, false, true)
	s.NoError(err)

	// load dataset
	err = s.runLoadDatasetTask()
	s.NoError(err)
	s.Equal(11, s.rankingTrainSet.UserCount())
	s.Equal(10, s.rankingTrainSet.ItemCount())
	s.Equal(11, s.rankingTestSet.UserCount())
	s.Equal(10, s.rankingTestSet.ItemCount())
	s.Equal(55, s.rankingTrainSet.Count()+s.rankingTestSet.Count())
	s.Equal(11, s.clickTrainSet.UserCount())
	s.Equal(10, s.clickTrainSet.ItemCount())
	s.Equal(11, s.clickTestSet.UserCount())
	s.Equal(10, s.clickTestSet.ItemCount())
	s.Equal(int32(3), s.clickTrainSet.Index.CountItemLabels())
	s.Equal(int32(5), s.clickTrainSet.Index.CountUserLabels())
	s.Equal(int32(3), s.clickTestSet.Index.CountItemLabels())
	s.Equal(int32(5), s.clickTestSet.Index.CountUserLabels())
	s.Equal(90, s.clickTrainSet.Count()+s.clickTestSet.Count())
	s.Equal(45, s.clickTrainSet.PositiveCount+s.clickTestSet.PositiveCount)
	s.Equal(45, s.clickTrainSet.NegativeCount+s.clickTestSet.NegativeCount)

	// check latest items
	latest, err := s.CacheClient.SearchDocuments(ctx, cache.LatestItems, "", []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]cache.Document{
		{Id: items[8].ItemId, Score: float64(items[8].Timestamp.Unix())},
		{Id: items[7].ItemId, Score: float64(items[7].Timestamp.Unix())},
		{Id: items[6].ItemId, Score: float64(items[6].Timestamp.Unix())},
	}, lo.Map(latest, func(document cache.Document, _ int) cache.Document {
		return cache.Document{Id: document.Id, Score: document.Score}
	}))
	latest, err = s.CacheClient.SearchDocuments(ctx, cache.LatestItems, "", []string{"2"}, 0, 100)
	s.NoError(err)
	s.Equal([]cache.Document{
		{Id: items[8].ItemId, Score: float64(items[8].Timestamp.Unix())},
		{Id: items[5].ItemId, Score: float64(items[5].Timestamp.Unix())},
		{Id: items[2].ItemId, Score: float64(items[2].Timestamp.Unix())},
	}, lo.Map(latest, func(document cache.Document, _ int) cache.Document {
		return cache.Document{Id: document.Id, Score: document.Score}
	}))

	// check popular items
	popular, err := s.CacheClient.SearchDocuments(ctx, cache.PopularItems, "", []string{""}, 0, 3)
	s.NoError(err)
	s.Equal([]cache.Document{
		{Id: items[8].ItemId, Score: 9},
		{Id: items[7].ItemId, Score: 8},
		{Id: items[6].ItemId, Score: 7},
	}, lo.Map(popular, func(document cache.Document, _ int) cache.Document {
		return cache.Document{Id: document.Id, Score: document.Score}
	}))
	popular, err = s.CacheClient.SearchDocuments(ctx, cache.PopularItems, "", []string{"2"}, 0, 3)
	s.NoError(err)
	s.Equal([]cache.Document{
		{Id: items[8].ItemId, Score: 9},
		{Id: items[5].ItemId, Score: 6},
		{Id: items[2].ItemId, Score: 3},
	}, lo.Map(popular, func(document cache.Document, _ int) cache.Document {
		return cache.Document{Id: document.Id, Score: document.Score}
	}))

	// check categories
	categories, err := s.CacheClient.GetSet(ctx, cache.ItemCategories)
	s.NoError(err)
	s.Equal([]string{"0", "1", "2"}, categories)
}

func (s *MasterTestSuite) TestCheckItemNeighborCacheTimeout() {
	s.Config = config.GetDefaultConfig()
	ctx := context.Background()

	// empty cache
	s.True(s.checkItemNeighborCacheTimeout("1", nil))
	err := s.CacheClient.AddDocuments(ctx, cache.ItemNeighbors, "1", []cache.Document{
		{Id: "2", Score: 1, Categories: []string{""}},
		{Id: "3", Score: 2, Categories: []string{""}},
		{Id: "4", Score: 3, Categories: []string{""}},
	})
	s.NoError(err)

	// digest mismatch
	err = s.CacheClient.Set(ctx, cache.String(cache.Key(cache.ItemNeighborsDigest, "1"), "digest"))
	s.NoError(err)
	s.True(s.checkItemNeighborCacheTimeout("1", nil))

	// staled cache
	err = s.CacheClient.Set(ctx, cache.String(cache.Key(cache.ItemNeighborsDigest, "1"), s.Config.ItemNeighborDigest()))
	s.NoError(err)
	s.True(s.checkItemNeighborCacheTimeout("1", nil))
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyItemTime, "1"), time.Now().Add(-time.Minute)))
	s.NoError(err)
	s.True(s.checkItemNeighborCacheTimeout("1", nil))
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastUpdateItemNeighborsTime, "1"), time.Now().Add(-time.Hour)))
	s.NoError(err)
	s.True(s.checkItemNeighborCacheTimeout("1", nil))

	// not staled cache
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastUpdateItemNeighborsTime, "1"), time.Now()))
	s.NoError(err)
	s.False(s.checkItemNeighborCacheTimeout("1", nil))
}

func (s *MasterTestSuite) TestCheckUserNeighborCacheTimeout() {
	ctx := context.Background()
	s.Config = config.GetDefaultConfig()

	// empty cache
	s.True(s.checkUserNeighborCacheTimeout("1"))
	err := s.CacheClient.AddDocuments(ctx, cache.UserNeighbors, "1", []cache.Document{
		{Id: "1", Score: 1, Categories: []string{""}},
		{Id: "2", Score: 2, Categories: []string{""}},
		{Id: "3", Score: 3, Categories: []string{""}},
	})
	s.NoError(err)

	// digest mismatch
	err = s.CacheClient.Set(ctx, cache.String(cache.Key(cache.UserNeighborsDigest, "1"), "digest"))
	s.NoError(err)
	s.True(s.checkUserNeighborCacheTimeout("1"))

	// staled cache
	err = s.CacheClient.Set(ctx, cache.String(cache.Key(cache.UserNeighborsDigest, "1"), s.Config.UserNeighborDigest()))
	s.NoError(err)
	s.True(s.checkUserNeighborCacheTimeout("1"))
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, "1"), time.Now().Add(-time.Minute)))
	s.NoError(err)
	s.True(s.checkUserNeighborCacheTimeout("1"))
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastUpdateUserNeighborsTime, "1"), time.Now().Add(-time.Hour)))
	s.NoError(err)
	s.True(s.checkUserNeighborCacheTimeout("1"))

	// not staled cache
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastUpdateUserNeighborsTime, "1"), time.Now()))
	s.NoError(err)
	s.False(s.checkUserNeighborCacheTimeout("1"))
}
