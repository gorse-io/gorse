package base

import (
	"gonum.org/v1/gonum/stat"
	"sync"
)

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
