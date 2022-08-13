package task

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskScheduler(t *testing.T) {
	taskScheduler := NewTaskScheduler()
	var wg sync.WaitGroup

	// pre-lock for privileged tasks
	for i := 0; i < 50; i++ {
		taskScheduler.PreLock(fmt.Sprintf("privileged_%d", i))
	}

	// start ragtag tasks
	result := make([]string, 0, 1000)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(name string) {
			taskScheduler.Lock(name)
			result = append(result, name)
			taskScheduler.UnLock(name)
			wg.Done()
		}(fmt.Sprintf("ragtag_%d", i))
	}

	// start privileged tasks
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(locker *Runner) {
			locker.Lock()
			result = append(result, locker.Name)
			locker.UnLock()
			wg.Done()
		}(taskScheduler.NewRunner(fmt.Sprintf("privileged_%d", i)))
	}

	// check result
	wg.Wait()
	for i := 0; i < 100; i++ {
		if i < 50 {
			assert.Contains(t, result[i], "privileged_")
		} else {
			assert.Contains(t, result[i], "ragtag_")
		}
	}
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

func TestDynamicJobsAllocator(t *testing.T) {
	s := NewJobsScheduler(8)
	s.Register("a", 1, true)
	s.Register("b", 2, true)
	s.Register("c", 3, true)
	s.Register("d", 4, false)
	s.Register("e", 4, false)
	c := s.GetJobsAllocator("c")
	assert.Equal(t, 8, c.MaxJobs())
	assert.Equal(t, 3, c.AvailableJobs())
	b := s.GetJobsAllocator("b")
	assert.Equal(t, 3, b.AvailableJobs())
	a := s.GetJobsAllocator("a")
	assert.Equal(t, 2, a.AvailableJobs())

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		d := s.GetJobsAllocator("d")
		assert.Equal(t, 4, d.AvailableJobs())
	}()
	go func() {
		defer wg.Done()
		e := s.GetJobsAllocator("e")
		assert.Equal(t, 4, e.AvailableJobs())
	}()

	s.Unregister("a")
	s.Unregister("b")
	s.Unregister("c")
	wg.Wait()
}

func TestJobsScheduler(t *testing.T) {
	s := NewJobsScheduler(8)
	s.Register("a", 1, true)
	s.Register("b", 2, true)
	s.Register("c", 3, true)
	s.Register("d", 4, false)
	s.Register("e", 4, false)
	assert.Equal(t, 3, s.allocateJobsForTask("c", false))
	assert.Equal(t, 3, s.allocateJobsForTask("b", false))
	assert.Equal(t, 2, s.allocateJobsForTask("a", false))
	assert.Equal(t, 0, s.allocateJobsForTask("d", false))
	assert.Equal(t, 0, s.allocateJobsForTask("e", false))

	// several tasks complete
	s.Unregister("b")
	s.Unregister("c")
	assert.Equal(t, 8, s.allocateJobsForTask("a", false))

	// privileged tasks complete
	s.Unregister("a")
	assert.Equal(t, 4, s.allocateJobsForTask("d", false))
	assert.Equal(t, 4, s.allocateJobsForTask("e", false))

	// block privileged tasks if normal tasks are running
	s.Register("a", 1, true)
	s.Register("b", 2, true)
	s.Register("c", 3, true)
	assert.Equal(t, 0, s.allocateJobsForTask("c", false))
	assert.Equal(t, 0, s.allocateJobsForTask("b", false))
	assert.Equal(t, 0, s.allocateJobsForTask("a", false))
}
