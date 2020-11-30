package base

import (
	"log"
)

// Max finds the maximum in a vector of integers. Panic if the slice is empty.
func Max(a []int) int {
	if len(a) == 0 {
		log.Panicf("Can't get the maximum from empty vec")
	}
	maximum := a[0]
	for _, m := range a {
		if m > maximum {
			maximum = m
		}
	}
	return maximum
}

// NewMatrix creates a matrix.
func NewMatrix(row, col int) [][]float64 {
	ret := make([][]float64, row)
	for i := range ret {
		ret[i] = make([]float64, col)
	}
	return ret
}
