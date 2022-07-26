// Copyright 2022 gorse Project Authors
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

package task

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConstantJobsAllocator(t *testing.T) {
	allocator := NewConstantJobsAllocator(314)
	assert.Equal(t, 314, allocator.MaxJobs())
	assert.Equal(t, 314, allocator.AvailableJobs(nil))

	allocator = NewConstantJobsAllocator(-1)
	assert.Equal(t, 1, allocator.MaxJobs())
	assert.Equal(t, 1, allocator.AvailableJobs(nil))

	allocator = nil
	assert.Equal(t, 1, allocator.MaxJobs())
	assert.Equal(t, 1, allocator.AvailableJobs(nil))
}

func TestDynamicJobsAllocator(t *testing.T) {
	s := NewJobsScheduler(8)
	s.Register("a", 1, true)
	s.Register("b", 2, true)
	s.Register("c", 3, true)
	s.Register("d", 4, false)
	s.Register("e", 4, false)
	c := s.GetJobsAllocator("c")
	assert.Equal(t, 8, c.MaxJobs())
	assert.Equal(t, 3, c.AvailableJobs(nil))
	b := s.GetJobsAllocator("b")
	assert.Equal(t, 3, b.AvailableJobs(nil))
	a := s.GetJobsAllocator("a")
	assert.Equal(t, 2, a.AvailableJobs(nil))

	barrier := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		barrier <- struct{}{}
		d := s.GetJobsAllocator("d")
		assert.Equal(t, 4, d.AvailableJobs(nil))
	}()
	go func() {
		defer wg.Done()
		barrier <- struct{}{}
		e := s.GetJobsAllocator("e")
		e.Init()
		assert.Equal(t, 4, s.allocateJobsForTask("e", false, nil))
	}()

	<-barrier
	<-barrier
	s.Unregister("a")
	s.Unregister("b")
	s.Unregister("c")
	wg.Wait()
}

func TestJobsScheduler(t *testing.T) {
	s := NewJobsScheduler(8)
	assert.True(t, s.Register("a", 1, true))
	assert.True(t, s.Register("b", 2, true))
	assert.True(t, s.Register("c", 3, true))
	assert.True(t, s.Register("d", 4, false))
	assert.True(t, s.Register("e", 4, false))
	assert.False(t, s.Register("c", 1, true))
	assert.Equal(t, 3, s.allocateJobsForTask("c", false, nil))
	assert.Equal(t, 3, s.allocateJobsForTask("b", false, nil))
	assert.Equal(t, 2, s.allocateJobsForTask("a", false, nil))
	assert.Equal(t, 0, s.allocateJobsForTask("d", false, nil))
	assert.Equal(t, 0, s.allocateJobsForTask("e", false, nil))

	// several tasks complete
	s.Unregister("b")
	s.Unregister("c")
	assert.Equal(t, 8, s.allocateJobsForTask("a", false, nil))

	// privileged tasks complete
	s.Unregister("a")
	assert.Equal(t, 4, s.allocateJobsForTask("d", false, nil))
	assert.Equal(t, 4, s.allocateJobsForTask("e", false, nil))

	// block privileged tasks if normal tasks are running
	s.Register("a", 1, true)
	s.Register("b", 2, true)
	s.Register("c", 3, true)
	assert.Equal(t, 0, s.allocateJobsForTask("c", false, nil))
	assert.Equal(t, 0, s.allocateJobsForTask("b", false, nil))
	assert.Equal(t, 0, s.allocateJobsForTask("a", false, nil))
}

func TestConstantJobsAllocator(t *testing.T) {
	allocator := NewConstantJobsAllocator(314)
	assert.Equal(t, 314, allocator.MaxJobs())
	assert.Equal(t, 314, allocator.AvailableJobs())

	allocator = NewConstantJobsAllocator(-1)
	assert.Equal(t, 1, allocator.MaxJobs())
	assert.Equal(t, 1, allocator.AvailableJobs())

	allocator = nil
	assert.Equal(t, 1, allocator.MaxJobs())
	assert.Equal(t, 1, allocator.AvailableJobs())
}
