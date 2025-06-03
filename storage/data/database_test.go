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
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/jaswdr/faker"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

var (
	positiveFeedbackType  = "positiveFeedbackType"
	negativeFeedbackType  = "negativeFeedbackType"
	duplicateFeedbackType = "duplicateFeedbackType"
	dateTime64Zero        = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
)

type baseTestSuite struct {
	suite.Suite
	Database
}

func (suite *baseTestSuite) getUsers(ctx context.Context, batchSize int) []User {
	users := make([]User, 0)
	var err error
	var data []User
	cursor := ""
	for {
		cursor, data, err = suite.Database.GetUsers(ctx, cursor, batchSize)
		suite.NoError(err)
		users = append(users, data...)
		if cursor == "" {
			suite.LessOrEqual(len(data), batchSize)
			return users
		} else {
			suite.Equal(batchSize, len(data))
		}
	}
}

func (suite *baseTestSuite) getUsersStream(ctx context.Context, batchSize int) []User {
	var users []User
	userChan, errChan := suite.Database.GetUserStream(ctx, batchSize)
	for batchUsers := range userChan {
		users = append(users, batchUsers...)
	}
	suite.NoError(<-errChan)
	return users
}

func (suite *baseTestSuite) getItems(ctx context.Context, batchSize int) []Item {
	items := make([]Item, 0)
	var err error
	var data []Item
	cursor := ""
	for {
		cursor, data, err = suite.Database.GetItems(ctx, cursor, batchSize, nil)
		suite.NoError(err)
		items = append(items, data...)
		if cursor == "" {
			suite.LessOrEqual(len(data), batchSize)
			return items
		} else {
			suite.Equal(batchSize, len(data))
		}
	}
}

func (suite *baseTestSuite) getItemStream(ctx context.Context, batchSize int) []Item {
	var items []Item
	itemChan, errChan := suite.Database.GetItemStream(ctx, batchSize, nil)
	for batchUsers := range itemChan {
		items = append(items, batchUsers...)
	}
	suite.NoError(<-errChan)
	return items
}

func (suite *baseTestSuite) getFeedback(ctx context.Context, batchSize int, beginTime, endTime *time.Time, feedbackTypes ...string) []Feedback {
	feedback := make([]Feedback, 0)
	var err error
	var data []Feedback
	cursor := ""
	for {
		cursor, data, err = suite.Database.GetFeedback(ctx, cursor, batchSize, beginTime, endTime, feedbackTypes...)
		suite.NoError(err)
		feedback = append(feedback, data...)
		if cursor == "" {
			suite.LessOrEqual(len(data), batchSize)
			return feedback
		} else {
			suite.Equal(batchSize, len(data))
		}
	}
}

func (suite *baseTestSuite) getFeedbackStream(ctx context.Context, batchSize int, scanOptions ...ScanOption) []Feedback {
	var feedbacks []Feedback
	feedbackChan, errChan := suite.Database.GetFeedbackStream(ctx, batchSize, scanOptions...)
	for batchFeedback := range feedbackChan {
		feedbacks = append(feedbacks, batchFeedback...)
	}
	suite.NoError(<-errChan)
	return feedbacks
}

func (suite *baseTestSuite) isClickHouse() bool {
	if sqlDB, isSQL := suite.Database.(*SQLDatabase); !isSQL {
		return false
	} else {
		return sqlDB.driver == ClickHouse
	}
}

func (suite *baseTestSuite) analyzeTables() {
	sqlDatabase, ok := suite.Database.(*SQLDatabase)
	if ok && sqlDatabase.driver == Postgres {
		sqlDatabase := suite.Database.(*SQLDatabase)
		err := sqlDatabase.gormDB.Exec(fmt.Sprintf("ANALYZE %s", sqlDatabase.ItemsTable())).Error
		suite.NoError(err)
		err = sqlDatabase.gormDB.Exec(fmt.Sprintf("ANALYZE %s", sqlDatabase.UsersTable())).Error
		suite.NoError(err)
		err = sqlDatabase.gormDB.Exec(fmt.Sprintf("ANALYZE %s", sqlDatabase.FeedbackTable())).Error
		suite.NoError(err)
	}
}

func (suite *baseTestSuite) TearDownSuite() {
	err := suite.Database.Close()
	suite.NoError(err)
}

func (suite *baseTestSuite) SetupTest() {
	err := suite.Database.Ping()
	suite.NoError(err)
	err = suite.Database.Purge()
	suite.NoError(err)
}

func (suite *baseTestSuite) TearDownTest() {
	err := suite.Database.Purge()
	suite.NoError(err)
}

