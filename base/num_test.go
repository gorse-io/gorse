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

func TestNeg(t *testing.T) {
	a := []float64{0.0, 1.0, 2.0}
	b := []float64{0.0, -1.0, -2.0}
	Neg(a)
	assert.Equal(t, b, a)
}

func TestDivConst(t *testing.T) {
	a := []float64{0.0, 1.0, 2.0}
	b := []float64{0.0, 0.5, 1.0}
	DivConst(2.0, a)
	assert.Equal(t, b, a)
}

func TestArgMin(t *testing.T) {
	a := []float64{1, 3, 0, 1, 7, 8, 7, 0, 3, 0, 0}
	assert.Equal(t, 2, Argmin(a))
}
