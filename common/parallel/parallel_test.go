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
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/common/util"
	"github.com/stretchr/testify/assert"
)

func TestParallel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := util.RangeInt(10000)
		b := make([]int, len(a))
		workerIds := make([]int, len(a))
		// multiple threads
		_ = Parallel(context.Background(), len(a), 4, func(workerId, jobId int) error {
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
		_ = Parallel(context.Background(), len(a), 1, func(workerId, jobId int) error {
			b[jobId] = a[jobId]
			workerIds[jobId] = workerId
			return nil
		})
		workersSet = mapset.NewSet(workerIds...)
		assert.Equal(t, a, b)
		assert.Equal(t, 1, workersSet.Cardinality())
	})
}

func TestFor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// multiple threads
		a := util.RangeInt(10000)
		b := make([]int, len(a))
		err := For(context.Background(), len(a), 4, func(jobId int) {
			b[jobId] = a[jobId]
			time.Sleep(time.Microsecond)
		})
		assert.NoError(t, err)
		assert.Equal(t, a, b)
		// single thread
		err = For(context.Background(), len(a), 1, func(jobId int) {
			b[jobId] = a[jobId]
			time.Sleep(time.Microsecond)
		})
		assert.NoError(t, err)
		assert.Equal(t, a, b)
	})
}

func TestForCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		var count atomic.Int32

		err := For(ctx, 1000, 4, func(jobId int) {
			if jobId == 0 {
				cancel()
			}
			count.Add(1)
			time.Sleep(100 * time.Microsecond)
		})

		assert.ErrorIs(t, err, context.Canceled)
		assert.Less(t, int(count.Load()), 1000)
	})
}

func TestForEach(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := util.RangeInt(10000)
		b := make([]int, len(a))
		// multiple threads
		err := ForEach(context.Background(), a, 4, func(i, v int) {
			assert.Equal(t, i, v)
			b[i] = v
			time.Sleep(time.Microsecond)
		})
		assert.NoError(t, err)
		assert.Equal(t, a, b)
		// single thread
		err = ForEach(context.Background(), a, 1, func(i, v int) {
			assert.Equal(t, i, v)
			b[i] = v
			time.Sleep(time.Microsecond)
		})
		assert.NoError(t, err)
		assert.Equal(t, a, b)
	})
}

func TestForEachCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		var count atomic.Int32

		err := ForEach(ctx, util.RangeInt(1000), 4, func(i, v int) {
			if i == 0 {
				cancel()
			}
			count.Add(1)
			time.Sleep(100 * time.Microsecond)
		})

		assert.ErrorIs(t, err, context.Canceled)
		assert.Less(t, int(count.Load()), 1000)
	})
}

func TestParallelFail(t *testing.T) {
	// multiple threads
	err := Parallel(context.Background(), 10000, 4, func(workerId, jobId int) error {
		if jobId%2 == 1 {
			return fmt.Errorf("error from %d", jobId)
		}
		return nil
	})
	assert.Error(t, err)
	// single thread
	err = Parallel(context.Background(), 10000, 1, func(workerId, jobId int) error {
		if jobId%2 == 1 {
			return fmt.Errorf("error from %d", jobId)
		}
		return nil
	})
	assert.Error(t, err)
}

func TestParallelCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		var count atomic.Int32

		err := Parallel(ctx, 100, 4, func(_, jobId int) error {
			if jobId == 0 {
				cancel()
			}
			count.Add(1)
			time.Sleep(100 * time.Millisecond)
			return nil
		})

		assert.ErrorIs(t, err, context.Canceled)
		assert.Less(t, int(count.Load()), 100)
	})
}

func TestSplit(t *testing.T) {
	a := []int{1, 2, 3, 4, 5, 6}
	b := Split(a, 3)
	assert.Equal(t, [][]int{{1, 2}, {3, 4}, {5, 6}}, b)

	a = []int{1, 2, 3, 4, 5, 6, 7}
	b = Split(a, 3)
	assert.Equal(t, [][]int{{1, 2, 3}, {4, 5}, {6, 7}}, b)
}

func TestDetachable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		err := Detachable(context.Background(), 100, 1, 100, func(ctx *Context, jobId int) {
			ctx.Detach()
			time.Sleep(time.Second)
			ctx.Attach()
		})
		assert.NoError(t, err)
		assert.Less(t, time.Since(start), time.Second*2)
	})

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		err := Detachable(context.Background(), 100, 1, 10, func(ctx *Context, jobId int) {
			ctx.Detach()
			time.Sleep(time.Second)
			ctx.Attach()
		})
		assert.NoError(t, err)
		assert.Less(t, time.Since(start), time.Second*11)
	})
}

func TestDetachableCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		var count atomic.Int32

		err := Detachable(ctx, 100, 4, 10, func(c *Context, jobId int) {
			if jobId == 0 {
				cancel()
			}
			count.Add(1)
			time.Sleep(10 * time.Millisecond)
		})

		assert.ErrorIs(t, err, context.Canceled)
		assert.Less(t, int(count.Load()), 20)
	})
}
