// Copyright 2023 gorse Project Authors
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

package progress

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type spanKeyType string

var spanKeyName = spanKeyType(uuid.New().String())

type Status string

const (
	StatusPending   Status = "Pending"
	StatusComplete  Status = "Complete"
	StatusRunning   Status = "Running"
	StatusSuspended Status = "Suspended"
	StatusFailed    Status = "Failed"
)

type Tracer struct {
	name  string
	spans sync.Map
}

func NewTracer(name string) *Tracer {
	return &Tracer{name: name}
}

// Start creates a root span.
func (t *Tracer) Start(ctx context.Context, name string, total int) (context.Context, *Span) {
	span := &Span{name: name, total: total}
	t.spans.Store(name, span)
	return context.WithValue(ctx, spanKeyName, span), span
}

func (t *Tracer) List() []Progress {
	var progress []Progress
	t.spans.Range(func(key, value interface{}) bool {
		// span := value.(*Span)
		// progress = append(progress, Progress{
		// 	Name:  span.name,
		// 	Total: span.total,
		// 	Count: span.count,
		// })
		return true
	})
	return progress
}

type Span struct {
	name     string
	status   Status
	total    int
	count    int
	err      error
	start    time.Time
	finish   time.Time
	children sync.Map
}

func (s *Span) Add(n int) {
	s.count += n
}

func (s *Span) End() {
	s.count = s.total
}

func (s *Span) Error(err error) {
	s.err = err
}

func (s *Span) Count() int {
	return s.count
}

func Start(ctx context.Context, name string, total int) (context.Context, *Span) {
	childSpan := &Span{
		name:   name,
		status: StatusRunning,
		total:  total,
		count:  0,
		start:  time.Now(),
	}
	if ctx == nil {
		return nil, childSpan
	}
	span, ok := (ctx).Value(spanKeyName).(*Span)
	if !ok {
		return nil, childSpan
	}
	span.children.Store(name, childSpan)
	return context.WithValue(ctx, spanKeyName, childSpan), childSpan
}

type Progress struct {
	Tracer     string
	Name       string
	Status     Status
	Error      string
	Count      int
	Total      int
	StartTime  time.Time
	FinishTime time.Time
}
