package base

import (
	"math"
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

func Max(a []int) int {
	maximum := 0
	for _, m := range a {
		if m > maximum {
			maximum = m
		}
	}
	return maximum
}

// Neg
func Neg(dst []float64) {
	for i := 0; i < len(dst); i++ {
		dst[i] = -dst[i]
	}
}

// MulConst multiples a vector with a scalar.
func MulConst(c float64, dst []float64) {
	for i := 0; i < len(dst); i++ {
		dst[i] *= c
	}
}

// MulConst divides a vector by a scalar.
func DivConst(c float64, dst []float64) {
	for i := 0; i < len(dst); i++ {
		dst[i] /= c
	}
}

// ArgMin finds the index of the minimum in a vector.
func ArgMin(a []float64) int {
	minIndex := 0
	for index, value := range a {
		if value < a[minIndex] {
			minIndex = index
		}
	}
	return minIndex
}

// TODO: Remove this function latter.
func NewNanMatrix(row, col int) [][]float64 {
	ret := make([][]float64, row)
	for i := range ret {
		ret[i] = make([]float64, col)
		for j := range ret[i] {
			ret[i][j] = math.NaN()
		}
	}
	return ret
}

// MakeMatrix makes a matrix.
func MakeMatrix(row, col int) [][]float64 {
	ret := make([][]float64, row)
	for i := range ret {
		ret[i] = make([]float64, col)
	}
	return ret
}

// ResetVector resets a vector to all zeros.
func ResetVector(a []float64) {
	for i := range a {
		a[i] = 0
	}
}

// ResetMatrix resets a matrix to all zeros.
func ResetMatrix(m [][]float64) {
	for i := range m {
		for j := range m[i] {
			m[i][j] = 0
		}
	}
}
