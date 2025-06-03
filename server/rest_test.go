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
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/samber/lo"
	"github.com/steinfletcher/apitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"google.golang.org/protobuf/proto"
)

const apiKey = "test_api_key"

type ServerTestSuite struct {
	suite.Suite
	RestServer
	handler *restful.Container
}

func (suite *ServerTestSuite) SetupSuite() {
	// create mock redis server
	var err error
	// open database
	suite.Settings = config.NewSettings()
	suite.DataClient, err = data.Open(fmt.Sprintf("sqlite://%s/data.db", suite.T().TempDir()), "")
	suite.NoError(err)
	suite.CacheClient, err = cache.Open(fmt.Sprintf("sqlite://%s/cache.db", suite.T().TempDir()), "")
	suite.NoError(err)
	// init database
	err = suite.DataClient.Init()
	suite.NoError(err)
	err = suite.CacheClient.Init()
	suite.NoError(err)

	suite.WebService = new(restful.WebService)
	suite.CreateWebService()
	// create handler
	suite.handler = restful.NewContainer()
	suite.handler.Add(suite.WebService)
}

func (suite *ServerTestSuite) TearDownSuite() {
	err := suite.DataClient.Close()
	suite.NoError(err)
	err = suite.CacheClient.Close()
	suite.NoError(err)
}

func (suite *ServerTestSuite) SetupTest() {
	err := suite.DataClient.Purge()
	suite.NoError(err)
	err = suite.CacheClient.Purge()
	suite.NoError(err)
	// configuration
	suite.Config = config.GetDefaultConfig()
	suite.Config.Server.APIKey = apiKey
}

func (suite *ServerTestSuite) marshal(v interface{}) string {
	s, err := json.Marshal(v)
	suite.NoError(err)
	return string(s)
}

func (suite *ServerTestSuite) TestUsers() {
	t := suite.T()
	users := []data.User{
		{UserId: "0"},
		{UserId: "1"},
		{UserId: "2"},
		{UserId: "3"},
		{UserId: "4"},
	}
	apitest.New().
		Handler(suite.handler).
		Post("/api/user").
		Header("X-API-Key", apiKey).
		JSON(users[0]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected":1}`).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/user/0").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(users[0])).
		End()
	apitest.New().
		Handler(suite.handler).
		Post("/api/users").
		Header("X-API-Key", apiKey).
		JSON(users[1:]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected":4}`).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/users").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"cursor": "",
			"n":      "100",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(UserIterator{
			Cursor: "",
			Users:  users,
		})).
		End()
	apitest.New().
		Handler(suite.handler).
		Delete("/api/user/0").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/user/0").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusNotFound).
		End()
	// test modify
	apitest.New().
		Handler(suite.handler).
		Patch("/api/user/1").
		Header("X-API-Key", apiKey).
		JSON(data.UserPatch{Labels: []string{"a", "b", "c"}, Comment: proto.String("modified")}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/user/1").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(data.User{
			UserId:  "1",
			Comment: "modified",
			Labels:  []string{"a", "b", "c"},
		})).
		End()

	// malicious labels
	apitest.New().
		Handler(suite.handler).
		Post("/api/user").
		Header("X-API-Key", apiKey).
		JSON(data.User{UserId: "malicious", Labels: []any{"price", 100}}).
		Expect(t).
		Status(http.StatusBadRequest).
		End()
	apitest.New().
		Handler(suite.handler).
		Post("/api/users").
		Header("X-API-Key", apiKey).
		JSON([]data.User{{UserId: "malicious", Labels: []any{"price", 100}}}).
		Expect(t).
		Status(http.StatusBadRequest).
		End()
	apitest.New().
		Handler(suite.handler).
		Patch("/api/user/malicious").
		Header("X-API-Key", apiKey).
		JSON(data.UserPatch{Labels: []any{"price", 100}}).
		Expect(t).
		Status(http.StatusBadRequest).
		End()
}

