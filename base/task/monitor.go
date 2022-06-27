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

func (t *Task) Update(done int) {
	t.Done = done
	t.Status = StatusRunning
}

func (t *Task) Add(done int) {
	if t != nil {
		t.Done += done
	}
}

func (t *Task) Finish() {
	t.Status = StatusComplete
	t.Done = t.Total
	t.FinishTime = time.Now()
}

func (t *Task) Suspend(flag bool) {
	if flag {
		t.Status = StatusSuspended
	} else {
		t.Status = StatusRunning
	}
}

func (t *Task) SubTask(done int) *SubTask {
	if t == nil {
		return nil
	}
	return &SubTask{
		Parent: t,
		Start:  t.Done,
		End:    t.Done + done,
	}
}

type SubTask struct {
	Start  int
	End    int
	Parent *Task
}

func (t *SubTask) Add(done int) {
	if t != nil {
		t.Parent.Add(done)
	}
}

func (t *SubTask) Finish() {
	if t != nil {
		t.Parent.Update(t.End)
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

func (tm *Monitor) GetTask(name string) *Task {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	return tm.Tasks[name]
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
func (tm *Monitor) Start(name string, total int) *Task {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	t := NewTask(name, total)
	tm.Tasks[name] = t
	return t
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

// Add the progress of a task.
func (tm *Monitor) Add(name string, done int) {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	task, exist := tm.Tasks[name]
	if exist {
		task.Update(task.Done + done)
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
