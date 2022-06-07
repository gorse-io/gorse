// Copyright 2021 gorse Project Authors
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

package data

import (
	"fmt"
	"github.com/juju/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strconv"
	"testing"
	"time"
)

var (
	positiveFeedbackType  = "positiveFeedbackType"
	negativeFeedbackType  = "negativeFeedbackType"
	duplicateFeedbackType = "duplicateFeedbackType"
)

func getUsers(t *testing.T, db Database, batchSize int) []User {
	users := make([]User, 0)
	var err error
	var data []User
	cursor := ""
	for {
		cursor, data, err = db.GetUsers(cursor, batchSize)
		assert.NoError(t, err)
		users = append(users, data...)
		if cursor == "" {
			if _, ok := db.(*Redis); !ok {
				assert.LessOrEqual(t, len(data), batchSize)
			}
			return users
		} else {
			if _, ok := db.(*Redis); !ok {
				assert.Equal(t, batchSize, len(data))
			}
		}
	}
}

func getUsersStream(t *testing.T, db Database, batchSize int) []User {
	var users []User
	userChan, errChan := db.GetUserStream(batchSize)
	for batchUsers := range userChan {
		users = append(users, batchUsers...)
	}
	assert.NoError(t, <-errChan)
	return users
}

func getItems(t *testing.T, db Database, batchSize int) []Item {
	items := make([]Item, 0)
	var err error
	var data []Item
	cursor := ""
	for {
		cursor, data, err = db.GetItems(cursor, batchSize, nil)
		assert.NoError(t, err)
		items = append(items, data...)
		if cursor == "" {
			if _, ok := db.(*Redis); !ok {
				assert.LessOrEqual(t, len(data), batchSize)
			}
			return items
		} else {
			if _, ok := db.(*Redis); !ok {
				assert.Equal(t, batchSize, len(data))
			}
		}
	}
}

func getItemStream(t *testing.T, db Database, batchSize int) []Item {
	var items []Item
	itemChan, errChan := db.GetItemStream(batchSize, nil)
	for batchUsers := range itemChan {
		items = append(items, batchUsers...)
	}
	assert.NoError(t, <-errChan)
	return items
}

func getFeedback(t *testing.T, db Database, batchSize int, feedbackTypes ...string) []Feedback {
	feedback := make([]Feedback, 0)
	var err error
	var data []Feedback
	cursor := ""
	for {
		cursor, data, err = db.GetFeedback(cursor, batchSize, nil, feedbackTypes...)
		assert.NoError(t, err)
		feedback = append(feedback, data...)
		if cursor == "" {
			if _, ok := db.(*Redis); !ok {
				assert.LessOrEqual(t, len(data), batchSize)
			}
			return feedback
		} else {
			if _, ok := db.(*Redis); !ok {
				assert.Equal(t, batchSize, len(data))
			}
		}
	}
}

func getFeedbackStream(t *testing.T, db Database, batchSize int, feedbackTypes ...string) []Feedback {
	var feedbacks []Feedback
	feedbackChan, errChan := db.GetFeedbackStream(batchSize, nil, feedbackTypes...)
	for batchFeedback := range feedbackChan {
		feedbacks = append(feedbacks, batchFeedback...)
	}
	assert.NoError(t, <-errChan)
	return feedbacks
}

