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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func testMeta(t *testing.T, db Database) {
	// Set meta string
	err := db.SetString("meta", "1", "2")
	assert.NoError(t, err)
	// Get meta string
	value, err := db.GetString("meta", "1")
	assert.NoError(t, err)
	assert.Equal(t, "2", value)
	// Delete string
	err = db.Delete("meta", "1")
	assert.NoError(t, err)
	// Get meta not existed
	value, err = db.GetString("meta", "1")
	assert.True(t, errors.IsNotFound(err))
	assert.Equal(t, "", value)
	// Set meta int
	err = db.SetInt("meta", "1", 2)
	assert.NoError(t, err)
	// Get meta int
	valInt, err := db.GetInt("meta", "1")
	assert.NoError(t, err)
	assert.Equal(t, 2, valInt)
	// increase meta int
	err = db.IncrInt("meta", "1")
	assert.NoError(t, err)
	valInt, err = db.GetInt("meta", "1")
	assert.NoError(t, err)
	assert.Equal(t, 3, valInt)
	// set meta time
	err = db.SetTime("meta", "1", time.Date(1996, 4, 8, 0, 0, 0, 0, time.UTC))
	assert.NoError(t, err)
	// get meta time
	valTime, err := db.GetTime("meta", "1")
	assert.NoError(t, err)
	assert.Equal(t, 1996, valTime.Year())
	assert.Equal(t, time.Month(4), valTime.Month())
	assert.Equal(t, 8, valTime.Day())
	// test exists
	exists, err := db.Exists("meta", "1", "10000")
	assert.NoError(t, err)
	assert.Equal(t, []int{1, 0}, exists)
}

func testScores(t *testing.T, db Database) {
	// Put scores
	scores := []Scored{
		{"0", 0},
		{"1", 1.1},
		{"2", 1.2},
		{"3", 1.3},
		{"4", 1.4},
	}
	err := db.SetScores("list", "0", scores)
	assert.NoError(t, err)
	// Get scores
	totalItems, err := db.GetScores("list", "0", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, scores, totalItems)
	// Get n scores
	headItems, err := db.GetScores("list", "0", 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, scores[:3], headItems)
	// Get n scores with offset
	offsetItems, err := db.GetScores("list", "0", 1, 3)
	assert.NoError(t, err)
	assert.Equal(t, scores[1:4], offsetItems)
	// Get empty
	noItems, err := db.GetScores("list", "1", 0, 0)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(noItems))
	// test overwrite
	overwriteScores := []Scored{
		{"10", 10.0},
		{"11", 10.1},
		{"12", 10.2},
		{"13", 10.3},
		{"14", 10.4},
	}
	err = db.SetScores("list", "0", overwriteScores)
	assert.NoError(t, err)
	totalItems, err = db.GetScores("list", "0", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, overwriteScores, totalItems)
	// test append
	err = db.AppendScores("append", "0", scores...)
	assert.NoError(t, err)
	totalItems, err = db.GetScores("append", "0", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, scores, totalItems)
	// append
	err = db.AppendScores("append", "0", overwriteScores...)
	assert.NoError(t, err)
	totalItems, err = db.GetScores("append", "0", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, append(scores, overwriteScores...), totalItems)
	// clear
	err = db.ClearScores("append", "0")
	assert.NoError(t, err)
	totalItems, err = db.GetScores("append", "0", 0, -1)
	assert.NoError(t, err)
	assert.Empty(t, totalItems)

	// Put scores in category
	scores = []Scored{
		{"0", 0},
		{"1", 1.1},
		{"2", 1.2},
		{"3", 1.3},
		{"4", 1.4},
	}
	err = db.SetCategoryScores("list", "0", "cat", scores)
	assert.NoError(t, err)
	// Get scores in category
	totalItems, err = db.GetCategoryScores("list", "0", "cat", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, scores, totalItems)
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
	err := db.SetSorted("sort", scores)
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
	// Increase score
	err = db.IncrSorted("sort", "0")
	assert.NoError(t, err)
	err = db.IncrSorted("sort", "0")
	assert.NoError(t, err)
	totalItems, err = db.GetSorted("sort", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []Scored{
		{"0", 2},
		{"4", 1.4},
		{"3", 1.3},
		{"2", 1.2},
		{"1", 1.1},
	}, totalItems)
	// Remove score
	err = db.RemSorted("sort", "0")
	assert.NoError(t, err)
	totalItems, err = db.GetSorted("sort", 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []Scored{
		{"4", 1.4},
		{"3", 1.3},
		{"2", 1.2},
		{"1", 1.1},
	}, totalItems)
	// Get score
	score, err := db.GetSortedScore("sort", "2")
	assert.NoError(t, err)
	assert.Equal(t, float32(1.2), score)

	// test set empty
	err = db.SetSorted("sort", []Scored{})
	assert.NoError(t, err)
	// test get empty
	scores, err = db.GetSorted("unknown_sort", 0, -1)
	assert.NoError(t, err)
	assert.Empty(t, scores)
	// test get non-existed score
	_, err = db.GetSortedScore("sort", "10086")
	assert.True(t, errors.IsNotFound(err))
}

func TestScored(t *testing.T) {
	itemIds := []string{"2", "4", "6"}
	scores := []float32{2, 4, 6}
	scored := []Scored{{Id: "2", Score: 2}, {Id: "4", Score: 4}, {Id: "6", Score: 6}}
	assert.Equal(t, scored, CreateScoredItems(itemIds, scores))
	assert.Equal(t, itemIds, RemoveScores(scored))
}

func TestKey(t *testing.T) {
	assert.Empty(t, Key())
	assert.Equal(t, "a", Key("a"))
	assert.Equal(t, "a", Key("a", ""))
	assert.Equal(t, "a/b", Key("a", "b"))
}
