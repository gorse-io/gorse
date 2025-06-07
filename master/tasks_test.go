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
	"runtime"
	"strconv"
	"time"

	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/common/expression"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
)

func (s *MasterTestSuite) TestFindItemToItem() {
	ctx := context.Background()
	// create config
	s.Config = &config.Config{}
	s.Config.Recommend.CacheSize = 3
	s.Config.Master.NumJobs = 4
	// collect similar
	items := []data.Item{
		{ItemId: "0", IsHidden: false, Categories: []string{"*"}, Timestamp: time.Now(), Labels: []string{"a", "b", "c", "d"}, Comment: ""},
		{ItemId: "1", IsHidden: false, Categories: []string{"*"}, Timestamp: time.Now(), Labels: []string{}, Comment: ""},
		{ItemId: "2", IsHidden: false, Categories: []string{"*"}, Timestamp: time.Now(), Labels: []string{"b", "c", "d"}, Comment: ""},
		{ItemId: "3", IsHidden: false, Categories: nil, Timestamp: time.Now(), Labels: []string{}, Comment: ""},
		{ItemId: "4", IsHidden: false, Categories: nil, Timestamp: time.Now(), Labels: []string{"b", "c"}, Comment: ""},
		{ItemId: "5", IsHidden: false, Categories: []string{"*"}, Timestamp: time.Now(), Labels: []string{}, Comment: ""},
		{ItemId: "6", IsHidden: false, Categories: []string{"*"}, Timestamp: time.Now(), Labels: []string{"c"}, Comment: ""},
		{ItemId: "7", IsHidden: false, Categories: []string{"*"}, Timestamp: time.Now(), Labels: []string{}, Comment: ""},
		{ItemId: "8", IsHidden: false, Categories: []string{"*"}, Timestamp: time.Now(), Labels: []string{"a", "b", "c", "d", "e"}, Comment: ""},
		{ItemId: "9", IsHidden: false, Categories: nil, Timestamp: time.Now(), Labels: []string{}, Comment: ""},
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
	_, dataSet, err := s.LoadDataFromDatabase(context.Background(), s.DataClient,
		[]expression.FeedbackTypeExpression{expression.MustParseFeedbackTypeExpression("FeedbackType")},
		nil, 0, 0, NewOnlineEvaluator(), nil)
	s.NoError(err)
	s.rankingTrainSet = dataSet

	// similar items (common users)
	s.Config.Recommend.ItemToItem = []config.ItemToItemConfig{{Name: "default", Type: "users"}}
	s.NoError(s.updateItemToItem(dataSet))
	similar, err := s.CacheClient.SearchScores(ctx, cache.ItemToItem, cache.Key("default", "9"), nil, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5", "3"}, cache.ConvertDocumentsToValues(similar))
	// similar items in category (common users)
	similar, err = s.CacheClient.SearchScores(ctx, cache.ItemToItem, cache.Key("default", "9"), []string{"*"}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5"}, cache.ConvertDocumentsToValues(similar))

	// similar items (common labels)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyItemTime, "8"), time.Now()))
	s.NoError(err)
	s.Config.Recommend.ItemToItem = []config.ItemToItemConfig{{Name: "default", Type: "tags", Column: "item.Labels"}}
	s.NoError(s.updateItemToItem(dataSet))
	similar, err = s.CacheClient.SearchScores(ctx, cache.ItemToItem, cache.Key("default", "8"), nil, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2", "4"}, cache.ConvertDocumentsToValues(similar))
	// similar items in category (common labels)
	similar, err = s.CacheClient.SearchScores(ctx, cache.ItemToItem, cache.Key("default", "8"), []string{"*"}, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2"}, cache.ConvertDocumentsToValues(similar))

	// similar items (auto)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyItemTime, "8"), time.Now()))
	s.NoError(err)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyItemTime, "9"), time.Now()))
	s.NoError(err)
	s.Config.Recommend.ItemToItem = []config.ItemToItemConfig{{Name: "default", Type: "auto"}}
	s.NoError(s.updateItemToItem(dataSet))
	similar, err = s.CacheClient.SearchScores(ctx, cache.ItemToItem, cache.Key("default", "8"), nil, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2", "4"}, cache.ConvertDocumentsToValues(similar))
	similar, err = s.CacheClient.SearchScores(ctx, cache.ItemToItem, cache.Key("default", "9"), nil, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5", "3"}, cache.ConvertDocumentsToValues(similar))
}

func (s *MasterTestSuite) TestUserToUser() {
	ctx := context.Background()
	// create config
	s.Config = &config.Config{}
	s.Config.Recommend.CacheSize = 3
	s.Config.Master.NumJobs = 4
	// collect similar
	users := []data.User{
		{UserId: "0", Labels: []string{"a", "b", "c", "d"}, Subscribe: nil, Comment: ""},
		{UserId: "1", Labels: []string{}, Subscribe: nil, Comment: ""},
		{UserId: "2", Labels: []string{"b", "c", "d"}, Subscribe: nil, Comment: ""},
		{UserId: "3", Labels: []string{}, Subscribe: nil, Comment: ""},
		{UserId: "4", Labels: []string{"b", "c"}, Subscribe: nil, Comment: ""},
		{UserId: "5", Labels: []string{}, Subscribe: nil, Comment: ""},
		{UserId: "6", Labels: []string{"c"}, Subscribe: nil, Comment: ""},
		{UserId: "7", Labels: []string{}, Subscribe: nil, Comment: ""},
		{UserId: "8", Labels: []string{"a", "b", "c", "d", "e"}, Subscribe: nil, Comment: ""},
		{UserId: "9", Labels: []string{}, Subscribe: nil, Comment: ""},
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
	_, dataSet, err := s.LoadDataFromDatabase(context.Background(), s.DataClient,
		[]expression.FeedbackTypeExpression{expression.MustParseFeedbackTypeExpression("FeedbackType")},
		nil, 0, 0, NewOnlineEvaluator(), nil)
	s.NoError(err)
	s.rankingTrainSet = dataSet

	// similar items (common users)
	s.Config.Recommend.UserToUser = []config.UserToUserConfig{{Name: "default", Type: "items"}}
	s.NoError(s.updateUserToUser(dataSet))
	similar, err := s.CacheClient.SearchScores(ctx, cache.UserToUser, cache.Key("default", "9"), nil, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5", "3"}, cache.ConvertDocumentsToValues(similar))

	// similar items (common labels)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, "8"), time.Now()))
	s.NoError(err)
	s.Config.Recommend.UserToUser = []config.UserToUserConfig{{Name: "default", Type: "tags", Column: "user.Labels"}}
	s.NoError(s.updateUserToUser(dataSet))
	similar, err = s.CacheClient.SearchScores(ctx, cache.UserToUser, cache.Key("default", "8"), nil, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2", "4"}, cache.ConvertDocumentsToValues(similar))

	// similar items (auto)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, "8"), time.Now()))
	s.NoError(err)
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.LastModifyUserTime, "9"), time.Now()))
	s.NoError(err)
	s.Config.Recommend.UserToUser = []config.UserToUserConfig{{Name: "default", Type: "auto"}}
	s.NoError(s.updateUserToUser(dataSet))
	similar, err = s.CacheClient.SearchScores(ctx, cache.UserToUser, cache.Key("default", "8"), nil, 0, 100)
	s.NoError(err)
	s.Equal([]string{"0", "2", "4"}, cache.ConvertDocumentsToValues(similar))
	similar, err = s.CacheClient.SearchScores(ctx, cache.UserToUser, cache.Key("default", "9"), nil, 0, 100)
	s.NoError(err)
	s.Equal([]string{"7", "5", "3"}, cache.ConvertDocumentsToValues(similar))
}

func (s *MasterTestSuite) TestLoadDataFromDatabase() {
	ctx := context.Background()
	// create config
	s.Config = &config.Config{}
	s.Config.Recommend.CacheSize = 3
	s.Config.Recommend.DataSource.PositiveFeedbackTypes = []expression.FeedbackTypeExpression{
		expression.MustParseFeedbackTypeExpression("positive")}
	s.Config.Recommend.DataSource.ReadFeedbackTypes = []expression.FeedbackTypeExpression{
		expression.MustParseFeedbackTypeExpression("negative")}
	s.Config.Master.NumJobs = runtime.NumCPU()

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
	_, _, err = s.loadDataset()
	s.NoError(err)
	s.Equal(11, s.rankingTrainSet.CountUsers())
	s.Equal(10, s.rankingTrainSet.CountItems())
	s.Equal(11, s.rankingTestSet.CountUsers())
	s.Equal(10, s.rankingTestSet.CountItems())
	s.Equal(55, s.rankingTrainSet.CountFeedback()+s.rankingTestSet.CountFeedback())
	s.Equal(11, s.clickTrainSet.CountUsers())
	s.Equal(10, s.clickTrainSet.CountItems())
	s.Equal(11, s.clickTestSet.CountUsers())
	s.Equal(10, s.clickTestSet.CountItems())
	s.Equal(int32(3), s.clickTrainSet.Index.CountItemLabels())
	s.Equal(int32(5), s.clickTrainSet.Index.CountUserLabels())
	s.Equal(int32(3), s.clickTestSet.Index.CountItemLabels())
	s.Equal(int32(5), s.clickTestSet.Index.CountUserLabels())
	s.Equal(90, s.clickTrainSet.Count()+s.clickTestSet.Count())
	s.Equal(45, s.clickTrainSet.PositiveCount+s.clickTestSet.PositiveCount)
	s.Equal(45, s.clickTrainSet.NegativeCount+s.clickTestSet.NegativeCount)

	// check latest items
	latest, err := s.CacheClient.SearchScores(ctx, cache.NonPersonalized, cache.Latest, []string{""}, 0, 100)
	s.NoError(err)
	s.Equal([]cache.Score{
		{Id: items[8].ItemId, Score: float64(items[8].Timestamp.Unix())},
		{Id: items[7].ItemId, Score: float64(items[7].Timestamp.Unix())},
		{Id: items[6].ItemId, Score: float64(items[6].Timestamp.Unix())},
	}, lo.Map(latest, func(document cache.Score, _ int) cache.Score {
		return cache.Score{Id: document.Id, Score: document.Score}
	}))
	latest, err = s.CacheClient.SearchScores(ctx, cache.NonPersonalized, cache.Latest, []string{"2"}, 0, 100)
	s.NoError(err)
	s.Equal([]cache.Score{
		{Id: items[8].ItemId, Score: float64(items[8].Timestamp.Unix())},
		{Id: items[5].ItemId, Score: float64(items[5].Timestamp.Unix())},
		{Id: items[2].ItemId, Score: float64(items[2].Timestamp.Unix())},
	}, lo.Map(latest, func(document cache.Score, _ int) cache.Score {
		return cache.Score{Id: document.Id, Score: document.Score}
	}))

	// check popular items
	popular, err := s.CacheClient.SearchScores(ctx, cache.NonPersonalized, cache.Popular, []string{""}, 0, 3)
	s.NoError(err)
	s.Equal([]cache.Score{
		{Id: items[8].ItemId, Score: 9},
		{Id: items[7].ItemId, Score: 8},
		{Id: items[6].ItemId, Score: 7},
	}, lo.Map(popular, func(document cache.Score, _ int) cache.Score {
		return cache.Score{Id: document.Id, Score: document.Score}
	}))
	popular, err = s.CacheClient.SearchScores(ctx, cache.NonPersonalized, cache.Popular, []string{"2"}, 0, 3)
	s.NoError(err)
	s.Equal([]cache.Score{
		{Id: items[8].ItemId, Score: 9},
		{Id: items[5].ItemId, Score: 6},
		{Id: items[2].ItemId, Score: 3},
	}, lo.Map(popular, func(document cache.Score, _ int) cache.Score {
		return cache.Score{Id: document.Id, Score: document.Score}
	}))

	// check categories
	categories, err := s.CacheClient.GetSet(ctx, cache.ItemCategories)
	s.NoError(err)
	s.Equal([]string{"0", "1", "2"}, categories)
}

func (s *MasterTestSuite) TestNonPersonalizedRecommend() {
	ctx := context.Background()
	// create config
	s.Config = &config.Config{}
	s.Config.Recommend.CacheSize = 3
	s.Config.Recommend.DataSource.PositiveFeedbackTypes = []expression.FeedbackTypeExpression{
		expression.MustParseFeedbackTypeExpression("positive")}
	s.Config.Recommend.DataSource.ReadFeedbackTypes = []expression.FeedbackTypeExpression{
		expression.MustParseFeedbackTypeExpression("negative")}
	s.Config.Master.NumJobs = runtime.NumCPU()

	// insert items
	var items []data.Item
	for i := 0; i < 10; i++ {
		items = append(items, data.Item{
			ItemId:    strconv.Itoa(i),
			Timestamp: time.Date(2000+i%2, 1, 1, i, 1, 0, 0, time.UTC),
		})
	}
	err := s.DataClient.BatchInsertItems(ctx, items)
	s.NoError(err)

	// insert users
	var users []data.User
	for i := 0; i < 10; i++ {
		users = append(users, data.User{
			UserId: strconv.Itoa(i),
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
		// item 8: user 0 ... user 8
		if i%2 == 0 {
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
		}
	}
	err = s.DataClient.BatchInsertFeedback(ctx, feedbacks, false, false, true)
	s.NoError(err)

	// load dataset
	_, _, err = s.loadDataset()
	s.NoError(err)

	// check latest items
	latest, err := s.CacheClient.SearchScores(ctx, cache.NonPersonalized, cache.Latest, []string{""}, 0, 3)
	s.NoError(err)
	s.Equal([]cache.Score{
		{Id: items[9].ItemId, Score: float64(items[9].Timestamp.Unix())},
		{Id: items[7].ItemId, Score: float64(items[7].Timestamp.Unix())},
		{Id: items[5].ItemId, Score: float64(items[5].Timestamp.Unix())},
	}, lo.Map(latest, func(document cache.Score, _ int) cache.Score {
		return cache.Score{Id: document.Id, Score: document.Score}
	}))

	// check popular items
	popular, err := s.CacheClient.SearchScores(ctx, cache.NonPersonalized, cache.Popular, []string{""}, 0, 3)
	s.NoError(err)
	s.Equal([]cache.Score{
		{Id: items[8].ItemId, Score: 9},
		{Id: items[6].ItemId, Score: 7},
		{Id: items[4].ItemId, Score: 5},
	}, lo.Map(popular, func(document cache.Score, _ int) cache.Score {
		return cache.Score{Id: document.Id, Score: document.Score}
	}))
}

func (s *MasterTestSuite) TestNeedUpdateItemToItem() {
	s.Config = config.GetDefaultConfig()
	recommendConfig := config.ItemToItemConfig{Name: "default"}
	ctx := context.Background()

	// empty cache
	s.True(s.needUpdateItemToItem("1", recommendConfig))
	err := s.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "1"), []cache.Score{
		{Id: "2", Score: 1, Categories: []string{""}},
		{Id: "3", Score: 2, Categories: []string{""}},
		{Id: "4", Score: 3, Categories: []string{""}},
	})
	s.NoError(err)

	// digest mismatch
	err = s.CacheClient.Set(ctx, cache.String(cache.Key(cache.ItemToItemDigest, "default", "1"), "digest"))
	s.NoError(err)
	s.True(s.needUpdateItemToItem("1", recommendConfig))

	// staled cache
	err = s.CacheClient.Set(ctx, cache.String(cache.Key(cache.ItemToItemDigest, "default", "1"), recommendConfig.Hash()))
	s.NoError(err)
	s.True(s.needUpdateItemToItem("1", recommendConfig))
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.ItemToItemUpdateTime, "default", "1"), time.Now().Add(-s.Config.Recommend.CacheExpire)))
	s.NoError(err)
	s.True(s.needUpdateItemToItem("1", recommendConfig))

	// not staled cache
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.ItemToItemUpdateTime, "default", "1"), time.Now()))
	s.NoError(err)
	s.False(s.needUpdateItemToItem("1", recommendConfig))
}

