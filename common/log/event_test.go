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

package log

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestEvent(t *testing.T) {
	t.Cleanup(func() { SetEventHandler(nil) })
	name := make(chan string, 1)
	values := make(chan []zap.Field, 1)
	SetEventHandler(func(event string, fields ...zap.Field) {
		name <- event
		values <- fields
	})
	now := time.Now()
	fields := []zap.Field{
		zap.String("custom", "value"),
		zap.Int64("bytes", 123),
		zap.Duration("duration", time.Millisecond),
		zap.Time("timestamp", now),
	}
	Event("custom_event", fields...)
	fields[0] = zap.String("custom", "changed")
	select {
	case event := <-name:
		require.Equal(t, "custom_event", event)
	case <-time.After(time.Second):
		t.Fatal("event was not emitted")
	}
	require.Equal(t, []zap.Field{
		zap.String("custom", "value"),
		zap.Int64("bytes", 123),
		zap.Duration("duration", time.Millisecond),
		zap.Time("timestamp", now),
	}, <-values)
	SetEventHandler(nil)
	require.NotPanics(t, func() { Event("ignored") })
}

func TestEventConcurrentHandlerReplacement(t *testing.T) {
	t.Cleanup(func() { SetEventHandler(nil) })
	const workers, count = 4, 100
	var delivered, producers sync.WaitGroup
	delivered.Add(workers * count)
	for range workers {
		producers.Go(func() {
			for range count {
				SetEventHandler(func(string, ...zap.Field) { delivered.Done() })
				Event("concurrent")
			}
		})
	}
	producers.Wait()
	done := make(chan struct{})
	go func() {
		delivered.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("events were not emitted")
	}
}

func TestEventBindsHandlerBeforeDispatch(t *testing.T) {
	t.Cleanup(func() { SetEventHandler(nil) })
	const count = 100
	var delivered sync.WaitGroup
	delivered.Add(count)
	wrongHandler := make(chan struct{}, count)
	for range count {
		SetEventHandler(func(string, ...zap.Field) { delivered.Done() })
		Event("queued")
		SetEventHandler(func(string, ...zap.Field) {
			wrongHandler <- struct{}{}
			delivered.Done()
		})
	}
	done := make(chan struct{})
	go func() {
		delivered.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("events were not emitted")
	}
	require.Empty(t, wrongHandler)
}
