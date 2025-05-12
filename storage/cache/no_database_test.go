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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNoDatabase(t *testing.T) {
	ctx := context.Background()
	var database NoDatabase
	err := database.Close()
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Ping()
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Init()
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Scan(nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Purge()
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Set(ctx)
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.Get(ctx, Key("", "")).String()
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.Get(ctx, Key("", "")).Integer()
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.Get(ctx, Key("", "")).Time()
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Delete(ctx, Key("", ""))
	assert.ErrorIs(t, err, ErrNoDatabase)

	_, err = database.GetSet(ctx, "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.SetSet(ctx, "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.AddSet(ctx, "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.RemSet(ctx, "", "")
	assert.ErrorIs(t, err, ErrNoDatabase)

	err = database.Push(ctx, "", "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.Pop(ctx, "")
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.Remain(ctx, "")
	assert.ErrorIs(t, err, ErrNoDatabase)

	err = database.AddScores(ctx, "", "", nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.SearchScores(ctx, "", "", nil, 0, 0)
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.UpdateScores(ctx, nil, nil, "", ScorePatch{})
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.DeleteScores(ctx, nil, ScoreCondition{})
	assert.ErrorIs(t, err, ErrNoDatabase)

	err = database.AddTimeSeriesPoints(ctx, nil)
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.GetTimeSeriesPoints(ctx, "", time.Time{}, time.Time{}, 0)
	assert.ErrorIs(t, err, ErrNoDatabase)
}