func (suite *ServerTestSuite) TestItems() {
	ctx := context.Background()
	t := suite.T()
	// Items
	items := []data.Item{
		{
			ItemId:    "0",
			IsHidden:  true,
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a"},
			Comment:   "comment_0",
		},
		{
			ItemId:     "2",
			Categories: []string{"*"},
			Timestamp:  time.Date(1997, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:     []string{"a"},
			Comment:    "comment_2",
		},
		{
			ItemId:    "4",
			IsHidden:  true,
			Timestamp: time.Date(1998, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a", "b"},
			Comment:   "comment_4",
		},
		{
			ItemId:     "6",
			Categories: []string{"*"},
			Timestamp:  time.Date(1999, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:     []string{"b"},
			Comment:    "comment_6",
		},
		{
			ItemId:    "8",
			IsHidden:  true,
			Timestamp: time.Date(2000, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"b"},
			Comment:   "comment_8",
		},
	}
	// insert popular scores
	err := suite.CacheClient.AddScores(ctx, cache.NonPersonalized, cache.Popular, []cache.Score{
		{Id: "0", Score: 10},
		{Id: "2", Score: 12},
		{Id: "4", Score: 14},
		{Id: "6", Score: 16},
		{Id: "8", Score: 18},
	})
	assert.NoError(t, err)
	// insert items
	apitest.New().
		Handler(suite.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(items[0]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	// batch insert items
	apitest.New().
		Handler(suite.handler).
		Post("/api/items").
		Header("X-API-Key", apiKey).
		JSON(items[1:]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 4}`).
		End()
	// get items
	apitest.New().
		Handler(suite.handler).
		Get("/api/items").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"cursor": "",
			"n":      "100",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(ItemIterator{
			Cursor: "",
			Items:  items,
		})).
		End()
	// get latest items
	apitest.New().
		Handler(suite.handler).
		Get("/api/latest").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{
			{Id: items[3].ItemId, Score: float64(items[3].Timestamp.Unix())},
			{Id: items[1].ItemId, Score: float64(items[1].Timestamp.Unix())},
		})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/latest/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{
			{Id: items[3].ItemId, Score: float64(items[3].Timestamp.Unix())},
			{Id: items[1].ItemId, Score: float64(items[1].Timestamp.Unix())},
		})).
		End()
	// get popular items
	apitest.New().
		Handler(suite.handler).
		Get("/api/popular").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{
			{Id: items[3].ItemId, Score: 16},
			{Id: items[1].ItemId, Score: 12},
		})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/popular/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{
			{Id: items[3].ItemId, Score: 16},
			{Id: items[1].ItemId, Score: 12},
		})).
		End()
	// get categories
	categories, err := suite.CacheClient.GetSet(ctx, cache.ItemCategories)
	assert.NoError(t, err)
	assert.Equal(t, []string{"*"}, categories)

	// delete item
	apitest.New().
		Handler(suite.handler).
		Delete("/api/item/6").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	// get item
	apitest.New().
		Handler(suite.handler).
		Get("/api/item/6").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusNotFound).
		End()
	// get latest items
	apitest.New().
		Handler(suite.handler).
		Get("/api/latest").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{
			{Id: items[1].ItemId, Score: float64(items[1].Timestamp.Unix())},
		})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/latest/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{
			{Id: items[1].ItemId, Score: float64(items[1].Timestamp.Unix())},
		})).
		End()
	// get popular items
	apitest.New().
		Handler(suite.handler).
		Get("/api/popular").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{
			{Id: items[1].ItemId, Score: 12},
		})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/popular/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{
			{Id: items[1].ItemId, Score: 12},
		})).
		End()

	// test modify
	timestamp := time.Date(2010, 1, 1, 1, 1, 1, 0, time.UTC)
	apitest.New().
		Handler(suite.handler).
		Patch("/api/item/2").
		Header("X-API-Key", apiKey).
		JSON(data.ItemPatch{
			IsHidden:   proto.Bool(true),
			Categories: []string{"-"},
			Labels:     []string{"a", "b", "c"},
			Comment:    proto.String("modified"),
			Timestamp:  &timestamp,
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/item/2").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(data.Item{
			ItemId:     "2",
			IsHidden:   true,
			Categories: []string{"-"},
			Comment:    "modified",
			Labels:     []string{"a", "b", "c"},
			Timestamp:  timestamp,
		})).
		End()
	apitest.New().
		Handler(suite.handler).
		Patch("/api/item/2").
		Header("X-API-Key", apiKey).
		JSON(data.ItemPatch{
			IsHidden: proto.Bool(false),
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	// get latest items
	apitest.New().
		Handler(suite.handler).
		Get("/api/latest/-").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "1",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{
			{Id: "2", Score: float64(timestamp.Unix())},
		})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/latest/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{})).
		End()
	// get popular items
	apitest.New().
		Handler(suite.handler).
		Get("/api/popular/-").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "1",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{
			{Id: "2", Score: 12},
		})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/popular/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{})).
		End()

	// insert category
	apitest.New().
		Handler(suite.handler).
		Put("/api/item/2/category/@").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(Success{RowAffected: 1})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/item/2").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(data.Item{
			ItemId:     "2",
			IsHidden:   false,
			Categories: []string{"-", "@"},
			Comment:    "modified",
			Labels:     []string{"a", "b", "c"},
			Timestamp:  timestamp,
		})).
		End()
	// get latest items
	apitest.New().
		Handler(suite.handler).
		Get("/api/latest/@").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "1",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{
			{Id: "2", Score: float64(timestamp.Unix())},
		})).
		End()
	// get popular items
	apitest.New().
		Handler(suite.handler).
		Get("/api/popular/@").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "1",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{
			{Id: "2", Score: 12},
		})).
		End()

	// delete category
	apitest.New().
		Handler(suite.handler).
		Delete("/api/item/2/category/@").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(Success{RowAffected: 1})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/item/2").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(data.Item{
			ItemId:     "2",
			IsHidden:   false,
			Categories: []string{"-"},
			Comment:    "modified",
			Labels:     []string{"a", "b", "c"},
			Timestamp:  timestamp,
		})).
		End()
	// get latest items
	apitest.New().
		Handler(suite.handler).
		Get("/api/latest/@").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "1",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{})).
		End()
	// get popular items
	apitest.New().
		Handler(suite.handler).
		Get("/api/popular/@").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "1",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{})).
		End()

	// insert items without timestamp
	apitest.New().
		Handler(suite.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "256"}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()

	// malicious labels
	apitest.New().
		Handler(suite.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "malicious", Labels: []any{"price", 1}}).
		Expect(t).
		Status(http.StatusBadRequest).
		End()
	apitest.New().
		Handler(suite.handler).
		Post("/api/items").
		Header("X-API-Key", apiKey).
		JSON([]Item{{ItemId: "malicious", Labels: []any{"price", 1}}}).
		Expect(t).
		Status(http.StatusBadRequest).
		End()
	apitest.New().
		Handler(suite.handler).
		Patch("/api/item/malicious").
		Header("X-API-Key", apiKey).
		JSON(data.ItemPatch{Labels: []any{"price", 1}}).
		Expect(t).
		Status(http.StatusBadRequest).
		End()
}

func (suite *ServerTestSuite) TestFeedback() {
	ctx := context.Background()
	t := suite.T()
	// Insert ret
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "0"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "1", ItemId: "2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "2", ItemId: "4"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "3", ItemId: "6"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "4", ItemId: "8"}},
	}
	//BatchInsertFeedback
	apitest.New().
		Handler(suite.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 5}`).
		End()
	//Get Feedback
	apitest.New().
		Handler(suite.handler).
		Get("/api/feedback").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"cursor": "",
			"n":      "100",
		}).
		Expect(t).
		Body(suite.marshal(FeedbackIterator{
			Cursor:   "",
			Feedback: feedback,
		})).
		Status(http.StatusOK).
		End()
	// get feedback by user
	apitest.New().
		Handler(suite.handler).
		Get("/api/user/1/feedback").
		Header("X-API-Key", apiKey).
		Expect(t).
		Body(suite.marshal([]data.Feedback{feedback[1]})).
		Status(http.StatusOK).
		End()
	// get feedback by item
	apitest.New().
		Handler(suite.handler).
		Get("/api/item/2/feedback").
		Header("X-API-Key", apiKey).
		Expect(t).
		Body(suite.marshal([]data.Feedback{feedback[1]})).
		Status(http.StatusOK).
		End()
	//Get Items
	apitest.New().
		Handler(suite.handler).
		Get("/api/items").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(ItemIterator{
			Cursor: "",
			Items: []data.Item{
				{ItemId: "0"},
				{ItemId: "2"},
				{ItemId: "4"},
				{ItemId: "6"},
				{ItemId: "8"},
			},
		})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/users").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(UserIterator{
			Cursor: "",
			Users: []data.User{
				{UserId: "0"},
				{UserId: "1"},
				{UserId: "2"},
				{UserId: "3"},
				{UserId: "4"}},
		})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/user/2/feedback/click").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"FeedbackType":"click", "UserId": "2", "ItemId": "4", "Timestamp":"0001-01-01T00:00:00Z", "Comment":"", "Value":0}]`).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/item/4/feedback/click").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"FeedbackType":"click", "UserId": "2", "ItemId": "4", "Timestamp":"0001-01-01T00:00:00Z", "Comment":"", "Value":0}]`).
		End()
	// test overwrite
	apitest.New().
		Handler(suite.handler).
		Put("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON([]data.Feedback{{
			FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "0"},
			Comment:     "override",
		}}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	ret, err := suite.DataClient.GetUserFeedback(ctx, "0", suite.Config.Now(), "click")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "override", ret[0].Comment)
	// test not overwrite
	apitest.New().
		Handler(suite.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON([]data.Feedback{{
			FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "0", ItemId: "0"},
			Comment:     "not_override",
		}}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	ret, err = suite.DataClient.GetUserFeedback(ctx, "0", suite.Config.Now(), "click")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "override", ret[0].Comment)

	// insert feedback without timestamp
	apitest.New().
		Handler(suite.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON([]Feedback{{FeedbackKey: data.FeedbackKey{UserId: "100", ItemId: "100", FeedbackType: "Type"}}}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
}

func (suite *ServerTestSuite) TestNonPersonalizedRecommend() {
	ctx := context.Background()
	suite.Config.Recommend.ItemToItem = []config.ItemToItemConfig{{Name: "default"}}
	type ListOperator struct {
		Name       string
		Collection string
		Subset     string
		Category   string
		URL        string
	}
	operators := []ListOperator{
		// TODO: Support hide users in the future.
		//{"User Neighbors", cache.Collection(cache.UserNeighbors, "0"), "/api/user/0/neighbors"},
		{"Item Neighbors", cache.ItemToItem, cache.Key("default", "0"), "", "/api/item/0/neighbors"},
		{"Item Neighbors in Category", cache.ItemToItem, cache.Key("default", "0"), "0", "/api/item/0/neighbors/0"},
		{"LatestItems", cache.NonPersonalized, cache.Latest, "", "/api/latest/"},
		{"LatestItemsCategory", cache.NonPersonalized, cache.Latest, "0", "/api/latest/0"},
		{"PopularItems", cache.NonPersonalized, cache.Popular, "", "/api/popular/"},
		{"PopularItemsCategory", cache.NonPersonalized, cache.Popular, "0", "/api/popular/0"},
		{"NonPersonalized", cache.NonPersonalized, "trending", "", "/api/non-personalized/trending"},
		{"NonPersonalizedCategory", cache.NonPersonalized, "trending", "0", "/api/non-personalized/trending"},
		{"ItemToItem", cache.ItemToItem, cache.Key("lookalike", "0"), "", "/api/item-to-item/lookalike/0"},
		{"ItemToItemCategory", cache.ItemToItem, cache.Key("lookalike", "0"), "0", "/api/item-to-item/lookalike/0"},
		{"Offline Recommend", cache.OfflineRecommend, "0", "", "/api/intermediate/recommend/0"},
		{"Offline Recommend in Category", cache.OfflineRecommend, "0", "0", "/api/intermediate/recommend/0/0"},
	}
	lastModified := time.Now()

	for i, operator := range operators {
		suite.T().Run(operator.Name, func(t *testing.T) {
			// insert documents
			documents := []cache.Score{
				{Id: strconv.Itoa(i) + "0", Score: 100, Categories: []string{operator.Category}},
				{Id: strconv.Itoa(i) + "1", Score: 99, Categories: []string{operator.Category}},
				{Id: strconv.Itoa(i) + "2", Score: 98, Categories: []string{operator.Category}},
				{Id: strconv.Itoa(i) + "3", Score: 97, Categories: []string{operator.Category}},
				{Id: strconv.Itoa(i) + "4", Score: 96, Categories: []string{operator.Category}},
			}
			err := suite.CacheClient.AddScores(ctx, operator.Collection, operator.Subset, documents)
			assert.NoError(t, err)
			err = suite.CacheClient.Set(ctx, cache.Time(cache.Key(operator.Collection+"_update_time", operator.Subset), lastModified))
			assert.NoError(t, err)
			// hidden item
			apitest.New().
				Handler(suite.handler).
				Patch("/api/item/"+strconv.Itoa(i)+"3").
				Header("X-API-Key", apiKey).
				JSON(data.ItemPatch{IsHidden: proto.Bool(true)}).
				Expect(t).
				Status(http.StatusOK).
				End()
			// insert read feedback
			err = suite.DataClient.BatchInsertFeedback(ctx, []data.Feedback{{
				FeedbackKey: data.FeedbackKey{
					FeedbackType: "read",
					UserId:       "0",
					ItemId:       strconv.Itoa(i) + "1",
				},
				Timestamp: time.Now().Add(-time.Hour),
			}}, true, true, true)
			assert.NoError(t, err)

			apitest.New().
				Handler(suite.handler).
				Get(operator.URL).
				Query("category", operator.Category).
				Header("X-API-Key", apiKey).
				Expect(t).
				Status(http.StatusOK).
				HeaderPresent("Last-Modified").
				Body(suite.marshal([]cache.Score{documents[0], documents[1], documents[2], documents[4]})).
				End()
			apitest.New().
				Handler(suite.handler).
				Get(operator.URL).
				Query("category", operator.Category).
				Header("X-API-Key", apiKey).
				QueryParams(map[string]string{
					"offset": "0",
					"n":      "3"}).
				Expect(t).
				Status(http.StatusOK).
				HeaderPresent("Last-Modified").
				Body(suite.marshal([]cache.Score{documents[0], documents[1], documents[2]})).
				End()
			apitest.New().
				Handler(suite.handler).
				Get(operator.URL).
				Query("category", operator.Category).
				Header("X-API-Key", apiKey).
				QueryParams(map[string]string{
					"offset": "1",
					"n":      "3"}).
				Expect(t).
				Status(http.StatusOK).
				HeaderPresent("Last-Modified").
				Body(suite.marshal([]cache.Score{documents[1], documents[2], documents[4]})).
				End()
			apitest.New().
				Handler(suite.handler).
				Get(operator.URL).
				Query("category", operator.Category).
				Header("X-API-Key", apiKey).
				QueryParams(map[string]string{
					"offset": "0",
					"n":      "0"}).
				Expect(t).
				Status(http.StatusOK).
				HeaderPresent("Last-Modified").
				Body(suite.marshal([]cache.Score{documents[0], documents[1], documents[2], documents[4]})).
				End()
			apitest.New().
				Handler(suite.handler).
				Get(operator.URL).
				Query("category", operator.Category).
				Header("X-API-Key", apiKey).
				QueryParams(map[string]string{
					"user-id": "0",
					"offset":  "0",
					"n":       "0"}).
				Expect(t).
				Status(http.StatusOK).
				HeaderPresent("Last-Modified").
				Body(suite.marshal([]cache.Score{documents[0], documents[2], documents[4]})).
				End()
		})
	}
}

func (suite *ServerTestSuite) TestDeleteFeedback() {
	t := suite.T()
	// Insert feedback
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "type1", UserId: "2", ItemId: "3"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "type2", UserId: "2", ItemId: "3"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "type3", UserId: "2", ItemId: "3"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "type1", UserId: "1", ItemId: "6"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "type1", UserId: "4", ItemId: "8"}},
	}
	apitest.New().
		Handler(suite.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 5}`).
		End()
	// Get Feedback
	apitest.New().
		Handler(suite.handler).
		Get("/api/feedback/2/3").
		Header("X-API-Key", apiKey).
		Expect(t).
		Body(suite.marshal([]data.Feedback{feedback[0], feedback[1], feedback[2]})).
		Status(http.StatusOK).
		End()
	// Get typed feedback
	apitest.New().
		Handler(suite.handler).
		Get("/api/feedback/type2/2/3").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(feedback[1])).
		End()
	// delete feedback
	apitest.New().
		Handler(suite.handler).
		Delete("/api/feedback/2/3").
		Header("X-API-Key", apiKey).
		Expect(t).
		Body(`{"RowAffected": 3}`).
		Status(http.StatusOK).
		End()
	// delete typed feedback
	apitest.New().
		Handler(suite.handler).
		Delete("/api/feedback/type1/4/8").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
}

