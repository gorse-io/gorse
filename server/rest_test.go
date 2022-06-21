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
	"encoding/json"
	"github.com/alicebob/miniredis/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/samber/lo"
	"github.com/steinfletcher/apitest"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"google.golang.org/protobuf/proto"
	"net/http"
	"strconv"
	"testing"
	"time"
)

const apiKey = "test_api_key"

type mockServer struct {
	dataStoreServer  *miniredis.Miniredis
	cacheStoreServer *miniredis.Miniredis
	handler          *restful.Container
	RestServer
}

func newMockServer(t *testing.T) *mockServer {
	s := new(mockServer)
	// create mock redis server
	var err error
	s.dataStoreServer, err = miniredis.Run()
	assert.NoError(t, err)
	s.cacheStoreServer, err = miniredis.Run()
	assert.NoError(t, err)
	// open database
	s.Settings = config.NewSettings()
	s.DataClient, err = data.Open("redis://" + s.dataStoreServer.Addr())
	assert.NoError(t, err)
	s.CacheClient, err = cache.Open("redis://" + s.cacheStoreServer.Addr())
	assert.NoError(t, err)
	// configuration
	s.Config = config.GetDefaultConfig()
	s.Config.Server.APIKey = apiKey
	s.PopularItemsCache = newPopularItemsCacheForTest(&s.RestServer)
	s.HiddenItemsManager = newHiddenItemsManagerForTest(&s.RestServer)
	s.WebService = new(restful.WebService)
	s.CreateWebService()
	// create handler
	s.handler = restful.NewContainer()
	s.handler.Add(s.WebService)
	return s
}

func (s *mockServer) Close(t *testing.T) {
	err := s.DataClient.Close()
	assert.NoError(t, err)
	err = s.CacheClient.Close()
	assert.NoError(t, err)
	s.dataStoreServer.Close()
	s.cacheStoreServer.Close()
}

func marshal(t *testing.T, v interface{}) string {
	s, err := json.Marshal(v)
	assert.NoError(t, err)
	return string(s)
}

func TestServer_Users(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)
	users := []data.User{
		{UserId: "0"},
		{UserId: "1"},
		{UserId: "2"},
		{UserId: "3"},
		{UserId: "4"},
	}
	apitest.New().
		Handler(s.handler).
		Post("/api/user").
		Header("X-API-Key", apiKey).
		JSON(users[0]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected":1}`).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/user/0").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, users[0])).
		End()
	apitest.New().
		Handler(s.handler).
		Post("/api/users").
		Header("X-API-Key", apiKey).
		JSON(users[1:]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected":4}`).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/users").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"cursor": "",
			"n":      "100",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, UserIterator{
			Cursor: "",
			Users:  users,
		})).
		End()
	apitest.New().
		Handler(s.handler).
		Delete("/api/user/0").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/user/0").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusNotFound).
		End()
	// test modify
	apitest.New().
		Handler(s.handler).
		Patch("/api/user/1").
		Header("X-API-Key", apiKey).
		JSON(data.UserPatch{Labels: []string{"a", "b", "c"}, Comment: proto.String("modified")}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/user/1").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, data.User{
			UserId:  "1",
			Comment: "modified",
			Labels:  []string{"a", "b", "c"},
		})).
		End()
}

