package base

import (
	"log"
)

// Concatenate arrays of integers.
func Concatenate(arrays ...[]int) []int {
	// Sum lengths
	total := 0
	for _, arr := range arrays {
		total += len(arr)
	}
	// concatenate
	ret := make([]int, total)
	pos := 0
	for _, arr := range arrays {
		for _, val := range arr {
			ret[pos] = val
			pos++
		}
	}
	return ret
}

// Max finds the maximum in a vec. Return 0 if the slice is empty.
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

// Min finds the minimum in a vec. Return 0 if the slice is empty.
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

// Neg gets the negative of a vec.
func Neg(dst []float64) {
	for i := 0; i < len(dst); i++ {
		dst[i] = -dst[i]
	}
}

// Argmin finds the index of the minimum in a vec.
func Argmin(a []float64) int {
	minIndex := 0
	for index, value := range a {
		if value < a[minIndex] {
			minIndex = index
		}
	}
	return minIndex
}

// MakeMatrix makes a matrix.
func MakeMatrix(row, col int) [][]float64 {
	ret := make([][]float64, row)
	for i := range ret {
		ret[i] = make([]float64, col)
	}
	return ret
}

// FillZeroVector fills a vec with zeros.
func FillZeroVector(a []float64) {
	for i := range a {
		a[i] = 0
	}
}

// FillZeroMatrix fills a matrix with zeros.
func FillZeroMatrix(m [][]float64) {
	for i := range m {
		for j := range m[i] {
			m[i][j] = 0
		}
	}
}
