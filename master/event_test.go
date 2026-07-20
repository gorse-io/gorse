// Copyright 2026 gorse Project Authors
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

package master

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorse-io/gorse/common/event"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/require"
)

type storageEventRecorder struct {
	storageEvents chan event.StorageEvent
}

func (r *storageEventRecorder) RecordAPI(context.Context, event.APIEvent) {}

func (r *storageEventRecorder) RecordStorage(_ context.Context, e event.StorageEvent) {
	r.storageEvents <- e
}

func TestRecordStorageUsage(t *testing.T) {
	database, err := data.Open("sqlite://"+filepath.Join(t.TempDir(), "data.db"), "")
	require.NoError(t, err)
	require.NoError(t, database.Init())
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	_, ok := database.(data.ExactCounter)
	require.True(t, ok)

	ctx := t.Context()
	require.NoError(t, database.BatchInsertUsers(ctx, []data.User{{UserId: "user"}}))
	require.NoError(t, database.BatchInsertItems(ctx, []data.Item{{ItemId: "item"}}))
	require.NoError(t, database.BatchInsertFeedback(ctx, []data.Feedback{{
		FeedbackKey: data.FeedbackKey{FeedbackType: "like", UserId: "user", ItemId: "item"},
	}}, false, false, false))

	recorder := &storageEventRecorder{storageEvents: make(chan event.StorageEvent, 1)}
	event.SetEventRecorder(recorder)
	t.Cleanup(func() { event.SetEventRecorder(&event.NopRecorder{}) })

	datasetBuiltAt := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	master := &Master{}
	master.DataClient = database
	before := time.Now()
	master.recordStorageUsage(ctx, datasetBuiltAt)

	recorded := <-recorder.storageEvents
	after := time.Now()
	require.Equal(t, 1, recorded.UserCount)
	require.Equal(t, 1, recorded.ItemCount)
	require.Equal(t, 1, recorded.FeedbackCount)
	require.Equal(t, datasetBuiltAt, recorded.DatasetBuiltAt)
	require.False(t, recorded.ObservedAt.Before(before))
	require.False(t, recorded.ObservedAt.After(after))
}
