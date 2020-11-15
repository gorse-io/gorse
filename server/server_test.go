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
	"github.com/emicklei/go-restful"
	"github.com/steinfletcher/apitest"
	"github.com/thanhpk/randstr"
	"github.com/zhenghaoz/gorse/storage"
	"log"
	"net/http"
	"os"
	"path"
	"testing"
	"time"
)

func setupServer() storage.Database {
	dir := path.Join(os.TempDir(), randstr.String(16))
	err := os.Mkdir(dir, 0777)
	if err != nil {
		log.Fatal(err)
	}
	db, err := storage.Open(dir)
	if err != nil {
		log.Fatal(err)
	}
	server := Server{DB: db}
	server.SetupWebService()
	return db
}

func TestServer_Users(t *testing.T) {
	// Setup
	db := setupServer()
	defer db.Close()
	// Run server using httptest
	apitest.New().
		Handler(restful.DefaultContainer).
		Post("/user").
		JSON(storage.User{UserId: "0"}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected":1}`).
		End()
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/user/0").
		Expect(t).
		Status(http.StatusOK).
		Body(`{"UserId": "0"}`).
		End()
	apitest.New().
		Handler(restful.DefaultContainer).
		Post("/users").
		JSON([4]storage.User{{UserId: "1"}, {UserId: "2"}, {UserId: "3"}, {UserId: "4"}}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected":4}`).
		End()
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/users").
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"UserId": "0"}, {"UserId": "1"}, {"UserId": "2"}, {"UserId": "3"}, {"UserId": "4"}]`).
		End()
	apitest.New().
		Handler(restful.DefaultContainer).
		Delete("/user/0").
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/user/0").
		Expect(t).
		Status(http.StatusInternalServerError).
		End()
}

func TestServer_Items(t *testing.T) {
	// Setup
	db := setupServer()
	defer db.Close()
	// Run server using httptest

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
		Handler(restful.DefaultContainer).
		Post("/items").
		JSON(items[0:1]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	// batch insert items
	apitest.New().
		Handler(restful.DefaultContainer).
		Post("/items").
		JSON(items[1:]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 4}`).
		End()
	//get items
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/items").
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"ItemId": "0", "Timestamp": "1996-03-15T00:00:00Z","Labels": ["a"]}, {"ItemId": "2", "Timestamp": "1996-03-15T00:00:00Z", "Labels": ["a"]},{"ItemId": "4", "Timestamp": "1996-03-15T00:00:00Z","Labels":["a","b"]}, {"ItemId": "6","Timestamp": "1996-03-15T00:00:00Z","Labels":["b"]},{"ItemId": "8", "Timestamp": "1996-03-15T00:00:00Z", "Labels":["b"]}]`).
		End()
	// get items[0:3]
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/items").QueryParams(map[string]string{
		"number": "3",
		"offset": "0"}).
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"ItemId": "0","Timestamp": "1996-03-15T00:00:00Z","Labels": ["a"]}, {"ItemId": "2", "Timestamp": "1996-03-15T00:00:00Z", "Labels": ["a"]},{"ItemId": "4", "Timestamp": "1996-03-15T00:00:00Z","Labels":["a","b"]}]`).
		End()
	// get items[1:4]
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/items").QueryParams(map[string]string{
		"number": "3",
		"offset": "1"}).
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"ItemId": "2", "Timestamp": "1996-03-15T00:00:00Z", "Labels": ["a"]},{"ItemId": "4", "Timestamp": "1996-03-15T00:00:00Z","Labels":["a","b"]}, {"ItemId": "6","Timestamp": "1996-03-15T00:00:00Z","Labels":["b"]}]`).
		End()
	//get labels
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/labels").
		Expect(t).
		Status(http.StatusOK).
		Body(`["a","b"]`).
		End()
	// get items by label
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/label/a/items").
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"ItemId": "0","Timestamp": "1996-03-15T00:00:00Z","Labels": ["a"]}, {"ItemId": "2", "Timestamp": "1996-03-15T00:00:00Z", "Labels": ["a"]},{"ItemId": "4", "Timestamp": "1996-03-15T00:00:00Z","Labels":["a","b"]}]`).
		End()
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/label/b/items").
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"ItemId": "4", "Timestamp": "1996-03-15T00:00:00Z","Labels":["a","b"]}, {"ItemId": "6","Timestamp": "1996-03-15T00:00:00Z","Labels":["b"]},{"ItemId": "8", "Timestamp": "1996-03-15T00:00:00Z", "Labels":["b"]}]`).
		End()
	// delete item
	apitest.New().
		Handler(restful.DefaultContainer).
		Delete("/item/0").
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	// delete item
	apitest.New().
		Handler(restful.DefaultContainer).
		Delete("/item/0").
		Expect(t).
		Status(http.StatusInternalServerError).
		End()
}

func TestServer_Feedback(t *testing.T) {
	// Setup
	db := setupServer()
	defer db.Close()
	// Run server using httptest
	// Insert ret
	feedback := []storage.Feedback{
		{"0", "0", 0},
		{"1", "2", 3},
		{"2", "4", 6},
		{"3", "6", 9},
		{"4", "8", 12},
	}
	// InsertFeedback
	apitest.New().
		Handler(restful.DefaultContainer).
		Post("/feedback").
		JSON(feedback[0:1]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	//BatchInsertFeedback
	apitest.New().
		Handler(restful.DefaultContainer).
		Post("/feedback").
		JSON(feedback[1:]).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 4}`).
		End()
	//Get Feedback
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/feedback").
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"UserId": "0", "ItemId": "0", "Rating": 0}, {"UserId": "1", "ItemId": "2", "Rating": 3}, {"UserId": "2", "ItemId": "4", "Rating": 6}, {"UserId": "3", "ItemId": "6", "Rating": 9}, {"UserId": "4", "ItemId": "8", "Rating": 12}]`).
		End()
	//Get Items
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/items").
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"ItemId":"0", "Labels": null, "Timestamp":"0001-01-01T00:00:00Z"}, {"ItemId":"2", "Labels": null, "Timestamp":"0001-01-01T00:00:00Z"}, {"ItemId":"4", "Labels": null, "Timestamp":"0001-01-01T00:00:00Z"}, {"ItemId":"6", "Labels": null, "Timestamp":"0001-01-01T00:00:00Z"}, {"ItemId":"8", "Labels": null, "Timestamp":"0001-01-01T00:00:00Z"}]`).
		End()
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/users").
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"UserId": "0"}, {"UserId": "1"}, {"UserId": "2"}, {"UserId": "3"}, {"UserId": "4"}]`).
		End()
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/user/2/feedback").
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"UserId": "2", "ItemId": "4", "Rating": 6}]`).
		End()
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/item/4/feedback").
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"UserId": "2", "ItemId": "4", "Rating": 6}]`).
		End()
}

func TestServer_Ignore(t *testing.T) {
	// Setup
	db := setupServer()
	defer db.Close()

	// Run server using httptest
	// Insert ignore
	apitest.New().
		Handler(restful.DefaultContainer).
		Post("/user/0/ignore").
		JSON([]string{"0", "1", "2", "3", "4", "5"}).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"RowAffected": 1}`).
		End()
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/user/0/ignore").
		Expect(t).
		Status(http.StatusOK).
		Body(`["0","1","2","3","4","5"]`).
		End()
}