func TestServer_Items(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)
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
	err := s.CacheClient.SetSorted(cache.PopularItems, []cache.Scored{
		{Id: "0", Score: 10},
		{Id: "2", Score: 12},
		{Id: "4", Score: 14},
		{Id: "6", Score: 16},
		{Id: "8", Score: 18},
	})
	assert.NoError(t, err)
	// insert items
	apitest.New().
		Handler(s.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(items[0]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	// batch insert items
	apitest.New().
		Handler(s.handler).
		Post("/api/items").
		Header("X-API-Key", apiKey).
		JSON(items[1:]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 4}`).
		End()
	// get items
	apitest.New().
		Handler(s.handler).
		Get("/api/items").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"cursor": "",
			"n":      "100",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, ItemIterator{
			Cursor: "",
			Items:  items,
		})).
		End()
	// get latest items
	apitest.New().
		Handler(s.handler).
		Get("/api/latest").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{
			{Id: items[3].ItemId, Score: float64(items[3].Timestamp.Unix())},
			{Id: items[1].ItemId, Score: float64(items[1].Timestamp.Unix())},
		})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/latest/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{
			{Id: items[3].ItemId, Score: float64(items[3].Timestamp.Unix())},
			{Id: items[1].ItemId, Score: float64(items[1].Timestamp.Unix())},
		})).
		End()
	// get popular items
	apitest.New().
		Handler(s.handler).
		Get("/api/popular").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{
			{Id: items[3].ItemId, Score: 16},
			{Id: items[1].ItemId, Score: 12},
		})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/popular/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{
			{Id: items[3].ItemId, Score: 16},
			{Id: items[1].ItemId, Score: 12},
		})).
		End()
	// get categories
	categories, err := s.CacheClient.GetSet(cache.ItemCategories)
	assert.NoError(t, err)
	assert.Equal(t, []string{"*"}, categories)

	// delete item
	apitest.New().
		Handler(s.handler).
		Delete("/api/item/6").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	// get item
	apitest.New().
		Handler(s.handler).
		Get("/api/item/6").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusNotFound).
		End()
	isHidden, err := s.HiddenItemsManager.IsHidden([]string{"6"}, "")
	assert.NoError(t, err)
	assert.True(t, isHidden[0])
	// get latest items
	apitest.New().
		Handler(s.handler).
		Get("/api/latest").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{
			{Id: items[1].ItemId, Score: float64(items[1].Timestamp.Unix())},
		})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/latest/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{
			{Id: items[1].ItemId, Score: float64(items[1].Timestamp.Unix())},
		})).
		End()
	// get popular items
	apitest.New().
		Handler(s.handler).
		Get("/api/popular").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{
			{Id: items[1].ItemId, Score: 12},
		})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/popular/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{
			{Id: items[1].ItemId, Score: 12},
		})).
		End()

	// test modify
	timestamp := time.Date(2010, 1, 1, 1, 1, 1, 0, time.UTC)
	apitest.New().
		Handler(s.handler).
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
		Handler(s.handler).
		Get("/api/item/2").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, data.Item{
			ItemId:     "2",
			IsHidden:   true,
			Categories: []string{"-"},
			Comment:    "modified",
			Labels:     []string{"a", "b", "c"},
			Timestamp:  timestamp,
		})).
		End()
	isHidden, err = s.HiddenItemsManager.IsHidden([]string{"2"}, "")
	assert.NoError(t, err)
	assert.True(t, isHidden[0])
	apitest.New().
		Handler(s.handler).
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
		Handler(s.handler).
		Get("/api/latest/-").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "1",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{
			{Id: "2", Score: float64(timestamp.Unix())},
		})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/latest/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{})).
		End()
	// get popular items
	apitest.New().
		Handler(s.handler).
		Get("/api/popular/-").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "1",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{
			{Id: "2", Score: 12},
		})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/popular/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{})).
		End()

	// insert category
	apitest.New().
		Handler(s.handler).
		Put("/api/item/2/category/@").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, Success{RowAffected: 1})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/item/2").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, data.Item{
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
		Handler(s.handler).
		Get("/api/latest/@").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "1",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{
			{Id: "2", Score: float64(timestamp.Unix())},
		})).
		End()
	// get popular items
	apitest.New().
		Handler(s.handler).
		Get("/api/popular/@").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "1",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{
			{Id: "2", Score: 12},
		})).
		End()

	// delete category
	apitest.New().
		Handler(s.handler).
		Delete("/api/item/2/category/@").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, Success{RowAffected: 1})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/item/2").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, data.Item{
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
		Handler(s.handler).
		Get("/api/latest/@").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "1",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{})).
		End()
	// get popular items
	apitest.New().
		Handler(s.handler).
		Get("/api/popular/@").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "1",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{})).
		End()

	// insert items without timestamp
	apitest.New().
		Handler(s.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "256"}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
}

func TestServer_Feedback(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)
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
		Handler(s.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 5}`).
		End()
	//Get Feedback
	apitest.New().
		Handler(s.handler).
		Get("/api/feedback").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"cursor": "",
			"n":      "100",
		}).
		Expect(t).
		Body(marshal(t, FeedbackIterator{
			Cursor:   "",
			Feedback: feedback,
		})).
		Status(http.StatusOK).
		End()
	// get feedback by user
	apitest.New().
		Handler(s.handler).
		Get("/api/user/1/feedback").
		Header("X-API-Key", apiKey).
		Expect(t).
		Body(marshal(t, []data.Feedback{feedback[1]})).
		Status(http.StatusOK).
		End()
	// get feedback by item
	apitest.New().
		Handler(s.handler).
		Get("/api/item/2/feedback").
		Header("X-API-Key", apiKey).
		Expect(t).
		Body(marshal(t, []data.Feedback{feedback[1]})).
		Status(http.StatusOK).
		End()
	//Get Items
	apitest.New().
		Handler(s.handler).
		Get("/api/items").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, ItemIterator{
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
		Handler(s.handler).
		Get("/api/users").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, UserIterator{
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
		Handler(s.handler).
		Get("/api/user/2/feedback/click").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"FeedbackType":"click", "UserId": "2", "ItemId": "4", "Timestamp":"0001-01-01T00:00:00Z","Comment":""}]`).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/item/4/feedback/click").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"FeedbackType":"click", "UserId": "2", "ItemId": "4", "Timestamp":"0001-01-01T00:00:00Z","Comment":""}]`).
		End()
	// test overwrite
	apitest.New().
		Handler(s.handler).
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
	ret, err := s.DataClient.GetUserFeedback("0", false, "click")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "override", ret[0].Comment)
	// test not overwrite
	apitest.New().
		Handler(s.handler).
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
	ret, err = s.DataClient.GetUserFeedback("0", false, "click")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "override", ret[0].Comment)

	// insert feedback without timestamp
	apitest.New().
		Handler(s.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON([]Feedback{{FeedbackKey: data.FeedbackKey{UserId: "100", ItemId: "100", FeedbackType: "Type"}}}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
}

func TestServer_Sort(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)
	type ListOperator struct {
		Name string
		Key  string
		Get  string
	}
	operators := []ListOperator{
		// TODO: Support hide users in the future.
		//{"User Neighbors", cache.Key(cache.UserNeighbors, "0"), "/api/user/0/neighbors"},
		{"Item Neighbors", cache.Key(cache.ItemNeighbors, "0"), "/api/item/0/neighbors"},
		{"Item Neighbors in Category", cache.Key(cache.ItemNeighbors, "0", "0"), "/api/item/0/neighbors/0"},
		{"Latest Items", cache.LatestItems, "/api/latest/"},
		{"Latest Items in Category", cache.Key(cache.LatestItems, "0"), "/api/latest/0"},
		{"Popular Items", cache.PopularItems, "/api/popular/"},
		{"Popular Items in Category", cache.Key(cache.PopularItems, "0"), "/api/popular/0"},
		{"Offline Recommend", cache.Key(cache.OfflineRecommend, "0"), "/api/intermediate/recommend/0"},
		{"Offline Recommend in Category", cache.Key(cache.OfflineRecommend, "0", "0"), "/api/intermediate/recommend/0/0"},
	}

	for i, operator := range operators {
		t.Run(operator.Name, func(t *testing.T) {
			// Put scores
			scores := []cache.Scored{
				{strconv.Itoa(i) + "0", 100},
				{strconv.Itoa(i) + "1", 99},
				{strconv.Itoa(i) + "2", 98},
				{strconv.Itoa(i) + "3", 97},
				{strconv.Itoa(i) + "4", 96},
			}
			err := s.CacheClient.SetSorted(operator.Key, scores)
			assert.NoError(t, err)
			err = NewCacheModification(s.CacheClient, s.HiddenItemsManager).HideItem(strconv.Itoa(i) + "3").Exec()
			assert.NoError(t, err)
			apitest.New().
				Handler(s.handler).
				Get(operator.Get).
				Header("X-API-Key", apiKey).
				Expect(t).
				Status(http.StatusOK).
				Body(marshal(t, []cache.Scored{scores[0], scores[1], scores[2], scores[4]})).
				End()
			apitest.New().
				Handler(s.handler).
				Get(operator.Get).
				Header("X-API-Key", apiKey).
				QueryParams(map[string]string{
					"offset": "0",
					"n":      "3"}).
				Expect(t).
				Status(http.StatusOK).
				Body(marshal(t, []cache.Scored{scores[0], scores[1], scores[2]})).
				End()
			apitest.New().
				Handler(s.handler).
				Get(operator.Get).
				Header("X-API-Key", apiKey).
				QueryParams(map[string]string{
					"offset": "1",
					"n":      "3"}).
				Expect(t).
				Status(http.StatusOK).
				Body(marshal(t, []cache.Scored{scores[1], scores[2], scores[4]})).
				End()
			apitest.New().
				Handler(s.handler).
				Get(operator.Get).
				Header("X-API-Key", apiKey).
				QueryParams(map[string]string{
					"offset": "0",
					"n":      "0"}).
				Expect(t).
				Status(http.StatusOK).
				Body(marshal(t, []cache.Scored{scores[0], scores[1], scores[2], scores[4]})).
				End()
		})
	}
}

