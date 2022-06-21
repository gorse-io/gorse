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
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestTaskMonitor(t *testing.T) {
	taskMonitor := NewTaskMonitor()
	taskMonitor.Start("a", 100)
	assert.Equal(t, 0, taskMonitor.Get("a"))
	assert.Equal(t, "a", taskMonitor.Tasks["a"].Name)
	assert.Equal(t, 100, taskMonitor.Tasks["a"].Total)
	assert.Equal(t, 0, taskMonitor.Tasks["a"].Done)
	assert.Equal(t, StatusRunning, taskMonitor.Tasks["a"].Status)

	taskMonitor.Update("a", 50)
	assert.Equal(t, 50, taskMonitor.Get("a"))
	assert.Equal(t, "a", taskMonitor.Tasks["a"].Name)
	assert.Equal(t, 100, taskMonitor.Tasks["a"].Total)
	assert.Equal(t, 50, taskMonitor.Tasks["a"].Done)
	assert.Equal(t, StatusRunning, taskMonitor.Tasks["a"].Status)
	taskMonitor.Suspend("a", true)
	assert.Equal(t, StatusSuspended, taskMonitor.Tasks["a"].Status)
	taskMonitor.Suspend("a", false)
	assert.Equal(t, StatusRunning, taskMonitor.Tasks["a"].Status)

	taskMonitor.Finish("a")
	assert.Equal(t, 100, taskMonitor.Get("a"))
	assert.Equal(t, "a", taskMonitor.Tasks["a"].Name)
	assert.Equal(t, 100, taskMonitor.Tasks["a"].Total)
	assert.Equal(t, 100, taskMonitor.Tasks["a"].Done)
	assert.Equal(t, StatusComplete, taskMonitor.Tasks["a"].Status)

	tracker := taskMonitor.NewTaskTracker("b")
	tracker.Start(10)
	assert.Equal(t, "b", taskMonitor.Tasks["b"].Name)
	assert.Equal(t, 10, taskMonitor.Tasks["b"].Total)
	assert.Equal(t, 0, taskMonitor.Tasks["b"].Done)
	assert.Equal(t, StatusRunning, taskMonitor.Tasks["b"].Status)

	tracker.Update(5)
	assert.Equal(t, "b", taskMonitor.Tasks["b"].Name)
	assert.Equal(t, 10, taskMonitor.Tasks["b"].Total)
	assert.Equal(t, 5, taskMonitor.Tasks["b"].Done)
	assert.Equal(t, StatusRunning, taskMonitor.Tasks["b"].Status)
	tracker.Suspend(true)
	assert.Equal(t, StatusSuspended, taskMonitor.Tasks["b"].Status)
	tracker.Suspend(false)
	assert.Equal(t, StatusRunning, taskMonitor.Tasks["b"].Status)

	tracker.Finish()
	assert.Equal(t, "b", taskMonitor.Tasks["b"].Name)
	assert.Equal(t, 10, taskMonitor.Tasks["b"].Total)
	assert.Equal(t, 10, taskMonitor.Tasks["b"].Done)
	assert.Equal(t, StatusComplete, taskMonitor.Tasks["b"].Status)

	tracker = taskMonitor.NewTaskTracker("c")
	tracker.Start(100)
	tracker.Update(50)
	subTracker := tracker.SubTracker()
	subTracker.Start(10)
	assert.Equal(t, "c", taskMonitor.Tasks["c"].Name)
	assert.Equal(t, 100, taskMonitor.Tasks["c"].Total)
	assert.Equal(t, 50, taskMonitor.Tasks["c"].Done)
	assert.Equal(t, StatusRunning, taskMonitor.Tasks["c"].Status)

	subTracker.Update(5)
	assert.Equal(t, "c", taskMonitor.Tasks["c"].Name)
	assert.Equal(t, 100, taskMonitor.Tasks["c"].Total)
	assert.Equal(t, 55, taskMonitor.Tasks["c"].Done)
	assert.Equal(t, StatusRunning, taskMonitor.Tasks["c"].Status)

	subTracker.Finish()
	subTracker = subTracker.SubTracker()
	subTracker.Start(10)
	assert.Equal(t, "c", taskMonitor.Tasks["c"].Name)
	assert.Equal(t, 100, taskMonitor.Tasks["c"].Total)
	assert.Equal(t, 60, taskMonitor.Tasks["c"].Done)
	assert.Equal(t, StatusRunning, taskMonitor.Tasks["c"].Status)

	subTracker.Update(5)
	assert.Equal(t, "c", taskMonitor.Tasks["c"].Name)
	assert.Equal(t, 100, taskMonitor.Tasks["c"].Total)
	assert.Equal(t, 65, taskMonitor.Tasks["c"].Done)
	assert.Equal(t, StatusRunning, taskMonitor.Tasks["c"].Status)
	subTracker.Suspend(true)
	assert.Equal(t, StatusSuspended, taskMonitor.Tasks["c"].Status)
	subTracker.Suspend(false)
	assert.Equal(t, StatusRunning, taskMonitor.Tasks["c"].Status)

	taskMonitor.Pending("d")
	tasks := taskMonitor.List()
	assert.Equal(t, 4, len(tasks))
}

func TestEncodeDecode(t *testing.T) {
	task := &Task{
		Name:       "a",
		Total:      100,
		Done:       50,
		Status:     StatusRunning,
		StartTime:  time.Date(2018, time.January, 1, 0, 0, 0, 0, time.Local),
		FinishTime: time.Date(2018, time.January, 2, 0, 0, 0, 0, time.Local),
	}
	pb := task.ToPB()
	assert.Equal(t, task, NewTaskFromPB(pb))
}
