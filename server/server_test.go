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
	"github.com/emicklei/go-restful"
	"github.com/steinfletcher/apitest"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"net/http"
	"testing"
	"time"
)

type mockServer struct {
	dataStoreServer  *miniredis.Miniredis
	cacheStoreServer *miniredis.Miniredis
	dataStoreClient  data.Database
	cacheStoreClient cache.Database
	handler          *restful.Container
}

func newMockServer(t *testing.T) *mockServer {
	s := new(mockServer)
	// create mock redis server
	var err error
	s.dataStoreServer, err = miniredis.Run()
	assert.Nil(t, err)
	s.cacheStoreServer, err = miniredis.Run()
	assert.Nil(t, err)
	// open database
	s.dataStoreClient, err = data.Open("redis://" + s.dataStoreServer.Addr())
	assert.Nil(t, err)
	s.cacheStoreClient, err = cache.Open("redis://" + s.cacheStoreServer.Addr())
	assert.Nil(t, err)
	// create server
	server := &Server{
		DataStore:  s.dataStoreClient,
		CacheStore: s.cacheStoreClient,
		Config:     (*config.Config)(nil).LoadDefaultIfNil(),
	}
	ws := server.CreateWebService()
	// create handler
	s.handler = restful.NewContainer()
	s.handler.Add(ws)
	return s
}

func (s *mockServer) Close(t *testing.T) {
	err := s.dataStoreClient.Close()
	assert.Nil(t, err)
}

func marshal(t *testing.T, v interface{}) string {
	s, err := json.Marshal(v)
	assert.Nil(t, err)
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
		Post("/user").
		JSON(users[0]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected":1}`).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/user/0").
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, users[0])).
		End()
	apitest.New().
		Handler(s.handler).
		Post("/users").
		JSON(users[1:]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected":4}`).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/users").
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
		Delete("/user/0").
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/user/0").
		Expect(t).
		Status(http.StatusInternalServerError).
		End()
}

func TestServer_Items(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)
	// Items
	items := []data.Item{
		{
			ItemId:    "0",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a"},
		},
		{
			ItemId:    "2",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a"},
		},
		{
			ItemId:    "4",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a", "b"},
		},
		{
			ItemId:    "6",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"b"},
		},
		{
			ItemId:    "8",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"b"},
		},
	}
	// insert items
	apitest.New().
		Handler(s.handler).
		Post("/item").
		JSON(items[0]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	// batch insert items
	apitest.New().
		Handler(s.handler).
		Post("/items").
		JSON(items[1:]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 4}`).
		End()
	//get items
	apitest.New().
		Handler(s.handler).
		Get("/items").
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
	// delete item
	apitest.New().
		Handler(s.handler).
		Delete("/item/0").
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	// get item
	apitest.New().
		Handler(s.handler).
		Get("/item/0").
		Expect(t).
		Status(http.StatusInternalServerError).
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
		Post("/feedback").
		JSON(feedback).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 5}`).
		End()
	//Get Feedback
	apitest.New().
		Handler(s.handler).
		Get("/feedback").
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
	apitest.New().
		Handler(s.handler).
		Get("/feedback").
		QueryParams(map[string]string{
			"cursor": "",
			"n":      "3",
		}).
		Expect(t).
		Body(marshal(t, FeedbackIterator{
			Cursor:   "feedback/{\"UserId\":\"3\",\"ItemId\":\"6\"}",
			Feedback: feedback[:3],
		})).
		Status(http.StatusOK).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/feedback").
		QueryParams(map[string]string{
			"cursor": "feedback/{\"UserId\":\"3\",\"ItemId\":\"6\"}",
			"n":      "3",
		}).
		Expect(t).
		Body(marshal(t, FeedbackIterator{
			Cursor:   "",
			Feedback: feedback[3:],
		})).
		Status(http.StatusOK).
		End()
	//Get Items
	apitest.New().
		Handler(s.handler).
		Get("/items").
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
		Get("/users").
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
		Get("/user/2/feedback").
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"UserId": "2", "ItemId": "4"}]`).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/item/4/feedback").
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"UserId": "2", "ItemId": "4"}]`).
		End()
}

