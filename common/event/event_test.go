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

package event

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRecorder struct {
	mock.Mock
}

func (m *MockRecorder) RecordAPI(ctx context.Context, event APIEvent) {
	m.Called(ctx, event)
}

func (m *MockRecorder) RecordStorage(ctx context.Context, event StorageEvent) {
	m.Called(ctx, event)
}

func TestSetEventRecorder(t *testing.T) {
	t.Cleanup(func() {
		SetEventRecorder(&NopRecorder{})
	})

	ctx := context.Background()
	now := time.Now()
	apiEvent := APIEvent{
		RequestID:    "request-id",
		Method:       "GET",
		Path:         "/api/recommend/user-id",
		StatusCode:   200,
		ResponseTime: 10,
		Timestamp:    now,
		RemoteAddr:   "127.0.0.1",
	}
	storageEvent := StorageEvent{
		UserCount:     1,
		ItemCount:     2,
		FeedbackCount: 3,
		Timestamp:     now,
	}

	recorder := new(MockRecorder)
	recorder.On("RecordAPI", ctx, apiEvent).Once()
	recorder.On("RecordStorage", ctx, storageEvent).Once()

	SetEventRecorder(recorder)

	assert.Same(t, recorder, EventRecorder())
	EventRecorder().RecordAPI(ctx, apiEvent)
	EventRecorder().RecordStorage(ctx, storageEvent)

	recorder.AssertExpectations(t)
}
