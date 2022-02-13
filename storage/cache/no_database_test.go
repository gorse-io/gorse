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
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNoDatabase(t *testing.T) {
	var database NoDatabase
	err := database.Close()
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.SetString("", "", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetString("", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.SetInt("", "", 0)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.IncrInt("", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetInt("", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.SetTime("", "", time.Time{})
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetTime("", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Delete("", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.Exists("")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.SetScores("", "", nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetScores("", "", 0, 0)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.AppendScores("", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.ClearScores("", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.SetCategoryScores("", "", "", nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetCategoryScores("", "", "", 0, 0)
	assert.ErrorIs(t, err, ErrNoDatabase)

	_, err = database.GetSet("")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.SetSet("")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.AddSet("")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.RemSet("", "")
	assert.ErrorIs(t, err, ErrNoDatabase)

	_, err = database.GetSortedScore("", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetSorted("", 0, 0)
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetSortedByScore("", 0, 0)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.SetSorted("", nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.AddSorted("", nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.IncrSorted("", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.RemSorted("", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
}