func (suite *ServerTestSuite) TestTimeSeries() {
	ctx := context.Background()
	t := suite.T()
	timestamp := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour * -2)
	points := []cache.TimeSeriesPoint{
		{"Test_NDCG", timestamp.Add(24 * time.Hour * 0), 0},
		{"Test_NDCG", timestamp.Add(24 * time.Hour * 1), 1},
		{"Test_NDCG", timestamp.Add(24 * time.Hour * 2), 2},
		{"Test_NDCG", timestamp.Add(24 * time.Hour * 3), 3},
		{"Test_NDCG", timestamp.Add(24 * time.Hour * 4), 4},
		{"Test_Recall", timestamp, 1},
	}
	err := suite.CacheClient.AddTimeSeriesPoints(ctx, points)
	assert.NoError(t, err)
	apitest.New().
		Handler(suite.handler).
		Get("/api/measurements/Test_NDCG").
		Query("n", "3").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.TimeSeriesPoint{
			points[0],
			points[1],
			points[2],
		})).
		End()
	// test auth fail
	apitest.New().
		Handler(suite.handler).
		Get("/api/measurements/Test_NDCG").
		Query("n", "3").
		Expect(t).
		Status(http.StatusUnauthorized).
		End()
}

func (suite *ServerTestSuite) TestGetRecommends() {
	ctx := context.Background()
	t := suite.T()
	// insert hidden items
	err := suite.CacheClient.AddScores(ctx, cache.OfflineRecommend, "0", []cache.Score{{Id: "0", Score: 100, Categories: []string{""}}})
	assert.NoError(t, err)
	// hide item
	apitest.New().
		Handler(suite.handler).
		Patch("/api/item/0").
		Header("X-API-Key", apiKey).
		JSON(data.ItemPatch{IsHidden: proto.Bool(true)}).
		Expect(t).
		Status(http.StatusOK).
		End()
	// insert items
	err = suite.DataClient.BatchInsertItems(ctx, []data.Item{
		{ItemId: "1"},
		{ItemId: "2"},
		{ItemId: "3"},
		{ItemId: "4"},
		{ItemId: "5"},
		{ItemId: "6"},
		{ItemId: "7"},
		{ItemId: "8"},
	})
	assert.NoError(t, err)
	// insert recommendation
	err = suite.CacheClient.AddScores(ctx, cache.OfflineRecommend, "0", []cache.Score{
		{Id: "1", Score: 99, Categories: []string{""}},
		{Id: "2", Score: 98, Categories: []string{""}},
		{Id: "3", Score: 97, Categories: []string{""}},
		{Id: "4", Score: 96, Categories: []string{""}},
		{Id: "5", Score: 95, Categories: []string{""}},
		{Id: "6", Score: 94, Categories: []string{""}},
		{Id: "7", Score: 93, Categories: []string{""}},
		{Id: "8", Score: 92, Categories: []string{""}},
	})
	assert.NoError(t, err)
	// insert feedback
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "4"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "1"}, Timestamp: time.Now().Add(time.Hour)},
	}
	apitest.New().
		Handler(suite.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 3}`).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"1", "3", "5"})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n":      "3",
			"offset": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"6", "7", "8"})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n":      "3",
			"offset": "10000",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n":               "3",
			"write-back-type": "read",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"1", "3", "5"})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n":                "3",
			"write-back-type":  "read",
			"write-back-delay": "10m",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"6", "7", "8"})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"6", "7", "8"})).
		End()
}

func (suite *ServerTestSuite) TestGetRecommendsWithMultiCategories() {
	ctx := context.Background()
	t := suite.T()
	// insert recommendation
	err := suite.CacheClient.AddScores(ctx, cache.OfflineRecommend, "0", []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}},
		{Id: "2", Score: 2, Categories: []string{"", "2"}},
		{Id: "3", Score: 3, Categories: []string{"", "3"}},
		{Id: "4", Score: 4, Categories: []string{"", "2"}},
		{Id: "5", Score: 5, Categories: []string{"", "5"}},
		{Id: "6", Score: 6, Categories: []string{"", "2", "3"}},
		{Id: "7", Score: 7, Categories: []string{"", "7"}},
		{Id: "8", Score: 8, Categories: []string{"", "2"}},
		{Id: "9", Score: 9, Categories: []string{"", "3"}},
	})
	suite.NoError(err)
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryCollection(map[string][]string{
			"n":        {"3"},
			"category": {"2", "3"},
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"6"})).
		End()
}

func (suite *ServerTestSuite) TestGetRecommendsWithReplacement() {
	ctx := context.Background()
	t := suite.T()
	suite.Config.Recommend.Replacement.EnableReplacement = true
	// insert recommendation
	err := suite.CacheClient.AddScores(ctx, cache.OfflineRecommend, "0", []cache.Score{
		{Id: "0", Score: 100, Categories: []string{""}},
		{Id: "1", Score: 99, Categories: []string{""}},
		{Id: "2", Score: 98, Categories: []string{""}},
		{Id: "3", Score: 97, Categories: []string{""}},
		{Id: "4", Score: 96, Categories: []string{""}},
		{Id: "5", Score: 95, Categories: []string{""}},
		{Id: "6", Score: 94, Categories: []string{""}},
		{Id: "7", Score: 93, Categories: []string{""}},
		{Id: "8", Score: 92, Categories: []string{""}},
	})
	assert.NoError(t, err)
	// hide item
	apitest.New().
		Handler(suite.handler).
		Patch("/api/item/0").
		Header("X-API-Key", apiKey).
		JSON(data.ItemPatch{IsHidden: proto.Bool(true)}).
		Expect(t).
		Status(http.StatusOK).
		End()
	// insert feedback
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "4"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "1"}, Timestamp: time.Now().Add(time.Hour)},
	}
	apitest.New().
		Handler(suite.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 3}`).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"1", "2", "3"})).
		End()
}

