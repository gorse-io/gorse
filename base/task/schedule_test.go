package task

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
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
