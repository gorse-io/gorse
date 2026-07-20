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
	Route     string // API route template (e.g., /api/recommend/{user-id})
	Path      string // Deprecated: use Route instead; contains the route template for compatibility

	// Payload processing metadata
	RequestBytes  int64 // Number of request body bytes read by the handler
	ResponseBytes int64 // Number of response body bytes written by the handler

	// Response metadata
	StatusCode   int       // HTTP response status code
	ResponseTime int64     // Response time in milliseconds
	Timestamp    time.Time // Event timestamp

	// Additional metadata
	RemoteAddr string // Client remote address
}

// StorageEvent represents data storage usage for billing purposes.
type StorageEvent struct {
	UserCount     int // Number of users in storage
	ItemCount     int // Number of items in storage
	FeedbackCount int // Number of feedbacks in storage

	ObservedAt     time.Time // Time when storage usage was measured
	DatasetBuiltAt time.Time // Time when the current recommendation dataset was built
	Timestamp      time.Time // Deprecated: use ObservedAt instead
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
