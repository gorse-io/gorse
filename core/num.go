package core

import (
	"github.com/gonum/floats"
	"github.com/gonum/stat"
	"math"
	"math/rand"
)

func concatenate(arrs ...[]int) []int {
	// Sum lengths
	total := 0
	for _, arr := range arrs {
		total += len(arr)
	}
	// concatenate
	ret := make([]int, total)
	pos := 0
	for _, arr := range arrs {
		for _, val := range arr {
			ret[pos] = val
			pos++
		}
	}
	return ret
}

func selectFloat(a []float64, indices []int) []float64 {
	ret := make([]float64, len(indices))
	for i, index := range indices {
		ret[i] = a[index]
	}
	return ret
}

func selectInt(a []int, indices []int) []int {
	ret := make([]int, len(indices))
	for i, index := range indices {
		ret[i] = a[index]
	}
	return ret
}

func unique(a []int) Set {
	set := make(map[int]interface{})
	for _, val := range a {
		set[val] = nil
	}
	return set
}

// Linear algebra

func abs(dst []float64) {
	for i := 0; i < len(dst); i++ {
		dst[i] = math.Abs(dst[i])
	}
}

func mulConst(c float64, dst []float64) {
	for i := 0; i < len(dst); i++ {
		dst[i] *= c
	}
}

func divConst(c float64, dst []float64) {
	for i := 0; i < len(dst); i++ {
		dst[i] /= c
	}
}

func means(a [][]float64) []float64 {
	m := make([]float64, len(a))
	for i := range a {
		sum, count := 0.0, 0.0
		for j := range a[i] {
			if !math.IsNaN(a[i][j]) {
				sum += a[i][j]
				count++
			}
		}
		m[i] = sum / count
	}
	return m
}

// Generator

func newNormalVector(size int, mean, stdDev float64) []float64 {
	ret := make([]float64, size)
	for i := 0; i < len(ret); i++ {
		ret[i] = rand.NormFloat64()*stdDev + mean
	}
	return ret
}

func newUniformVector(size int, low, high float64) []float64 {
	ret := make([]float64, size)
	scale := high - low
	for i := 0; i < len(ret); i++ {
		ret[i] = rand.Float64()*scale + low
	}
	return ret
}

func resetZeroVector(a []float64) {
	for i := range a {
		a[i] = 0
	}
}

func newUniformVectorInt(size, low, high int) []int {
	ret := make([]int, size)
	scale := high - low
	for i := 0; i < len(ret); i++ {
		ret[i] = rand.Intn(scale) + low
	}
	return ret
}

func newUniformMatrix(row, col int, low, high float64) [][]float64 {
	ret := make([][]float64, row)
	for i := range ret {
		ret[i] = newUniformVector(col, low, high)
	}
	return ret
}

func newNanMatrix(row, col int) [][]float64 {
	ret := make([][]float64, row)
	for i := range ret {
		ret[i] = make([]float64, col)
		for j := range ret[i] {
			ret[i][j] = math.NaN()
		}
	}
	return ret
}

func newZeroMatrix(row, col int) [][]float64 {
	ret := make([][]float64, row)
	for i := range ret {
		ret[i] = make([]float64, col)
	}
	return ret
}

func resetZeroMatrix(m [][]float64) {
	for i := range m {
		for j := range m[i] {
			m[i][j] = 0
		}
	}
}

func newSparseMatrix(row int) []map[int]float64 {
	m := make([]map[int]float64, row)
	for i := range m {
		m[i] = make(map[int]float64)
	}
	return m
}

// Evaluator

type Evaluator func([]float64, []float64) float64

func RMSE(predictions []float64, truth []float64) float64 {
	temp := make([]float64, len(predictions))
	floats.SubTo(temp, predictions, truth)
	floats.Mul(temp, temp)
	return math.Sqrt(stat.Mean(temp, nil))
}

func MAE(predictions []float64, truth []float64) float64 {
	temp := make([]float64, len(predictions))
	floats.SubTo(temp, predictions, truth)
	abs(temp)
	return stat.Mean(temp, nil)
}
