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

type Request struct {
	// Request metadata
	RequestID    string // Unique request identifier (X-Request-ID)
	RequestBytes int64  // Number of request body bytes read by the handler
	Method       string // HTTP method (GET, POST, PUT, DELETE, PATCH)
	Route        string // API route template (e.g., /api/recommend/{user-id})

	// Response metadata
	StatusCode    int           // HTTP response status code
	ResponseTime  time.Duration // Response time
	ResponseBytes int64         // Number of response body bytes written by the handler

	// Additional metadata
	Timestamp  time.Time // Event timestamp
	RemoteAddr string    // Client remote address
}

type Snapshot struct {
	UserCount     int64
	UserBytes     int64
	ItemCount     int64
	ItemBytes     int64
	FeedbackCount int64
	FeedbackBytes int64
	Timestamp     time.Time
}

type Handler interface {
	EmitRequest(ctx context.Context, event Request)
	EmitSnapshot(ctx context.Context, event Snapshot)
}

type NopHandler struct{}

func (n *NopHandler) EmitRequest(ctx context.Context, event Request) {}

func (n *NopHandler) EmitSnapshot(ctx context.Context, event Snapshot) {}

var handler Handler = &NopHandler{}

func SetEventHandler(r Handler) {
	handler = r
}

func Emit[T Request | Snapshot](ctx context.Context, event T) {
	switch e := any(event).(type) {
	case Request:
		handler.EmitRequest(ctx, e)
	case Snapshot:
		handler.EmitSnapshot(ctx, e)
	}
}
