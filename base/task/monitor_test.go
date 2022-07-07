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
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
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

	taskMonitor.Add("a", 30)
	assert.Equal(t, 80, taskMonitor.Get("a"))
	assert.Equal(t, "a", taskMonitor.Tasks["a"].Name)
	assert.Equal(t, 100, taskMonitor.Tasks["a"].Total)
	assert.Equal(t, 80, taskMonitor.Tasks["a"].Done)
	assert.Equal(t, StatusRunning, taskMonitor.Tasks["a"].Status)

	taskMonitor.Finish("a")
	assert.Equal(t, 100, taskMonitor.Get("a"))
	assert.Equal(t, "a", taskMonitor.Tasks["a"].Name)
	assert.Equal(t, 100, taskMonitor.Tasks["a"].Total)
	assert.Equal(t, 100, taskMonitor.Tasks["a"].Done)
	assert.Equal(t, StatusComplete, taskMonitor.Tasks["a"].Status)

	taskMonitor.Start("b", 100)
	taskMonitor.Fail("b", "error")
	assert.Equal(t, "error", taskMonitor.Tasks["b"].Error)
	assert.Equal(t, StatusFailed, taskMonitor.Tasks["b"].Status)

	taskMonitor.Pending("c")
	assert.Equal(t, StatusPending, taskMonitor.Tasks["c"].Status)

	taskMonitor.Start("d [a]", 100)
	taskMonitor.Start("d [b]", 100)
	tasks := taskMonitor.List("a")
	assert.ElementsMatch(t, lo.ToSlicePtr(tasks), []*Task{
		taskMonitor.GetTask("a"),
		taskMonitor.GetTask("b"),
		taskMonitor.GetTask("c"),
		taskMonitor.GetTask("d [a]"),
	})
}

func TestSubTask(t *testing.T) {
	task := NewTask("a", 100)
	task.Add(10)
	assert.Equal(t, 10, task.Done)
	s := task.SubTask(80)
	assert.Equal(t, 10, s.Start)
	assert.Equal(t, 90, s.End)
	s.Add(20)
	assert.Equal(t, 30, task.Done)
	s.Finish()
	assert.Equal(t, 90, task.Done)
}
