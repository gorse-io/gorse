package core

import (
	"math"
	"sync"
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

/* Linear Algebra */

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

/* Vectors */

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

/* Parallel Computing */

func parallel(nTask int, nJob int, worker func(begin, end int)) {
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
