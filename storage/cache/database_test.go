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
	"math"
	"testing"
	"time"

	"github.com/juju/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
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

func (suite *baseTestSuite) TestSort() {
	ctx := context.Background()
	// Put scores
	scores := []Scored{
		{"0", 0},
		{"1", 1.1},
		{"2", 1.2},
		{"3", 1.3},
		{"4", 1.4},
	}
	err := suite.Database.SetSorted(ctx, "sort", scores[:3])
	suite.NoError(err)
	err = suite.Database.AddSorted(ctx, SortedSet{"sort", scores[3:]})
	suite.NoError(err)
	// Get scores
	totalItems, err := suite.Database.GetSorted(ctx, "sort", 0, -1)
	suite.NoError(err)
	suite.Equal([]Scored{
		{"4", 1.4},
		{"3", 1.3},
		{"2", 1.2},
		{"1", 1.1},
		{"0", 0},
	}, totalItems)
	halfItems, err := suite.Database.GetSorted(ctx, "sort", 0, 2)
	suite.NoError(err)
	suite.Equal([]Scored{
		{"4", 1.4},
		{"3", 1.3},
		{"2", 1.2},
	}, halfItems)
	halfItems, err = suite.Database.GetSorted(ctx, "sort", 1, 2)
	suite.NoError(err)
	suite.Equal([]Scored{
		{"3", 1.3},
		{"2", 1.2},
	}, halfItems)
	// get scores by score
	partItems, err := suite.Database.GetSortedByScore(ctx, "sort", 1.1, 1.3)
	suite.NoError(err)
	suite.Equal([]Scored{
		{"1", 1.1},
		{"2", 1.2},
		{"3", 1.3},
	}, partItems)
	// remove scores by score
	err = suite.Database.AddSorted(ctx, SortedSet{"sort", []Scored{
		{"5", -5},
		{"6", -6},
	}})
	suite.NoError(err)
	err = suite.Database.RemSortedByScore(ctx, "sort", math.Inf(-1), -1)
	suite.NoError(err)
	partItems, err = suite.Database.GetSortedByScore(ctx, "sort", math.Inf(-1), -1)
	suite.NoError(err)
	suite.Empty(partItems)
	// Remove score
	err = suite.Database.RemSorted(ctx, Member("sort", "0"))
	suite.NoError(err)
	totalItems, err = suite.Database.GetSorted(ctx, "sort", 0, -1)
	suite.NoError(err)
	suite.Equal([]Scored{
		{"4", 1.4},
		{"3", 1.3},
		{"2", 1.2},
		{"1", 1.1},
	}, totalItems)

	// test set empty
	err = suite.Database.SetSorted(ctx, "sort", []Scored{})
	suite.NoError(err)
	// test set duplicate
	err = suite.Database.SetSorted(ctx, "sort1000", []Scored{
		{"100", 1},
		{"100", 2},
	})
	suite.NoError(err)
	// test add empty
	err = suite.Database.AddSorted(ctx)
	suite.NoError(err)
	err = suite.Database.AddSorted(ctx, SortedSet{})
	suite.NoError(err)
	// test add duplicate
	err = suite.Database.AddSorted(ctx, SortedSet{"sort1000", []Scored{{"100", 1}, {"100", 2}}})
	suite.NoError(err)
	// test get empty
	scores, err = suite.Database.GetSorted(ctx, "sort", 0, -1)
	suite.NoError(err)
	suite.Empty(scores)
	scores, err = suite.Database.GetSorted(ctx, "sort", 10, 5)
	suite.NoError(err)
	suite.Empty(scores)
}

func (suite *baseTestSuite) TestScan() {
	ctx := context.Background()
	err := suite.Database.Set(ctx, String("1", "1"))
	suite.NoError(err)
	err = suite.Database.SetSet(ctx, "2", "21", "22", "23")
	suite.NoError(err)
	err = suite.Database.SetSorted(ctx, "3", []Scored{{"1", 1}, {"2", 2}, {"3", 3}})
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

	err = suite.Database.AddSorted(ctx, Sorted("sorted", []Scored{{Id: "a", Score: 1}, {Id: "b", Score: 2}, {Id: "c", Score: 3}}))
	suite.NoError(err)
	z, err := suite.Database.GetSorted(ctx, "sorted", 0, -1)
	suite.NoError(err)
	suite.ElementsMatch([]Scored{{Id: "a", Score: 1}, {Id: "b", Score: 2}, {Id: "c", Score: 3}}, z)

	// purge data
	err = suite.Database.Purge()
	suite.NoError(err)
	ret = suite.Database.Get(ctx, "key")
	suite.ErrorIs(ret.err, errors.NotFound)
	s, err = suite.Database.GetSet(ctx, "set")
	suite.NoError(err)
	suite.Empty(s)
	z, err = suite.Database.GetSorted(ctx, "sorted", 0, -1)
	suite.NoError(err)
	suite.Empty(z)

	// purge empty dataset
	err = suite.Database.Purge()
	suite.NoError(err)
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
