// Copyright 2020 gorse Project Authors
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
package parallel

import (
	"fmt"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/task"
)

func TestParallel(t *testing.T) {
	a := base.RangeInt(10000)
	b := make([]int, len(a))
	workerIds := make([]int, len(a))
	// multiple threads
	_ = Parallel(len(a), 4, func(workerId, jobId int) error {
		b[jobId] = a[jobId]
		workerIds[jobId] = workerId
		time.Sleep(time.Microsecond)
		return nil
	})
	workersSet := mapset.NewSet(workerIds...)
	assert.Equal(t, a, b)
	assert.GreaterOrEqual(t, 4, workersSet.Cardinality())
	assert.Less(t, 1, workersSet.Cardinality())
	// single thread
	_ = Parallel(len(a), 1, func(workerId, jobId int) error {
		b[jobId] = a[jobId]
		workerIds[jobId] = workerId
		return nil
	})
	workersSet = mapset.NewSet(workerIds...)
	assert.Equal(t, a, b)
	assert.Equal(t, 1, workersSet.Cardinality())
}

func TestDynamicParallel(t *testing.T) {
	a := base.RangeInt(10000)
	b := make([]int, len(a))
	workerIds := make([]int, len(a))
	_ = DynamicParallel(len(a), task.NewConstantJobsAllocator(3), func(workerId, jobId int) error {
		b[jobId] = a[jobId]
		workerIds[jobId] = workerId
		time.Sleep(time.Microsecond)
		return nil
	})
	workersSet := mapset.NewSet(workerIds...)
	assert.Equal(t, a, b)
	assert.GreaterOrEqual(t, 4, workersSet.Cardinality())
	assert.Less(t, 1, workersSet.Cardinality())
}

func TestBatchParallel(t *testing.T) {
	a := base.RangeInt(10000)
	b := make([]int, len(a))
	workerIds := make([]int, len(a))
	// multiple threads
	_ = BatchParallel(len(a), 4, 10, func(workerId, beginJobId, endJobId int) error {
		for jobId := beginJobId; jobId < endJobId; jobId++ {
			b[jobId] = a[jobId]
			workerIds[jobId] = workerId
		}
		time.Sleep(time.Microsecond)
		return nil
	})
	workersSet := mapset.NewSet(workerIds...)
	assert.Equal(t, a, b)
	assert.GreaterOrEqual(t, 4, workersSet.Cardinality())
	assert.Less(t, 1, workersSet.Cardinality())
	// single thread
	_ = Parallel(len(a), 1, func(workerId, jobId int) error {
		b[jobId] = a[jobId]
		workerIds[jobId] = workerId
		return nil
	})
	workersSet = mapset.NewSet(workerIds...)
	assert.Equal(t, a, b)
	assert.Equal(t, 1, workersSet.Cardinality())
}

func TestParallelFail(t *testing.T) {
	// multiple threads
	err := Parallel(10000, 4, func(workerId, jobId int) error {
		if jobId%2 == 1 {
			return fmt.Errorf("error from %d", jobId)
		}
		return nil
	})
	assert.Error(t, err)
	// single thread
	err = Parallel(10000, 1, func(workerId, jobId int) error {
		if jobId%2 == 1 {
			return fmt.Errorf("error from %d", jobId)
		}
		return nil
	})
	assert.Error(t, err)
}

func TestBatchParallelFail(t *testing.T) {
	// multiple threads
	err := BatchParallel(1000000, 2, 1, func(workerId, beginJobId, endJobId int) error {
		if workerId%2 == 1 {
			return fmt.Errorf("error from %d", workerId)
		}
		return nil
	})
	assert.Error(t, err)
	// single thread
	err = BatchParallel(1000000, 2, 1, func(workerId, beginJobId, endJobId int) error {
		if workerId%2 == 1 {
			return fmt.Errorf("error from %d", workerId)
		}
		return nil
	})
	assert.Error(t, err)
}

func TestSplit(t *testing.T) {
	a := []int{1, 2, 3, 4, 5, 6}
	b := Split(a, 3)
	assert.Equal(t, [][]int{{1, 2}, {3, 4}, {5, 6}}, b)

	a = []int{1, 2, 3, 4, 5, 6, 7}
	b = Split(a, 3)
	assert.Equal(t, [][]int{{1, 2, 3}, {4, 5}, {6, 7}}, b)
}

// check panic
func TestParallelPanic(t *testing.T) {
	err := Parallel(10000, 4, func(workerId, jobId int) error {
		panic("panic")
	})
	if err != nil {
		return
	}
	time.Sleep(time.Second * 3)
}
