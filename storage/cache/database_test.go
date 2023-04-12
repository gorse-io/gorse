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
package cache

import (
	"context"
	"io"
	"math"
	"testing"
	"time"

	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

type baseTestSuite struct {
	suite.Suite
	Database
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

func (suite *baseTestSuite) TestInit() {
	err := suite.Database.Init()
	suite.NoError(err)
}

func (suite *baseTestSuite) TestMeta() {
	ctx := context.Background()
	// Set meta string
	err := suite.Database.Set(ctx, String(Key("meta", "1"), "2"), String(Key("meta", "1000"), "10"))
	suite.NoError(err)
	// Get meta string
	value, err := suite.Database.Get(ctx, Key("meta", "1")).String()
	suite.NoError(err)
	suite.Equal("2", value)
	value, err = suite.Database.Get(ctx, Key("meta", "1000")).String()
	suite.NoError(err)
	suite.Equal("10", value)
	// Delete string
	err = suite.Database.Delete(ctx, Key("meta", "1"))
	suite.NoError(err)
	// Get meta not existed
	value, err = suite.Database.Get(ctx, Key("meta", "1")).String()
	suite.True(errors.Is(err, errors.NotFound), err)
	suite.Equal("", value)
	// Set meta int
	err = suite.Database.Set(ctx, Integer(Key("meta", "1"), 2))
	suite.NoError(err)
	// Get meta int
	valInt, err := suite.Database.Get(ctx, Key("meta", "1")).Integer()
	suite.NoError(err)
	suite.Equal(2, valInt)
	// set meta time
	err = suite.Database.Set(ctx, Time(Key("meta", "1"), time.Date(1996, 4, 8, 0, 0, 0, 0, time.UTC)))
	suite.NoError(err)
	// get meta time
	valTime, err := suite.Database.Get(ctx, Key("meta", "1")).Time()
	suite.NoError(err)
	suite.Equal(1996, valTime.Year())
	suite.Equal(time.Month(4), valTime.Month())
	suite.Equal(8, valTime.Day())

	// test set empty
	err = suite.Database.Set(ctx)
	suite.NoError(err)
	// test set duplicate
	err = suite.Database.Set(ctx, String("100", "1"), String("100", "2"))
	suite.NoError(err)
}

func (suite *baseTestSuite) TestSet() {
	ctx := context.Background()
	err := suite.Database.SetSet(ctx, "set", "1")
	suite.NoError(err)
	// test add
	err = suite.Database.AddSet(ctx, "set", "2")
	suite.NoError(err)
	var members []string
	members, err = suite.Database.GetSet(ctx, "set")
	suite.NoError(err)
	suite.Equal([]string{"1", "2"}, members)
	// test rem
	err = suite.Database.RemSet(ctx, "set", "1")
	suite.NoError(err)
	members, err = suite.Database.GetSet(ctx, "set")
	suite.NoError(err)
	suite.Equal([]string{"2"}, members)
	// test set
	err = suite.Database.SetSet(ctx, "set", "3")
	suite.NoError(err)
	members, err = suite.Database.GetSet(ctx, "set")
	suite.NoError(err)
	suite.Equal([]string{"3"}, members)

	// test add empty
	err = suite.Database.AddSet(ctx, "set")
	suite.NoError(err)
	// test set empty
	err = suite.Database.SetSet(ctx, "set")
	suite.NoError(err)
	// test get empty
	members, err = suite.Database.GetSet(ctx, "unknown_set")
	suite.NoError(err)
	suite.Empty(members)
	// test rem empty
	err = suite.Database.RemSet(ctx, "set")
	suite.NoError(err)
}

func (suite *baseTestSuite) TestScan() {
	ctx := context.Background()
	err := suite.Database.Set(ctx, String("1", "1"))
	suite.NoError(err)
	err = suite.Database.SetSet(ctx, "2", "21", "22", "23")
	suite.NoError(err)

	var keys []string
	err = suite.Database.Scan(func(s string) error {
		keys = append(keys, s)
		return nil
	})
	suite.NoError(err)
	suite.ElementsMatch([]string{"1", "2", "3"}, keys)
}

func (suite *baseTestSuite) TestPurge() {
	ctx := context.Background()
	// insert data
	err := suite.Database.Set(ctx, String("key", "value"))
	suite.NoError(err)
	ret := suite.Database.Get(ctx, "key")
	suite.NoError(ret.err)
	suite.Equal("value", ret.value)

	err = suite.Database.AddSet(ctx, "set", "a", "b", "c")
	suite.NoError(err)
	s, err := suite.Database.GetSet(ctx, "set")
	suite.NoError(err)
	suite.ElementsMatch([]string{"a", "b", "c"}, s)

	// purge data
	err = suite.Database.Purge()
	suite.NoError(err)
	ret = suite.Database.Get(ctx, "key")
	suite.ErrorIs(ret.err, errors.NotFound)
	s, err = suite.Database.GetSet(ctx, "set")
	suite.NoError(err)
	suite.Empty(s)

	// purge empty dataset
	err = suite.Database.Purge()
	suite.NoError(err)
}

func (suite *baseTestSuite) TestPushPop() {
	ctx := context.Background()
	err := suite.Push(ctx, "a", "1")
	suite.NoError(err)
	err = suite.Push(ctx, "a", "2")
	suite.NoError(err)
	count, err := suite.Remain(ctx, "a")
	suite.NoError(err)
	suite.Equal(int64(2), count)

	err = suite.Push(ctx, "b", "1")
	suite.NoError(err)
	err = suite.Push(ctx, "b", "2")
	suite.NoError(err)
	err = suite.Push(ctx, "b", "1")
	suite.NoError(err)
	count, err = suite.Remain(ctx, "b")
	suite.NoError(err)
	suite.Equal(int64(2), count)

	value, err := suite.Pop(ctx, "a")
	suite.NoError(err)
	suite.Equal("1", value)
	value, err = suite.Pop(ctx, "a")
	suite.NoError(err)
	suite.Equal("2", value)
	_, err = suite.Pop(ctx, "a")
	suite.ErrorIs(err, io.EOF)

	value, err = suite.Pop(ctx, "b")
	suite.NoError(err)
	suite.Equal("2", value)
	value, err = suite.Pop(ctx, "b")
	suite.NoError(err)
	suite.Equal("1", value)
	_, err = suite.Pop(ctx, "b")
	suite.ErrorIs(err, io.EOF)
}

func (suite *baseTestSuite) TestDocument() {
	ts := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx := context.Background()
	err := suite.AddDocuments(ctx, "a", "", Document{
		Value:      "0",
		Score:      math.MaxFloat64,
		IsHidden:   true,
		Categories: []string{"a", "b"},
		Timestamp:  ts,
	})
	suite.NoError(err)
	err = suite.AddDocuments(ctx, "a", "", Document{
		Value:      "1",
		Score:      100,
		Categories: []string{"a", "b"},
		Timestamp:  ts,
	})
	suite.NoError(err)
	err = suite.AddDocuments(ctx, "a", "", Document{
		Value:      "1",
		Score:      1,
		Categories: []string{"a", "b"},
		Timestamp:  ts,
	})
	suite.NoError(err)
	err = suite.AddDocuments(ctx, "a", "", Document{
		Value:      "2",
		Score:      2,
		Categories: []string{"b", "c"},
		Timestamp:  ts,
	})
	suite.NoError(err)
	err = suite.AddDocuments(ctx, "a", "", Document{
		Value:      "3",
		Score:      3,
		Categories: []string{"b"},
		Timestamp:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	suite.NoError(err)
	err = suite.AddDocuments(ctx, "a", "", Document{
		Value:      "4",
		Score:      4,
		Categories: []string{},
		Timestamp:  ts,
	})
	suite.NoError(err)
	err = suite.AddDocuments(ctx, "a", "", Document{
		Value:      "5",
		Score:      5,
		Categories: []string{"b"},
		Timestamp:  ts,
	})
	suite.NoError(err)
	err = suite.AddDocuments(ctx, "b", "", Document{
		Value:      "6",
		Score:      6,
		Categories: []string{"b"},
		Timestamp:  ts,
	})
	suite.NoError(err)

	// search documents
	documents, err := suite.SearchDocuments(ctx, "a", "", []string{"b"}, 1, 3)
	suite.NoError(err)
	suite.Equal([]Document{
		{Value: "3", Score: 3, Categories: []string{"b"}, Timestamp: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Value: "2", Score: 2, Categories: []string{"b", "c"}, Timestamp: ts},
	}, documents)
	documents, err = suite.SearchDocuments(ctx, "a", "", []string{"b"}, 0, -1)
	suite.NoError(err)
	suite.Equal([]Document{
		{Value: "5", Score: 5, Categories: []string{"b"}, Timestamp: ts},
		{Value: "3", Score: 3, Categories: []string{"b"}, Timestamp: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Value: "2", Score: 2, Categories: []string{"b", "c"}, Timestamp: ts},
		{Value: "1", Score: 1, Categories: []string{"a", "b"}, Timestamp: ts},
	}, documents)

	// delete nothing
	err = suite.DeleteDocuments(ctx, []string{"a"}, DocumentCondition{})
	suite.ErrorIs(err, errors.NotValid)
	// delete by value
	err = suite.DeleteDocuments(ctx, []string{"a"}, DocumentCondition{Value: proto.String("5")})
	suite.NoError(err)
	documents, err = suite.SearchDocuments(ctx, "a", "", []string{"b"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("3", documents[0].Value)
	// delete by timestamp
	err = suite.DeleteDocuments(ctx, []string{"a"}, DocumentCondition{Before: lo.ToPtr(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC))})
	suite.NoError(err)
	documents, err = suite.SearchDocuments(ctx, "a", "", []string{"b"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Value)

	// update categories
	err = suite.UpdateDocuments(ctx, []string{"a"}, "2", DocumentPatch{Categories: []string{"c", "s"}})
	suite.NoError(err)
	documents, err = suite.SearchDocuments(ctx, "a", "", []string{"s"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Value)
	err = suite.UpdateDocuments(ctx, []string{"a"}, "2", DocumentPatch{Categories: []string{"c"}})
	suite.NoError(err)
	documents, err = suite.SearchDocuments(ctx, "a", "", []string{"s"}, 0, 1)
	suite.NoError(err)
	suite.Empty(documents)

	// update is hidden
	err = suite.UpdateDocuments(ctx, []string{"a"}, "0", DocumentPatch{IsHidden: proto.Bool(false)})
	suite.NoError(err)
	documents, err = suite.SearchDocuments(ctx, "a", "", []string{"b"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("0", documents[0].Value)
}

func (suite *baseTestSuite) TestSubsetDocument() {
	ts := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx := context.Background()
	err := suite.AddDocuments(ctx, "a", "a", Document{
		Value:      "1",
		Score:      1,
		Categories: []string{"a", "b"},
		Timestamp:  ts,
	})
	suite.NoError(err)
	err = suite.AddDocuments(ctx, "a", "a", Document{
		Value:      "2",
		Score:      2,
		Categories: []string{"b", "c"},
		Timestamp:  ts,
	})
	suite.NoError(err)
	err = suite.AddDocuments(ctx, "a", "a", Document{
		Value:      "3",
		Score:      3,
		Categories: []string{"b"},
		Timestamp:  ts,
	})
	suite.NoError(err)
	err = suite.AddDocuments(ctx, "b", "", Document{
		Value:      "4",
		Score:      4,
		Categories: []string{},
		Timestamp:  ts,
	})
	suite.NoError(err)
	err = suite.AddDocuments(ctx, "b", "", Document{
		Value:      "3",
		Score:      3,
		Categories: []string{"b"},
		Timestamp:  ts,
	})
	suite.NoError(err)
	err = suite.AddDocuments(ctx, "b", "", Document{
		Value:      "2",
		Score:      2,
		Categories: []string{"b"},
		Timestamp:  ts,
	})
	suite.NoError(err)

	// seach documents
	documents, err := suite.SearchDocuments(ctx, "a", "a", []string{"b"}, 0, -1)
	suite.NoError(err)
	suite.Equal([]Document{
		{Value: "3", Score: 3, Categories: []string{"b"}, Timestamp: ts},
		{Value: "2", Score: 2, Categories: []string{"b", "c"}, Timestamp: ts},
		{Value: "1", Score: 1, Categories: []string{"a", "b"}, Timestamp: ts},
	}, documents)

	// update categories
	err = suite.UpdateDocuments(ctx, []string{"a", "b"}, "2", DocumentPatch{Categories: []string{"b", "s"}})
	suite.NoError(err)
	documents, err = suite.SearchDocuments(ctx, "a", "a", []string{"s"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Value)
	documents, err = suite.SearchDocuments(ctx, "b", "", []string{"s"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Value)

	// delete by value
	err = suite.DeleteDocuments(ctx, []string{"a", "b"}, DocumentCondition{Value: proto.String("3")})
	suite.NoError(err)
	documents, err = suite.SearchDocuments(ctx, "a", "a", []string{"b"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Value)
	documents, err = suite.SearchDocuments(ctx, "b", "", []string{"b"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Value)

	// delete in subset
	err = suite.DeleteDocuments(ctx, []string{"a", "b"}, DocumentCondition{
		Subset: proto.String("a"),
		Value:  proto.String("2"),
	})
	suite.NoError(err)
	documents, err = suite.SearchDocuments(ctx, "a", "a", []string{"b"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("1", documents[0].Value)
	documents, err = suite.SearchDocuments(ctx, "b", "", []string{"b"}, 0, 1)
	suite.NoError(err)
	suite.Len(documents, 1)
	suite.Equal("2", documents[0].Value)
}

func TestScored(t *testing.T) {
	itemIds := []string{"2", "4", "6"}
	scores := []float64{2, 4, 6}
	scored := []Scored{{Id: "2", Score: 2}, {Id: "4", Score: 4}, {Id: "6", Score: 6}}
	assert.Equal(t, scored, CreateScoredItems(itemIds, scores))
	assert.Equal(t, itemIds, RemoveScores(scored))
	assert.Equal(t, scores, GetScores(scored))
	SortScores(scored)
	assert.Equal(t, []Scored{{Id: "6", Score: 6}, {Id: "4", Score: 4}, {Id: "2", Score: 2}}, scored)
}

func TestKey(t *testing.T) {
	assert.Empty(t, Key())
	assert.Equal(t, "a", Key("a"))
	assert.Equal(t, "a", Key("a", ""))
	assert.Equal(t, "a/b", Key("a", "b"))
}
