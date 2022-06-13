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
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/protocol"
	"sort"
	"strings"
	"sync"
	"time"
)

type Status string

const (
	StatusPending   Status = "Pending"
	StatusComplete  Status = "Complete"
	StatusRunning   Status = "Running"
	StatusSuspended Status = "Suspended"
	StatusFailed    Status = "Failed"
)

// Task progress information.
type Task struct {
	Name       string
	Status     Status
	Done       int
	Total      int
	StartTime  time.Time
	FinishTime time.Time
	Error      string
}

func NewTask(name string, total int) *Task {
	return &Task{
		Name:       name,
		Status:     StatusRunning,
		Done:       0,
		Total:      total,
		StartTime:  time.Now(),
		FinishTime: time.Time{},
	}
}

func NewTaskFromPB(in *protocol.PushTaskInfoRequest) *Task {
	return &Task{
		Name:       in.GetName(),
		Status:     Status(in.GetStatus()),
		Done:       int(in.GetDone()),
		Total:      int(in.GetTotal()),
		StartTime:  time.UnixMilli(in.GetStartTime()),
		FinishTime: time.UnixMilli(in.GetFinishTime()),
	}
}

func (t *Task) Update(done int) {
	t.Done = done
	t.Status = StatusRunning
}

func (t *Task) Finish() {
	t.Status = StatusComplete
	t.Done = t.Total
	t.FinishTime = time.Now()
}

func (t *Task) ToPB() *protocol.PushTaskInfoRequest {
	return &protocol.PushTaskInfoRequest{
		Name:       t.Name,
		Status:     string(t.Status),
		Done:       int64(t.Done),
		Total:      int64(t.Total),
		StartTime:  t.StartTime.UnixMilli(),
		FinishTime: t.FinishTime.UnixMilli(),
	}
}

// Monitor monitors the progress of all tasks.
type Monitor struct {
	TaskLock sync.Mutex
	Tasks    map[string]*Task
}

// NewTaskMonitor creates a Monitor and add pending tasks.
func NewTaskMonitor() *Monitor {
	return &Monitor{
		Tasks: make(map[string]*Task),
	}
}

// Pending a task.
func (tm *Monitor) Pending(name string) {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	tm.Tasks[name] = &Task{
		Name:   name,
		Status: StatusPending,
	}
}

// Start a task.
func (tm *Monitor) Start(name string, total int) {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	tm.Tasks[name] = NewTask(name, total)
}

// Finish a task.
func (tm *Monitor) Finish(name string) {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	task, exist := tm.Tasks[name]
	if exist {
		task.Finish()
	}
}

// Update the progress of a task.
func (tm *Monitor) Update(name string, done int) {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	task, exist := tm.Tasks[name]
	if exist {
		task.Update(done)
	}
}

// Suspend a task.
func (tm *Monitor) Suspend(name string, flag bool) {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	task, exist := tm.Tasks[name]
	if exist {
		if flag {
			task.Status = StatusSuspended
		} else {
			task.Status = StatusRunning
		}
	}
}

func (tm *Monitor) Fail(name, err string) {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	task, exist := tm.Tasks[name]
	if exist {
		task.Error = err
		task.Status = StatusFailed
	}
}

// List all tasks and remove tasks from disconnected workers.
func (tm *Monitor) List(workers ...string) []Task {
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
func (tm *Monitor) Get(name string) int {
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
	if t[i].Status != StatusPending && t[j].Status == StatusPending {
		return true
	} else if t[i].Status == StatusPending && t[j].Status != StatusPending {
		return false
	} else if t[i].Status == StatusPending && t[j].Status == StatusPending {
		return t[i].Name < t[j].Name
	} else {
		return t[i].StartTime.Before(t[j].StartTime)
	}
}

// Tracker tracks the progress of a task.
type Tracker struct {
	name    string
	monitor *Monitor
}

func (tt *Tracker) Fail(err string) {
	tt.monitor.Fail(tt.name, err)
}

// NewTaskTracker creates a Tracker from Monitor.
func (tm *Monitor) NewTaskTracker(name string) *Tracker {
	return &Tracker{
		name:    name,
		monitor: tm,
	}
}

// Start the task.
func (tt *Tracker) Start(total int) {
	tt.monitor.Start(tt.name, total)
}

// Update the progress of this task.
func (tt *Tracker) Update(done int) {
	tt.monitor.Update(tt.name, done)
}

func (tt *Tracker) Suspend(flag bool) {
	tt.monitor.Suspend(tt.name, flag)
}

// Finish the task.
func (tt *Tracker) Finish() {
	tt.monitor.Finish(tt.name)
}

// SubTracker creates a sub tracker
func (tt *Tracker) SubTracker() model.Tracker {
	return &SubTaskTracker{
		Tracker: tt,
	}
}

// SubTaskTracker tracks part of progress of a task.
type SubTaskTracker struct {
	*Tracker
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
		Tracker: tt.Tracker,
	}
}
