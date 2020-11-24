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
package storage

import (
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
	"time"
)

type mockSQLDatabase struct {
	Database
}

func newMockSQLDatabase(t *testing.T, name string) *mockSQLDatabase {
	database := new(mockSQLDatabase)
	var err error
	database.Database, err = Open("ramsql://" + name)
	assert.Nil(t, err)
	err = database.Init()
	assert.Nil(t, err)
	return database
}

func (mock *mockSQLDatabase) Close(t *testing.T) {
	err := mock.Database.Close()
	assert.Nil(t, err)
}

func TestSQLDatabase_Users(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_Users")
	defer db.Close(t)
	// Insert users
	for i := 0; i < 10; i++ {
		if err := db.InsertUser(User{UserId: strconv.Itoa(i)}); err != nil {
			t.Fatal(err)
		}
	}
	// Get users
	if users, err := db.GetUsers(); err != nil {
		t.Fatal(err)
	} else {
		for i, user := range users {
			assert.Equal(t, strconv.Itoa(i), user.UserId)
		}
	}
	// Get this user
	if user, err := db.GetUser("0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, "0", user.UserId)
	}
	// Delete this user
	if err := db.DeleteUser("0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetUser("0"); err == nil {
		t.Fatal(err)
	}
}

func TestSQLDatabase_Feedback(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_Feedback")
	defer db.Close(t)
	// Insert ret
	feedback := []Feedback{
		{"0", "0", 0},
		{"1", "2", 3},
		{"2", "4", 6},
		{"3", "6", 9},
		{"4", "8", 12},
	}
	if err := db.InsertFeedback(feedback[0]); err != nil {
		t.Fatal(err)
	}
	if err := db.BatchInsertFeedback(feedback[1:]); err != nil {
		t.Fatal(err)
	}
	// idempotent
	if err := db.InsertFeedback(feedback[0]); err != nil {
		t.Fatal(err)
	}
	if err := db.BatchInsertFeedback(feedback[1:]); err != nil {
		t.Fatal(err)
	}
	// Get feedback
	ret, err := db.GetFeedback()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, feedback, feedback)
	// Get items
	items, err := db.GetItems(0, 0)
	assert.Nil(t, err)
	assert.Equal(t, 5, len(items))
	for i, item := range items {
		assert.Equal(t, strconv.Itoa(i*2), item.ItemId)
	}
	// Get users
	if users, err := db.GetUsers(); err != nil {
		t.Fatal(err)
	} else {
		for i, user := range users {
			assert.Equal(t, strconv.Itoa(i), user.UserId)
		}
	}
	// Get ret by user
	ret, err = db.GetUserFeedback("2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "2", ret[0].UserId)
	assert.Equal(t, "4", ret[0].ItemId)
	assert.Equal(t, float64(6), ret[0].Rating)
	// Get ret by item
	ret, err = db.GetItemFeedback("4")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "2", ret[0].UserId)
	assert.Equal(t, "4", ret[0].ItemId)
	assert.Equal(t, float64(6), ret[0].Rating)
}

func TestSQLDatabase_Item(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_Item")
	defer db.Close(t)
	// Items
	items := []Item{
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
	// Insert item
	if err := db.InsertItem(items[0]); err != nil {
		t.Fatal(err)
	}
	if err := db.BatchInsertItem(items[1:]); err != nil {
		t.Fatal(err)
	}
	// Get items
	totalItems, err := db.GetItems(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items, totalItems)
	partItems, err := db.GetItems(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[1:4], partItems)
	// Get item
	for _, item := range items {
		ret, err := db.GetItem(item.ItemId)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, item, ret)
	}
	// Get labels
	if labels, err := db.GetLabels(); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, []string{"a", "b"}, labels)
	}
	// Get items by labels
	labelAItems, err := db.GetLabelItems("a")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[:3], labelAItems)
	labelBItems, err := db.GetLabelItems("b")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, items[2:], labelBItems)
	// Delete item
	if err := db.DeleteItem("0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetItem("0"); err == nil {
		t.Fatal("delete item failed")
	}
}

