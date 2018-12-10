package base

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParallel(t *testing.T) {
	a := []int{1, 2, 3, 4, 5, 6}
	Parallel(len(a), 4, func(begin, end int) {
		for i := begin; i < end; i++ {
			a[i] *= 2
		}
	})
	assert.Equal(t, []int{2, 4, 6, 8, 10, 12}, a)
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