func (suite *ServerTestSuite) TestServerGetRecommendsFallbackItemBasedSimilar() {
	ctx := context.Background()
	t := suite.T()
	suite.Config.Recommend.Online.NumFeedbackFallbackItemBased = 4
	suite.Config.Recommend.DataSource.PositiveFeedbackTypes = []string{"a"}
	suite.Config.Recommend.ItemToItem = []config.ItemToItemConfig{{Name: "default"}}
	// insert recommendation
	err := suite.CacheClient.AddScores(ctx, cache.OfflineRecommend, "0", []cache.Score{
		{Id: "1", Score: 99},
		{Id: "2", Score: 98},
		{Id: "3", Score: 97},
		{Id: "4", Score: 96}})
	assert.NoError(t, err)
	// insert feedback
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "1"}, Timestamp: time.Date(2010, 1, 1, 1, 1, 1, 1, time.UTC)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "2"}, Timestamp: time.Date(2009, 1, 1, 1, 1, 1, 1, time.UTC)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "3"}, Timestamp: time.Date(2008, 1, 1, 1, 1, 1, 1, time.UTC)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "4"}, Timestamp: time.Date(2007, 1, 1, 1, 1, 1, 1, time.UTC)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "5"}, Timestamp: time.Date(2006, 1, 1, 1, 1, 1, 1, time.UTC)},
	}
	apitest.New().
		Handler(suite.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 5}`).
		End()

	// insert similar items
	err = suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "1"), []cache.Score{
		{Id: "2", Score: 100000, Categories: []string{""}},
		{Id: "9", Score: 1, Categories: []string{"", "*"}},
	})
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "2"), []cache.Score{
		{Id: "3", Score: 100000, Categories: []string{"", "*"}},
		{Id: "8", Score: 1, Categories: []string{""}},
		{Id: "9", Score: 1, Categories: []string{"", "*"}},
	})
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "3"), []cache.Score{
		{Id: "4", Score: 100000, Categories: []string{""}},
		{Id: "7", Score: 1, Categories: []string{"", "*"}},
		{Id: "8", Score: 1, Categories: []string{""}},
		{Id: "9", Score: 1, Categories: []string{"", "*"}},
	})
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "4"), []cache.Score{
		{Id: "1", Score: 100000, Categories: []string{"", "*"}},
		{Id: "6", Score: 1, Categories: []string{""}},
		{Id: "7", Score: 1, Categories: []string{"", "*"}},
		{Id: "8", Score: 1, Categories: []string{""}},
		{Id: "9", Score: 1, Categories: []string{"", "*"}},
	})
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "5"), []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}},
		{Id: "6", Score: 1, Categories: []string{""}},
		{Id: "7", Score: 100000, Categories: []string{""}},
		{Id: "8", Score: 100, Categories: []string{""}},
		{Id: "9", Score: 1, Categories: []string{""}},
	})
	assert.NoError(t, err)

	// test fallback
	suite.Config.Recommend.Online.FallbackRecommend = []string{"item_based"}
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"9", "8", "7"})).
		End()
	suite.Config.Recommend.Online.FallbackRecommend = []string{"item_based"}
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"9", "7"})).
		End()
}

func (suite *ServerTestSuite) TestGetRecommendsFallbackUserBasedSimilar() {
	ctx := context.Background()
	suite.Config.Recommend.UserToUser = []config.UserToUserConfig{{Name: "default"}}
	t := suite.T()
	// insert recommendation
	err := suite.CacheClient.AddScores(ctx, cache.OfflineRecommend, "0",
		[]cache.Score{{Id: "1", Score: 99}, {Id: "2", Score: 98}, {Id: "3", Score: 97}, {Id: "4", Score: 96}})
	assert.NoError(t, err)
	// insert feedback
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "1"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "3"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "4"}},
	}
	apitest.New().
		Handler(suite.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 4}`).
		End()
	// insert similar users
	err = suite.CacheClient.AddScores(ctx, cache.UserToUser, cache.Key("default", "0"), []cache.Score{
		{Id: "1", Score: 2, Categories: []string{""}},
		{Id: "2", Score: 1.5, Categories: []string{""}},
		{Id: "3", Score: 1, Categories: []string{""}},
	})
	assert.NoError(t, err)
	err = suite.DataClient.BatchInsertFeedback(ctx, []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "1", ItemId: "11"}},
	}, true, true, true)
	assert.NoError(t, err)
	err = suite.DataClient.BatchInsertFeedback(ctx, []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "12"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "48"}},
	}, true, true, true)
	assert.NoError(t, err)
	err = suite.DataClient.BatchInsertFeedback(ctx, []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "13"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "48"}},
	}, true, true, true)
	assert.NoError(t, err)
	// insert categorized items
	err = suite.DataClient.BatchInsertItems(ctx, []data.Item{
		{ItemId: "12", Categories: []string{"*"}},
		{ItemId: "48", Categories: []string{"*"}},
	})
	assert.NoError(t, err)
	// test fallback
	suite.Config.Recommend.Online.FallbackRecommend = []string{"user_based"}
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"48", "11", "12"})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"48", "12"})).
		End()
}