func (suite *baseTestSuite) TestUsers() {
	ctx := context.Background()
	// Insert users
	var insertedUsers []User
	fake := faker.New()
	for i := 9; i >= 0; i-- {
		insertedUsers = append(insertedUsers, User{
			UserId: strconv.Itoa(i),
			Labels: map[string]any{
				"color": fake.Color().ColorName(),
				"company": lo.Map(lo.Range(3), func(_, _ int) any {
					return fake.Genre().Name()
				}),
			},
			Comment: fmt.Sprintf("comment %d", i),
		})
	}
	err := suite.Database.BatchInsertUsers(ctx, insertedUsers)
	suite.NoError(err)
	// Count users
	suite.analyzeTables()
	count, err := suite.Database.CountUsers(ctx)
	suite.NoError(err)
	suite.Equal(10, count)
	// Get users
	users := suite.getUsers(ctx, 3)
	suite.Equal(10, len(users))
	for i, user := range users {
		suite.Equal(insertedUsers[9-i], user)
	}
	// Get user stream
	usersFromStream := suite.getUsersStream(ctx, 3)
	suite.ElementsMatch(insertedUsers, usersFromStream)
	// Get this user
	user, err := suite.Database.GetUser(ctx, "0")
	suite.NoError(err)
	suite.Equal("0", user.UserId)
	// Delete this user
	err = suite.Database.DeleteUser(ctx, "0")
	suite.NoError(err)
	_, err = suite.Database.GetUser(ctx, "0")
	suite.True(errors.Is(err, errors.NotFound), err)
	// test override
	err = suite.Database.BatchInsertUsers(ctx, []User{{UserId: "1", Comment: "override"}})
	suite.NoError(err)
	err = suite.Database.Optimize()
	suite.NoError(err)
	user, err = suite.Database.GetUser(ctx, "1")
	suite.NoError(err)
	suite.Equal("override", user.Comment)
	// test modify
	err = suite.Database.ModifyUser(ctx, "1", UserPatch{Comment: proto.String("modify")})
	suite.NoError(err)
	err = suite.Database.ModifyUser(ctx, "1", UserPatch{Labels: []string{"a", "b", "c"}})
	suite.NoError(err)
	err = suite.Database.ModifyUser(ctx, "1", UserPatch{Subscribe: []string{"d", "e", "f"}})
	suite.NoError(err)
	err = suite.Database.Optimize()
	suite.NoError(err)
	user, err = suite.Database.GetUser(ctx, "1")
	suite.NoError(err)
	suite.Equal("modify", user.Comment)
	suite.Equal([]any{"a", "b", "c"}, user.Labels)
	suite.Equal([]string{"d", "e", "f"}, user.Subscribe)

	// test insert empty
	err = suite.Database.BatchInsertUsers(ctx, nil)
	suite.NoError(err)

	// insert duplicate users
	err = suite.Database.BatchInsertUsers(ctx, []User{{UserId: "1"}, {UserId: "1"}})
	suite.NoError(err)
}

