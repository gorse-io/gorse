// Copyright 2025 gorse Project Authors
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

package logics

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorse-io/gorse/common/expression"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/suite"
)

type RecommenderTestSuite struct {
	suite.Suite
	dataClient  data.Database
	cacheClient cache.Database
}

func (suite *RecommenderTestSuite) SetupSuite() {
	var err error
	// open database
	suite.dataClient, err = data.Open(fmt.Sprintf("sqlite://%s/data.db", suite.T().TempDir()), "")
	suite.NoError(err)
	suite.cacheClient, err = cache.Open(fmt.Sprintf("sqlite://%s/cache.db", suite.T().TempDir()), "")
	suite.NoError(err)
	// init database
	err = suite.dataClient.Init()
	suite.NoError(err)
	err = suite.cacheClient.Init()
	suite.NoError(err)
}

func (suite *RecommenderTestSuite) TearDownSuite() {
	err := suite.dataClient.Close()
	suite.NoError(err)
	err = suite.cacheClient.Close()
	suite.NoError(err)
}

func (suite *RecommenderTestSuite) SetupTest() {
	err := suite.dataClient.Purge()
	suite.NoError(err)
	err = suite.cacheClient.Purge()
	suite.NoError(err)
}

func (suite *RecommenderTestSuite) TestLatest() {
	items := make([]data.Item, 20)
	for i := 0; i < 20; i++ {
		items[i] = data.Item{
			ItemId:    fmt.Sprintf("item_%d", i),
			Timestamp: time.Unix(int64(i), 0),
		}
		if i%2 == 0 {
			items[i].Categories = []string{"cat_1"}
		}
	}
	err := suite.dataClient.BatchInsertItems(suite.T().Context(), items)
	suite.NoError(err)

	feedback := make([]data.Feedback, 10)
	for i := 0; i < 10; i++ {
		feedback[i] = data.Feedback{
			FeedbackKey: data.FeedbackKey{
				FeedbackType: "click",
				UserId:       "user_1",
				ItemId:       fmt.Sprintf("item_%d", i),
			},
		}
	}
	err = suite.dataClient.BatchInsertFeedback(suite.T().Context(), feedback, true, true, false)
	suite.NoError(err)

	recommender, err := NewRecommender(config.RecommendConfig{}, suite.cacheClient, suite.dataClient, true, "user_1", nil)
	suite.NoError(err)
	scores, digest, err := recommender.recommendLatest(suite.T().Context())
	suite.NoError(err)
	suite.Equal("latest", digest)
	if suite.Equal(10, len(scores)) {
		for i := 0; i < 10; i++ {
			suite.Equal(fmt.Sprintf("item_%d", 19-i), scores[i].Id)
			suite.Equal(float64(19-i), scores[i].Score)
		}
	}

	recommender, err = NewRecommender(config.RecommendConfig{}, suite.cacheClient, suite.dataClient, true, "user_1", []string{"cat_1"})
	suite.NoError(err)
	scores, digest, err = recommender.recommendLatest(suite.T().Context())
	suite.NoError(err)
	suite.Equal("latest", digest)
	if suite.Equal(5, len(scores)) {
		for i := 0; i < 5; i++ {
			suite.Equal(fmt.Sprintf("item_%d", 18-2*i), scores[i].Id)
			suite.Equal(float64(18-2*i), scores[i].Score)
		}
	}
}