func (suite *ServerTestSuite) TestGetRecommendsFallbackPreCached() {
	ctx := context.Background()
	t := suite.T()
	// insert offline recommendation
	err := suite.CacheClient.AddScores(ctx, cache.OfflineRecommend, "0", []cache.Score{
		{Id: "1", Score: 99, Categories: []string{""}},
		{Id: "2", Score: 98, Categories: []string{""}},
		{Id: "3", Score: 97, Categories: []string{""}},
		{Id: "4", Score: 96, Categories: []string{""}}})
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.OfflineRecommend, "0", []cache.Score{
		{Id: "101", Score: 99, Categories: []string{"*"}},
		{Id: "102", Score: 98, Categories: []string{"*"}},
		{Id: "103", Score: 97, Categories: []string{"*"}},
		{Id: "104", Score: 96, Categories: []string{"*"}}})
	assert.NoError(t, err)
	// insert latest
	err = suite.CacheClient.AddScores(ctx, cache.NonPersonalized, cache.Latest, []cache.Score{
		{Id: "5", Score: 95, Categories: []string{""}},
		{Id: "6", Score: 94, Categories: []string{""}},
		{Id: "7", Score: 93, Categories: []string{""}},
		{Id: "8", Score: 92, Categories: []string{""}}})
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.NonPersonalized, cache.Latest, []cache.Score{
		{Id: "105", Score: 95, Categories: []string{"*"}},
		{Id: "106", Score: 94, Categories: []string{"*"}},
		{Id: "107", Score: 93, Categories: []string{"*"}},
		{Id: "108", Score: 92, Categories: []string{"*"}}})
	assert.NoError(t, err)
	// insert popular
	err = suite.CacheClient.AddScores(ctx, cache.NonPersonalized, cache.Popular, []cache.Score{
		{Id: "9", Score: 91, Categories: []string{""}},
		{Id: "10", Score: 90, Categories: []string{""}},
		{Id: "11", Score: 89, Categories: []string{""}},
		{Id: "12", Score: 88, Categories: []string{""}}})
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.NonPersonalized, cache.Popular, []cache.Score{
		{Id: "109", Score: 91, Categories: []string{"*"}},
		{Id: "110", Score: 90, Categories: []string{"*"}},
		{Id: "111", Score: 89, Categories: []string{"*"}},
		{Id: "112", Score: 88, Categories: []string{"*"}}})
	assert.NoError(t, err)
	// insert collaborative filtering
	err = suite.CacheClient.AddScores(ctx, cache.CollaborativeFiltering, "0", []cache.Score{
		{Id: "13", Score: 79, Categories: []string{""}},
		{Id: "14", Score: 78, Categories: []string{""}},
		{Id: "15", Score: 77, Categories: []string{""}},
		{Id: "16", Score: 76, Categories: []string{""}}})
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.CollaborativeFiltering, "0", []cache.Score{
		{Id: "113", Score: 79, Categories: []string{"*"}},
		{Id: "114", Score: 78, Categories: []string{"*"}},
		{Id: "115", Score: 77, Categories: []string{"*"}},
		{Id: "116", Score: 76, Categories: []string{"*"}}})
	assert.NoError(t, err)
	// test popular fallback
	suite.Config.Recommend.Online.FallbackRecommend = []string{"popular"}
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "8",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"1", "2", "3", "4", "9", "10", "11", "12"})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "8",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"101", "102", "103", "104", "109", "110", "111", "112"})).
		End()
	// test latest fallback
	suite.Config.Recommend.Online.FallbackRecommend = []string{"latest"}
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "8",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"1", "2", "3", "4", "5", "6", "7", "8"})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "8",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"101", "102", "103", "104", "105", "106", "107", "108"})).
		End()
	// test collaborative filtering
	suite.Config.Recommend.Online.FallbackRecommend = []string{"collaborative"}
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "8",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"1", "2", "3", "4", "13", "14", "15", "16"})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "8",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]string{"101", "102", "103", "104", "113", "114", "115", "116"})).
		End()
	// test wrong fallback
	suite.Config.Recommend.Online.FallbackRecommend = []string{""}
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "8",
		}).
		Expect(t).
		Status(http.StatusInternalServerError).
		End()
}

