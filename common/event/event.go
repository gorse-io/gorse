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
	RequestID    string    // Unique request identifier (X-Request-ID)
	APIKey       string    // API key used for authentication
	Method       string    // HTTP method (GET, POST, PUT, DELETE, PATCH)
	Path         string    // API path (e.g., /api/recommend/{user-id})
	RoutePath    string    // Route path template (e.g., /api/recommend/{user-id})
	
	// Response metadata
	StatusCode   int       // HTTP response status code
	ResponseTime int64     // Response time in milliseconds
	Timestamp    time.Time // Event timestamp
	
	// Request details (for billing)
	UserID       string    // User ID involved in the request (if applicable)
	ItemID       string    // Item ID involved in the request (if applicable)
	
	// Quantities (for billing calculation)
	UserCount    int       // Number of users involved (batch operations)
	ItemCount    int       // Number of items involved (batch operations)
	FeedbackCount int      // Number of feedbacks involved
	RecommendCount int     // Number of recommendations returned
	
	// Additional metadata
	RemoteAddr   string    // Client remote address
	Categories   []string  // Categories involved in the request
}

// Recorder defines the interface for recording API events.
// Implementations can store events to different backends:
// - Database (for persistent storage)
// - Message queue (for async processing)
// - In-memory buffer (for batched writes)
// - External billing service
type Recorder interface {
	// Record records an API event.
	// The implementation should handle errors gracefully and not block
	// the request processing. For async implementations, errors can be
	// logged but not returned.
	Record(ctx context.Context, event APIEvent) error
	
	// Close closes the recorder and releases any resources.
	// For async implementations, this should flush pending events.
	Close() error
}

// NopRecorder is a no-operation recorder that does nothing.
// Used when event recording is disabled.
type NopRecorder struct{}

// Record does nothing and returns nil.
func (n *NopRecorder) Record(ctx context.Context, event APIEvent) error {
	return nil
}

// Close does nothing and returns nil.
func (n *NopRecorder) Close() error {
	return nil
}

// Global recorder instance
var globalRecorder Recorder = &NopRecorder{}

// SetRecorder sets the global recorder instance.
func SetRecorder(recorder Recorder) {
	globalRecorder = recorder
}

// GetRecorder returns the global recorder instance.
func GetRecorder() Recorder {
	return globalRecorder
}

// Record records an API event using the global recorder.
func Record(ctx context.Context, event APIEvent) error {
	return globalRecorder.Record(ctx, event)
}
