package base

import (
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestParallel(t *testing.T) {
	a := make([]int, 10000)
	for i := range a {
		a[i] = i
	}
	b := make([]int, len(a))
	workerIds := make([]int, len(a))
	// multiple threads
	_ = Parallel(len(a), 4, func(workerId, jobId int) error {
		b[jobId] = a[jobId]
		workerIds[jobId] = workerId
		time.Sleep(time.Microsecond)
		return nil
	})
	workersSet := NewSet(workerIds)
	assert.Equal(t, a, b)
	assert.Equal(t, 4, workersSet.Len())
	// single thread
	_ = Parallel(len(a), 1, func(workerId, jobId int) error {
		b[jobId] = a[jobId]
		workerIds[jobId] = workerId
		return nil
	})
	workersSet = NewSet(workerIds)
	assert.Equal(t, a, b)
	assert.Equal(t, 1, workersSet.Len())
}

func TestParallelHandleError(t *testing.T) {
	errs := []error{nil, errors.Errorf("1"), nil, errors.Errorf("2"), nil}
	err := Parallel(len(errs), 4, func(workerId, jobId int) error {
		return errs[jobId]
	})
	assert.Equal(t, "1", err.Error())
}

func TestParallelMean(t *testing.T) {
	a := []float64{1, 2, 3, 4, 5, 6}
	mean := ParallelMean(len(a), 4, func(begin, end int) (sum float64) {
		for i := begin; i < end; i++ {
			sum += a[i]
		}
		return sum
	})
	assert.Equal(t, 3.5, mean)
}

func TestParallelFor(t *testing.T) {
	a := make([]int, 5)
	ParallelFor(0, 5, func(i int) {
		a[i] = i
	})
	assert.Equal(t, []int{0, 1, 2, 3, 4}, a)
}
