package base

import (
	"log"
)

// Concatenate merges vectors of integers to one.
func Concatenate(vectors ...[]int) []int {
	// Sum lengths
	total := 0
	for _, arr := range vectors {
		total += len(arr)
	}
	// concatenate
	vec := make([]int, total)
	pos := 0
	for _, arr := range vectors {
		for _, val := range arr {
			vec[pos] = val
			pos++
		}
	}
	return vec
}

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

// Min finds the minimum in a vector of integers. Panic if the slice is empty.
func Min(a []int) int {
	if len(a) == 0 {
		log.Panicf("Can't get the minimum from empty vec")
	}
	minimum := a[0]
	for _, m := range a {
		if m < minimum {
			minimum = m
		}
	}
	return minimum
}

// NewMatrix creates a matrix.
func NewMatrix(row, col int) [][]float64 {
	ret := make([][]float64, row)
	for i := range ret {
		ret[i] = make([]float64, col)
	}
	return ret
}

// FillZeroVector fills a vector with zeros.
func FillZeroVector(vec []float64) {
	for i := range vec {
		vec[i] = 0
	}
}

// FillZeroMatrix fills a matrix with zeros.
func FillZeroMatrix(mat [][]float64) {
	for i := range mat {
		for j := range mat[i] {
			mat[i][j] = 0
		}
	}
}
