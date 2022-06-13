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

package master

import (
	"github.com/scylladb/go-set/strset"
	"github.com/zhenghaoz/gorse/model"
	"sort"
	"strings"
	"sync"
	"time"
)

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "Pending"
	TaskStatusComplete  TaskStatus = "Complete"
	TaskStatusRunning   TaskStatus = "Running"
	TaskStatusSuspended TaskStatus = "Suspended"
	TaskStatusFailed    TaskStatus = "Failed"
)

// Task progress information.
type Task struct {
	Name       string
	Status     TaskStatus
	Done       int
	Total      int
	StartTime  time.Time
	FinishTime time.Time
	Error      string
}

// TaskMonitor monitors the progress of all tasks.
type TaskMonitor struct {
	TaskLock sync.Mutex
	Tasks    map[string]*Task
}

// NewTaskMonitor creates a TaskMonitor and add pending tasks.
func NewTaskMonitor() *TaskMonitor {
	return &TaskMonitor{
		Tasks: make(map[string]*Task),
	}
}

// Pending a task.
func (tm *TaskMonitor) Pending(name string) {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	tm.Tasks[name] = &Task{
		Name:   name,
		Status: TaskStatusPending,
	}
}

// Start a task.
func (tm *TaskMonitor) Start(name string, total int) {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	task, exist := tm.Tasks[name]
	if !exist {
		task = &Task{}
		tm.Tasks[name] = task
	}
	task.Name = name
	task.Status = TaskStatusRunning
	task.Done = 0
	task.Total = total
	task.StartTime = time.Now()
	task.FinishTime = time.Time{}
}

// Finish a task.
func (tm *TaskMonitor) Finish(name string) {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	task, exist := tm.Tasks[name]
	if exist {
		task.Status = TaskStatusComplete
		task.Done = task.Total
		task.FinishTime = time.Now()
	}
}

// Update the progress of a task.
func (tm *TaskMonitor) Update(name string, done int) {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	task, exist := tm.Tasks[name]
	if exist {
		task.Done = done
		task.Status = TaskStatusRunning
	}
}

// Suspend a task.
func (tm *TaskMonitor) Suspend(name string, flag bool) {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	task, exist := tm.Tasks[name]
	if exist {
		if flag {
			task.Status = TaskStatusSuspended
		} else {
			task.Status = TaskStatusRunning
		}
	}
}

func (tm *TaskMonitor) Fail(name, err string) {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	task, exist := tm.Tasks[name]
	if exist {
		task.Error = err
		task.Status = TaskStatusFailed
	}
}

// List all tasks and remove tasks from disconnected workers.
func (tm *TaskMonitor) List(workers ...string) []Task {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	workerSet := strset.New(workers...)
	var task []Task
	for name, t := range tm.Tasks {
		// remove tasks from disconnected workers
		if strings.Contains(name, "[") && strings.Contains(name, "]") {
			begin := strings.Index(name, "[") + 1
			end := strings.Index(name, "]")
			if !workerSet.Has(name[begin:end]) {
				delete(tm.Tasks, name)
				continue
			}
		}
		task = append(task, *t)
	}
	sort.Sort(Tasks(task))
	return task
}

// Get the progress of a task.
func (tm *TaskMonitor) Get(name string) int {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	task, exist := tm.Tasks[name]
	if exist {
		return task.Done
	}
	return 0
}

// Tasks is used to sort []Task.
type Tasks []Task

// Len is used to sort []Task.
func (t Tasks) Len() int {
	return len(t)
}

// Swap is used to sort []Task.
func (t Tasks) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

// Less is used to sort []Task.
func (t Tasks) Less(i, j int) bool {
	if t[i].Status != TaskStatusPending && t[j].Status == TaskStatusPending {
		return true
	} else if t[i].Status == TaskStatusPending && t[j].Status != TaskStatusPending {
		return false
	} else if t[i].Status == TaskStatusPending && t[j].Status == TaskStatusPending {
		return t[i].Name < t[j].Name
	} else {
		return t[i].StartTime.Before(t[j].StartTime)
	}
}

