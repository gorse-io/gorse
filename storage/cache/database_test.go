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
	"github.com/juju/errors"
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
	"time"
)

func testMeta(t *testing.T, db Database) {
	// Set meta string
	err := db.Set(String(Key("meta", "1"), "2"), String(Key("meta", "1000"), "10"))
	assert.NoError(t, err)
	// Get meta string
	value, err := db.Get(Key("meta", "1")).String()
	assert.NoError(t, err)
	assert.Equal(t, "2", value)
	value, err = db.Get(Key("meta", "1000")).String()
	assert.NoError(t, err)
	assert.Equal(t, "10", value)
	// Delete string
	err = db.Delete(Key("meta", "1"))
	assert.NoError(t, err)
	// Get meta not existed
	value, err = db.Get(Key("meta", "1")).String()
	assert.True(t, errors.IsNotFound(err), err)
	assert.Equal(t, "", value)
	// Set meta int
	err = db.Set(Integer(Key("meta", "1"), 2))
	assert.NoError(t, err)
	// Get meta int
	valInt, err := db.Get(Key("meta", "1")).Integer()
	assert.NoError(t, err)
	assert.Equal(t, 2, valInt)
	// set meta time
	err = db.Set(Time(Key("meta", "1"), time.Date(1996, 4, 8, 0, 0, 0, 0, time.UTC)))
	assert.NoError(t, err)
	// get meta time
	valTime, err := db.Get(Key("meta", "1")).Time()
	assert.NoError(t, err)
	assert.Equal(t, 1996, valTime.Year())
	assert.Equal(t, time.Month(4), valTime.Month())
	assert.Equal(t, 8, valTime.Day())

	// test set empty
	err = db.Set()
	assert.NoError(t, err)
	// test set duplicate
	err = db.Set(String("100", "1"), String("100", "2"))
	assert.NoError(t, err)
}

func testSet(t *testing.T, db Database) {
	err := db.SetSet("set", "1")
	assert.NoError(t, err)
	// test add
	err = db.AddSet("set", "2")
	assert.NoError(t, err)
	var members []string
	members, err = db.GetSet("set")
	assert.NoError(t, err)
	assert.Equal(t, []string{"1", "2"}, members)
	// test rem
	err = db.RemSet("set", "1")
	assert.NoError(t, err)
	members, err = db.GetSet("set")
	assert.NoError(t, err)
	assert.Equal(t, []string{"2"}, members)
	// test set
	err = db.SetSet("set", "3")
	assert.NoError(t, err)
	members, err = db.GetSet("set")
	assert.NoError(t, err)
	assert.Equal(t, []string{"3"}, members)

	// test add empty
	err = db.AddSet("set")
	assert.NoError(t, err)
	// test set empty
	err = db.SetSet("set")
	assert.NoError(t, err)
	// test get empty
	members, err = db.GetSet("unknown_set")
	assert.NoError(t, err)
	assert.Empty(t, members)
	// test rem empty
	err = db.RemSet("set")
	assert.NoError(t, err)
}

func testSort(t *testing.T, db Database) {
	// Put scores
	scores := []Scored{
		{"0", 0},
		{"1", 1.1},
		{"2", 1.2},
		{"3", 1.3},
		{"4", 1.4},
	}
	err := db.SetSorted("sort", scores[:3])
	assert.NoError(t, err)
	err = db.AddSorted(SortedSet{"sort", scores[3:]})
	assert.NoError(t, err)
	// Get scores
	totalItems, err := db.GetSorted("sort", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []Scored{
		{"4", 1.4},
		{"3", 1.3},
		{"2", 1.2},
		{"1", 1.1},
		{"0", 0},
	}, totalItems)
	halfItems, err := db.GetSorted("sort", 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, []Scored{
		{"4", 1.4},
		{"3", 1.3},
		{"2", 1.2},
	}, halfItems)
	halfItems, err = db.GetSorted("sort", 1, 2)
	assert.NoError(t, err)
	assert.Equal(t, []Scored{
		{"3", 1.3},
		{"2", 1.2},
	}, halfItems)
	// get scores by score
	partItems, err := db.GetSortedByScore("sort", 1.1, 1.3)
	assert.NoError(t, err)
	assert.Equal(t, []Scored{
		{"1", 1.1},
		{"2", 1.2},
		{"3", 1.3},
	}, partItems)
	// remove scores by score
	err = db.AddSorted(SortedSet{"sort", []Scored{
		{"5", -5},
		{"6", -6},
	}})
	assert.NoError(t, err)
	err = db.RemSortedByScore("sort", math.Inf(-1), -1)
	assert.NoError(t, err)
	partItems, err = db.GetSortedByScore("sort", math.Inf(-1), -1)
	assert.NoError(t, err)
	assert.Empty(t, partItems)
	// Remove score
	err = db.RemSorted(Member("sort", "0"))
	assert.NoError(t, err)
	totalItems, err = db.GetSorted("sort", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []Scored{
		{"4", 1.4},
		{"3", 1.3},
		{"2", 1.2},
		{"1", 1.1},
	}, totalItems)

	// test set empty
	err = db.SetSorted("sort", []Scored{})
	assert.NoError(t, err)
	// test set duplicate
	err = db.SetSorted("sort1000", []Scored{
		{"100", 1},
		{"100", 2},
	})
	assert.NoError(t, err)
	// test add empty
	err = db.AddSorted()
	assert.NoError(t, err)
	err = db.AddSorted(SortedSet{})
	assert.NoError(t, err)
	// test add duplicate
	err = db.AddSorted(SortedSet{"sort1000", []Scored{{"100", 1}, {"100", 2}}})
	assert.NoError(t, err)
	// test get empty
	scores, err = db.GetSorted("sort", 0, -1)
	assert.NoError(t, err)
	assert.Empty(t, scores)
	scores, err = db.GetSorted("sort", 10, 5)
	assert.NoError(t, err)
	assert.Empty(t, scores)
}

func testScan(t *testing.T, db Database) {
	err := db.Set(String("1", "1"))
	assert.NoError(t, err)
	err = db.SetSet("2", "21", "22", "23")
	assert.NoError(t, err)
	err = db.SetSorted("3", []Scored{{"1", 1}, {"2", 2}, {"3", 3}})
	assert.NoError(t, err)

	var keys []string
	err = db.Scan(func(s string) error {
		keys = append(keys, s)
		return nil
	})
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"1", "2", "3"}, keys)
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