func TestServer_List(t *testing.T) {
	// Setup
	db := setupServer()
	defer db.Close()
	// Run server using httptest
	type ListOperator struct {
		Set func(label string, items []storage.RecommendedItem) error
		Get string
	}
	operators := []ListOperator{
		{db.SetRecommend, "/user/0/recommend"},
		{db.SetLatest, "/latest/0"},
		{db.SetPop, "/popular/0"},
		{db.SetNeighbors, "/item/0/neighbors"},
	}

	for _, operator := range operators {
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
			Handler(restful.DefaultContainer).
			Get(operator.Get).
			Expect(t).
			Status(http.StatusOK).
			Body(`[{"ItemId": "0", "Score":0.0}, {"ItemId": "1", "Score":0.1}, {"ItemId": "2", "Score":0.2}, {"ItemId": "3", "Score":0.3}, {"ItemId": "4", "Score":0.4}]`).
			End()

		apitest.New().
			Handler(restful.DefaultContainer).
			Get(operator.Get).
			QueryParams(map[string]string{
				"number": "3",
				"offset": "0"}).
			Expect(t).
			Status(http.StatusOK).
			Body(`[{"ItemId": "0", "Score":0.0}, {"ItemId": "1", "Score":0.1}, {"ItemId": "2", "Score":0.2}]`).
			End()

		apitest.New().
			Handler(restful.DefaultContainer).
			Get(operator.Get).
			QueryParams(map[string]string{
				"number": "3",
				"offset": "1"}).
			Expect(t).
			Status(http.StatusOK).
			Body(`[{"ItemId": "1", "Score":0.1}, {"ItemId": "2", "Score":0.2}, {"ItemId": "3", "Score":0.3}]`).
			End()

		// get empty
		apitest.New().
			Handler(restful.DefaultContainer).
			Get(operator.Get).
			QueryParams(map[string]string{
				"number": "0",
				"offset": "0"}).
			Expect(t).
			Status(http.StatusOK).
			Body(``).
			End()
	}
}

func TestServer_GetRecommends(t *testing.T) {
	// Setup
	db := setupServer()
	defer db.Close()
	// Run server using httptest
	// Put neighbors
	items := []storage.RecommendedItem{
		{"0", 0.0},
		{"1", 0.1},
		{"2", 0.2},
		{"3", 0.3},
		{"4", 0.4},
	}
	if err := db.SetRecommend("0", items); err != nil {
		t.Fatal(err)
	}

	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/user/0/recommend").
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"ItemId": "0", "Score":0.0}, {"ItemId": "1", "Score":0.1}, {"ItemId": "2", "Score":0.2}, {"ItemId": "3", "Score":0.3}, {"ItemId": "4", "Score":0.4}]`).
		End()
	// Consume 2
	apitest.New().
		Handler(restful.DefaultContainer).
		Get("/consume/0").
		QueryParams(map[string]string{"number": "2"}).
		Expect(t).
		Status(http.StatusOK).
		Body(`[{"ItemId": "0", "Score":0.0}, {"ItemId": "1", "Score":0.1}]`).
		End()

}
