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
