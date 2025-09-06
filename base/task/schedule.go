// Copyright 2021 gorse Project Authors
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
	"runtime"
	"sort"
	"sync"

	"github.com/gorse-io/gorse/base/log"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"modernc.org/mathutil"
)

type JobsAllocator struct {
	numJobs   int    // the max number of jobs
	taskName  string // its task name in scheduler
	scheduler *JobsScheduler
}

func NewConstantJobsAllocator(num int) *JobsAllocator {
	return &JobsAllocator{
		numJobs: num,
	}
}

func (allocator *JobsAllocator) MaxJobs() int {
	if allocator == nil || allocator.numJobs < 1 {
		// Return 1 for invalid allocator
		return 1
	}
	return allocator.numJobs
}

func (allocator *JobsAllocator) AvailableJobs() int {
	if allocator == nil || allocator.numJobs < 1 {
		// Return 1 for invalid allocator
		return 1
	} else if allocator.scheduler != nil {
		// Use jobs scheduler
		return allocator.scheduler.allocateJobsForTask(allocator.taskName, true)
	}
	return allocator.numJobs
}

// Init jobs allocation. This method is used to request allocation of jobs for the first time.
func (allocator *JobsAllocator) Init() {
	if allocator.scheduler != nil {
		allocator.scheduler.allocateJobsForTask(allocator.taskName, true)
	}
}

// taskInfo represents a task in JobsScheduler.
type taskInfo struct {
	name       string // name of the task
	priority   int    // high priority tasks are allocated first
	privileged bool   // privileged tasks are allocated first
	jobs       int    // number of jobs allocated to the task
	previous   int    // previous number of jobs allocated to the task
}

// JobsScheduler allocates jobs to multiple tasks.
type JobsScheduler struct {
	*sync.Cond
	numJobs  int // number of jobs
	freeJobs int // number of free jobs
	tasks    map[string]*taskInfo
}

// NewJobsScheduler creates a JobsScheduler with num jobs.
func NewJobsScheduler(num int) *JobsScheduler {
	if num <= 0 {
		// Use all cores if num is less than 1.
		num = runtime.NumCPU()
	}
	return &JobsScheduler{
		Cond:     sync.NewCond(&sync.Mutex{}),
		numJobs:  num,
		freeJobs: num,
		tasks:    make(map[string]*taskInfo),
	}
}

// Register a task in the JobsScheduler. Registered tasks will be ignored and return false.
func (s *JobsScheduler) Register(taskName string, priority int, privileged bool) bool {
	s.L.Lock()
	defer s.L.Unlock()
	if _, exits := s.tasks[taskName]; !exits {
		s.tasks[taskName] = &taskInfo{name: taskName, priority: priority, privileged: privileged}
		return true
	} else {
		return false
	}
}

// Unregister a task from the JobsScheduler.
func (s *JobsScheduler) Unregister(taskName string) {
	s.L.Lock()
	defer s.L.Unlock()
	if task, exits := s.tasks[taskName]; exits {
		// Return allocated jobs.
		s.freeJobs += task.jobs
		delete(s.tasks, taskName)
		s.Broadcast()
	}
}

func (s *JobsScheduler) GetJobsAllocator(taskName string) *JobsAllocator {
	return &JobsAllocator{
		numJobs:   s.numJobs,
		taskName:  taskName,
		scheduler: s,
	}
}

func (s *JobsScheduler) allocateJobsForTask(taskName string, block bool) int {
	// Find current task and return the jobs temporarily.
	s.L.Lock()
	currentTask, exist := s.tasks[taskName]
	if !exist {
		panic("task not found")
	}
	s.freeJobs += currentTask.jobs
	currentTask.jobs = 0
	s.L.Unlock()

	s.L.Lock()
	defer s.L.Unlock()
	for {
		s.allocateJobsForAll()
		if currentTask.jobs == 0 && block {
			if currentTask.previous > 0 {
				log.Logger().Debug("suspend task", zap.String("task", currentTask.name))
				s.Broadcast()
			}
			s.Wait()
		} else {
			if currentTask.previous == 0 {
				log.Logger().Debug("resume task", zap.String("task", currentTask.name))
			}
			return currentTask.jobs
		}
	}
}

func (s *JobsScheduler) allocateJobsForAll() {
	// Separate privileged tasks and normal tasks
	privileged := make([]*taskInfo, 0)
	normal := make([]*taskInfo, 0)
	for _, task := range s.tasks {
		if task.privileged {
			privileged = append(privileged, task)
		} else {
			normal = append(normal, task)
		}
	}

	var tasks []*taskInfo
	if len(privileged) > 0 {
		tasks = privileged
	} else {
		tasks = normal
	}

	// allocate jobs
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].priority > tasks[j].priority
	})
	for i, task := range tasks {
		if s.freeJobs == 0 {
			return
		}
		targetJobs := s.numJobs/len(tasks) + lo.If(i < s.numJobs%len(tasks), 1).Else(0)
		targetJobs = mathutil.Min(targetJobs, s.freeJobs)
		if task.jobs < targetJobs {
			if task.previous != targetJobs {
				log.Logger().Debug("reallocate jobs for task",
					zap.String("task", task.name),
					zap.Int("previous_jobs", task.previous),
					zap.Int("target_jobs", targetJobs))
			}
			s.freeJobs -= targetJobs - task.jobs
			task.jobs = targetJobs
			task.previous = task.jobs
		}
	}
}