func (s *MasterTestSuite) TestNeedUpdateUserToUser() {
	ctx := context.Background()
	s.Config = config.GetDefaultConfig()
	recommendConfig := config.UserToUserConfig{Name: "default"}

	// empty cache
	s.True(s.needUpdateUserToUser("1", recommendConfig))
	err := s.CacheClient.AddScores(ctx, cache.UserToUser, cache.Key("default", "1"), []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}},
		{Id: "2", Score: 2, Categories: []string{""}},
		{Id: "3", Score: 3, Categories: []string{""}},
	})
	s.NoError(err)

	// digest mismatch
	err = s.CacheClient.Set(ctx, cache.String(cache.Key(cache.UserToUserDigest, "default", "1"), "digest"))
	s.NoError(err)
	s.True(s.needUpdateUserToUser("1", recommendConfig))

	// staled cache
	err = s.CacheClient.Set(ctx, cache.String(cache.Key(cache.UserToUserDigest, "default", "1"), recommendConfig.Hash()))
	s.NoError(err)
	s.True(s.needUpdateUserToUser("1", recommendConfig))
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.UserToUserUpdateTime, "default", "1"), time.Now().Add(-s.Config.Recommend.CacheExpire)))
	s.NoError(err)
	s.True(s.needUpdateUserToUser("1", recommendConfig))

	// not staled cache
	err = s.CacheClient.Set(ctx, cache.Time(cache.Key(cache.UserToUserUpdateTime, "default", "1"), time.Now()))
	s.NoError(err)
	s.False(s.needUpdateUserToUser("1", recommendConfig))
}