func (suite *RecommenderTestSuite) TestCollaborative() {
	recommends := make([]cache.Score, 20)
	items := make([]data.Item, 20)
	for i := 0; i < 20; i++ {
		recommends[i] = cache.Score{
			Id:    fmt.Sprintf("item_%d", i),
			Score: float64(i),
		}
		items[i] = data.Item{
			ItemId: fmt.Sprintf("item_%d", i),
		}
		if i%2 == 0 {
			recommends[i].Categories = []string{"cat_1"}
			items[i].Categories = []string{"cat_1"}
		}
	}
	err := suite.dataClient.BatchInsertItems(suite.T().Context(), items)
	suite.NoError(err)
	err = suite.cacheClient.AddScores(suite.T().Context(), cache.CollaborativeFiltering, "user_1", recommends)
	suite.NoError(err)
	err = suite.cacheClient.Set(suite.T().Context(), cache.String(cache.Key(cache.CollaborativeFilteringDigest, "user_1"), "digest"))
	suite.NoError(err)

	feedback := make([]data.Feedback, 10)
	for i := 0; i < 10; i++ {
		feedback[i] = data.Feedback{
			FeedbackKey: data.FeedbackKey{
				FeedbackType: "click",
				UserId:       "user_1",
				ItemId:       fmt.Sprintf("item_%d", i),
			},
		}
	}
	err = suite.dataClient.BatchInsertFeedback(suite.T().Context(), feedback, true, true, false)
	suite.NoError(err)

	recommender, err := NewRecommender(config.RecommendConfig{}, suite.cacheClient, suite.dataClient, true, "user_1", nil)
	suite.NoError(err)
	scores, digest, err := recommender.recommendCollaborative(suite.T().Context())
	suite.NoError(err)
	suite.Equal("digest", digest)
	if suite.Equal(10, len(scores)) {
		for i := 0; i < 10; i++ {
			suite.Equal(fmt.Sprintf("item_%d", 19-i), scores[i].Id)
			suite.Equal(float64(19-i), scores[i].Score)
		}
	}

	recommender, err = NewRecommender(config.RecommendConfig{}, suite.cacheClient, suite.dataClient, true, "user_1", []string{"cat_1"})
	suite.NoError(err)
	scores, digest, err = recommender.recommendCollaborative(suite.T().Context())
	suite.NoError(err)
	suite.Equal("digest", digest)
	if suite.Equal(5, len(scores)) {
		for i := 0; i < 5; i++ {
			suite.Equal(fmt.Sprintf("item_%d", 18-2*i), scores[i].Id)
			suite.Equal(float64(18-2*i), scores[i].Score)
		}
	}
}

func (suite *RecommenderTestSuite) TestNonPersonalized() {
	recommends := make([]cache.Score, 20)
	for i := 0; i < 20; i++ {
		recommends[i] = cache.Score{
			Id:    fmt.Sprintf("item_%d", i),
			Score: float64(i),
		}
		if i%2 == 0 {
			recommends[i].Categories = []string{"", "cat_1"}
		} else {
			recommends[i].Categories = []string{""}
		}
	}
	err := suite.cacheClient.AddScores(suite.T().Context(), cache.NonPersonalized, "a", recommends)
	suite.NoError(err)
	err = suite.cacheClient.Set(suite.T().Context(), cache.String(cache.Key(cache.NonPersonalizedDigest, "a"), "digest"))
	suite.NoError(err)

	feedback := make([]data.Feedback, 10)
	for i := 0; i < 10; i++ {
		feedback[i] = data.Feedback{
			FeedbackKey: data.FeedbackKey{
				FeedbackType: "click",
				UserId:       "user_1",
				ItemId:       fmt.Sprintf("item_%d", i),
			},
		}
	}
	err = suite.dataClient.BatchInsertFeedback(suite.T().Context(), feedback, true, true, false)
	suite.NoError(err)

	recommender, err := NewRecommender(config.RecommendConfig{}, suite.cacheClient, suite.dataClient, true, "user_1", nil)
	suite.NoError(err)
	recommendFunc := recommender.recommendNonPersonalized("a")
	scores, digest, err := recommendFunc(suite.T().Context())
	suite.NoError(err)
	suite.Equal("digest", digest)
	if suite.Equal(10, len(scores)) {
		for i := 0; i < 10; i++ {
			suite.Equal(fmt.Sprintf("item_%d", 19-i), scores[i].Id)
			suite.Equal(float64(19-i), scores[i].Score)
		}
	}

	recommender, err = NewRecommender(config.RecommendConfig{}, suite.cacheClient, suite.dataClient, true, "user_1", []string{"cat_1"})
	suite.NoError(err)
	recommendFunc = recommender.recommendNonPersonalized("a")
	scores, digest, err = recommendFunc(suite.T().Context())
	suite.NoError(err)
	suite.Equal("digest", digest)
	if suite.Equal(5, len(scores)) {
		for i := 0; i < 5; i++ {
			suite.Equal(fmt.Sprintf("item_%d", 18-2*i), scores[i].Id)
			suite.Equal(float64(18-2*i), scores[i].Score)
		}
	}
}

