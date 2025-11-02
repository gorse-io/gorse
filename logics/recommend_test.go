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
	"context"
	"fmt"
	"testing"
	"time"

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
	err := suite.dataClient.BatchInsertItems(context.Background(), items)
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
	err = suite.dataClient.BatchInsertFeedback(context.Background(), feedback, true, true, false)
	suite.NoError(err)

	recommender, err := NewRecommender(config.RecommendConfig{}, suite.cacheClient, suite.dataClient, true, "user_1", nil)
	suite.NoError(err)
	scores, err := recommender.recommendLatest(context.Background())
	suite.NoError(err)
	if suite.Equal(10, len(scores)) {
		for i := 0; i < 10; i++ {
			suite.Equal(fmt.Sprintf("item_%d", 19-i), scores[i].Id)
			suite.Equal(float64(19-i), scores[i].Score)
		}
	}

	recommender, err = NewRecommender(config.RecommendConfig{}, suite.cacheClient, suite.dataClient, true, "user_1", []string{"cat_1"})
	suite.NoError(err)
	scores, err = recommender.recommendLatest(context.Background())
	suite.NoError(err)
	if suite.Equal(5, len(scores)) {
		for i := 0; i < 5; i++ {
			suite.Equal(fmt.Sprintf("item_%d", 18-2*i), scores[i].Id)
			suite.Equal(float64(18-2*i), scores[i].Score)
		}
	}
}

func (suite *RecommenderTestSuite) TestCollaborative() {
	recommends := make([]cache.Score, 20)
	for i := 0; i < 20; i++ {
		recommends[i] = cache.Score{
			Id:    fmt.Sprintf("item_%d", i),
			Score: float64(i),
		}
		if i%2 == 0 {
			recommends[i].Categories = []string{"cat_1"}
		}
	}
	err := suite.cacheClient.AddScores(context.Background(), cache.CollaborativeFiltering, "user_1", recommends)
	suite.NoError(err)

	exclude := make([]string, 10)
	for i := 0; i < 10; i++ {
		exclude[i] = fmt.Sprintf("item_%d", i)
	}

	recommender, err := NewRecommender(config.RecommendConfig{}, suite.cacheClient, suite.dataClient, true, "user_1", nil)
	suite.NoError(err)
	scores, err := recommender.recommendCollaborative(context.Background())
	suite.NoError(err)
	if suite.Equal(10, len(scores)) {
		for i := 0; i < 10; i++ {
			suite.Equal(fmt.Sprintf("item_%d", 19-i), scores[i].Id)
			suite.Equal(float64(19-i), scores[i].Score)
		}
	}

	recommender, err = NewRecommender(config.RecommendConfig{}, suite.cacheClient, suite.dataClient, true, "user_1", []string{"cat_1"})
	suite.NoError(err)
	scores, err = recommender.recommendCollaborative(context.Background())
	suite.NoError(err)
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
	err := suite.cacheClient.AddScores(context.Background(), cache.NonPersonalized, "a", recommends)
	suite.NoError(err)

	exclude := make([]string, 10)
	for i := 0; i < 10; i++ {
		exclude[i] = fmt.Sprintf("item_%d", i)
	}

	recommender, err := NewRecommender(config.RecommendConfig{}, suite.cacheClient, suite.dataClient, true, "user_1", nil)
	suite.NoError(err)
	recommendFunc := recommender.recommendNonPersonalized("a")
	scores, err := recommendFunc(context.Background())
	suite.NoError(err)
	if suite.Equal(10, len(scores)) {
		for i := 0; i < 10; i++ {
			suite.Equal(fmt.Sprintf("item_%d", 19-i), scores[i].Id)
			suite.Equal(float64(19-i), scores[i].Score)
		}
	}

	recommender, err = NewRecommender(config.RecommendConfig{}, suite.cacheClient, suite.dataClient, true, "user_1", []string{"cat_1"})
	suite.NoError(err)
	recommendFunc = recommender.recommendNonPersonalized("a")
	scores, err = recommendFunc(context.Background())
	suite.NoError(err)
	if suite.Equal(5, len(scores)) {
		for i := 0; i < 5; i++ {
			suite.Equal(fmt.Sprintf("item_%d", 18-2*i), scores[i].Id)
			suite.Equal(float64(18-2*i), scores[i].Score)
		}
	}
}

func (suite *RecommenderTestSuite) TestItemToItem() {

}

func (suite *RecommenderTestSuite) TestUserToUser() {

}

func TestRecommenderTestSuite(t *testing.T) {
	suite.Run(t, new(RecommenderTestSuite))
}
