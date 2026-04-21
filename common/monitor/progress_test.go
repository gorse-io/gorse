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

package monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecode(t *testing.T) {
	progressList := []Progress{
		{
			Tracer:     "tracer",
			Name:       "a",
			Total:      100,
			Count:      50,
			Status:     StatusRunning,
			StartTime:  time.Date(2018, time.January, 1, 0, 0, 0, 0, time.Local),
			FinishTime: time.Date(2018, time.January, 2, 0, 0, 0, 0, time.Local),
		},
		{
			Tracer:     "tracer",
			Name:       "b",
			Total:      100,
			Count:      50,
			Status:     StatusRunning,
			StartTime:  time.Date(2018, time.January, 1, 0, 0, 0, 0, time.Local),
			FinishTime: time.Date(2018, time.January, 2, 0, 0, 0, 0, time.Local),
		},
	}
	pb := EncodeProgress(progressList)
	assert.Equal(t, progressList, DecodeProgress(pb))
}

func TestProgress(t *testing.T) {
	t.Run("divide by zero", func(t *testing.T) {
		// Test case: child with zero Total should not cause panic
		span := &Span{
			name:   "parent",
			status: StatusRunning,
			total:  10,
			count:  5,
			start:  time.Now(),
		}
		// Add a child with zero total
		childSpan := &Span{
			name:   "child-zero-total",
			status: StatusRunning,
			total:  0,
			count:  0,
			start:  time.Now(),
		}
		span.children.Store("child-zero-total", childSpan)

		// This should not panic. Running zero-total child contributes 0
		// (#1241), so parentCount = 5 against parentTotal = 10.
		progress := span.Progress()
		assert.Equal(t, "parent", progress.Name)
		assert.Equal(t, StatusRunning, progress.Status)
		assert.Equal(t, 5, progress.Count)
		assert.Equal(t, 10, progress.Total)
	})

	t.Run("child with zero total", func(t *testing.T) {
		// Test case: multiple children, one with zero Total. The zero-total
		// child is still Running, so it contributes 0 - it has no measurable
		// progress and counting it as complete would inflate the parent
		// past its real count (#1241).
		span := &Span{
			name:   "parent",
			status: StatusRunning,
			total:  10,
			count:  5,
			start:  time.Now(),
		}
		// Add a normal child
		child1 := &Span{
			name:   "child1",
			status: StatusRunning,
			total:  100,
			count:  50,
			start:  time.Now(),
		}
		// Add a Running zero-total child. Contributes 0 until it Completes.
		child2 := &Span{
			name:   "child2",
			status: StatusRunning,
			total:  0,
			count:  0,
			start:  time.Now(),
		}
		span.children.Store("child1", child1)
		span.children.Store("child2", child2)

		// This should not panic.
		// childTotal = max(100, 0) = 100. parentCount = 5*100 + 100*50/100 + 0 = 550.
		progress := span.Progress()
		assert.Equal(t, "parent", progress.Name)
		assert.Equal(t, 550, progress.Count)
		assert.Equal(t, 1000, progress.Total)
	})

	t.Run("all children with zero total", func(t *testing.T) {
		// All children Running with Total=0: childTotal falls back to 1,
		// children themselves contribute 0 (they have no progress yet).
		span := &Span{
			name:   "parent",
			status: StatusRunning,
			total:  10,
			count:  5,
			start:  time.Now(),
		}
		child1 := &Span{
			name:   "child1",
			status: StatusRunning,
			total:  0,
			count:  0,
			start:  time.Now(),
		}
		child2 := &Span{
			name:   "child2",
			status: StatusRunning,
			total:  0,
			count:  0,
			start:  time.Now(),
		}
		span.children.Store("child1", child1)
		span.children.Store("child2", child2)

		// childTotal=1, parentCount = 5*1 + 0 + 0 = 5, parentTotal = 10.
		progress := span.Progress()
		assert.Equal(t, "parent", progress.Name)
		assert.Equal(t, 5, progress.Count)
		assert.Equal(t, 10, progress.Total)
	})

	t.Run("zero-total running child does not exceed parent total", func(t *testing.T) {
		// Regression for #1241. Parent has consumed all its own count and
		// a Running zero-total child is registered. Old behaviour pushed
		// parentCount to 3 against parentTotal of 2 (150%); fixed
		// behaviour keeps Count <= Total.
		span := &Span{
			name:   "parent",
			status: StatusRunning,
			total:  2,
			count:  2,
			start:  time.Now(),
		}
		child := &Span{
			name:   "child",
			status: StatusRunning,
			total:  0,
			count:  0,
			start:  time.Now(),
		}
		span.children.Store("child", child)

		got := span.Progress()
		assert.LessOrEqual(t, got.Count, got.Total,
			"Count (%d) must not exceed Total (%d)", got.Count, got.Total)
	})
}