func (suite *RecommenderTestSuite) TestRecommendFallbackNonPersonalizedSkipsExpiredItems() {
	err := suite.cacheClient.AddScores(suite.T().Context(), cache.NonPersonalized, "ttl", []cache.Score{
		{Id: "non_personalized_expired", Score: 5, Categories: []string{""}},
		{Id: "non_personalized_valid_1", Score: 4, Categories: []string{""}},
		{Id: "non_personalized_valid_2", Score: 3, Categories: []string{""}},
	})
	suite.NoError(err)
	err = suite.cacheClient.Set(suite.T().Context(), cache.String(cache.Key(cache.NonPersonalizedDigest, "ttl"), "digest"))
	suite.NoError(err)
	now := time.Now()
	err = suite.dataClient.BatchInsertItems(suite.T().Context(), []data.Item{
		{ItemId: "non_personalized_expired", Timestamp: now.Add(-48 * time.Hour)},
		{ItemId: "non_personalized_valid_1", Timestamp: now.Add(-time.Hour)},
		{ItemId: "non_personalized_valid_2", Timestamp: now.Add(-2 * time.Hour)},
	})
	suite.NoError(err)

	cfg := config.RecommendConfig{
		CacheSize: 2,
		DataSource: config.DataSourceConfig{
			ItemTTL: 1,
		},
		Ranker: config.RankerConfig{
			Type: "fm",
		},
		Fallback: config.FallbackConfig{
			Recommenders: []string{"non-personalized/ttl"},
		},
	}
	recommender, err := NewRecommender(cfg, suite.cacheClient, suite.dataClient, true, "non_personalized_user", nil)
	suite.NoError(err)
	scores, err := recommender.Recommend(suite.T().Context(), 2)
	suite.NoError(err)
	suite.Equal([]cache.Score{
		{Id: "non_personalized_valid_1", Score: 4, Categories: []string{""}},
		{Id: "non_personalized_valid_2", Score: 3, Categories: []string{""}},
	}, scores)
}

func (suite *RecommenderTestSuite) TestRecommendFallbackItemToItemSkipsExpiredItems() {
	err := suite.cacheClient.AddScores(suite.T().Context(), cache.ItemToItem, cache.Key("default", "item_to_item_seed"), []cache.Score{
		{Id: "item_to_item_expired", Score: 5},
		{Id: "item_to_item_valid_1", Score: 4},
		{Id: "item_to_item_valid_2", Score: 3},
	})
	suite.NoError(err)
	err = suite.cacheClient.Set(suite.T().Context(), cache.String(cache.Key(cache.ItemToItemDigest, "default", "item_to_item_seed"), "digest"))
	suite.NoError(err)
	err = suite.dataClient.BatchInsertFeedback(suite.T().Context(), []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "item_to_item_user", ItemId: "item_to_item_seed"}},
	}, true, true, false)
	suite.NoError(err)
	now := time.Now()
	err = suite.dataClient.BatchInsertItems(suite.T().Context(), []data.Item{
		{ItemId: "item_to_item_expired", Timestamp: now.Add(-48 * time.Hour)},
		{ItemId: "item_to_item_valid_1", Timestamp: now.Add(-time.Hour)},
		{ItemId: "item_to_item_valid_2", Timestamp: now.Add(-2 * time.Hour)},
	})
	suite.NoError(err)

	cfg := config.RecommendConfig{
		CacheSize:   2,
		ContextSize: 10,
		DataSource: config.DataSourceConfig{
			ItemTTL:               1,
			PositiveFeedbackTypes: []expression.FeedbackTypeExpression{expression.MustParseFeedbackTypeExpression("click")},
		},
		Ranker: config.RankerConfig{
			Type: "fm",
		},
		Fallback: config.FallbackConfig{
			Recommenders: []string{"item-to-item/default"},
		},
	}
	recommender, err := NewRecommender(cfg, suite.cacheClient, suite.dataClient, true, "item_to_item_user", nil)
	suite.NoError(err)
	scores, err := recommender.Recommend(suite.T().Context(), 2)
	suite.NoError(err)
	suite.Equal([]cache.Score{
		{Id: "item_to_item_valid_1", Score: 4},
		{Id: "item_to_item_valid_2", Score: 3},
	}, scores)
}

