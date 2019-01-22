package base

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConcatenate(t *testing.T) {
	a := [][]int{
		{1, 2, 3},
		{5, 6, 7},
		{9, 10, 11},
	}
	b := []int{1, 2, 3, 5, 6, 7, 9, 10, 11}
	assert.Equal(t, b, Concatenate(a...))
}

func TestNewMatrix(t *testing.T) {
	a := NewMatrix(3, 4)
	assert.Equal(t, 3, len(a))
	assert.Equal(t, 4, len(a[0]))
	assert.Equal(t, 4, len(a[0]))
	assert.Equal(t, 4, len(a[0]))
}

func TestFillZeroMatrix(t *testing.T) {
	a := [][]float64{{1, 2, 3, 4}, {5, 6, 7, 8}}
	FillZeroMatrix(a)
	assert.Equal(t, [][]float64{{0, 0, 0, 0}, {0, 0, 0, 0}}, a)
}

func TestFillZeroVector(t *testing.T) {
	a := []float64{1, 2, 3, 4}
	FillZeroVector(a)
	assert.Equal(t, []float64{0, 0, 0, 0}, a)
}
