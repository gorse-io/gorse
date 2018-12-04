package base

import (
	"gonum.org/v1/gonum/stat"
	"math"
	"math/rand"
	"sync"
)

func Concatenate(arrs ...[]int) []int {
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

/* Linear Algebra */

func max(a []int) int {
	maximum := 0
	for _, m := range a {
		if m > maximum {
			maximum = m
		}
	}
	return maximum
}

func abs(dst []float64) {
	for i := 0; i < len(dst); i++ {
		dst[i] = math.Abs(dst[i])
	}
}

func neg(dst []float64) {
	for i := 0; i < len(dst); i++ {
		dst[i] = -dst[i]
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

/* Vectors */

func newRange(start, end int) []int {
	a := make([]int, end-start)
	for i := range a {
		a[i] = start + i
	}
	return a
}

func resetZeroVector(a []float64) {
	for i := range a {
		a[i] = 0
	}
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

func zeros(row, col int) [][]float64 {
	ret := make([][]float64, row)
	for i := range ret {
		ret[i] = make([]float64, col)
	}
	return ret
}

func zerosInt(row, col int) [][]int {
	ret := make([][]int, row)
	for i := range ret {
		ret[i] = make([]int, col)
	}
	return ret
}

func newVector(size int, v float64) []float64 {
	ret := make([]float64, size)
	for j := range ret {
		ret[j] = v
	}
	return ret
}

func newMatrix(row, col int, v float64) [][]float64 {
	ret := make([][]float64, row)
	for i := range ret {
		ret[i] = newVector(col, v)
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

/* Parallel Computing */

func Parallel(nTask int, nJob int, worker func(begin, end int)) {
	var wg sync.WaitGroup
	wg.Add(nJob)
	for j := 0; j < nJob; j++ {
		go func(jobId int) {
			begin := nTask * jobId / nJob
			end := nTask * (jobId + 1) / nJob
			worker(begin, end)
			wg.Done()
		}(j)
	}
	wg.Wait()
}

func parallelMean(nTask int, nJob int, worker func(begin, end int) float64) float64 {
	var wg sync.WaitGroup
	wg.Add(nJob)
	results := make([]float64, nJob)
	weights := make([]float64, nJob)
	for j := 0; j < nJob; j++ {
		go func(jobId int) {
			begin := nTask * jobId / nJob
			end := nTask * (jobId + 1) / nJob
			results = append(results, worker(begin, end))
			weights = append(weights, float64(end-begin)/float64(nTask))
			wg.Done()
		}(j)
	}
	wg.Wait()
	return stat.Mean(results, weights)
}

func randomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
