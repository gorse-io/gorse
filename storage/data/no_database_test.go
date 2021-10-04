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
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNoDatabase(t *testing.T) {
	var database NoDatabase

	err := database.Close()
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Optimize()
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Init()
	assert.ErrorIs(t, err, ErrNoDatabase)

	err = database.BatchInsertItems(nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetItem("")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, _, err = database.GetItems("", 0, nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.DeleteItem("")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, c := database.GetItemStream(0, nil)
	assert.ErrorIs(t, <-c, ErrNoDatabase)

	err = database.BatchInsertUsers(nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetUser("")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, _, err = database.GetUsers("", 0)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.DeleteUser("")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, c = database.GetUserStream(0)
	assert.ErrorIs(t, <-c, ErrNoDatabase)

	err = database.BatchInsertFeedback(nil, false, false, false)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.BatchInsertFeedback(nil, false, false, false)
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetUserFeedback("", false)
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetItemFeedback("")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, _, err = database.GetFeedback("", 0, nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetUserItemFeedback("", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.DeleteUserItemFeedback("", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, c = database.GetFeedbackStream(0, nil)
	assert.ErrorIs(t, <-c, ErrNoDatabase)

	err = database.InsertMeasurement(Measurement{})
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetMeasurements("", 0)
	assert.ErrorIs(t, err, ErrNoDatabase)
}