func TestServer_DeleteFeedback(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)
	// Insert feedback
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "type1", UserId: "2", ItemId: "3"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "type2", UserId: "2", ItemId: "3"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "type3", UserId: "2", ItemId: "3"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "type1", UserId: "1", ItemId: "6"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "type1", UserId: "4", ItemId: "8"}},
	}
	apitest.New().
		Handler(s.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 5}`).
		End()
	// Get Feedback
	apitest.New().
		Handler(s.handler).
		Get("/api/feedback/2/3").
		Header("X-API-Key", apiKey).
		Expect(t).
		Body(marshal(t, []data.Feedback{feedback[0], feedback[1], feedback[2]})).
		Status(http.StatusOK).
		End()
	// Get typed feedback
	apitest.New().
		Handler(s.handler).
		Get("/api/feedback/type2/2/3").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, feedback[1])).
		End()
	// delete feedback
	apitest.New().
		Handler(s.handler).
		Delete("/api/feedback/2/3").
		Header("X-API-Key", apiKey).
		Expect(t).
		Body(`{"RowAffected": 3}`).
		Status(http.StatusOK).
		End()
	// delete typed feedback
	apitest.New().
		Handler(s.handler).
		Delete("/api/feedback/type1/4/8").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
}

func TestServer_Measurement(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)
	measurements := []Measurement{
		{"Test_NDCG", time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC).Local(), 0},
		{"Test_NDCG", time.Date(2001, 1, 1, 1, 1, 1, 0, time.UTC).Local(), 1},
		{"Test_NDCG", time.Date(2002, 1, 1, 1, 1, 1, 0, time.UTC).Local(), 2},
		{"Test_NDCG", time.Date(2003, 1, 1, 1, 1, 1, 0, time.UTC).Local(), 3},
		{"Test_NDCG", time.Date(2004, 1, 1, 1, 1, 1, 0, time.UTC).Local(), 4},
		{"Test_Recall", time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC).Local(), 1},
	}
	err := s.InsertMeasurement(measurements...)
	assert.NoError(t, err)
	apitest.New().
		Handler(s.handler).
		Get("/api/measurements/Test_NDCG").
		Query("n", "3").
		Header("X-API-Key", apiKey).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []Measurement{
			measurements[4],
			measurements[3],
			measurements[2],
		})).
		End()
	// test auth fail
	apitest.New().
		Handler(s.handler).
		Get("/api/measurements/Test_NDCG").
		Query("n", "3").
		Expect(t).
		Status(http.StatusUnauthorized).
		End()
}

func TestServer_GetRecommends(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)
	// insert hidden items
	err := s.CacheClient.SetSorted(cache.Key(cache.OfflineRecommend, "0"), []cache.Scored{{"0", 100}})
	assert.NoError(t, err)
	err = NewCacheModification(s.CacheClient, s.HiddenItemsManager).HideItem("0").Exec()
	assert.NoError(t, err)
	// insert recommendation
	err = s.CacheClient.SetSorted(cache.Key(cache.OfflineRecommend, "0"), []cache.Scored{
		{Id: "1", Score: 99},
		{Id: "2", Score: 98},
		{Id: "3", Score: 97},
		{Id: "4", Score: 96},
		{Id: "5", Score: 95},
		{Id: "6", Score: 94},
		{Id: "7", Score: 93},
		{Id: "8", Score: 92},
	})
	assert.NoError(t, err)
	// insert feedback
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "4"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "1"}, Timestamp: time.Now().Add(time.Hour)},
	}
	apitest.New().
		Handler(s.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 3}`).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"1", "3", "5"})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n":      "3",
			"offset": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"6", "7", "8"})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n":      "3",
			"offset": "10000",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n":               "3",
			"write-back-type": "read",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"1", "3", "5"})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n":                "3",
			"write-back-type":  "read",
			"write-back-delay": "10m",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"6", "7", "8"})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"6", "7", "8"})).
		End()
}