func (suite *baseTestSuite) TestFeedback() {
	ctx := context.Background()
	// users that already exists
	err := suite.Database.BatchInsertUsers(ctx, []User{{"0", []string{"a"}, []string{"x"}, "comment"}})
	suite.NoError(err)
	// items that already exists
	err = suite.Database.BatchInsertItems(ctx, []Item{{ItemId: "0", Labels: []string{"b"}, Timestamp: time.Date(1996, 4, 8, 10, 0, 0, 0, time.UTC)}})
	suite.NoError(err)
	// insert feedbacks
	timestamp := time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)
	feedback := []Feedback{
		{FeedbackKey{positiveFeedbackType, "0", "8"}, 1, timestamp, "comment"},
		{FeedbackKey{positiveFeedbackType, "1", "6"}, 1, timestamp, "comment"},
		{FeedbackKey{positiveFeedbackType, "2", "4"}, 1, timestamp, "comment"},
		{FeedbackKey{positiveFeedbackType, "3", "2"}, 1, timestamp, "comment"},
		{FeedbackKey{positiveFeedbackType, "4", "0"}, 1, timestamp, "comment"},
	}
	err = suite.Database.BatchInsertFeedback(ctx, feedback, true, true, true)
	suite.NoError(err)
	// other type
	err = suite.Database.BatchInsertFeedback(ctx, []Feedback{{FeedbackKey: FeedbackKey{negativeFeedbackType, "0", "2"}}}, true, true, true)
	suite.NoError(err)
	err = suite.Database.BatchInsertFeedback(ctx, []Feedback{{FeedbackKey: FeedbackKey{negativeFeedbackType, "2", "4"}}}, true, true, true)
	suite.NoError(err)
	// future feedback
	futureFeedback := []Feedback{
		{FeedbackKey{duplicateFeedbackType, "0", "0"}, 0, time.Now().Add(time.Hour), "comment"},
		{FeedbackKey{duplicateFeedbackType, "1", "2"}, 0, time.Now().Add(time.Hour), "comment"},
		{FeedbackKey{duplicateFeedbackType, "2", "4"}, 0, time.Now().Add(time.Hour), "comment"},
		{FeedbackKey{duplicateFeedbackType, "3", "6"}, 0, time.Now().Add(time.Hour), "comment"},
		{FeedbackKey{duplicateFeedbackType, "4", "8"}, 0, time.Now().Add(time.Hour), "comment"},
	}
	err = suite.Database.BatchInsertFeedback(ctx, futureFeedback, true, true, true)
	suite.NoError(err)
	// Count feedback
	suite.analyzeTables()
	count, err := suite.Database.CountFeedback(ctx)
	suite.NoError(err)
	suite.Equal(12, count)
	// Get feedback
	ret := suite.getFeedback(ctx, 3, nil, lo.ToPtr(time.Now()), positiveFeedbackType)
	suite.Equal(feedback, ret)
	ret = suite.getFeedback(ctx, 2, nil, lo.ToPtr(time.Now()))
	suite.Equal(len(feedback)+2, len(ret))
	ret = suite.getFeedback(ctx, 2, lo.ToPtr(timestamp.Add(time.Second)), lo.ToPtr(time.Now()))
	suite.Empty(ret)
	// Get feedback stream
	feedbackFromStream := suite.getFeedbackStream(ctx, 3, WithEndTime(time.Now()), WithFeedbackTypes(positiveFeedbackType))
	suite.ElementsMatch(feedback, feedbackFromStream)
	feedbackFromStream = suite.getFeedbackStream(ctx, 3, WithEndTime(time.Now()))
	suite.Equal(len(feedback)+2, len(feedbackFromStream))
	feedbackFromStream = suite.getFeedbackStream(ctx, 3, WithBeginTime(timestamp.Add(time.Second)), WithEndTime(time.Now()))
	suite.Empty(feedbackFromStream)
	feedbackFromStream = suite.getFeedbackStream(ctx, 3, WithBeginUserId("1"), WithEndUserId("3"), WithEndTime(time.Now()), WithFeedbackTypes(positiveFeedbackType))
	suite.Equal(feedback[1:4], feedbackFromStream)
	feedbackFromStream = suite.getFeedbackStream(ctx, 3, WithBeginItemId("2"), WithEndItemId("6"), WithEndTime(time.Now()), WithFeedbackTypes(positiveFeedbackType), WithOrderByItemId())
	suite.Equal([]Feedback{feedback[3], feedback[2], feedback[1]}, feedbackFromStream)
	// Get items
	err = suite.Database.Optimize()
	suite.NoError(err)
	items := suite.getItems(ctx, 3)
	suite.Equal(5, len(items))
	for i, item := range items {
		suite.Equal(strconv.Itoa(i*2), item.ItemId)
		if item.ItemId != "0" {
			if suite.isClickHouse() {
				// ClickHouse returns 1900-01-01 00:00:00 +0000 UTC as zero date.
				suite.Equal(dateTime64Zero, item.Timestamp)
			} else {
				suite.Zero(item.Timestamp)
			}
			suite.Empty(item.Labels)
			suite.Empty(item.Comment)
		}
	}
	// Get users
	users := suite.getUsers(ctx, 2)
	suite.Equal(5, len(users))
	for i, user := range users {
		suite.Equal(strconv.Itoa(i), user.UserId)
		if user.UserId != "0" {
			suite.Empty(user.Labels)
			suite.Empty(user.Subscribe)
			suite.Empty(user.Comment)
		}
	}
	// check users that already exists
	user, err := suite.Database.GetUser(ctx, "0")
	suite.NoError(err)
	suite.Equal(User{"0", []any{"a"}, []string{"x"}, "comment"}, user)
	// check items that already exists
	item, err := suite.Database.GetItem(ctx, "0")
	suite.NoError(err)
	suite.Equal(Item{ItemId: "0", Labels: []any{"b"}, Timestamp: time.Date(1996, 4, 8, 10, 0, 0, 0, time.UTC)}, item)
	// Get typed feedback by user
	ret, err = suite.Database.GetUserFeedback(ctx, "2", lo.ToPtr(time.Now()), positiveFeedbackType)
	suite.NoError(err)
	if suite.Equal(1, len(ret)) {
		suite.Equal("2", ret[0].UserId)
		suite.Equal("4", ret[0].ItemId)
	}
	// Get all feedback by user
	ret, err = suite.Database.GetUserFeedback(ctx, "2", lo.ToPtr(time.Now()))
	suite.NoError(err)
	suite.Equal(2, len(ret))
	// Get typed feedback by item
	ret, err = suite.Database.GetItemFeedback(ctx, "4", positiveFeedbackType)
	suite.NoError(err)
	suite.Equal(1, len(ret))
	suite.Equal("2", ret[0].UserId)
	suite.Equal("4", ret[0].ItemId)
	// Get all feedback by item
	ret, err = suite.Database.GetItemFeedback(ctx, "4")
	suite.NoError(err)
	suite.Equal(2, len(ret))
	// test override
	err = suite.Database.BatchInsertFeedback(ctx, []Feedback{{
		FeedbackKey: FeedbackKey{positiveFeedbackType, "0", "8"},
		Comment:     "override",
	}}, true, true, true)
	suite.NoError(err)
	err = suite.Database.Optimize()
	suite.NoError(err)
	ret, err = suite.Database.GetUserFeedback(ctx, "0", lo.ToPtr(time.Now()), positiveFeedbackType)
	suite.NoError(err)
	suite.Equal(1, len(ret))
	suite.Equal("override", ret[0].Comment)
	// test not overwrite
	err = suite.Database.BatchInsertFeedback(ctx, []Feedback{{
		FeedbackKey: FeedbackKey{positiveFeedbackType, "0", "8"},
		Comment:     "not_override",
	}}, true, true, false)
	suite.NoError(err)
	err = suite.Database.Optimize()
	suite.NoError(err)
	ret, err = suite.Database.GetUserFeedback(ctx, "0", lo.ToPtr(time.Now()), positiveFeedbackType)
	suite.NoError(err)
	suite.Equal(1, len(ret))
	suite.Equal("override", ret[0].Comment)

	// insert no feedback
	err = suite.Database.BatchInsertFeedback(ctx, nil, true, true, true)
	suite.NoError(err)

	// not insert users or items
	err = suite.Database.BatchInsertFeedback(ctx, []Feedback{
		{FeedbackKey: FeedbackKey{"a", "100", "200"}},
		{FeedbackKey: FeedbackKey{"a", "0", "200"}},
		{FeedbackKey: FeedbackKey{"a", "100", "8"}},
	}, false, false, false)
	suite.NoError(err)
	result, err := suite.Database.GetUserItemFeedback(ctx, "100", "200")
	suite.NoError(err)
	suite.Empty(result)
	result, err = suite.Database.GetUserItemFeedback(ctx, "0", "200")
	suite.NoError(err)
	suite.Empty(result)
	result, err = suite.Database.GetUserItemFeedback(ctx, "100", "8")
	suite.NoError(err)
	suite.Empty(result)

	// insert valid feedback and invalid feedback at the same time
	err = suite.Database.BatchInsertFeedback(ctx, []Feedback{
		{FeedbackKey: FeedbackKey{"a", "0", "8"}},
		{FeedbackKey: FeedbackKey{"a", "100", "200"}},
	}, false, false, false)
	suite.NoError(err)

	// insert duplicate feedback
	err = suite.Database.BatchInsertFeedback(ctx, []Feedback{
		{FeedbackKey: FeedbackKey{"a", "0", "0"}, Value: 1, Timestamp: timestamp},
		{FeedbackKey: FeedbackKey{"a", "0", "0"}, Value: 1, Timestamp: timestamp},
	}, true, true, true)
	suite.NoError(err)
	err = suite.Database.BatchInsertFeedback(ctx, []Feedback{
		{FeedbackKey: FeedbackKey{"a", "0", "0"}, Value: 1, Timestamp: timestamp},
	}, true, true, false)
	suite.NoError(err)
	// check duplicate feedback
	ret, err = suite.Database.GetUserItemFeedback(ctx, "0", "0", "a")
	suite.NoError(err)
	suite.Equal([]Feedback{{FeedbackKey: FeedbackKey{"a", "0", "0"}, Value: 2, Timestamp: timestamp, Comment: ""}}, ret)
	// put duplicate feedback
	err = suite.Database.BatchInsertFeedback(ctx, []Feedback{
		{FeedbackKey: FeedbackKey{"a", "0", "0"}, Value: 1, Timestamp: timestamp},
	}, true, true, true)
	suite.NoError(err)
	// check duplicate feedback again
	ret, err = suite.Database.GetUserItemFeedback(ctx, "0", "0", "a")
	suite.NoError(err)
	suite.Equal([]Feedback{{FeedbackKey: FeedbackKey{"a", "0", "0"}, Value: 1, Timestamp: timestamp, Comment: ""}}, ret)
}

