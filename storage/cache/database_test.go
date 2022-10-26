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
	"github.com/juju/errors"
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
	"time"
)

func testMeta(t *testing.T, db Database) {
	ctx := context.Background()
	// Set meta string
	err := db.Set(ctx, String(Key("meta", "1"), "2"), String(Key("meta", "1000"), "10"))
	assert.NoError(t, err)
	// Get meta string
	value, err := db.Get(ctx, Key("meta", "1")).String()
	assert.NoError(t, err)
	assert.Equal(t, "2", value)
	value, err = db.Get(ctx, Key("meta", "1000")).String()
	assert.NoError(t, err)
	assert.Equal(t, "10", value)
	// Delete string
	err = db.Delete(ctx, Key("meta", "1"))
	assert.NoError(t, err)
	// Get meta not existed
	value, err = db.Get(ctx, Key("meta", "1")).String()
	assert.True(t, errors.Is(err, errors.NotFound), err)
	assert.Equal(t, "", value)
	// Set meta int
	err = db.Set(ctx, Integer(Key("meta", "1"), 2))
	assert.NoError(t, err)
	// Get meta int
	valInt, err := db.Get(ctx, Key("meta", "1")).Integer()
	assert.NoError(t, err)
	assert.Equal(t, 2, valInt)
	// set meta time
	err = db.Set(ctx, Time(Key("meta", "1"), time.Date(1996, 4, 8, 0, 0, 0, 0, time.UTC)))
	assert.NoError(t, err)
	// get meta time
	valTime, err := db.Get(ctx, Key("meta", "1")).Time()
	assert.NoError(t, err)
	assert.Equal(t, 1996, valTime.Year())
	assert.Equal(t, time.Month(4), valTime.Month())
	assert.Equal(t, 8, valTime.Day())

	// test set empty
	err = db.Set(ctx)
	assert.NoError(t, err)
	// test set duplicate
	err = db.Set(ctx, String("100", "1"), String("100", "2"))
	assert.NoError(t, err)
}

func testSet(t *testing.T, db Database) {
	ctx := context.Background()
	err := db.SetSet(ctx, "set", "1")
	assert.NoError(t, err)
	// test add
	err = db.AddSet(ctx, "set", "2")
	assert.NoError(t, err)
	var members []string
	members, err = db.GetSet(ctx, "set")
	assert.NoError(t, err)
	assert.Equal(t, []string{"1", "2"}, members)
	// test rem
	err = db.RemSet(ctx, "set", "1")
	assert.NoError(t, err)
	members, err = db.GetSet(ctx, "set")
	assert.NoError(t, err)
	assert.Equal(t, []string{"2"}, members)
	// test set
	err = db.SetSet(ctx, "set", "3")
	assert.NoError(t, err)
	members, err = db.GetSet(ctx, "set")
	assert.NoError(t, err)
	assert.Equal(t, []string{"3"}, members)

	// test add empty
	err = db.AddSet(ctx, "set")
	assert.NoError(t, err)
	// test set empty
	err = db.SetSet(ctx, "set")
	assert.NoError(t, err)
	// test get empty
	members, err = db.GetSet(ctx, "unknown_set")
	assert.NoError(t, err)
	assert.Empty(t, members)
	// test rem empty
	err = db.RemSet(ctx, "set")
	assert.NoError(t, err)
}