func testUsers(t *testing.T, db Database) {
	// Insert users
	var insertedUsers []User
	for i := 9; i >= 0; i-- {
		insertedUsers = append(insertedUsers, User{
			UserId:  strconv.Itoa(i),
			Labels:  []string{strconv.Itoa(i + 100)},
			Comment: fmt.Sprintf("comment %d", i),
		})
	}
	err := db.BatchInsertUsers(insertedUsers)
	assert.NoError(t, err)
	// Get users
	users := getUsers(t, db, 3)
	assert.Equal(t, 10, len(users))
	for i, user := range users {
		assert.Equal(t, strconv.Itoa(i), user.UserId)
		assert.Equal(t, []string{strconv.Itoa(i + 100)}, user.Labels)
		assert.Equal(t, fmt.Sprintf("comment %d", i), user.Comment)
	}
	// Get user stream
	usersFromStream := getUsersStream(t, db, 3)
	assert.ElementsMatch(t, insertedUsers, usersFromStream)
	// Get this user
	user, err := db.GetUser("0")
	assert.NoError(t, err)
	assert.Equal(t, "0", user.UserId)
	// Delete this user
	err = db.DeleteUser("0")
	assert.NoError(t, err)
	_, err = db.GetUser("0")
	assert.True(t, errors.IsNotFound(err), err)
	// test override
	err = db.BatchInsertUsers([]User{{UserId: "1", Comment: "override"}})
	assert.NoError(t, err)
	err = db.Optimize()
	assert.NoError(t, err)
	user, err = db.GetUser("1")
	assert.NoError(t, err)
	assert.Equal(t, "override", user.Comment)
	// test modify
	err = db.ModifyUser("1", UserPatch{Comment: proto.String("modify"), Labels: []string{"a", "b", "c"}})
	assert.NoError(t, err)
	err = db.Optimize()
	assert.NoError(t, err)
	user, err = db.GetUser("1")
	assert.NoError(t, err)
	assert.Equal(t, "modify", user.Comment)
	assert.Equal(t, []string{"a", "b", "c"}, user.Labels)

	// test insert empty
	err = db.BatchInsertUsers(nil)
	assert.NoError(t, err)

	// insert duplicate users
	err = db.BatchInsertUsers([]User{{UserId: "1"}, {UserId: "1"}})
	assert.NoError(t, err)
}