func (suite *ServerTestSuite) TestSessionRecommend() {
	ctx := context.Background()
	t := suite.T()
	suite.Config.Recommend.Online.NumFeedbackFallbackItemBased = 4
	suite.Config.Recommend.DataSource.PositiveFeedbackTypes = []string{"a"}
	suite.Config.Recommend.ItemToItem = []config.ItemToItemConfig{{Name: "default"}}

	// insert similar items
	err := suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "1"), []cache.Score{
		{Id: "2", Score: 100000, Categories: []string{""}},
		{Id: "9", Score: 1, Categories: []string{"", "*"}},
		{Id: "100", Score: 100000, Categories: []string{""}},
	})
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "2"), []cache.Score{
		{Id: "3", Score: 100000, Categories: []string{"", "*"}},
		{Id: "8", Score: 1, Categories: []string{""}},
		{Id: "9", Score: 1, Categories: []string{"", "*"}},
	})
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "3"), []cache.Score{
		{Id: "4", Score: 100000, Categories: []string{""}},
		{Id: "7", Score: 1, Categories: []string{"", "*"}},
		{Id: "8", Score: 1, Categories: []string{""}},
		{Id: "9", Score: 1, Categories: []string{"", "*"}},
	})
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "4"), []cache.Score{
		{Id: "1", Score: 100000, Categories: []string{"", "*"}},
		{Id: "6", Score: 1, Categories: []string{""}},
		{Id: "7", Score: 1, Categories: []string{"", "*"}},
		{Id: "8", Score: 1, Categories: []string{""}},
		{Id: "9", Score: 1, Categories: []string{"", "*"}},
	})
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "5"), []cache.Score{
		{Id: "1", Score: 1, Categories: []string{""}},
		{Id: "6", Score: 1, Categories: []string{""}},
		{Id: "7", Score: 100000, Categories: []string{""}},
		{Id: "8", Score: 100, Categories: []string{""}},
		{Id: "9", Score: 1, Categories: []string{""}},
	})
	assert.NoError(t, err)

	// hide items
	apitest.New().
		Handler(suite.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "100", IsHidden: true}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()

	// test fallback
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "1"}, Timestamp: time.Date(2010, 1, 1, 1, 1, 1, 1, time.UTC)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "2"}, Timestamp: time.Date(2009, 1, 1, 1, 1, 1, 1, time.UTC)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "3"}, Timestamp: time.Date(2008, 1, 1, 1, 1, 1, 1, time.UTC)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "4"}, Timestamp: time.Date(2007, 1, 1, 1, 1, 1, 1, time.UTC)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "5"}, Timestamp: time.Date(2006, 1, 1, 1, 1, 1, 1, time.UTC)},
	}
	apitest.New().
		Handler(suite.handler).
		Post("/api/session/recommend").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{{Id: "9", Score: 4}, {Id: "8", Score: 3}, {Id: "7", Score: 2}})).
		End()
	apitest.New().
		Handler(suite.handler).
		Post("/api/session/recommend").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"offset": "100",
		}).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score(nil))).
		End()
	suite.Config.Recommend.Online.FallbackRecommend = []string{"item_based"}
	apitest.New().
		Handler(suite.handler).
		Post("/api/session/recommend/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal([]cache.Score{{Id: "9", Score: 4}, {Id: "7", Score: 2}})).
		End()
}