// TaskTracker tracks the progress of a task.
type TaskTracker struct {
	name    string
	monitor *TaskMonitor
}

func (tt *TaskTracker) Fail(err string) {
	tt.monitor.Fail(tt.name, err)
}

// NewTaskTracker creates a TaskTracker from TaskMonitor.
func (tm *TaskMonitor) NewTaskTracker(name string) *TaskTracker {
	return &TaskTracker{
		name:    name,
		monitor: tm,
	}
}

// Start the task.
func (tt *TaskTracker) Start(total int) {
	tt.monitor.Start(tt.name, total)
}

// Update the progress of this task.
func (tt *TaskTracker) Update(done int) {
	tt.monitor.Update(tt.name, done)
}

func (tt *TaskTracker) Suspend(flag bool) {
	tt.monitor.Suspend(tt.name, flag)
}

// Finish the task.
func (tt *TaskTracker) Finish() {
	tt.monitor.Finish(tt.name)
}

// SubTracker creates a sub tracker
func (tt *TaskTracker) SubTracker() model.Tracker {
	return &SubTaskTracker{
		TaskTracker: tt,
	}
}

// SubTaskTracker tracks part of progress of a task.
type SubTaskTracker struct {
	*TaskTracker
	Offset int
	Total  int
}

// Fail reports the error message.
func (tt *SubTaskTracker) Fail(err string) {
	tt.monitor.Fail(tt.name, err)
}

// Start a task.
func (tt *SubTaskTracker) Start(total int) {
	tt.Offset = tt.monitor.Get(tt.name)
	tt.Total = total
}

// Update the progress of current task.
func (tt *SubTaskTracker) Update(done int) {
	tt.monitor.Update(tt.name, tt.Offset+done)
}

func (tt *SubTaskTracker) Suspend(flag bool) {
	tt.monitor.Suspend(tt.name, flag)
}

// Finish a task.
func (tt *SubTaskTracker) Finish() {
	tt.monitor.Update(tt.name, tt.Offset+tt.Total)
}

// SubTracker creates a sub tracker of a sub tracker.
func (tt *SubTaskTracker) SubTracker() model.Tracker {
	return &SubTaskTracker{
		TaskTracker: tt.TaskTracker,
	}
}

// TaskScheduler schedules that pre-locked tasks are executed first.
type TaskScheduler struct {
	*sync.Cond
	Privileged *strset.Set
	Running    bool
}

// NewTaskScheduler creates a TaskScheduler.
func NewTaskScheduler() *TaskScheduler {
	return &TaskScheduler{
		Cond:       sync.NewCond(&sync.Mutex{}),
		Privileged: strset.New(),
	}
}

// PreLock a task, the task has the privilege to run first than un-pre-clocked tasks.
func (t *TaskScheduler) PreLock(name string) {
	t.L.Lock()
	defer t.L.Unlock()
	t.Privileged.Add(name)
}

// Lock gets the permission to run task.
func (t *TaskScheduler) Lock(name string) {
	t.L.Lock()
	defer t.L.Unlock()
	for t.Running || (!t.Privileged.IsEmpty() && !t.Privileged.Has(name)) {
		t.Wait()
	}
	t.Running = true
}

// UnLock returns the permission to run task.
func (t *TaskScheduler) UnLock(name string) {
	t.L.Lock()
	defer t.L.Unlock()
	t.Running = false
	t.Privileged.Remove(name)
	t.Broadcast()
}

func (t *TaskScheduler) NewRunner(name string) *TaskRunner {
	return &TaskRunner{
		TaskScheduler: t,
		Name:          name,
	}
}

// TaskRunner is a TaskScheduler bounded with a task.
type TaskRunner struct {
	*TaskScheduler
	Name string
}

// Lock gets the permission to run task.
func (locker *TaskRunner) Lock() {
	locker.TaskScheduler.Lock(locker.Name)
}

// UnLock returns the permission to run task.
func (locker *TaskRunner) UnLock() {
	locker.TaskScheduler.UnLock(locker.Name)
}
