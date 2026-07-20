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
	"testing"
	"time"

	"github.com/gorse-io/gorse/common/event"
	"github.com/stretchr/testify/require"
)

type storageEventRecorder struct {
	storageEvents chan event.Snapshot
}

func (r *storageEventRecorder) EmitRequest(context.Context, event.Request) {}

func (r *storageEventRecorder) EmitSnapshot(_ context.Context, e event.Snapshot) {
	r.storageEvents <- e
}

func TestRecordStorageUsage(t *testing.T) {
	ctx := t.Context()
	recorder := &storageEventRecorder{storageEvents: make(chan event.Snapshot, 1)}
	event.SetEventHandler(recorder)
	t.Cleanup(func() { event.SetEventHandler(&event.NopHandler{}) })

	master := &Master{}
	before := time.Now()
	master.recordStorageUsage(ctx, 1, 2, 3)

	recorded := <-recorder.storageEvents
	after := time.Now()
	require.Equal(t, int64(1), recorded.UserCount)
	require.Equal(t, int64(2), recorded.ItemCount)
	require.Equal(t, int64(3), recorded.FeedbackCount)
	require.False(t, recorded.Timestamp.Before(before))
	require.False(t, recorded.Timestamp.After(after))
}