func TestServer_List(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)
	type ListOperator struct {
		Prefix string
		Label  string
		Get    string
	}
	operators := []ListOperator{
		{cache.MatchedItems, "0", "/user/0/match"},
		{cache.LatestItems, "", "/latest/"},
		{cache.PopularItems, "", "/popular/"},
		{cache.SimilarItems, "0", "/item/0/neighbors"},
	}

	for _, operator := range operators {
		// Put items
		items := []string{"0", "1", "2", "3", "4"}
		if err := s.cacheStoreClient.SetList(operator.Prefix, operator.Label, items); err != nil {
			t.Fatal(err)
		}
		apitest.New().
			Handler(s.handler).
			Get(operator.Get).
			Expect(t).
			Status(http.StatusOK).
			Body(`["0", "1", "2", "3", "4"]`).
			End()
		apitest.New().
			Handler(s.handler).
			Get(operator.Get).
			QueryParams(map[string]string{
				"n":      "3",
				"offset": "0"}).
			Expect(t).
			Status(http.StatusOK).
			Body(`["0", "1", "2"]`).
			End()
		apitest.New().
			Handler(s.handler).
			Get(operator.Get).
			QueryParams(map[string]string{
				"n":      "3",
				"offset": "1"}).
			Expect(t).
			Status(http.StatusOK).
			Body(`["1", "2", "3"]`).
			End()
		// get empty
		apitest.New().
			Handler(s.handler).
			Get(operator.Get).
			QueryParams(map[string]string{
				"n":      "0",
				"offset": "0"}).
			Expect(t).
			Status(http.StatusOK).
			Body(``).
			End()
	}
}

//func TestServer_GetRecommends(t *testing.T) {
//	s := newMockServer(t)
//	defer s.Close(t)
//	// Put recommends
//	items := []storage.RecommendedItem{
//		{"0", 0.0},
//		{"1", 0.1},
//		{"2", 0.2},
//		{"3", 0.3},
//		{"4", 0.4},
//		{"5", 0.5},
//		{"6", 0.6},
//		{"7", 0.7},
//		{"8", 0.8},
//		{"9", 0.9},
//	}
//	err := s.dataStoreClient.SetRecommend("0", items)
//	assert.Nil(t, err)
//	// Put feedback
//	apitest.New().
//		Handler(s.handler).
//		Post("/feedback").
//		JSON([]storage.Feedback{
//			{UserId: "0", ItemId: "0"},
//			{UserId: "0", ItemId: "1"},
//		}).
//		Expect(t).
//		Status(http.StatusOK).
//		End()
//	apitest.New().
//		Handler(s.handler).
//		Post("/user/0/ignore").
//		JSON([]string{"2", "3"}).
//		Expect(t).
//		Status(http.StatusOK).
//		End()
//	apitest.New().
//		Handler(s.handler).
//		Get("/recommend/0").
//		QueryParams(map[string]string{
//			"n": "4",
//		}).
//		Expect(t).
//		Status(http.StatusOK).
//		Body(marshal(t, items[4:8])).
//		End()
//	// Consume
//	apitest.New().
//		Handler(s.handler).
//		Get("/recommend/0").
//		QueryParams(map[string]string{
//			"n":       "4",
//			"consume": "1",
//		}).
//		Expect(t).
//		Status(http.StatusOK).
//		Body(marshal(t, items[4:8])).
//		End()
//	apitest.New().
//		Handler(s.handler).
//		Get("/recommend/0").
//		QueryParams(map[string]string{
//			"n": "4",
//		}).
//		Expect(t).
//		Status(http.StatusOK).
//		Body(marshal(t, items[8:])).
//		End()
//}
