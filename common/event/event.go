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
	"time"
)

// APIEvent represents an API call event for billing purposes.
type APIEvent struct {
	// Request metadata
	RequestID string // Unique request identifier (X-Request-ID)
	Method    string // HTTP method (GET, POST, PUT, DELETE, PATCH)
	Path      string // API path (e.g., /api/recommend/{user-id})

	// Response metadata
	StatusCode   int       // HTTP response status code
	ResponseTime int64     // Response time in milliseconds
	Timestamp    time.Time // Event timestamp

	// Additional metadata
	RemoteAddr string // Client remote address
}

// StorageEvent represents data storage usage for billing purposes.
type StorageEvent struct {
	UserCount     int       // Number of users in storage
	ItemCount     int       // Number of items in storage
	FeedbackCount int       // Number of feedbacks in storage
	Timestamp     time.Time // Event timestamp
}

type Recorder interface {
	RecordAPI(ctx context.Context, event APIEvent)
	RecordStorage(ctx context.Context, event StorageEvent)
}

type NopRecorder struct{}

func (n *NopRecorder) RecordAPI(ctx context.Context, event APIEvent) {
}

func (n *NopRecorder) RecordStorage(ctx context.Context, event StorageEvent) {
}

var recorder Recorder = &NopRecorder{}

func EventRecorder() Recorder {
	return recorder
}

func SetEventRecorder(r Recorder) {
	recorder = r
}