func testSort(t *testing.T, db Database) {
	ctx := context.Background()
	// Put scores
	scores := []Scored{
		{"0", 0},
		{"1", 1.1},
		{"2", 1.2},
		{"3", 1.3},
		{"4", 1.4},
	}
	err := db.SetSorted(ctx, "sort", scores[:3])
	assert.NoError(t, err)
	err = db.AddSorted(ctx, SortedSet{"sort", scores[3:]})
	assert.NoError(t, err)
	// Get scores
	totalItems, err := db.GetSorted(ctx, "sort", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []Scored{
		{"4", 1.4},
		{"3", 1.3},
		{"2", 1.2},
		{"1", 1.1},
		{"0", 0},
	}, totalItems)
	halfItems, err := db.GetSorted(ctx, "sort", 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []Scored{
		{"4", 1.4},
		{"3", 1.3},
		{"2", 1.2},
	}, halfItems)
	halfItems, err = db.GetSorted(ctx, "sort", 1, 2)
	assert.NoError(t, err)
	assert.Equal(t, []Scored{
		{"3", 1.3},
		{"2", 1.2},
	}, halfItems)
	// get scores by score
	partItems, err := db.GetSortedByScore(ctx, "sort", 1.1, 1.3)
	assert.NoError(t, err)
	assert.Equal(t, []Scored{
		{"1", 1.1},
		{"2", 1.2},
		{"3", 1.3},
	}, partItems)
	// remove scores by score
	err = db.AddSorted(ctx, SortedSet{"sort", []Scored{
		{"5", -5},
		{"6", -6},
	}})
	assert.NoError(t, err)
	err = db.RemSortedByScore(ctx, "sort", math.Inf(-1), -1)
	assert.NoError(t, err)
	partItems, err = db.GetSortedByScore(ctx, "sort", math.Inf(-1), -1)
	assert.NoError(t, err)
	assert.Empty(t, partItems)
	// Remove score
	err = db.RemSorted(ctx, Member("sort", "0"))
	assert.NoError(t, err)
	totalItems, err = db.GetSorted(ctx, "sort", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []Scored{
		{"4", 1.4},
		{"3", 1.3},
		{"2", 1.2},
		{"1", 1.1},
	}, totalItems)

	// test set empty
	err = db.SetSorted(ctx, "sort", []Scored{})
	assert.NoError(t, err)
	// test set duplicate
	err = db.SetSorted(ctx, "sort1000", []Scored{
		{"100", 1},
		{"100", 2},
	})
	assert.NoError(t, err)
	// test add empty
	err = db.AddSorted(ctx)
	assert.NoError(t, err)
	err = db.AddSorted(ctx, SortedSet{})
	assert.NoError(t, err)
	// test add duplicate
	err = db.AddSorted(ctx, SortedSet{"sort1000", []Scored{{"100", 1}, {"100", 2}}})
	assert.NoError(t, err)
	// test get empty
	scores, err = db.GetSorted(ctx, "sort", 0, -1)
	assert.NoError(t, err)
	assert.Empty(t, scores)
	scores, err = db.GetSorted(ctx, "sort", 10, 5)
	assert.NoError(t, err)
	assert.Empty(t, scores)
}

func testScan(t *testing.T, db Database) {
	ctx := context.Background()
	err := db.Set(ctx, String("1", "1"))
	assert.NoError(t, err)
	err = db.SetSet(ctx, "2", "21", "22", "23")
	assert.NoError(t, err)
	err = db.SetSorted(ctx, "3", []Scored{{"1", 1}, {"2", 2}, {"3", 3}})
	assert.NoError(t, err)

	var keys []string
	err = db.Scan(func(s string) error {
		keys = append(keys, s)
		return nil
	})
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"1", "2", "3"}, keys)
}

func testPurge(t *testing.T, db Database) {
	ctx := context.Background()
	// insert data
	err := db.Set(ctx, String("key", "value"))
	assert.NoError(t, err)
	ret := db.Get(ctx, "key")
	assert.NoError(t, ret.err)
	assert.Equal(t, "value", ret.value)

	err = db.AddSet(ctx, "set", "a", "b", "c")
	assert.NoError(t, err)
	s, err := db.GetSet(ctx, "set")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"a", "b", "c"}, s)

	err = db.AddSorted(ctx, Sorted("sorted", []Scored{{Id: "a", Score: 1}, {Id: "b", Score: 2}, {Id: "c", Score: 3}}))
	assert.NoError(t, err)
	z, err := db.GetSorted(ctx, "sorted", 0, -1)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []Scored{{Id: "a", Score: 1}, {Id: "b", Score: 2}, {Id: "c", Score: 3}}, z)

	// purge data
	err = db.Purge()
	assert.NoError(t, err)
	ret = db.Get(ctx, "key")
	assert.ErrorIs(t, ret.err, errors.NotFound)
	s, err = db.GetSet(ctx, "set")
	assert.NoError(t, err)
	assert.Empty(t, s)
	z, err = db.GetSorted(ctx, "sorted", 0, -1)
	assert.NoError(t, err)
	assert.Empty(t, z)

	// purge empty dataset
	err = db.Purge()
	assert.NoError(t, err)
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