func (suite *ServerTestSuite) TestVisibility() {
	ctx := context.Background()
	suite.Config.Recommend.ItemToItem = []config.ItemToItemConfig{{Name: "default"}}
	t := suite.T()
	// insert items: 0, 1, 2, 3, 4
	var items []Item
	for i := 0; i < 5; i++ {
		items = append(items, Item{
			ItemId:     strconv.Itoa(i),
			Categories: []string{"a"},
			Timestamp:  time.Date(1989, 6, i+1, 1, 1, 1, 1, time.UTC).String(),
		})
	}
	apitest.New().
		Handler(suite.handler).
		Post("/api/items").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		End()

	// insert cache
	var documents []cache.Score
	for i := range items {
		documents = append(documents, cache.Score{
			Id:         strconv.Itoa(i),
			Score:      float64(time.Date(1989, 6, i+1, 1, 1, 1, 1, time.UTC).Unix()),
			Categories: []string{"", "a"},
		})
	}
	lo.Reverse(documents)
	err := suite.CacheClient.AddScores(ctx, cache.NonPersonalized, cache.Latest, documents)
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.NonPersonalized, cache.Popular, documents)
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("default", "100"), documents)
	assert.NoError(t, err)
	err = suite.CacheClient.AddScores(ctx, cache.OfflineRecommend, "100", documents)
	assert.NoError(t, err)

	// delete item
	apitest.New().
		Handler(suite.handler).
		Delete("/api/item/0").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		End()
	// modify item
	apitest.New().
		Handler(suite.handler).
		Patch("/api/item/1").
		Header("X-API-Key", apiKey).
		JSON(data.ItemPatch{IsHidden: proto.Bool(true)}).
		Expect(t).
		Status(http.StatusOK).
		End()
	// overwrite item
	apitest.New().
		Handler(suite.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "2", IsHidden: true}).
		Expect(t).
		Status(http.StatusOK).
		End()

	// recommend
	apitest.New().
		Handler(suite.handler).
		Get("/api/popular").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(documents[:2])).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/latest").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(documents[:2])).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/item/100/neighbors/").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(documents[:2])).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/100/").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(cache.ConvertDocumentsToValues(documents[:2]))).
		End()

	// insert item
	apitest.New().
		Handler(suite.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "0", Timestamp: time.Date(1989, 6, 1, 1, 1, 1, 1, time.UTC).String()}).
		Expect(t).
		Status(http.StatusOK).
		End()
	// modify item
	apitest.New().
		Handler(suite.handler).
		Patch("/api/item/1").
		Header("X-API-Key", apiKey).
		JSON(data.ItemPatch{IsHidden: proto.Bool(false)}).
		Expect(t).
		Status(http.StatusOK).
		End()
	// overwrite item
	apitest.New().
		Handler(suite.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "2", IsHidden: false, Timestamp: time.Date(1989, 6, 3, 1, 1, 1, 1, time.UTC).String()}).
		Expect(t).
		Status(http.StatusOK).
		End()

	// recommend
	apitest.New().
		Handler(suite.handler).
		Get("/api/popular").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(documents[:len(documents)-1])).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/latest").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(documents)).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/item/100/neighbors/").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(documents[:len(documents)-1])).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/100/").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(cache.ConvertDocumentsToValues(documents))).
		End()

	// delete category
	apitest.New().
		Handler(suite.handler).
		Delete("/api/item/0/category/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		End()
	// modify category
	apitest.New().
		Handler(suite.handler).
		Patch("/api/item/1").
		Header("X-API-Key", apiKey).
		JSON(data.ItemPatch{Categories: []string{}}).
		Expect(t).
		Status(http.StatusOK).
		End()
	// overwrite category
	apitest.New().
		Handler(suite.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "2", Categories: []string{}}).
		Expect(t).
		Status(http.StatusOK).
		End()

	// recommend
	apitest.New().
		Handler(suite.handler).
		Get("/api/popular/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(documents[:2])).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/latest/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(documents[:2])).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/item/100/neighbors/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(documents[:2])).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/100/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(cache.ConvertDocumentsToValues(documents[:2]))).
		End()

	// delete category
	apitest.New().
		Handler(suite.handler).
		Put("/api/item/0/category/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		End()
	// modify category
	apitest.New().
		Handler(suite.handler).
		Patch("/api/item/1").
		Header("X-API-Key", apiKey).
		JSON(data.ItemPatch{Categories: []string{"a"}}).
		Expect(t).
		Status(http.StatusOK).
		End()
	// overwrite category
	apitest.New().
		Handler(suite.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "2", Categories: []string{"a"}, Timestamp: time.Date(1989, 6, 3, 1, 1, 1, 1, time.UTC).String()}).
		Expect(t).
		Status(http.StatusOK).
		End()

	// recommend
	apitest.New().
		Handler(suite.handler).
		Get("/api/popular/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(documents[:len(documents)-1])).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/latest/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(documents)).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/item/100/neighbors/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(documents[:len(documents)-1])).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/recommend/100/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(cache.ConvertDocumentsToValues(documents))).
		End()
}

func (suite *ServerTestSuite) TestHealth() {
	t := suite.T()
	// ready
	apitest.New().
		Handler(suite.handler).
		Get("/api/health/live").
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(HealthStatus{
			Ready:               true,
			DataStoreError:      nil,
			CacheStoreError:     nil,
			DataStoreConnected:  true,
			CacheStoreConnected: true,
		})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/health/ready").
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(HealthStatus{
			Ready:               true,
			DataStoreError:      nil,
			CacheStoreError:     nil,
			DataStoreConnected:  true,
			CacheStoreConnected: true,
		})).
		End()

	// not ready
	dataClient, cacheClient := suite.DataClient, suite.CacheClient
	suite.DataClient, suite.CacheClient = data.NoDatabase{}, cache.NoDatabase{}
	apitest.New().
		Handler(suite.handler).
		Get("/api/health/live").
		Expect(t).
		Status(http.StatusOK).
		Body(suite.marshal(HealthStatus{
			Ready:               false,
			DataStoreError:      data.ErrNoDatabase,
			CacheStoreError:     cache.ErrNoDatabase,
			DataStoreConnected:  false,
			CacheStoreConnected: false,
		})).
		End()
	apitest.New().
		Handler(suite.handler).
		Get("/api/health/ready").
		Expect(t).
		Status(http.StatusServiceUnavailable).
		Body(suite.marshal(HealthStatus{
			Ready:               false,
			DataStoreError:      data.ErrNoDatabase,
			CacheStoreError:     cache.ErrNoDatabase,
			DataStoreConnected:  false,
			CacheStoreConnected: false,
		})).
		End()
	suite.DataClient, suite.CacheClient = dataClient, cacheClient
}

func TestServer(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}
