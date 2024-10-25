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
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNoDatabase(t *testing.T) {
	ctx := context.Background()
	var database NoDatabase

	err := database.Close()
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Optimize()
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Init()
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Ping()
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Purge()
	assert.ErrorIs(t, err, ErrNoDatabase)

	err = database.BatchInsertItems(ctx, nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.BatchGetItems(ctx, nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.ModifyItem(ctx, "", ItemPatch{})
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetItem(ctx, "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, _, err = database.GetItems(ctx, "", 0, nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.DeleteItem(ctx, "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, c := database.GetItemStream(ctx, 0, nil)
	assert.ErrorIs(t, <-c, ErrNoDatabase)

	err = database.BatchInsertUsers(ctx, nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetUser(ctx, "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.ModifyUser(ctx, "", UserPatch{})
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, _, err = database.GetUsers(ctx, "", 0)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.DeleteUser(ctx, "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, c = database.GetUserStream(ctx, 0)
	assert.ErrorIs(t, <-c, ErrNoDatabase)

	err = database.BatchInsertFeedback(ctx, nil, false, false, false)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.BatchInsertFeedback(ctx, nil, false, false, false)
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetUserFeedback(ctx, "", lo.ToPtr(time.Now()))
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetItemFeedback(ctx, "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, _, err = database.GetFeedback(ctx, "", 0, nil, lo.ToPtr(time.Now()))
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetUserItemFeedback(ctx, "", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.DeleteUserItemFeedback(ctx, "", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, c = database.GetFeedbackStream(ctx, 0)
	assert.ErrorIs(t, <-c, ErrNoDatabase)
}