func (suite *baseTestSuite) TestItems() {
	ctx := context.Background()
	// Items
	items := []Item{
		{
			ItemId:     "0",
			IsHidden:   true,
			Categories: []string{"a"},
			Timestamp:  time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:     []any{"a"},
			Comment:    "comment 0",
		},
		{
			ItemId:     "2",
			Categories: []string{"b"},
			Timestamp:  time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:     []any{"a"},
			Comment:    "comment 2",
		},
		{
			ItemId:     "4",
			IsHidden:   true,
			Categories: []string{"a"},
			Timestamp:  time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:     []any{"a", "b"},
			Comment:    "comment 4",
		},
		{
			ItemId:     "6",
			Categories: []string{"b"},
			Timestamp:  time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:     []any{"b"},
			Comment:    "comment 6",
		},
		{
			ItemId:     "8",
			IsHidden:   true,
			Categories: []string{"a"},
			Timestamp:  time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:     []any{"b"},
			Comment:    "comment 8",
		},
	}
	// Insert item
	err := suite.Database.BatchInsertItems(ctx, items)
	suite.NoError(err)
	// Count items
	suite.analyzeTables()
	count, err := suite.Database.CountItems(ctx)
	suite.NoError(err)
	suite.Equal(5, count)
	// Get items
	totalItems := suite.getItems(ctx, 3)
	suite.Equal(items, totalItems)
	// Get item stream
	itemsFromStream := suite.getItemStream(ctx, 3)
	suite.ElementsMatch(items, itemsFromStream)
	// Get item
	for _, item := range items {
		ret, err := suite.Database.GetItem(ctx, item.ItemId)
		suite.NoError(err)
		suite.Equal(item, ret)
	}
	// batch get items
	batchItem, err := suite.Database.BatchGetItems(ctx, []string{"2", "6"})
	suite.NoError(err)
	suite.Equal([]Item{items[1], items[3]}, batchItem)
	// Delete item
	err = suite.Database.DeleteItem(ctx, "0")
	suite.NoError(err)
	_, err = suite.Database.GetItem(ctx, "0")
	suite.True(errors.Is(err, errors.NotFound), err)

	// test override
	err = suite.Database.BatchInsertItems(ctx, []Item{{ItemId: "4", IsHidden: false, Categories: []string{"b"}, Labels: []string{"o"}, Comment: "override"}})
	suite.NoError(err)
	err = suite.Database.Optimize()
	suite.NoError(err)
	item, err := suite.Database.GetItem(ctx, "4")
	suite.NoError(err)
	suite.False(item.IsHidden)
	suite.Equal([]string{"b"}, item.Categories)
	suite.Equal([]any{"o"}, item.Labels)
	suite.Equal("override", item.Comment)

	// test modify
	timestamp := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)
	err = suite.Database.ModifyItem(ctx, "2", ItemPatch{IsHidden: proto.Bool(true)})
	suite.NoError(err)
	err = suite.Database.ModifyItem(ctx, "2", ItemPatch{Categories: []string{"a"}})
	suite.NoError(err)
	err = suite.Database.ModifyItem(ctx, "2", ItemPatch{Comment: proto.String("modify")})
	suite.NoError(err)
	err = suite.Database.ModifyItem(ctx, "2", ItemPatch{Labels: []string{"a", "b", "c"}})
	suite.NoError(err)
	err = suite.Database.ModifyItem(ctx, "2", ItemPatch{Timestamp: &timestamp})
	suite.NoError(err)
	err = suite.Database.Optimize()
	suite.NoError(err)
	item, err = suite.Database.GetItem(ctx, "2")
	suite.NoError(err)
	suite.True(item.IsHidden)
	suite.Equal([]string{"a"}, item.Categories)
	suite.Equal("modify", item.Comment)
	suite.Equal([]any{"a", "b", "c"}, item.Labels)
	suite.Equal(timestamp, item.Timestamp)

	// test insert empty
	err = suite.Database.BatchInsertItems(ctx, nil)
	suite.NoError(err)
	// test get empty
	items, err = suite.Database.BatchGetItems(ctx, nil)
	suite.NoError(err)
	suite.Empty(items)

	// test insert duplicate items
	err = suite.Database.BatchInsertItems(ctx, []Item{{ItemId: "1"}, {ItemId: "1"}})
	suite.NoError(err)
}