func TestServer_GetRecommends_Replacement(t *testing.T) {
	s := newMockServer(t)
	s.Config.Recommend.Replacement.EnableReplacement = true
	defer s.Close(t)
	// insert hidden items
	err := s.CacheClient.SetSorted(cache.Key(cache.OfflineRecommend, "0"), []cache.Scored{{"0", 100}})
	assert.NoError(t, err)
	err = NewCacheModification(s.CacheClient, s.HiddenItemsManager).HideItem("0").Exec()
	assert.NoError(t, err)
	// insert recommendation
	err = s.CacheClient.SetSorted(cache.Key(cache.OfflineRecommend, "0"), []cache.Scored{
		{Id: "1", Score: 99},
		{Id: "2", Score: 98},
		{Id: "3", Score: 97},
		{Id: "4", Score: 96},
		{Id: "5", Score: 95},
		{Id: "6", Score: 94},
		{Id: "7", Score: 93},
		{Id: "8", Score: 92},
	})
	assert.NoError(t, err)
	// insert feedback
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "4"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "1"}, Timestamp: time.Now().Add(time.Hour)},
	}
	apitest.New().
		Handler(s.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 3}`).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"1", "2", "3"})).
		End()
}

func TestServer_GetRecommends_Fallback_ItemBasedSimilar(t *testing.T) {
	s := newMockServer(t)
	s.Config.Recommend.Online.NumFeedbackFallbackItemBased = 4
	s.Config.Recommend.DataSource.PositiveFeedbackTypes = []string{"a"}
	defer s.Close(t)
	// insert recommendation
	err := s.CacheClient.SetSorted(cache.Key(cache.OfflineRecommend, "0"),
		[]cache.Scored{{"1", 99}, {"2", 98}, {"3", 97}, {"4", 96}})
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
		Handler(s.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 5}`).
		End()

	// insert similar items
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "1"), []cache.Scored{
		{"2", 100000},
		{"9", 1},
	})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "2"), []cache.Scored{
		{"3", 100000},
		{"8", 1},
		{"9", 1},
	})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "3"), []cache.Scored{
		{"4", 100000},
		{"7", 1},
		{"8", 1},
		{"9", 1},
	})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "4"), []cache.Scored{
		{"1", 100000},
		{"6", 1},
		{"7", 1},
		{"8", 1},
		{"9", 1},
	})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "5"), []cache.Scored{
		{"1", 1},
		{"6", 1},
		{"7", 100000},
		{"8", 100},
		{"9", 1},
	})
	assert.NoError(t, err)

	// insert similar items of category *
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "1", "*"), []cache.Scored{
		{"9", 1},
	})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "2", "*"), []cache.Scored{
		{"3", 100000},
		{"9", 1},
	})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "3", "*"), []cache.Scored{
		{"7", 1},
		{"9", 1},
	})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "4", "*"), []cache.Scored{
		{"1", 100000},
		{"7", 1},
		{"9", 1},
	})
	assert.NoError(t, err)

	// test fallback
	s.Config.Recommend.Online.FallbackRecommend = []string{"item_based"}
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"9", "8", "7"})).
		End()
	s.Config.Recommend.Online.FallbackRecommend = []string{"item_based"}
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"9", "7"})).
		End()
}