func (suite *RecommenderTestSuite) TestRecommendFallbackLatestSkipsExpiredItems() {
	now := time.Now()
	err := suite.cacheClient.AddScores(suite.T().Context(), cache.Recommend, "ttl_user", []cache.Score{
		{Id: "ttl_rec_expired", Score: 10},
		{Id: "ttl_rec_valid", Score: 9},
		{Id: "ttl_rec_valid_2", Score: 8},
	})
	suite.NoError(err)
	err = suite.dataClient.BatchInsertItems(suite.T().Context(), []data.Item{
		{ItemId: "ttl_rec_expired", Timestamp: now.Add(-48 * time.Hour)},
		{ItemId: "ttl_rec_valid", Timestamp: now.Add(-time.Hour)},
		{ItemId: "ttl_rec_valid_2", Timestamp: now.Add(-2 * time.Hour)},
		{ItemId: "ttl_latest_valid", Timestamp: now.Add(time.Hour)},
		{ItemId: "ttl_latest_expired", Timestamp: now.Add(-47 * time.Hour)},
	})
	suite.NoError(err)

	cfg := config.RecommendConfig{
		CacheSize: 2,
		DataSource: config.DataSourceConfig{
			ItemTTL: 1,
		},
		Ranker: config.RankerConfig{
			Type: "fm",
		},
		Fallback: config.FallbackConfig{
			Recommenders: []string{"latest"},
		},
	}
	recommender, err := NewRecommender(cfg, suite.cacheClient, suite.dataClient, true, "ttl_user", nil)
	suite.NoError(err)
	scores, err := recommender.Recommend(suite.T().Context(), 3)
	suite.NoError(err)
	suite.Equal([]cache.Score{
		{Id: "ttl_rec_valid", Score: 9},
		{Id: "ttl_rec_valid_2", Score: 8},
		{Id: "ttl_latest_valid", Score: float64(now.Add(time.Hour).Unix())},
	}, scores)
}

func (suite *RecommenderTestSuite) TestRecommendFallbackUserToUserSkipsHiddenItems() {
	err := suite.cacheClient.AddScores(suite.T().Context(), cache.Recommend, "user_to_user_target", []cache.Score{
		{Id: "user_to_user_base_1", Score: 99},
		{Id: "user_to_user_base_2", Score: 98},
		{Id: "user_to_user_base_3", Score: 97},
		{Id: "user_to_user_base_4", Score: 96},
	})
	suite.NoError(err)
	err = suite.cacheClient.AddScores(suite.T().Context(), cache.UserToUser, cache.Key("default", "user_to_user_target"), []cache.Score{
		{Id: "similar_user_1", Score: 2},
		{Id: "similar_user_2", Score: 1.5},
		{Id: "similar_user_3", Score: 1},
	})
	suite.NoError(err)
	err = suite.cacheClient.Set(suite.T().Context(), cache.String(cache.Key(cache.UserToUserDigest, "default", "user_to_user_target"), "digest"))
	suite.NoError(err)
	err = suite.dataClient.BatchInsertFeedback(suite.T().Context(), []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "user_to_user_target", ItemId: "user_to_user_base_1"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "user_to_user_target", ItemId: "user_to_user_base_2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "user_to_user_target", ItemId: "user_to_user_base_3"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "user_to_user_target", ItemId: "user_to_user_base_4"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "similar_user_1", ItemId: "user_to_user_item_11"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "similar_user_2", ItemId: "user_to_user_item_12"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "similar_user_2", ItemId: "user_to_user_item_48"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "similar_user_3", ItemId: "user_to_user_item_13"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "similar_user_3", ItemId: "user_to_user_item_48"}},
	}, true, true, false)
	suite.NoError(err)
	err = suite.dataClient.BatchInsertItems(suite.T().Context(), []data.Item{
		{ItemId: "user_to_user_item_48", IsHidden: true},
	})
	suite.NoError(err)

	cfg := config.RecommendConfig{
		CacheSize: 10,
		UserToUser: []config.UserToUserConfig{{
			Name: "default",
		}},
		Ranker: config.RankerConfig{
			Type: "fm",
		},
		Fallback: config.FallbackConfig{
			Recommenders: []string{"user-to-user/default"},
		},
	}
	recommender, err := NewRecommender(cfg, suite.cacheClient, suite.dataClient, true, "user_to_user_target", nil)
	suite.NoError(err)
	scores, err := recommender.Recommend(suite.T().Context(), 3)
	suite.NoError(err)
	suite.Equal([]cache.Score{
		{Id: "user_to_user_item_11", Score: 2},
		{Id: "user_to_user_item_12", Score: 1.5},
		{Id: "user_to_user_item_13", Score: 1},
	}, scores)
}