func testFeedback(t *testing.T, db Database) {
	// users that already exists
	err := db.BatchInsertUsers([]User{{"0", []string{"a"}, []string{"x"}, "comment"}})
	assert.NoError(t, err)
	// items that already exists
	err = db.BatchInsertItems([]Item{{ItemId: "0", Labels: []string{"b"}, Timestamp: time.Date(1996, 4, 8, 10, 0, 0, 0, time.UTC)}})
	assert.NoError(t, err)
	// insert feedbacks
	feedback := []Feedback{
		{FeedbackKey{positiveFeedbackType, "0", "8"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "1", "6"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "2", "4"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "3", "2"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "4", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
	}
	err = db.BatchInsertFeedback(feedback, true, true, true)
	assert.NoError(t, err)
	// other type
	err = db.BatchInsertFeedback([]Feedback{{FeedbackKey: FeedbackKey{negativeFeedbackType, "0", "2"}}}, true, true, true)
	assert.NoError(t, err)
	err = db.BatchInsertFeedback([]Feedback{{FeedbackKey: FeedbackKey{negativeFeedbackType, "2", "4"}}}, true, true, true)
	assert.NoError(t, err)
	// future feedback
	futureFeedback := []Feedback{
		{FeedbackKey{duplicateFeedbackType, "0", "0"}, time.Now().Add(time.Hour), "comment"},
		{FeedbackKey{duplicateFeedbackType, "1", "2"}, time.Now().Add(time.Hour), "comment"},
		{FeedbackKey{duplicateFeedbackType, "2", "4"}, time.Now().Add(time.Hour), "comment"},
		{FeedbackKey{duplicateFeedbackType, "3", "6"}, time.Now().Add(time.Hour), "comment"},
		{FeedbackKey{duplicateFeedbackType, "4", "8"}, time.Now().Add(time.Hour), "comment"},
	}
	err = db.BatchInsertFeedback(futureFeedback, true, true, true)
	assert.NoError(t, err)
	// Get feedback
	ret := getFeedback(t, db, 3, positiveFeedbackType)
	assert.Equal(t, feedback, ret)
	ret = getFeedback(t, db, 2)
	assert.Equal(t, len(feedback)+2, len(ret))
	// Get feedback stream
	feedbackFromStream := getFeedbackStream(t, db, 3, positiveFeedbackType)
	assert.ElementsMatch(t, feedback, feedbackFromStream)
	feedbackFromStream = getFeedbackStream(t, db, 3)
	assert.Equal(t, len(feedback)+2, len(feedbackFromStream))
	// Get items
	err = db.Optimize()
	assert.NoError(t, err)
	items := getItems(t, db, 3)
	assert.Equal(t, 5, len(items))
	for i, item := range items {
		assert.Equal(t, strconv.Itoa(i*2), item.ItemId)
		if item.ItemId != "0" {
			if isClickHouse(db) {
				// ClickHouse returns 1970-01-01 as zero date.
				assert.Zero(t, item.Timestamp.Unix())
			} else {
				assert.Zero(t, item.Timestamp)
			}
			assert.Empty(t, item.Labels)
			assert.Empty(t, item.Comment)
		}
	}
	// Get users
	users := getUsers(t, db, 2)
	assert.Equal(t, 5, len(users))
	for i, user := range users {
		assert.Equal(t, strconv.Itoa(i), user.UserId)
		if user.UserId != "0" {
			assert.Empty(t, user.Labels)
			assert.Empty(t, user.Subscribe)
			assert.Empty(t, user.Comment)
		}
	}
	// check users that already exists
	user, err := db.GetUser("0")
	assert.NoError(t, err)
	assert.Equal(t, User{"0", []string{"a"}, []string{"x"}, "comment"}, user)
	// check items that already exists
	item, err := db.GetItem("0")
	assert.NoError(t, err)
	assert.Equal(t, Item{ItemId: "0", Labels: []string{"b"}, Timestamp: time.Date(1996, 4, 8, 10, 0, 0, 0, time.UTC)}, item)
	// Get typed feedback by user
	ret, err = db.GetUserFeedback("2", false, positiveFeedbackType)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "2", ret[0].UserId)
	assert.Equal(t, "4", ret[0].ItemId)
	// Get all feedback by user
	ret, err = db.GetUserFeedback("2", false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(ret))
	// Get typed feedback by item
	ret, err = db.GetItemFeedback("4", positiveFeedbackType)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "2", ret[0].UserId)
	assert.Equal(t, "4", ret[0].ItemId)
	// Get all feedback by item
	ret, err = db.GetItemFeedback("4")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(ret))
	// test override
	err = db.BatchInsertFeedback([]Feedback{{
		FeedbackKey: FeedbackKey{positiveFeedbackType, "0", "8"},
		Comment:     "override",
	}}, true, true, true)
	assert.NoError(t, err)
	err = db.Optimize()
	assert.NoError(t, err)
	ret, err = db.GetUserFeedback("0", false, positiveFeedbackType)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "override", ret[0].Comment)
	// test not overwrite
	err = db.BatchInsertFeedback([]Feedback{{
		FeedbackKey: FeedbackKey{positiveFeedbackType, "0", "8"},
		Comment:     "not_override",
	}}, true, true, false)
	assert.NoError(t, err)
	err = db.Optimize()
	assert.NoError(t, err)
	ret, err = db.GetUserFeedback("0", false, positiveFeedbackType)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "override", ret[0].Comment)

	// insert no feedback
	err = db.BatchInsertFeedback(nil, true, true, true)
	assert.NoError(t, err)

	// not insert users or items
	err = db.BatchInsertFeedback([]Feedback{
		{FeedbackKey: FeedbackKey{"a", "100", "200"}},
		{FeedbackKey: FeedbackKey{"a", "0", "200"}},
		{FeedbackKey: FeedbackKey{"a", "100", "8"}},
	}, false, false, false)
	assert.NoError(t, err)
	result, err := db.GetUserItemFeedback("100", "200")
	assert.NoError(t, err)
	assert.Empty(t, result)
	result, err = db.GetUserItemFeedback("0", "200")
	assert.NoError(t, err)
	assert.Empty(t, result)
	result, err = db.GetUserItemFeedback("100", "8")
	assert.NoError(t, err)
	assert.Empty(t, result)

	// insert valid feedback and invalid feedback at the same time
	err = db.BatchInsertFeedback([]Feedback{
		{FeedbackKey: FeedbackKey{"a", "0", "8"}},
		{FeedbackKey: FeedbackKey{"a", "100", "200"}},
	}, false, false, false)
	assert.NoError(t, err)

	// insert duplicate feedback
	err = db.BatchInsertFeedback([]Feedback{
		{FeedbackKey: FeedbackKey{"a", "0", "0"}},
		{FeedbackKey: FeedbackKey{"a", "0", "0"}},
	}, true, true, true)
	assert.NoError(t, err)
}

