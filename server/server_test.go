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
	"github.com/emicklei/go-restful"
	"github.com/steinfletcher/apitest"
	"github.com/stretchr/testify/assert"
	"github.com/thanhpk/randstr"
	"github.com/zhenghaoz/gorse/storage"
	"net/http"
	"os"
	"path"
	"testing"
	"time"
)

type mockServer struct {
	db      storage.Database
	handler *restful.Container
}

func newMockServer(t *testing.T) *mockServer {
	s := new(mockServer)
	// create data folder
	dir := path.Join(os.TempDir(), randstr.String(16))
	err := os.Mkdir(dir, 0777)
	assert.Nil(t, err)
	// open database
	s.db, err = storage.Open("badger://" + dir)
	assert.Nil(t, err)
	// create server
	server := NewServer(s.db, nil)
	ws := server.CreateWebService()
	// create handler
	s.handler = restful.NewContainer()
	s.handler.Add(ws)
	return s
}

func (s *mockServer) Close(t *testing.T) {
	err := s.db.Close()
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
	users := []storage.User{
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
		Get("/users").
		QueryParams(map[string]string{
			"cursor": "",
			"n":      "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, UserIterator{
			Cursor: "user/3",
			Users:  users[:3],
		})).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/users").
		QueryParams(map[string]string{
			"cursor": "user/3",
			"n":      "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, UserIterator{
			Cursor: "",
			Users:  users[3:],
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
	items := []storage.Item{
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
	// get items[0:3]
	apitest.New().
		Handler(s.handler).
		Get("/items").
		QueryParams(map[string]string{
			"cursor": "",
			"n":      "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, ItemIterator{
			Cursor: "item/6",
			Items:  items[:3],
		})).
		End()
	// get items[3:4]
	apitest.New().
		Handler(s.handler).
		Get("/items").
		QueryParams(map[string]string{
			"cursor": "item/6",
			"n":      "3",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, ItemIterator{
			Cursor: "",
			Items:  items[3:],
		})).
		End()
	//get labels
	apitest.New().
		Handler(s.handler).
		Get("/labels").
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, LabelIterator{
			Cursor: "",
			Labels: []string{"a", "b"},
		})).
		End()
	// get items by label
	apitest.New().
		Handler(s.handler).
		Get("/label/a/items").
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"ItemId": "0","Timestamp": "1996-03-15T00:00:00Z","Labels": ["a"]}, {"ItemId": "2", "Timestamp": "1996-03-15T00:00:00Z", "Labels": ["a"]},{"ItemId": "4", "Timestamp": "1996-03-15T00:00:00Z","Labels":["a","b"]}]`).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/label/b/items").
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"ItemId": "4", "Timestamp": "1996-03-15T00:00:00Z","Labels":["a","b"]}, {"ItemId": "6","Timestamp": "1996-03-15T00:00:00Z","Labels":["b"]},{"ItemId": "8", "Timestamp": "1996-03-15T00:00:00Z", "Labels":["b"]}]`).
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
	feedback := []storage.Feedback{
		{"0", "0"},
		{"1", "2"},
		{"2", "4"},
		{"3", "6"},
		{"4", "8"},
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
			Items: []storage.Item{
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
			Users:  []storage.User{{"0"}, {"1"}, {"2"}, {"3"}, {"4"}},
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

func TestServer_Ignore(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)
	// Insert ignore
	apitest.New().
		Handler(s.handler).
		Post("/user/0/ignore").
		JSON([]string{"0", "1", "2", "3", "4", "5"}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/user/0/ignore").
		Expect(t).
		Status(http.StatusOK).
		Body(`["0","1","2","3","4","5"]`).
		End()
}

func TestServer_List(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)
	type ListOperator struct {
		Set func(label string, items []storage.RecommendedItem) error
		Get string
	}
	operators := []ListOperator{
		{s.db.SetRecommend, "/user/0/recommend/cache"},
		{s.db.SetLatest, "/latest/0"},
		{s.db.SetPop, "/popular/0"},
		{s.db.SetNeighbors, "/item/0/neighbors"},
	}

	for _, operator := range operators {
		t.Log("testing", operator.Get)
		// Put items
		items := []storage.RecommendedItem{
			{"0", 0.0},
			{"1", 0.1},
			{"2", 0.2},
			{"3", 0.3},
			{"4", 0.4},
		}
		if err := operator.Set("0", items); err != nil {
			t.Fatal(err)
		}
		apitest.New().
			Handler(s.handler).
			Get(operator.Get).
			Expect(t).
			Status(http.StatusOK).
			Body(`[{"ItemId": "0", "Score":0.0}, {"ItemId": "1", "Score":0.1}, {"ItemId": "2", "Score":0.2}, {"ItemId": "3", "Score":0.3}, {"ItemId": "4", "Score":0.4}]`).
			End()
		apitest.New().
			Handler(s.handler).
			Get(operator.Get).
			QueryParams(map[string]string{
				"n":      "3",
				"offset": "0"}).
			Expect(t).
			Status(http.StatusOK).
			Body(`[{"ItemId": "0", "Score":0.0}, {"ItemId": "1", "Score":0.1}, {"ItemId": "2", "Score":0.2}]`).
			End()
		apitest.New().
			Handler(s.handler).
			Get(operator.Get).
			QueryParams(map[string]string{
				"n":      "3",
				"offset": "1"}).
			Expect(t).
			Status(http.StatusOK).
			Body(`[{"ItemId": "1", "Score":0.1}, {"ItemId": "2", "Score":0.2}, {"ItemId": "3", "Score":0.3}]`).
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

func TestServer_GetRecommends(t *testing.T) {
	s := newMockServer(t)
	defer s.Close(t)
	// Put recommends
	items := []storage.RecommendedItem{
		{"0", 0.0},
		{"1", 0.1},
		{"2", 0.2},
		{"3", 0.3},
		{"4", 0.4},
		{"5", 0.5},
		{"6", 0.6},
		{"7", 0.7},
		{"8", 0.8},
		{"9", 0.9},
	}
	err := s.db.SetRecommend("0", items)
	assert.Nil(t, err)
	// Put feedback
	apitest.New().
		Handler(s.handler).
		Post("/feedback").
		JSON([]storage.Feedback{
			{UserId: "0", ItemId: "0"},
			{UserId: "0", ItemId: "1"},
		}).
		Expect(t).
		Status(http.StatusOK).
		End()
	apitest.New().
		Handler(s.handler).
		Post("/user/0/ignore").
		JSON([]string{"2", "3"}).
		Expect(t).
		Status(http.StatusOK).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/recommend/0").
		QueryParams(map[string]string{
			"n": "4",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, items[4:8])).
		End()
	// Consume
	apitest.New().
		Handler(s.handler).
		Get("/recommend/0").
		QueryParams(map[string]string{
			"n":       "4",
			"consume": "1",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, items[4:8])).
		End()
	apitest.New().
		Handler(s.handler).
		Get("/recommend/0").
		QueryParams(map[string]string{
			"n": "4",
		}).
		Expect(t).
		Status(http.StatusOK).
		Body(marshal(t, items[8:])).
		End()
}