func (suite *RecommenderTestSuite) TestRecommendFallbackCollaborativeSkipsHiddenItems() {
	err := suite.cacheClient.AddScores(suite.T().Context(), cache.Recommend, "collaborative_target", []cache.Score{
		{Id: "collaborative_base_1", Score: 99},
		{Id: "collaborative_base_2", Score: 98},
		{Id: "collaborative_base_3", Score: 97},
		{Id: "collaborative_base_4", Score: 96},
	})
	suite.NoError(err)
	err = suite.cacheClient.AddScores(suite.T().Context(), cache.CollaborativeFiltering, "collaborative_target", []cache.Score{
		{Id: "collaborative_item_13", Score: 79},
		{Id: "collaborative_item_14", Score: 78},
		{Id: "collaborative_item_15", Score: 77},
		{Id: "collaborative_item_16", Score: 76},
	})
	suite.NoError(err)
	err = suite.cacheClient.Set(suite.T().Context(), cache.String(cache.Key(cache.CollaborativeFilteringDigest, "collaborative_target"), "digest"))
	suite.NoError(err)
	err = suite.dataClient.BatchInsertFeedback(suite.T().Context(), []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "collaborative_target", ItemId: "collaborative_base_1"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "collaborative_target", ItemId: "collaborative_base_2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "collaborative_target", ItemId: "collaborative_base_3"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "collaborative_target", ItemId: "collaborative_base_4"}},
	}, true, true, false)
	suite.NoError(err)
	err = suite.dataClient.BatchInsertItems(suite.T().Context(), []data.Item{
		{ItemId: "collaborative_item_13", IsHidden: true},
		{ItemId: "collaborative_item_14"},
		{ItemId: "collaborative_item_15"},
		{ItemId: "collaborative_item_16"},
	})
	suite.NoError(err)

	cfg := config.RecommendConfig{
		CacheSize: 3,
		Ranker: config.RankerConfig{
			Type: "fm",
		},
		Fallback: config.FallbackConfig{
			Recommenders: []string{"collaborative"},
		},
	}
	recommender, err := NewRecommender(cfg, suite.cacheClient, suite.dataClient, true, "collaborative_target", nil)
	suite.NoError(err)
	scores, err := recommender.Recommend(suite.T().Context(), 3)
	suite.NoError(err)
	suite.Equal([]cache.Score{
		{Id: "collaborative_item_14", Score: 78},
		{Id: "collaborative_item_15", Score: 77},
		{Id: "collaborative_item_16", Score: 76},
	}, scores)
}

func (suite *RecommenderTestSuite) TestExternal() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userId := r.URL.Query().Get("user_id")
		if userId == "user_1" {
			fmt.Fprintln(w, `["item_1", "item_2", "item_3", "item_100", "item_200", "item_300"]`)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	feedback := make([]data.Feedback, 10)
	for i := 0; i < 10; i++ {
		feedback[i] = data.Feedback{
			FeedbackKey: data.FeedbackKey{
				FeedbackType: "click",
				UserId:       "user_1",
				ItemId:       fmt.Sprintf("item_%d", i),
			},
		}
	}
	err := suite.dataClient.BatchInsertFeedback(suite.T().Context(), feedback, true, true, false)
	suite.NoError(err)

	cfg := config.RecommendConfig{
		DataSource: config.DataSourceConfig{
			ItemTTL: 1,
		},
		External: []config.ExternalConfig{{
			Script: fmt.Sprintf(`fetch("%s?user_id=user_1").body`, ts.URL),
			Name:   "test",
		}},
	}
	err = suite.dataClient.BatchInsertItems(suite.T().Context(), []data.Item{
		{ItemId: "item_200", IsHidden: true},
		{ItemId: "item_300", Timestamp: time.Now().Add(-48 * time.Hour)},
	})
	suite.NoError(err)
	recommender, err := NewRecommender(cfg, suite.cacheClient, suite.dataClient, true, "user_1", nil)
	suite.NoError(err)
	recommendFunc := recommender.recommendExternal("test")
	scores, digest, err := recommendFunc(suite.T().Context())
	suite.NoError(err)
	suite.Equal(cfg.External[0].Hash(), digest)
	suite.Equal([]cache.Score{
		{Id: "item_100", Score: 0},
	}, scores)
}

func TestRecommenderTestSuite(t *testing.T) {
	suite.Run(t, new(RecommenderTestSuite))
}