func TestServer_GetRecommends_Fallback_UserBasedSimilar(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)
	// insert recommendation
	err := s.CacheClient.SetSorted(cache.Key(cache.OfflineRecommend, "0"),
		[]cache.Scored{{"1", 99}, {"2", 98}, {"3", 97}, {"4", 96}})
	assert.NoError(t, err)
	// insert feedback
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "1"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "2"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "3"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "4"}},
	}
	apitest.New().
		Handler(s.handler).
		Post("/api/feedback").
		Header("X-API-Key", apiKey).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 4}`).
		End()
	// insert similar users
	err = s.CacheClient.SetSorted(cache.Key(cache.UserNeighbors, "0"), []cache.Scored{
		{"1", 2},
		{"2", 1.5},
		{"3", 1},
	})
	assert.NoError(t, err)
	err = s.DataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "1", ItemId: "11"}},
	}, true, true, true)
	assert.NoError(t, err)
	err = s.DataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "12"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "2", ItemId: "48"}},
	}, true, true, true)
	assert.NoError(t, err)
	err = s.DataClient.BatchInsertFeedback([]data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "13"}},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "3", ItemId: "48"}},
	}, true, true, true)
	assert.NoError(t, err)
	// insert categorized items
	err = s.DataClient.BatchInsertItems([]data.Item{
		{ItemId: "12", Categories: []string{"*"}},
		{ItemId: "48", Categories: []string{"*"}},
	})
	assert.NoError(t, err)
	// test fallback
	s.Config.Recommend.Online.FallbackRecommend = []string{"user_based"}
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"48", "11", "12"})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"48", "12"})).
		End()
}

func TestServer_GetRecommends_Fallback_PreCached(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)
	// insert offline recommendation
	err := s.CacheClient.SetSorted(cache.Key(cache.OfflineRecommend, "0"),
		[]cache.Scored{{"1", 99}, {"2", 98}, {"3", 97}, {"4", 96}})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.OfflineRecommend, "0", "*"),
		[]cache.Scored{{"101", 99}, {"102", 98}, {"103", 97}, {"104", 96}})
	assert.NoError(t, err)
	// insert latest
	err = s.CacheClient.SetSorted(cache.LatestItems,
		[]cache.Scored{{"5", 95}, {"6", 94}, {"7", 93}, {"8", 92}})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.LatestItems, "*"),
		[]cache.Scored{{"105", 95}, {"106", 94}, {"107", 93}, {"108", 92}})
	assert.NoError(t, err)
	// insert popular
	err = s.CacheClient.SetSorted(cache.PopularItems,
		[]cache.Scored{{"9", 91}, {"10", 90}, {"11", 89}, {"12", 88}})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.PopularItems, "*"),
		[]cache.Scored{{"109", 91}, {"110", 90}, {"111", 89}, {"112", 88}})
	assert.NoError(t, err)
	// insert collaborative filtering
	err = s.CacheClient.SetSorted(cache.Key(cache.CollaborativeRecommend, "0"),
		[]cache.Scored{{"13", 79}, {"14", 78}, {"15", 77}, {"16", 76}})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.CollaborativeRecommend, "0", "*"),
		[]cache.Scored{{"113", 79}, {"114", 78}, {"115", 77}, {"116", 76}})
	assert.NoError(t, err)
	// test popular fallback
	s.Config.Recommend.Online.FallbackRecommend = []string{"popular"}
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "8",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"1", "2", "3", "4", "9", "10", "11", "12"})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "8",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"101", "102", "103", "104", "109", "110", "111", "112"})).
		End()
	// test latest fallback
	s.Config.Recommend.Online.FallbackRecommend = []string{"latest"}
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "8",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"1", "2", "3", "4", "5", "6", "7", "8"})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "8",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"101", "102", "103", "104", "105", "106", "107", "108"})).
		End()
	// test collaborative filtering
	s.Config.Recommend.Online.FallbackRecommend = []string{"collaborative"}
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "8",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"1", "2", "3", "4", "13", "14", "15", "16"})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "8",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []string{"101", "102", "103", "104", "113", "114", "115", "116"})).
		End()
	// test wrong fallback
	s.Config.Recommend.Online.FallbackRecommend = []string{""}
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/0").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "8",
		}).
		Expect(t).
		Status(http.StatusInternalServerError).
		End()
}

func TestServer_SessionRecommend(t *testing.T) {
	s := newMockServer(t)
	s.Config.Recommend.Online.NumFeedbackFallbackItemBased = 4
	s.Config.Recommend.DataSource.PositiveFeedbackTypes = []string{"a"}
	defer s.Close(t)

	// insert hidden items
	apitest.New().
		Handler(s.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "100", IsHidden: true}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()

	// insert similar items
	err := s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "1"), []cache.Scored{
		{"2", 100000},
		{"9", 1},
		{"100", 100000},
	})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "2"), []cache.Scored{
		{"3", 100000},
		{"8", 1},
		{"9", 1},
	})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "3"), []cache.Scored{
		{"4", 100000},
		{"7", 1},
		{"8", 1},
		{"9", 1},
	})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "4"), []cache.Scored{
		{"1", 100000},
		{"6", 1},
		{"7", 1},
		{"8", 1},
		{"9", 1},
	})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "5"), []cache.Scored{
		{"1", 1},
		{"6", 1},
		{"7", 100000},
		{"8", 100},
		{"9", 1},
	})
	assert.NoError(t, err)

	// insert similar items of category *
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "1", "*"), []cache.Scored{
		{"9", 1},
	})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "2", "*"), []cache.Scored{
		{"3", 100000},
		{"9", 1},
	})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "3", "*"), []cache.Scored{
		{"7", 1},
		{"9", 1},
	})
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "4", "*"), []cache.Scored{
		{"1", 100000},
		{"7", 1},
		{"9", 1},
	})
	assert.NoError(t, err)

	// test fallback
	feedback := []data.Feedback{
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "1"}, Timestamp: time.Date(2010, 1, 1, 1, 1, 1, 1, time.UTC)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "2"}, Timestamp: time.Date(2009, 1, 1, 1, 1, 1, 1, time.UTC)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "3"}, Timestamp: time.Date(2008, 1, 1, 1, 1, 1, 1, time.UTC)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "4"}, Timestamp: time.Date(2007, 1, 1, 1, 1, 1, 1, time.UTC)},
		{FeedbackKey: data.FeedbackKey{FeedbackType: "a", UserId: "0", ItemId: "5"}, Timestamp: time.Date(2006, 1, 1, 1, 1, 1, 1, time.UTC)},
	}
	apitest.New().
		Handler(s.handler).
		Post("/api/session/recommend").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{{"9", 4}, {"8", 3}, {"7", 2}})).
		End()
	apitest.New().
		Handler(s.handler).
		Post("/api/session/recommend").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"offset": "100",
		}).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored(nil))).
		End()
	s.Config.Recommend.Online.FallbackRecommend = []string{"item_based"}
	apitest.New().
		Handler(s.handler).
		Post("/api/session/recommend/*").
		Header("X-API-Key", apiKey).
		QueryParams(map[string]string{
			"n": "3",
		}).
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, []cache.Scored{{"9", 4}, {"7", 2}})).
		End()
}

func TestServer_Visibility(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)

	// insert items
	var items []Item
	for i := 0; i < 5; i++ {
		items = append(items, Item{
			ItemId:     strconv.Itoa(i),
			Categories: []string{"a"},
			Timestamp:  time.Date(1989, 6, i+1, 1, 1, 1, 1, time.UTC).String(),
		})
	}
	apitest.New().
		Handler(s.handler).
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
	var scores []cache.Scored
	for i := range items {
		scores = append(scores, cache.Scored{Id: strconv.Itoa(i), Score: float64(time.Date(1989, 6, i+1, 1, 1, 1, 1, time.UTC).Unix())})
	}
	lo.Reverse(scores)
	err := s.CacheClient.SetSorted(cache.LatestItems, scores)
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.PopularItems, scores)
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "100"), scores)
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.OfflineRecommend, "100"), scores)
	assert.NoError(t, err)

	// delete item
	apitest.New().
		Handler(s.handler).
		Delete("/api/item/0").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		End()
	// modify item
	apitest.New().
		Handler(s.handler).
		Patch("/api/item/1").
		Header("X-API-Key", apiKey).
		JSON(data.ItemPatch{IsHidden: proto.Bool(true)}).
		Expect(t).
		Status(http.StatusOK).
		End()
	// overwrite item
	apitest.New().
		Handler(s.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "2", IsHidden: true}).
		Expect(t).
		Status(http.StatusOK).
		End()

	// recommend
	apitest.New().
		Handler(s.handler).
		Get("/api/popular").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, scores[:2])).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/latest").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, scores[:2])).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/item/100/neighbors/").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, scores[:2])).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/100/").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, cache.RemoveScores(scores[:2]))).
		End()

	// insert item
	apitest.New().
		Handler(s.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "0", Timestamp: time.Date(1989, 6, 1, 1, 1, 1, 1, time.UTC).String()}).
		Expect(t).
		Status(http.StatusOK).
		End()
	// modify item
	apitest.New().
		Handler(s.handler).
		Patch("/api/item/1").
		Header("X-API-Key", apiKey).
		JSON(data.ItemPatch{IsHidden: proto.Bool(false)}).
		Expect(t).
		Status(http.StatusOK).
		End()
	// overwrite item
	apitest.New().
		Handler(s.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "2", IsHidden: false, Timestamp: time.Date(1989, 6, 3, 1, 1, 1, 1, time.UTC).String()}).
		Expect(t).
		Status(http.StatusOK).
		End()

	// recommend
	apitest.New().
		Handler(s.handler).
		Get("/api/popular").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, scores)).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/latest").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, scores)).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/item/100/neighbors/").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, scores)).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/100/").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, cache.RemoveScores(scores))).
		End()

	// insert cache
	err = s.CacheClient.SetSorted(cache.Key(cache.LatestItems, "a"), scores)
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.PopularItems, "a"), scores)
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.ItemNeighbors, "100", "a"), scores)
	assert.NoError(t, err)
	err = s.CacheClient.SetSorted(cache.Key(cache.OfflineRecommend, "100", "a"), scores)
	assert.NoError(t, err)

	// delete category
	apitest.New().
		Handler(s.handler).
		Delete("/api/item/0/category/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		End()
	// modify category
	apitest.New().
		Handler(s.handler).
		Patch("/api/item/1").
		Header("X-API-Key", apiKey).
		JSON(data.ItemPatch{Categories: []string{}}).
		Expect(t).
		Status(http.StatusOK).
		End()
	// overwrite category
	apitest.New().
		Handler(s.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "2", Categories: []string{}}).
		Expect(t).
		Status(http.StatusOK).
		End()

	// recommend
	apitest.New().
		Handler(s.handler).
		Get("/api/popular/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, scores[:2])).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/latest/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, scores[:2])).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/item/100/neighbors/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, scores[:2])).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/100/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, cache.RemoveScores(scores[:2]))).
		End()

	// delete category
	apitest.New().
		Handler(s.handler).
		Put("/api/item/0/category/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		End()
	// modify category
	apitest.New().
		Handler(s.handler).
		Patch("/api/item/1").
		Header("X-API-Key", apiKey).
		JSON(data.ItemPatch{Categories: []string{"a"}}).
		Expect(t).
		Status(http.StatusOK).
		End()
	// overwrite category
	apitest.New().
		Handler(s.handler).
		Post("/api/item").
		Header("X-API-Key", apiKey).
		JSON(Item{ItemId: "2", Categories: []string{"a"}}).
		Expect(t).
		Status(http.StatusOK).
		End()

	// recommend
	apitest.New().
		Handler(s.handler).
		Get("/api/popular/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, scores)).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/latest/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, scores)).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/item/100/neighbors/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, scores)).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/api/recommend/100/a").
		Header("X-API-Key", apiKey).
		JSON(items).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, cache.RemoveScores(scores))).
		End()
}