func TestSQLDatabase_Ignore(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_Ignore")
	defer db.Close(t)
	// Insert ignore
	ignores := []string{"0", "1", "2", "3", "4", "5"}
	if err := db.InsertUserIgnore("0", ignores); err != nil {
		t.Fatal(err)
	}
	// idempotent
	if err := db.InsertUserIgnore("0", ignores); err != nil {
		t.Fatal(err)
	}
	// Count ignore
	if count, err := db.CountUserIgnore("0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, len(ignores), count)
	}
	// Get ignore
	if ret, err := db.GetUserIgnore("0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, ignores, ret)
	}
}

func TestSQLDatabase_Meta(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_Meta")
	defer db.Close(t)
	// Set meta string
	if err := db.SetString("1", "2"); err != nil {
		t.Fatal(err)
	}
	// Get meta string
	value, err := db.GetString("1")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "2", value)
	// Get meta not existed
	value, err = db.GetString("NULL")
	if err == nil {
		t.Fatal(err)
	}
	assert.Equal(t, "", value)
	// Set meta int
	if err = db.SetInt("1", 2); err != nil {
		t.Fatal(err)
	}
	// Get meta int
	if value, err := db.GetInt("1"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 2, value)
	}
}

func TestSQLDatabase_List(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_List")
	defer db.Close(t)
	type ListOperator struct {
		Set func(label string, items []RecommendedItem) error
		Get func(label string, n int, offset int) ([]RecommendedItem, error)
	}
	operators := []ListOperator{
		{db.SetRecommend, db.GetRecommend},
		{db.SetLatest, db.GetLatest},
		{db.SetPop, db.GetPop},
		{db.SetNeighbors, db.GetNeighbors},
	}
	for _, operator := range operators {
		// Put items
		items := []RecommendedItem{
			{"0", 0.0},
			{"1", 0.1},
			{"2", 0.2},
			{"3", 0.3},
			{"4", 0.4},
		}
		if err := operator.Set("0", items); err != nil {
			t.Fatal(err)
		}
		// Get items
		totalItems, err := operator.Get("0", 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, items, totalItems)
		// Get n items
		headItems, err := operator.Get("0", 3, 0)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, items[:3], headItems)
		// Get n items with offset
		offsetItems, err := operator.Get("0", 3, 1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, items[1:4], offsetItems)
		// Get empty
		noItems, err := operator.Get("1", 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 0, len(noItems))
	}
}

func TestSQLDatabase_DeleteUser(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_DeleteUser")
	defer db.Close(t)
	// Insert ret
	feedback := []Feedback{
		{"0", "0", 0},
		{"0", "2", 3},
		{"0", "4", 6},
		{"0", "6", 9},
		{"0", "8", 12},
	}
	if err := db.BatchInsertFeedback(feedback); err != nil {
		t.Fatal(err)
	}
	// Insert ignore
	if err := db.InsertUserIgnore("0", []string{"0", "1", "2"}); err != nil {
		t.Fatal(err)
	}
	// Delete user
	if err := db.DeleteUser("0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetUser("0"); err == nil {
		t.Fatal("failed to delete user")
	}
	if ret, err := db.GetUserFeedback("0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 0, len(ret))
	}
	if ret, err := db.GetUserIgnore("0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 0, len(ret))
	}
}

func TestSQLDatabase_DeleteItem(t *testing.T) {
	db := newMockSQLDatabase(t, "TestSQLDatabase_DeleteItem")
	defer db.Close(t)
	// Insert ret
	feedback := []Feedback{
		{"0", "0", 0},
		{"1", "0", 3},
		{"2", "0", 6},
		{"3", "0", 9},
		{"4", "0", 12},
	}
	if err := db.BatchInsertFeedback(feedback); err != nil {
		t.Fatal(err)
	}
	// Delete item
	if err := db.DeleteItem("0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetItem("0"); err == nil {
		t.Fatal("failed to delete item")
	}
	if ret, err := db.GetItemFeedback("0"); err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, 0, len(ret))
	}
}
