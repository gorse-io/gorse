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

	"github.com/google/uuid"
)

var spanKey = uuid.New().String()

type Tracer struct {
	spans sync.Map
}

// Start creates a root span.
func (t *Tracer) Start(ctx context.Context, name string, total int64) (context.Context, *Span) {
	span := &Span{name: name, total: total}
	t.spans.Store(name, span)
	return context.WithValue(ctx, spanKey, span), span
}

func (t *Tracer) List() []Progress {
	var progress []Progress
	t.spans.Range(func(key, value interface{}) bool {
		span := value.(*Span)
		progress = append(progress, Progress{
			Name:  span.name,
			Total: span.total,
			Count: span.count,
		})
		return true
	})
	return progress
}

type Span struct {
	name     string
	total    int64
	count    int64
	err      error
	children sync.Map
}

func (s *Span) Add(n int64) {
	s.count += n
}

func (s *Span) End() {
	s.count = s.total
}

func (s *Span) Error(err error) {
	s.err = err
}

func Start(ctx context.Context, name string, total int64) (context.Context, *Span) {
	childSpan := &Span{name: name, total: total}
	if ctx == nil {
		return nil, childSpan
	}
	span, ok := (ctx).Value(spanKey).(*Span)
	if !ok {
		return nil, childSpan
	}
	span.children.Store(name, childSpan)
	return context.WithValue(ctx, spanKey, childSpan), childSpan
}

type Progress struct {
	Name  string
	Total int64
	Count int64
}
