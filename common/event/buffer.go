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
	"sync"
)

// BufferHandler is an in-memory buffer handler that stores events
// in a buffer. This is useful for testing and debugging.
type BufferHandler struct {
	buffer []APIEvent
	mu     sync.Mutex
}

// NewBufferRecorder creates a new buffer handler.
func NewBufferRecorder() *BufferHandler {
	return &BufferHandler{
		buffer: make([]APIEvent, 0),
	}
}

// Handle adds an event to the buffer.
func (b *BufferHandler) Handle(ctx context.Context, event APIEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buffer = append(b.buffer, event)
	return nil
}

// Close clears the buffer.
func (b *BufferHandler) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buffer = nil
	return nil
}

// GetEvents returns all buffered events.
// This method is for testing and debugging purposes.
func (b *BufferHandler) GetEvents() []APIEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]APIEvent, len(b.buffer))
	copy(result, b.buffer)
	return result
}

// Clear clears the buffer.
func (b *BufferHandler) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buffer = make([]APIEvent, 0)
}
