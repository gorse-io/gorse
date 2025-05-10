//go:build integrate_test

// Copyright 2022 gorse Project Authors
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

package client

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	client "github.com/gorse-io/gorse-go"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"github.com/zhenghaoz/gorse/storage/cache"
)

const (
	RedisEndpoint = "redis://127.0.0.1:6379/0"
	GorseEndpoint = "http://127.0.0.1:8087"
	GorseApiKey   = ""
)

type GorseClientTestSuite struct {
	suite.Suite
	client *client.GorseClient
	redis  *redis.Client
}

func (suite *GorseClientTestSuite) SetupSuite() {
	suite.client = client.NewGorseClient(GorseEndpoint, GorseApiKey)
	options, err := redis.ParseURL(RedisEndpoint)
	suite.NoError(err)
	suite.redis = redis.NewClient(options)
}

func (suite *GorseClientTestSuite) TearDownSuite() {
	err := suite.redis.Close()
	suite.NoError(err)
}

func (suite *GorseClientTestSuite) TestFeedback() {
	ctx := context.TODO()
	timestamp := time.Unix(1660459054, 0).UTC().Format(time.RFC3339)
	userId := "800"
	insertFeedbackResp, err := suite.client.InsertFeedback(ctx, []client.Feedback{{
		FeedbackType: "like",
		UserId:       userId,
		Timestamp:    timestamp,
		ItemId:       "200",
	}})
	suite.NoError(err)
	suite.Equal(1, insertFeedbackResp.RowAffected)

	insertFeedbacksResp, err := suite.client.InsertFeedback(ctx, []client.Feedback{{
		FeedbackType: "read",
		UserId:       userId,
		Timestamp:    timestamp,
		ItemId:       "300",
	}, {
		FeedbackType: "read",
		UserId:       userId,
		Timestamp:    timestamp,
		ItemId:       "400",
	}})
	suite.NoError(err)
	suite.Equal(2, insertFeedbacksResp.RowAffected)

	feedbacks, err := suite.client.ListFeedbacks(ctx, "read", userId)
	suite.NoError(err)
	suite.ElementsMatch([]client.Feedback{
		{
			FeedbackType: "read",
			UserId:       userId,
			Timestamp:    timestamp,
			ItemId:       "300",
		}, {
			FeedbackType: "read",
			UserId:       userId,
			Timestamp:    timestamp,
			ItemId:       "400",
		},
	}, feedbacks)
}

func (suite *GorseClientTestSuite) TestRecommend() {
	ctx := context.TODO()
	suite.hSet("offline_recommend", "100", []client.Score{
		{Id: "1", Score: 1},
		{Id: "2", Score: 2},
		{Id: "3", Score: 3},
	})
	resp, err := suite.client.GetRecommend(ctx, "100", "", 10, 0)
	suite.NoError(err)
	suite.Equal([]string{"3", "2", "1"}, resp)
}

func (suite *GorseClientTestSuite) TestSessionRecommend() {
	ctx := context.Background()
	suite.hSet("item-to-item", cache.Key("neighbors", "1"), []client.Score{
		{Id: "2", Score: 100000},
		{Id: "9", Score: 1},
	})
	suite.hSet("item-to-item", cache.Key("neighbors", "2"), []client.Score{
		{Id: "3", Score: 100000},
		{Id: "8", Score: 1},
		{Id: "9", Score: 1},
	})
	suite.hSet("item-to-item", cache.Key("neighbors", "3"), []client.Score{
		{Id: "4", Score: 100000},
		{Id: "7", Score: 1},
		{Id: "8", Score: 1},
		{Id: "9", Score: 1},
	})
	suite.hSet("item-to-item", cache.Key("neighbors", "4"), []client.Score{
		{Id: "1", Score: 100000},
		{Id: "6", Score: 1},
		{Id: "7", Score: 1},
		{Id: "8", Score: 1},
		{Id: "9", Score: 1},
	})

	feedbackType := "like"
	userId := "0"
	timestamp := time.Unix(1660459054, 0).UTC().Format(time.RFC3339)
	resp, err := suite.client.SessionRecommend(ctx, []client.Feedback{
		{
			FeedbackType: feedbackType,
			UserId:       userId,
			ItemId:       "1",
			Timestamp:    timestamp,
		},
		{
			FeedbackType: feedbackType,
			UserId:       userId,
			ItemId:       "2",
			Timestamp:    timestamp,
		},
		{
			FeedbackType: feedbackType,
			UserId:       userId,
			ItemId:       "3",
			Timestamp:    timestamp,
		},
		{
			FeedbackType: feedbackType,
			UserId:       userId,
			ItemId:       "4",
			Timestamp:    timestamp,
		},
	}, 3)
	suite.NoError(err)
	suite.Equal([]client.Score{
		{
			Id:    "9",
			Score: 4,
		},
		{
			Id:    "8",
			Score: 3,
		},
		{
			Id:    "7",
			Score: 2,
		},
	}, resp)
}