func (suite *baseTestSuite) TestDeleteUser() {
	ctx := context.Background()
	// Insert ret
	feedback := []Feedback{
		{FeedbackKey{positiveFeedbackType, "a", "0"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "a", "2"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "a", "4"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "a", "6"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "a", "8"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
	}
	err := suite.Database.BatchInsertFeedback(ctx, feedback, true, true, true)
	suite.NoError(err)
	// Delete user
	err = suite.Database.DeleteUser(ctx, "a")
	suite.NoError(err)
	_, err = suite.Database.GetUser(ctx, "a")
	suite.NotNil(err, "failed to delete user")
	ret, err := suite.Database.GetUserFeedback(ctx, "a", lo.ToPtr(time.Now()), positiveFeedbackType)
	suite.NoError(err)
	suite.Equal(0, len(ret))
	_, ret, err = suite.Database.GetFeedback(ctx, "", 100, nil, lo.ToPtr(time.Now()), positiveFeedbackType)
	suite.NoError(err)
	suite.Empty(ret)
}

func (suite *baseTestSuite) TestDeleteItem() {
	ctx := context.Background()
	// Insert ret
	feedbacks := []Feedback{
		{FeedbackKey{positiveFeedbackType, "0", "b"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "1", "b"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "2", "b"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "3", "b"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{positiveFeedbackType, "4", "b"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
	}
	err := suite.Database.BatchInsertFeedback(ctx, feedbacks, true, true, true)
	suite.NoError(err)
	// Delete item
	err = suite.Database.DeleteItem(ctx, "b")
	suite.NoError(err)
	_, err = suite.Database.GetItem(ctx, "b")
	suite.Error(err, "failed to delete item")
	ret, err := suite.Database.GetItemFeedback(ctx, "b", positiveFeedbackType)
	suite.NoError(err)
	suite.Equal(0, len(ret))
	_, ret, err = suite.Database.GetFeedback(ctx, "", 100, nil, lo.ToPtr(time.Now()), positiveFeedbackType)
	suite.NoError(err)
	suite.Empty(ret)
}

func (suite *baseTestSuite) TestDeleteFeedback() {
	ctx := context.Background()
	feedbacks := []Feedback{
		{FeedbackKey{"type1", "2", "3"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type2", "2", "3"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type3", "2", "3"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type1", "2", "4"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type1", "1", "3"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
	}
	err := suite.Database.BatchInsertFeedback(ctx, feedbacks, true, true, true)
	suite.NoError(err)
	// get user-item feedback
	ret, err := suite.Database.GetUserItemFeedback(ctx, "2", "3")
	suite.NoError(err)
	suite.ElementsMatch([]Feedback{feedbacks[0], feedbacks[1], feedbacks[2]}, ret)
	feedbackType2 := "type2"
	ret, err = suite.Database.GetUserItemFeedback(ctx, "2", "3", feedbackType2)
	suite.NoError(err)
	suite.Equal([]Feedback{feedbacks[1]}, ret)
	// delete user-item feedback
	deleteCount, err := suite.Database.DeleteUserItemFeedback(ctx, "2", "3")
	suite.NoError(err)
	if !suite.isClickHouse() {
		// RowAffected isn't supported by ClickHouse,
		suite.Equal(3, deleteCount)
	}
	err = suite.Database.Optimize()
	suite.NoError(err)
	ret, err = suite.Database.GetUserItemFeedback(ctx, "2", "3")
	suite.NoError(err)
	suite.Empty(ret)
	feedbackType1 := "type1"
	deleteCount, err = suite.Database.DeleteUserItemFeedback(ctx, "1", "3", feedbackType1)
	suite.NoError(err)
	if !suite.isClickHouse() {
		// RowAffected isn't supported by ClickHouse,
		suite.Equal(1, deleteCount)
	}
	ret, err = suite.Database.GetUserItemFeedback(ctx, "1", "3", feedbackType2)
	suite.NoError(err)
	suite.Empty(ret)
}

func (suite *baseTestSuite) TestTimeLimit() {
	ctx := context.Background()
	// insert items
	items := []Item{
		{
			ItemId:    "0",
			Timestamp: time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []any{"a"},
			Comment:   "comment 0",
		},
		{
			ItemId:    "2",
			Timestamp: time.Date(1997, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []any{"a"},
			Comment:   "comment 2",
		},
		{
			ItemId:    "4",
			Timestamp: time.Date(1998, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []any{"a", "b"},
			Comment:   "comment 4",
		},
		{
			ItemId:    "6",
			Timestamp: time.Date(1999, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []any{"b"},
			Comment:   "comment 6",
		},
		{
			ItemId:    "8",
			Timestamp: time.Date(2000, 3, 15, 0, 0, 0, 0, time.UTC),
			Labels:    []any{"b"},
			Comment:   "comment 8",
		},
	}
	err := suite.Database.BatchInsertItems(ctx, items)
	suite.NoError(err)
	timeLimit := time.Date(1998, 1, 1, 0, 0, 0, 0, time.UTC)
	_, ret, err := suite.Database.GetItems(ctx, "", 100, &timeLimit)
	suite.NoError(err)
	suite.Equal([]Item{items[2], items[3], items[4]}, ret)

	// insert feedback
	feedbacks := []Feedback{
		{FeedbackKey{"type1", "2", "3"}, 0, time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type2", "2", "3"}, 0, time.Date(1997, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type3", "2", "3"}, 0, time.Date(1998, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type1", "2", "4"}, 0, time.Date(1999, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
		{FeedbackKey{"type1", "1", "3"}, 0, time.Date(2000, 3, 15, 0, 0, 0, 0, time.UTC), "comment"},
	}
	err = suite.Database.BatchInsertFeedback(ctx, feedbacks, true, true, true)
	suite.NoError(err)
	_, retFeedback, err := suite.Database.GetFeedback(ctx, "", 100, &timeLimit, lo.ToPtr(time.Now()))
	suite.NoError(err)
	suite.Equal([]Feedback{feedbacks[4], feedbacks[3], feedbacks[2]}, retFeedback)
	typeFilter := "type1"
	_, retFeedback, err = suite.Database.GetFeedback(ctx, "", 100, &timeLimit, lo.ToPtr(time.Now()), typeFilter)
	suite.NoError(err)
	suite.Equal([]Feedback{feedbacks[4], feedbacks[3]}, retFeedback)
}

func (suite *baseTestSuite) TestTimezone() {
	ctx := context.Background()
	loc, err := time.LoadLocation("Asia/Tokyo")
	suite.NoError(err)
	// insert feedbacks
	err = suite.Database.BatchInsertFeedback(ctx, []Feedback{
		{FeedbackKey: FeedbackKey{"read", "1", "1"}, Timestamp: time.Now().Add(-time.Second).In(loc)},
		{FeedbackKey: FeedbackKey{"read", "1", "2"}, Timestamp: time.Now().Add(-time.Second).In(loc)},
		{FeedbackKey: FeedbackKey{"read", "2", "2"}, Timestamp: time.Now().Add(-time.Second).In(loc)},
		{FeedbackKey: FeedbackKey{"like", "1", "1"}, Timestamp: time.Now().Add(time.Hour).In(loc)},
		{FeedbackKey: FeedbackKey{"like", "1", "2"}, Timestamp: time.Now().Add(time.Hour).In(loc)},
		{FeedbackKey: FeedbackKey{"like", "2", "2"}, Timestamp: time.Now().Add(time.Hour).In(loc)},
	}, true, true, true)
	suite.NoError(err)
	// get feedback stream
	feedback := suite.getFeedback(ctx, 10, nil, lo.ToPtr(time.Now()))
	suite.Equal(3, len(feedback))
	// get feedback
	_, feedback, err = suite.Database.GetFeedback(ctx, "", 10, nil, lo.ToPtr(time.Now()))
	suite.NoError(err)
	suite.Equal(3, len(feedback))
	// get user feedback
	feedback, err = suite.Database.GetUserFeedback(ctx, "1", lo.ToPtr(time.Now()))
	suite.NoError(err)
	suite.Equal(2, len(feedback))
	// get item feedback
	feedback, err = suite.Database.GetItemFeedback(ctx, "2") // no future feedback by default
	suite.NoError(err)
	suite.Equal(2, len(feedback))
	// get user item feedback
	feedback, err = suite.Database.GetUserItemFeedback(ctx, "1", "1") // return future feedback by default
	suite.NoError(err)
	suite.Equal(2, len(feedback))

	// insert items
	now := time.Now().In(loc)
	err = suite.Database.BatchInsertItems(ctx, []Item{{ItemId: "100", Timestamp: now}, {ItemId: "200"}})
	suite.NoError(err)
	err = suite.Database.ModifyItem(ctx, "200", ItemPatch{Timestamp: &now})
	suite.NoError(err)
	err = suite.Database.Optimize()
	suite.NoError(err)
	switch database := suite.Database.(type) {
	case *SQLDatabase:
		switch suite.Database.(*SQLDatabase).driver {
		case Postgres:
			item, err := suite.Database.GetItem(ctx, "100")
			suite.NoError(err)
			suite.Equal(now.Round(time.Microsecond).In(time.UTC), item.Timestamp)
			item, err = suite.Database.GetItem(ctx, "200")
			suite.NoError(err)
			suite.Equal(now.Round(time.Microsecond).In(time.UTC), item.Timestamp)
		case ClickHouse:
			item, err := suite.Database.GetItem(ctx, "100")
			suite.NoError(err)
			suite.Equal(now.Truncate(time.Second).In(time.UTC), item.Timestamp)
			item, err = suite.Database.GetItem(ctx, "200")
			suite.NoError(err)
			suite.Equal(now.Truncate(time.Second).In(time.UTC), item.Timestamp)
		case SQLite:
			item, err := suite.Database.GetItem(ctx, "100")
			suite.NoError(err)
			suite.Equal(now.In(time.UTC), item.Timestamp.In(time.UTC))
			item, err = suite.Database.GetItem(ctx, "200")
			suite.NoError(err)
			suite.Equal(now.In(time.UTC), item.Timestamp.In(time.UTC))
		default:
			suite.T().Skipf("unknown sql database: %v", database.driver)
		}
	case *MongoDB:
		item, err := suite.Database.GetItem(ctx, "100")
		suite.NoError(err)
		suite.Equal(now.Truncate(time.Millisecond).In(time.UTC), item.Timestamp)
		item, err = suite.Database.GetItem(ctx, "200")
		suite.NoError(err)
		suite.Equal(now.Truncate(time.Millisecond).In(time.UTC), item.Timestamp)
	default:
		suite.T().Skipf("unknown database: %v", reflect.TypeOf(suite.Database))
	}
}

func (suite *baseTestSuite) TestPurge() {
	ctx := context.Background()
	// insert data
	err := suite.Database.BatchInsertFeedback(ctx, lo.Map(lo.Range(100), func(t int, i int) Feedback {
		return Feedback{FeedbackKey: FeedbackKey{
			FeedbackType: "click",
			UserId:       strconv.Itoa(t),
			ItemId:       strconv.Itoa(t),
		}}
	}), true, true, true)
	suite.NoError(err)
	_, users, err := suite.Database.GetUsers(ctx, "", 100)
	suite.NoError(err)
	suite.Equal(100, len(users))
	_, items, err := suite.Database.GetItems(ctx, "", 100, nil)
	suite.NoError(err)
	suite.Equal(100, len(items))
	_, feedbacks, err := suite.Database.GetFeedback(ctx, "", 100, nil, lo.ToPtr(time.Now()))
	suite.NoError(err)
	suite.Equal(100, len(feedbacks))
	// purge data
	err = suite.Database.Purge()
	suite.NoError(err)
	_, users, err = suite.Database.GetUsers(ctx, "", 100)
	suite.NoError(err)
	suite.Empty(users)
	_, items, err = suite.Database.GetItems(ctx, "", 100, nil)
	suite.NoError(err)
	suite.Empty(items)
	_, feedbacks, err = suite.Database.GetFeedback(ctx, "", 100, nil, lo.ToPtr(time.Now()))
	suite.NoError(err)
	suite.Empty(feedbacks)
	// purge empty database
	err = suite.Database.Purge()
	suite.NoError(err)
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

func TestValidateLabels(t *testing.T) {
	assert.NoError(t, ValidateLabels(nil))
	assert.NoError(t, ValidateLabels(json.Number("1")))
	assert.NoError(t, ValidateLabels("label"))
	assert.NoError(t, ValidateLabels([]any{json.Number("1"), json.Number("2"), json.Number("3")}))
	assert.NoError(t, ValidateLabels([]any{"1", "2", "3"}))
	assert.NoError(t, ValidateLabels(map[string]any{"city": json.Number("1"), "tags": []any{json.Number("1"), json.Number("2"), json.Number("3")}}))
	assert.NoError(t, ValidateLabels(map[string]any{"city": "wenzhou", "tags": []any{"1", "2", "3"}}))
	assert.NoError(t, ValidateLabels(map[string]any{"address": map[string]any{"province": json.Number("1"), "city": json.Number("2")}}))
	assert.NoError(t, ValidateLabels(map[string]any{"address": map[string]any{"province": "zhejiang", "city": "wenzhou"}}))

	assert.Error(t, ValidateLabels(map[string]any{"price": 100, "tags": []any{json.Number("1"), "2", "3"}}))
	assert.Error(t, ValidateLabels(map[string]any{"city": "wenzhou", "tags": []any{"1", json.Number("2"), "3"}}))
	assert.Error(t, ValidateLabels(map[string]any{"city": "wenzhou", "tags": []any{"1", "2", json.Number("3")}}))
}

func benchmarkCountItems(b *testing.B, db Database) {
	ctx := context.Background()
	// Insert 10,000 items
	items := make([]Item, 100000)
	for i := range items {
		items[i] = Item{ItemId: strconv.Itoa(i)}
	}
	err := db.BatchInsertItems(ctx, items)
	require.NoError(b, err)
	// Benchmark count items
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n, err := db.CountItems(ctx)
		require.NoError(b, err)
		require.Equal(b, 100000, n)
	}
}
