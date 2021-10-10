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
	"github.com/stretchr/testify/assert"
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
	assert.ErrorIs(t, err, ErrUserNotExist)
	// test override
	err = db.BatchInsertUsers([]User{{UserId: "1", Comment: "override"}})
	assert.NoError(t, err)
	err = db.Optimize()
	assert.NoError(t, err)
	user, err = db.GetUser("1")
	assert.NoError(t, err)
	assert.Equal(t, "override", user.Comment)
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
}

func testItems(t *testing.T, db Database) {
	// Items
	items := []Item{
		{
			ItemId:    "0",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a"},
			Comment:   "comment 0",
		},
		{
			ItemId:    "2",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a"},
			Comment:   "comment 2",
		},
		{
			ItemId:    "4",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"a", "b"},
			Comment:   "comment 4",
		},
		{
			ItemId:    "6",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"b"},
			Comment:   "comment 6",
		},
		{
			ItemId:    "8",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []string{"b"},
			Comment:   "comment 8",
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
	// Delete item
	err = db.DeleteItem("0")
	assert.NoError(t, err)
	_, err = db.GetItem("0")
	assert.ErrorIs(t, err, ErrItemNotExist)
	// test override
	err = db.BatchInsertItems([]Item{{ItemId: "2", Comment: "override"}})
	assert.NoError(t, err)
	err = db.Optimize()
	assert.NoError(t, err)
	item, err := db.GetItem("2")
	assert.NoError(t, err)
	assert.Equal(t, "override", item.Comment)
}

func testDeleteUser(t *testing.T, db Database) {
	// Insert ret
	feedback := []Feedback{
		{FeedbackKey{positiveFeedbackType, "0", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "0", "2"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "0", "4"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "0", "6"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "0", "8"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
	}
	err := db.BatchInsertFeedback(feedback, true, true, true)
	assert.NoError(t, err)
	// Delete user
	err = db.DeleteUser("0")
	assert.NoError(t, err)
	_, err = db.GetUser("0")
	assert.NotNil(t, err, "failed to delete user")
	ret, err := db.GetUserFeedback("0", false, positiveFeedbackType)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(ret))
	_, ret, err = db.GetFeedback("", 100, nil, positiveFeedbackType)
	assert.NoError(t, err)
	assert.Empty(t, ret)
}

func testDeleteItem(t *testing.T, db Database) {
	// Insert ret
	feedbacks := []Feedback{
		{FeedbackKey{positiveFeedbackType, "0", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "1", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "2", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "3", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "4", "0"}, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
	}
	err := db.BatchInsertFeedback(feedbacks, true, true, true)
	assert.NoError(t, err)
	// Delete item
	err = db.DeleteItem("0")
	assert.NoError(t, err)
	_, err = db.GetItem("0")
	assert.Error(t, err, "failed to delete item")
	ret, err := db.GetItemFeedback("0", positiveFeedbackType)
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

func testMeasurements(t *testing.T, db Database) {
	err := db.InsertMeasurement(Measurement{"Test_NDCG", time.Date(2004, 1, 1, 1, 1, 1, 0, time.UTC), 100, "a"})
	assert.NoError(t, err)
	measurements := []Measurement{
		{"Test_NDCG", time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC), 0, "a"},
		{"Test_NDCG", time.Date(2001, 1, 1, 1, 1, 1, 0, time.UTC), 1, "b"},
		{"Test_NDCG", time.Date(2002, 1, 1, 1, 1, 1, 0, time.UTC), 2, "c"},
		{"Test_NDCG", time.Date(2003, 1, 1, 1, 1, 1, 0, time.UTC), 3, "d"},
		{"Test_NDCG", time.Date(2004, 1, 1, 1, 1, 1, 0, time.UTC), 4, "e"},
		{"Test_Recall", time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC), 1, "f"},
	}
	for _, measurement := range measurements {
		err := db.InsertMeasurement(measurement)
		assert.NoError(t, err)
	}
	err = db.Optimize()
	assert.NoError(t, err)
	ret, err := db.GetMeasurements("Test_NDCG", 3)
	assert.NoError(t, err)
	assert.Equal(t, []Measurement{
		measurements[4],
		measurements[3],
		measurements[2],
	}, ret)
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

func testGetClickThroughRate(t *testing.T, db Database) {
	// get empty click-through-rate
	rate, err := db.GetClickThroughRate(time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC), []string{"star", "like"}, []string{"read"})
	assert.NoError(t, err)
	assert.Zero(t, rate)
	// insert feedback
	// user 1: star(1,1), like(1,1), read(1,1), read(1,2), read(1,3), read(1,4) - 0.25
	// user 2: star(2,1), star(2,3), read(2,1), read(2,2) - 0.5
	// user 3: read(3,2), star(3,3) - 0.0
	err = db.BatchInsertFeedback([]Feedback{
		{FeedbackKey: FeedbackKey{"star", "1", "1"}, Timestamp: time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"like", "1", "1"}, Timestamp: time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"read", "1", "1"}, Timestamp: time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"read", "1", "2"}, Timestamp: time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"read", "1", "3"}, Timestamp: time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"read", "1", "4"}, Timestamp: time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"read", "1", "5"}, Timestamp: time.Date(2001, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"star", "2", "1"}, Timestamp: time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"star", "2", "3"}, Timestamp: time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"read", "2", "1"}, Timestamp: time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"read", "2", "2"}, Timestamp: time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"read", "2", "4"}, Timestamp: time.Date(2001, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"read", "3", "2"}, Timestamp: time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC)},
		{FeedbackKey: FeedbackKey{"star", "3", "3"}, Timestamp: time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC)},
	}, true, true, true)
	assert.NoError(t, err)
	// get click-through-rate
	rate, err = db.GetClickThroughRate(time.Date(2000, 10, 1, 0, 0, 0, 0, time.UTC), []string{"star", "like"}, []string{"read"})
	assert.NoError(t, err)
	assert.Equal(t, 0.375, rate)
}

func isClickHouse(db Database) bool {
	if sqlDB, isSQL := db.(*SQLDatabase); !isSQL {
		return false
	} else {
		return sqlDB.driver == ClickHouse
	}
}
