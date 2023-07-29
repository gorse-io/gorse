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
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"modernc.org/mathutil"
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
	span := &Span{
		name:   name,
		status: StatusRunning,
		total:  total,
		start:  time.Now(),
	}
	t.spans.Store(name, span)
	return context.WithValue(ctx, spanKeyName, span), span
}

func (t *Tracer) List() []Progress {
	var progress []Progress
	t.spans.Range(func(key, value interface{}) bool {
		span := value.(*Span)
		progress = append(progress, span.Progress())
		return true
	})
	// sort by start time
	sort.Slice(progress, func(i, j int) bool {
		return progress[i].StartTime.Before(progress[j].StartTime)
	})
	return progress
}

type Span struct {
	name     string
	status   Status
	total    int
	count    int
	err      string
	start    time.Time
	finish   time.Time
	children sync.Map
}

func (s *Span) Add(n int) {
	s.count = mathutil.Min(s.count+n, s.total)
}

func (s *Span) End() {
	if s.status == StatusRunning {
		s.status = StatusComplete
		s.count = s.total
		s.finish = time.Now()
	}
}

func (s *Span) Fail(err error) {
	s.status = StatusFailed
	s.err = err.Error()
}

func (s *Span) Count() int {
	return s.count
}

func (s *Span) Progress() Progress {
	// find running children
	var children []Progress
	s.children.Range(func(key, value interface{}) bool {
		child := value.(*Span)
		progress := child.Progress()
		if progress.Status == StatusRunning {
			children = append(children, progress)
		}
		if s.err == "" && progress.Error != "" {
			s.err = progress.Error
			s.status = StatusFailed
		}
		return true
	})
	// leaf node
	if len(children) == 0 {
		return Progress{
			Name:       s.name,
			Status:     s.status,
			Error:      s.err,
			Count:      s.count,
			Total:      s.total,
			StartTime:  s.start,
			FinishTime: s.finish,
		}
	}
	// non-leaf node
	childTotal := children[0].Total
	parentTotal := s.total * childTotal
	parentCount := s.count * childTotal
	for _, child := range children {
		parentCount += childTotal * child.Count / child.Total
	}
	return Progress{
		Name:       s.name,
		Status:     s.status,
		Error:      s.err,
		Count:      parentCount,
		Total:      parentTotal,
		StartTime:  s.start,
		FinishTime: s.finish,
	}
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

func Fail(ctx context.Context, err error) {
	span, ok := (ctx).Value(spanKeyName).(*Span)
	if !ok {
		return
	}
	span.Fail(err)
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
