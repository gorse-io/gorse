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

	"github.com/spf13/pflag"
)

// APIEvent represents an API call event for billing purposes.
type APIEvent struct {
	// Request metadata
	RequestID  string    // Unique request identifier (X-Request-ID)
	APIKey     string    // API key used for authentication
	Method     string    // HTTP method (GET, POST, PUT, DELETE, PATCH)
	Path       string    // API path (e.g., /api/recommend/{user-id})
	RoutePath  string    // Route path template (e.g., /api/recommend/{user-id})

	// Response metadata
	StatusCode   int       // HTTP response status code
	ResponseTime int64     // Response time in milliseconds
	Timestamp    time.Time // Event timestamp

	// Request details (for billing)
	UserID  string // User ID involved in the request (if applicable)
	ItemID  string // Item ID involved in the request (if applicable)

	// Quantities (for billing calculation)
	UserCount      int // Number of users involved (batch operations)
	ItemCount      int // Number of items involved (batch operations)
	FeedbackCount  int // Number of feedbacks involved
	RecommendCount int // Number of recommendations returned

	// Additional metadata
	RemoteAddr string   // Client remote address
	Categories []string // Categories involved in the request
}

// Handler defines the interface for handling API events.
// Implementations can process events differently:
// - Database (for persistent storage)
// - Message queue (for async processing)
// - In-memory buffer (for batched writes)
// - External billing service
type Handler interface {
	// Handle handles an API event.
	// The implementation should handle errors gracefully and not block
	// the request processing. For async implementations, errors can be
	// logged but not returned.
	Handle(ctx context.Context, event APIEvent) error

	// Close closes the handler and releases any resources.
	// For async implementations, this should flush pending events.
	Close() error
}

// NopHandler is a no-operation handler that does nothing.
// Used when event recording is disabled.
type NopHandler struct{}

// Handle does nothing and returns nil.
func (n *NopHandler) Handle(ctx context.Context, event APIEvent) error {
	return nil
}

// Close does nothing and returns nil.
func (n *NopHandler) Close() error {
	return nil
}

// Global handler instance
var globalHandler Handler = &NopHandler{}

func init() {
	// Default to no-op handler
}

// AddFlags adds flags for event handler configuration.
func AddFlags(flagSet *pflag.FlagSet) {
	flagSet.Bool("enable-event-recording", false, "enable API event recording for billing")
	flagSet.String("event-recorder-type", "buffer", "event recorder type (buffer, database, kafka)")
}

// SetHandler sets the global handler from flag configuration.
func SetHandler(flagSet *pflag.FlagSet) {
	if !flagSet.Changed("enable-event-recording") {
		return
	}
	enabled, _ := flagSet.GetBool("enable-event-recording")
	if !enabled {
		return
	}

	recorderType, _ := flagSet.GetString("event-recorder-type")
	switch recorderType {
	case "buffer":
		globalHandler = NewBufferRecorder()
	default:
		globalHandler = NewBufferRecorder()
	}
}

// SetGlobalHandler sets the global handler instance directly.
func SetGlobalHandler(handler Handler) {
	globalHandler = handler
}

// GetGlobalHandler returns the global handler instance.
func GetGlobalHandler() Handler {
	return globalHandler
}

// Handle handles an API event using the global handler.
func Handle(ctx context.Context, event APIEvent) error {
	return globalHandler.Handle(ctx, event)
}

// CloseGlobalHandler closes the global handler.
func CloseGlobalHandler() error {
	return globalHandler.Close()
}