func testItems(t *testing.T, db Database) {
	// Items
	items := []Item{
		{
			ItemId:     "0",
			IsHidden:   true,
			Categories: []string{"a"},
			Timestamp:  time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:     []string{"a"},
			Comment:    "comment 0",
		},
		{
			ItemId:     "2",
			Categories: []string{"b"},
			Timestamp:  time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:     []string{"a"},
			Comment:    "comment 2",
		},
		{
			ItemId:     "4",
			IsHidden:   true,
			Categories: []string{"a"},
			Timestamp:  time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:     []string{"a", "b"},
			Comment:    "comment 4",
		},
		{
			ItemId:     "6",
			Categories: []string{"b"},
			Timestamp:  time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:     []string{"b"},
			Comment:    "comment 6",
		},
		{
			ItemId:     "8",
			IsHidden:   true,
			Categories: []string{"a"},
			Timestamp:  time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:     []string{"b"},
			Comment:    "comment 8",
		},
	}
	// Insert item
	err := db.BatchInsertItems(items)
	assert.NoError(t, err)
	// Get items
	totalItems := getItems(t, db, 3)
	assert.Equal(t, items, totalItems)
	// Get item stream
	itemsFromStream := getItemStream(t, db, 3)
	assert.ElementsMatch(t, items, itemsFromStream)
	// Get item
	for _, item := range items {
		ret, err := db.GetItem(item.ItemId)
		assert.NoError(t, err)
		assert.Equal(t, item, ret)
	}
	// batch get items
	batchItem, err := db.BatchGetItems([]string{"2", "6"})
	assert.NoError(t, err)
	assert.Equal(t, []Item{items[1], items[3]}, batchItem)
	// Delete item
	err = db.DeleteItem("0")
	assert.NoError(t, err)
	_, err = db.GetItem("0")
	assert.True(t, errors.IsNotFound(err))

	// test override
	err = db.BatchInsertItems([]Item{{ItemId: "4", IsHidden: false, Categories: []string{"b"}, Labels: []string{"o"}, Comment: "override"}})
	assert.NoError(t, err)
	err = db.Optimize()
	assert.NoError(t, err)
	item, err := db.GetItem("4")
	assert.NoError(t, err)
	assert.False(t, item.IsHidden)
	assert.Equal(t, []string{"b"}, item.Categories)
	assert.Equal(t, []string{"o"}, item.Labels)
	assert.Equal(t, "override", item.Comment)

	// test modify
	timestamp := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)
	err = db.ModifyItem("2", ItemPatch{IsHidden: proto.Bool(true), Categories: []string{"a"}, Comment: proto.String("modify"), Labels: []string{"a", "b", "c"}, Timestamp: &timestamp})
	assert.NoError(t, err)
	err = db.Optimize()
	assert.NoError(t, err)
	item, err = db.GetItem("2")
	assert.NoError(t, err)
	assert.True(t, item.IsHidden)
	assert.Equal(t, []string{"a"}, item.Categories)
	assert.Equal(t, "modify", item.Comment)
	assert.Equal(t, []string{"a", "b", "c"}, item.Labels)
	assert.Equal(t, timestamp, item.Timestamp)

	// test insert empty
	err = db.BatchInsertItems(nil)
	assert.NoError(t, err)
	// test get empty
	items, err = db.BatchGetItems(nil)
	assert.NoError(t, err)
	assert.Empty(t, items)

	// test insert duplicate items
	err = db.BatchInsertItems([]Item{{ItemId: "1"}, {ItemId: "1"}})
	assert.NoError(t, err)
}

