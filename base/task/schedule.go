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
	"github.com/scylladb/go-set/strset"
	"sync"
)

// Scheduler schedules that pre-locked tasks are executed first.
type Scheduler struct {
	*sync.Cond
	Privileged *strset.Set
	Running    bool
}

// NewTaskScheduler creates a Scheduler.
func NewTaskScheduler() *Scheduler {
	return &Scheduler{
		Cond:       sync.NewCond(&sync.Mutex{}),
		Privileged: strset.New(),
	}
}

// PreLock a task, the task has the privilege to run first than un-pre-clocked tasks.
func (t *Scheduler) PreLock(name string) {
	t.L.Lock()
	defer t.L.Unlock()
	t.Privileged.Add(name)
}

// Lock gets the permission to run task.
func (t *Scheduler) Lock(name string) {
	t.L.Lock()
	defer t.L.Unlock()
	for t.Running || (!t.Privileged.IsEmpty() && !t.Privileged.Has(name)) {
		t.Wait()
	}
	t.Running = true
}

// UnLock returns the permission to run task.
func (t *Scheduler) UnLock(name string) {
	t.L.Lock()
	defer t.L.Unlock()
	t.Running = false
	t.Privileged.Remove(name)
	t.Broadcast()
}

func (t *Scheduler) NewRunner(name string) *Runner {
	return &Runner{
		Scheduler: t,
		Name:      name,
	}
}

// Runner is a Scheduler bounded with a task.
type Runner struct {
	*Scheduler
	Name string
}

// Lock gets the permission to run task.
func (locker *Runner) Lock() {
	locker.Scheduler.Lock(locker.Name)
}

// UnLock returns the permission to run task.
func (locker *Runner) UnLock() {
	locker.Scheduler.UnLock(locker.Name)
}