func (s *MasterTestSuite) TestGarbageCollection() {
	// create config
	s.Config = &config.Config{}
	s.Config.Master.NumJobs = 1
	s.Config.Recommend.NonPersonalized = []config.NonPersonalizedConfig{{Name: "custom", Score: "1"}}
	s.Config.Recommend.ItemToItem = []config.ItemToItemConfig{{Name: "default", Type: "users"}}
	s.Config.Recommend.UserToUser = []config.UserToUserConfig{{Name: "default", Type: "items"}}

	// insert items
	ctx := context.Background()
	err := s.DataClient.BatchInsertItems(ctx, []data.Item{
		{ItemId: "1", Timestamp: time.Now(), Categories: []string{"*"}, Labels: []string{"a", "b", "c", "d"}, Comment: ""},
		{ItemId: "2", Timestamp: time.Now(), Categories: []string{"*"}, Labels: []string{}, Comment: ""},
	})
	s.NoError(err)

	// insert users
	err = s.DataClient.BatchInsertUsers(ctx, []data.User{
		{UserId: "1", Labels: []string{"a", "b", "c", "d"}, Subscribe: nil, Comment: ""},
		{UserId: "2", Labels: []string{}, Subscribe: nil, Comment: ""},
	})
	s.NoError(err)

	// insert non-personalized cache
	timestamp := time.Now().Add(time.Hour)
	err = s.CacheClient.AddScores(ctx, cache.NonPersonalized, cache.Latest, []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}, Timestamp: timestamp},
		{Id: "2", Score: 2, Categories: []string{""}, Timestamp: timestamp},
	})
	s.NoError(err)
	err = s.CacheClient.AddScores(ctx, cache.NonPersonalized, "custom", []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}, Timestamp: timestamp},
		{Id: "2", Score: 2, Categories: []string{""}, Timestamp: timestamp},
	})
	s.NoError(err)
	err = s.CacheClient.AddScores(ctx, cache.NonPersonalized, "unknown", []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}, Timestamp: timestamp},
		{Id: "2", Score: 2, Categories: []string{""}, Timestamp: timestamp},
	})
	s.NoError(err)

	// insert item-to-item cache
	err = s.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "1"), []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}},
		{Id: "2", Score: 2, Categories: []string{""}},
	})
	s.NoError(err)
	err = s.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "3"), []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}},
		{Id: "2", Score: 2, Categories: []string{""}},
	})
	s.NoError(err)
	err = s.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("unknown", "1"), []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}},
		{Id: "2", Score: 2, Categories: []string{""}},
	})
	s.NoError(err)

	// insert user-to-user cache
	err = s.CacheClient.AddScores(ctx, cache.UserToUser, cache.Key("default", "1"), []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}},
		{Id: "2", Score: 2, Categories: []string{""}},
	})
	s.NoError(err)
	err = s.CacheClient.AddScores(ctx, cache.UserToUser, cache.Key("default", "3"), []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}},
		{Id: "2", Score: 2, Categories: []string{""}},
	})
	s.NoError(err)
	err = s.CacheClient.AddScores(ctx, cache.UserToUser, cache.Key("unknown", "1"), []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}},
		{Id: "2", Score: 2, Categories: []string{""}},
	})
	s.NoError(err)

	// insert collaborative filtering cache
	err = s.CacheClient.AddScores(ctx, cache.CollaborativeFiltering, "1", []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}},
		{Id: "2", Score: 2, Categories: []string{""}},
	})
	s.NoError(err)
	err = s.CacheClient.AddScores(ctx, cache.CollaborativeFiltering, "3", []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}},
		{Id: "2", Score: 2, Categories: []string{""}},
	})
	s.NoError(err)

	// load dataset and run garbage collection
	_, dataSet, err := s.loadDataset()
	s.NoError(err)
	err = s.collectGarbage(context.Background(), dataSet)
	s.NoError(err)

	// check non-personalized cache
	np, err := s.CacheClient.SearchScores(ctx, cache.NonPersonalized, cache.Latest, nil, 0, 100)
	s.NoError(err)
	s.Equal([]string{"2", "1"}, cache.ConvertDocumentsToValues(np))
	np, err = s.CacheClient.SearchScores(ctx, cache.NonPersonalized, "custom", nil, 0, 100)
	s.NoError(err)
	s.Equal([]string{"2", "1"}, cache.ConvertDocumentsToValues(np))
	np, err = s.CacheClient.SearchScores(ctx, cache.NonPersonalized, "unknown", nil, 0, 100)
	s.NoError(err)
	s.Empty(np)

	// check item-to-item cache
	similar, err := s.CacheClient.SearchScores(ctx, cache.ItemToItem, cache.Key("default", "1"), nil, 0, 100)
	s.NoError(err)
	s.Equal([]string{"2", "1"}, cache.ConvertDocumentsToValues(similar))
	similar, err = s.CacheClient.SearchScores(ctx, cache.ItemToItem, cache.Key("default", "3"), nil, 0, 100)
	s.NoError(err)
	s.Empty(similar)
	similar, err = s.CacheClient.SearchScores(ctx, cache.ItemToItem, cache.Key("unknown", "1"), nil, 0, 100)
	s.NoError(err)
	s.Empty(similar)

	// check user-to-user cache
	similar, err = s.CacheClient.SearchScores(ctx, cache.UserToUser, cache.Key("default", "1"), nil, 0, 100)
	s.NoError(err)
	s.Equal([]string{"2", "1"}, cache.ConvertDocumentsToValues(similar))
	similar, err = s.CacheClient.SearchScores(ctx, cache.UserToUser, cache.Key("default", "3"), nil, 0, 100)
	s.NoError(err)
	s.Empty(similar)
	similar, err = s.CacheClient.SearchScores(ctx, cache.UserToUser, cache.Key("unknown", "1"), nil, 0, 100)
	s.NoError(err)
	s.Empty(similar)

	// check collaborative filtering cache
	cf, err := s.CacheClient.SearchScores(ctx, cache.CollaborativeFiltering, "1", nil, 0, 100)
	s.NoError(err)
	s.Equal([]string{"2", "1"}, cache.ConvertDocumentsToValues(cf))
	cf, err = s.CacheClient.SearchScores(ctx, cache.CollaborativeFiltering, "3", nil, 0, 100)
	s.NoError(err)
	s.Empty(cf)
}