func (suite *GorseClientTestSuite) TestNeighbors() {
	ctx := context.Background()
	suite.hSet("item-to-item", cache.Key("neighbors", "100"), []client.Score{
		{Id: "1", Score: 1},
		{Id: "2", Score: 2},
		{Id: "3", Score: 3},
	})

	itemId := "100"
	resp, err := suite.client.GetNeighbors(ctx, itemId, 3)
	suite.NoError(err)
	suite.Equal([]client.Score{
		{
			Id:    "3",
			Score: 3,
		}, {
			Id:    "2",
			Score: 2,
		}, {
			Id:    "1",
			Score: 1,
		},
	}, resp)
}

func (suite *GorseClientTestSuite) TestUsers() {
	ctx := context.TODO()
	user := client.User{
		UserId:    "100",
		Labels:    []string{"a", "b", "c"},
		Subscribe: []string{"d", "e"},
		Comment:   "comment",
	}
	userPatch := client.UserPatch{
		Comment: &user.Comment,
	}

	rowAffected, err := suite.client.InsertUser(ctx, user)
	suite.NoError(err)
	suite.Equal(1, rowAffected.RowAffected)

	rowAffected, err = suite.client.UpdateUser(ctx, user.UserId, userPatch)
	suite.NoError(err)
	suite.Equal(1, rowAffected.RowAffected)

	userResp, err := suite.client.GetUser(ctx, "100")
	suite.NoError(err)
	suite.Equal(user, userResp)

	deleteAffect, err := suite.client.DeleteUser(ctx, "100")
	suite.NoError(err)
	suite.Equal(1, deleteAffect.RowAffected)

	_, err = suite.client.GetUser(ctx, "100")
	suite.Equal("100: user not found", err.Error())
}

func (suite *GorseClientTestSuite) TestItems() {
	ctx := context.TODO()
	timestamp := time.Unix(1660459054, 0).UTC().Format(time.RFC3339)
	item := client.Item{
		ItemId:   "100",
		IsHidden: true,
		Labels: map[string]any{
			"Topics":    []any{"a", "b", "c"},
			"Embedding": []any{float64(1), float64(2), float64(3)},
		},
		Categories: []string{"d", "e"},
		Timestamp:  timestamp,
		Comment:    "comment",
	}
	itemPatch := client.ItemPatch{
		Comment: &item.Comment,
	}
	rowAffected, err := suite.client.InsertItem(ctx, item)
	suite.NoError(err)
	suite.Equal(1, rowAffected.RowAffected)

	rowAffected, err = suite.client.UpdateItem(ctx, item.ItemId, itemPatch)
	suite.NoError(err)
	suite.Equal(1, rowAffected.RowAffected)

	itemResp, err := suite.client.GetItem(ctx, "100")
	suite.NoError(err)
	suite.Equal(item, itemResp)

	deleteAffect, err := suite.client.DeleteItem(ctx, "100")
	suite.NoError(err)
	suite.Equal(1, deleteAffect.RowAffected)

	_, err = suite.client.GetItem(ctx, "100")
	suite.Equal("100: item not found", err.Error())
}

func (suite *GorseClientTestSuite) hSet(collection, subset string, scores []client.Score) {
	for _, score := range scores {
		err := suite.redis.HSet(context.TODO(), "documents:"+collection+":"+subset+":"+score.Id,
			"collection", collection,
			"subset", subset,
			"id", score.Id,
			"score", score.Score,
			"is_hidden", 0,
			"categories", base64.RawStdEncoding.EncodeToString([]byte("_")),
			"timestamp", time.Now().UnixMicro(),
		).Err()
		suite.NoError(err)
	}
}

func TestGorseClientTestSuite(t *testing.T) {
	suite.Run(t, new(GorseClientTestSuite))
}
