package base

import (
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParallel(t *testing.T) {
	a := []int{1, 2, 3, 4, 5, 6}
	_ = Parallel(len(a), 4, func(i int) error {
		a[i] *= 2
		return nil
	})
	assert.Equal(t, []int{2, 4, 6, 8, 10, 12}, a)
}

func TestParallelHandleError(t *testing.T) {
	errs := []error{nil, errors.Errorf("1"), nil, errors.Errorf("2"), nil}
	err := Parallel(len(errs), 4, func(i int) error {
		return errs[i]
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