func testDeleteUser(t *testing.T, db Database) {
	// Insert ret
	feedback := []Feedback{
		{FeedbackKey{positiveFeedbackType, "a", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "a", "2"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "a", "4"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "a", "6"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "a", "8"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
	}
	err := db.BatchInsertFeedback(feedback, true, true, true)
	assert.NoError(t, err)
	// Delete user
	err = db.DeleteUser("a")
	assert.NoError(t, err)
	_, err = db.GetUser("a")
	assert.NotNil(t, err, "failed to delete user")
	ret, err := db.GetUserFeedback("a", false, positiveFeedbackType)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(ret))
	_, ret, err = db.GetFeedback("", 100, nil, positiveFeedbackType)
	assert.NoError(t, err)
	assert.Empty(t, ret)
}

func testDeleteItem(t *testing.T, db Database) {
	// Insert ret
	feedbacks := []Feedback{
		{FeedbackKey{positiveFeedbackType, "0", "b"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "1", "b"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "2", "b"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "3", "b"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "4", "b"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
	}
	err := db.BatchInsertFeedback(feedbacks, true, true, true)
	assert.NoError(t, err)
	// Delete item
	err = db.DeleteItem("b")
	assert.NoError(t, err)
	_, err = db.GetItem("b")
	assert.Error(t, err, "failed to delete item")
	ret, err := db.GetItemFeedback("b", positiveFeedbackType)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(ret))
	_, ret, err = db.GetFeedback("", 100, nil, positiveFeedbackType)
	assert.NoError(t, err)
	assert.Empty(t, ret)
}

func testDeleteFeedback(t *testing.T, db Database) {
	feedbacks := []Feedback{
		{FeedbackKey{"type1", "2", "3"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type2", "2", "3"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type3", "2", "3"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type1", "2", "4"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type1", "1", "3"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
	}
	err := db.BatchInsertFeedback(feedbacks, true, true, true)
	assert.NoError(t, err)
	// get user-item feedback
	ret, err := db.GetUserItemFeedback("2", "3")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []Feedback{feedbacks[0], feedbacks[1], feedbacks[2]}, ret)
	feedbackType2 := "type2"
	ret, err = db.GetUserItemFeedback("2", "3", feedbackType2)
	assert.NoError(t, err)
	assert.Equal(t, []Feedback{feedbacks[1]}, ret)
	// delete user-item feedback
	deleteCount, err := db.DeleteUserItemFeedback("2", "3")
	assert.NoError(t, err)
	if !isClickHouse(db) {
		// RowAffected isn't supported by ClickHouse,
		assert.Equal(t, 3, deleteCount)
	}
	ret, err = db.GetUserItemFeedback("2", "3")
	assert.NoError(t, err)
	assert.Empty(t, ret)
	feedbackType1 := "type1"
	deleteCount, err = db.DeleteUserItemFeedback("1", "3", feedbackType1)
	assert.NoError(t, err)
	if !isClickHouse(db) {
		// RowAffected isn't supported by ClickHouse,
		assert.Equal(t, 1, deleteCount)
	}
	ret, err = db.GetUserItemFeedback("1", "3", feedbackType2)
	assert.NoError(t, err)
	assert.Empty(t, ret)
}

func testTimeLimit(t *testing.T, db Database) {
	// insert items
	items := []Item{
		{
			ItemId:    "0",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a"},
			Comment:   "comment 0",
		},
		{
			ItemId:    "2",
			Timestamp: time.Date(1997, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a"},
			Comment:   "comment 2",
		},
		{
			ItemId:    "4",
			Timestamp: time.Date(1998, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a", "b"},
			Comment:   "comment 4",
		},
		{
			ItemId:    "6",
			Timestamp: time.Date(1999, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"b"},
			Comment:   "comment 6",
		},
		{
			ItemId:    "8",
			Timestamp: time.Date(2000, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"b"},
			Comment:   "comment 8",
		},
	}
	err := db.BatchInsertItems(items)
	assert.NoError(t, err)
	timeLimit := time.Date(1998, 1, 1, 0, 0, 0, 0, time.UTC)
	_, ret, err := db.GetItems("", 100, &timeLimit)
	assert.NoError(t, err)
	assert.Equal(t, []Item{items[2], items[3], items[4]}, ret)

	// insert feedback
	feedbacks := []Feedback{
		{FeedbackKey{"type1", "2", "3"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type2", "2", "3"}, time.Date(1997, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type3", "2", "3"}, time.Date(1998, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type1", "2", "4"}, time.Date(1999, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type1", "1", "3"}, time.Date(2000, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
	}
	err = db.BatchInsertFeedback(feedbacks, true, true, true)
	assert.NoError(t, err)
	_, retFeedback, err := db.GetFeedback("", 100, &timeLimit)
	assert.NoError(t, err)
	assert.Equal(t, []Feedback{feedbacks[4], feedbacks[3], feedbacks[2]}, retFeedback)
	typeFilter := "type1"
	_, retFeedback, err = db.GetFeedback("", 100, &timeLimit, typeFilter)
	assert.NoError(t, err)
	assert.Equal(t, []Feedback{feedbacks[4], feedbacks[3]}, retFeedback)
}

func testTimeZone(t *testing.T, db Database) {
	loc, err := time.LoadLocation("Asia/Tokyo")
	assert.NoError(t, err)
	// insert feedbacks
	err = db.BatchInsertFeedback([]Feedback{
		{FeedbackKey: FeedbackKey{"read", "1", "1"}, Timestamp: time.Now().Add(-time.Second).In(loc)},
		{FeedbackKey: FeedbackKey{"read", "1", "2"}, Timestamp: time.Now().Add(-time.Second).In(loc)},
		{FeedbackKey: FeedbackKey{"read", "2", "2"}, Timestamp: time.Now().Add(-time.Second).In(loc)},
		{FeedbackKey: FeedbackKey{"like", "1", "1"}, Timestamp: time.Now().Add(time.Hour).In(loc)},
		{FeedbackKey: FeedbackKey{"like", "1", "2"}, Timestamp: time.Now().Add(time.Hour).In(loc)},
		{FeedbackKey: FeedbackKey{"like", "2", "2"}, Timestamp: time.Now().Add(time.Hour).In(loc)},
	}, true, true, true)
	assert.NoError(t, err)
	// get feedback stream
	feedback := getFeedback(t, db, 10)
	assert.Equal(t, 3, len(feedback))
	// get feedback
	_, feedback, err = db.GetFeedback("", 10, nil)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(feedback))
	// get user feedback
	feedback, err = db.GetUserFeedback("1", false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(feedback))
	// get item feedback
	feedback, err = db.GetItemFeedback("2") // no future feedback by default
	assert.NoError(t, err)
	assert.Equal(t, 2, len(feedback))
	// get user item feedback
	feedback, err = db.GetUserItemFeedback("1", "1") // return future feedback by default
	assert.NoError(t, err)
	assert.Equal(t, 2, len(feedback))

	// insert items
	now := time.Now().In(loc)
	err = db.BatchInsertItems([]Item{{ItemId: "100", Timestamp: now}, {ItemId: "200"}})
	assert.NoError(t, err)
	err = db.ModifyItem("200", ItemPatch{Timestamp: &now})
	assert.NoError(t, err)
	err = db.Optimize()
	assert.NoError(t, err)
	switch database := db.(type) {
	case *SQLDatabase:
		switch db.(*SQLDatabase).driver {
		case Postgres:
			item, err := db.GetItem("100")
			assert.NoError(t, err)
			assert.Equal(t, now.Round(time.Microsecond).In(time.UTC), item.Timestamp)
			item, err = db.GetItem("200")
			assert.NoError(t, err)
			assert.Equal(t, now.Round(time.Microsecond).In(time.UTC), item.Timestamp)
		case ClickHouse:
			item, err := db.GetItem("100")
			assert.NoError(t, err)
			assert.Equal(t, now.Truncate(time.Second).In(time.UTC), item.Timestamp)
			item, err = db.GetItem("200")
			assert.NoError(t, err)
			assert.Equal(t, now.Truncate(time.Second).In(time.UTC), item.Timestamp)
		case SQLite:
			item, err := db.GetItem("100")
			assert.NoError(t, err)
			assert.Equal(t, now.In(time.UTC), item.Timestamp)
			item, err = db.GetItem("200")
			assert.NoError(t, err)
			assert.Equal(t, now.In(time.UTC), item.Timestamp)
		default:
			t.Skipf("unknown sql database: %v", database.driver)
		}
	case *MongoDB:
		item, err := db.GetItem("100")
		assert.NoError(t, err)
		assert.Equal(t, now.Truncate(time.Millisecond).In(time.UTC), item.Timestamp)
		item, err = db.GetItem("200")
		assert.NoError(t, err)
		assert.Equal(t, now.Truncate(time.Millisecond).In(time.UTC), item.Timestamp)
	default:
		t.Skipf("unknown database: %v", reflect.TypeOf(db))
	}
}

func isClickHouse(db Database) bool {
	if sqlDB, isSQL := db.(*SQLDatabase); !isSQL {
		return false
	} else {
		return sqlDB.driver == ClickHouse
	}
}

func TestSortFeedbacks(t *testing.T) {
	feedback := []Feedback{
		{FeedbackKey: FeedbackKey{"star", "1", "1"}, Timestamp: time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"like", "1", "1"}, Timestamp: time.Date(2001, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"read", "1", "1"}, Timestamp: time.Date(2002, 10, 1, 0, 0, 0, 0, time.UTC)},
	}
	SortFeedbacks(feedback)
	assert.Equal(t, []Feedback{
		{FeedbackKey: FeedbackKey{"read", "1", "1"}, Timestamp: time.Date(2002, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"like", "1", "1"}, Timestamp: time.Date(2001, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"star", "1", "1"}, Timestamp: time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC)},
	}, feedback)
}
