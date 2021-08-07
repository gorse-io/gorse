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
	"github.com/zhenghaoz/gorse/model"
	"sort"
	"sync"
	"time"
)

const (
	TaskStatusPending  = "Pending"
	TaskStatusComplete = "Complete"
	TaskStatusRunning  = "Running"

	TaskFindLatest         = "Find latest items"
	TaskFindPopular        = "Find popular items"
	TaskFindNeighbor       = "Find neighbors of items"
	TaskAnalyze            = "Analyze click-through rate"
	TaskFitRankingModel    = "Fit ranking model"
	TaskFitClickModel      = "Fit click model"
	TaskSearchRankingModel = "Search ranking model"
	TaskSearchClickModel   = "Search click model"
)

// Task progress information.
type Task struct {
	Name       string
	Status     string
	Done       int
	Total      int
	StartTime  time.Time
	FinishTime time.Time
}

// TaskMonitor monitors the progress of all tasks.
type TaskMonitor struct {
	TaskLock sync.Mutex
	Tasks    map[string]*Task
}

// NewTaskMonitor creates a TaskMonitor and add pending tasks.
func NewTaskMonitor() *TaskMonitor {
	task := make(map[string]*Task)
	for _, taskName := range []string{
		TaskFindLatest,
		TaskFindPopular,
		TaskFindNeighbor,
		TaskAnalyze,
		TaskFitRankingModel,
		TaskFitClickModel,
		TaskSearchRankingModel,
		TaskSearchClickModel,
	} {
		task[taskName] = &Task{
			Name:   taskName,
			Status: TaskStatusPending,
		}
	}
	return &TaskMonitor{Tasks: task}
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
	}
}

// List all tasks.
func (tm *TaskMonitor) List() []Task {
	tm.TaskLock.Lock()
	defer tm.TaskLock.Unlock()
	var task []Task
	for _, t := range tm.Tasks {
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
	Name    string
	Monitor *TaskMonitor
}

// NewTaskTracker creates a TaskTracker from TaskMonitor.
func (tm *TaskMonitor) NewTaskTracker(name string) *TaskTracker {
	return &TaskTracker{
		Name:    name,
		Monitor: tm,
	}
}

// Start the task.
func (tt *TaskTracker) Start(total int) {
	tt.Monitor.Start(tt.Name, total)
}

// Update the progress of this task.
func (tt *TaskTracker) Update(done int) {
	tt.Monitor.Update(tt.Name, done)
}

// Finish the task.
func (tt *TaskTracker) Finish() {
	tt.Monitor.Finish(tt.Name)
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

// Start a task.
func (tt *SubTaskTracker) Start(total int) {
	tt.Offset = tt.Monitor.Get(tt.Name)
	tt.Total = total
}

// Update the progress of current task.
func (tt *SubTaskTracker) Update(done int) {
	tt.Monitor.Update(tt.Name, tt.Offset+done)
}

// Finish a task.
func (tt *SubTaskTracker) Finish() {
	tt.Monitor.Update(tt.Name, tt.Offset+tt.Total)
}

// SubTracker creates a sub tracker of a sub tracker.
func (tt *SubTaskTracker) SubTracker() model.Tracker {
	return &SubTaskTracker{
		TaskTracker: tt.TaskTracker,
	}
}
