package base

import (
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"sync"
)

/* Parallel Schedulers */

// Parallel schedules and runs tasks in parallel. nTask is the number of tasks. nJob is
// the number of executors. worker is the executed function which passed a range of task
// Names (begin, end).
func Parallel(nJobs int, nWorkers int, worker func(workerId, jobId int) error) error {
	if nWorkers == 1 {
		for i := 0; i < nJobs; i++ {
			if err := worker(0, i); err != nil {
				return err
			}
		}
	} else {
		const chanSize = 64
		const chanEOF = -1
		c := make(chan int, chanSize)
		// producer
		go func() {
			// send jobs
			for i := 0; i < nJobs; i++ {
				c <- i
			}
			// send EOF
			for i := 0; i < nWorkers; i++ {
				c <- chanEOF
			}
		}()
		// consumer
		var wg sync.WaitGroup
		wg.Add(nWorkers)
		errs := make([]error, nJobs)
		for j := 0; j < nWorkers; j++ {
			// start workers
			go func(workerId int) {
				defer wg.Done()
				for {
					// read job
					jobId := <-c
					if jobId == chanEOF {
						return
					}
					// run job
					if err := worker(workerId, jobId); err != nil {
						errs[jobId] = err
						return
					}
				}
			}(j)
		}
		wg.Wait()
		// check errors
		for _, err := range errs {
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ParallelFor runs for loop in parallel.
func ParallelFor(begin, end int, worker func(i int)) {
	var wg sync.WaitGroup
	wg.Add(end - begin)
	for j := begin; j < end; j++ {
		go func(i int) {
			worker(i)
			wg.Done()
		}(j)
	}
	wg.Wait()
}

// ParallelForSum runs for loop in parallel.
func ParallelForSum(begin, end int, worker func(i int) float64) float64 {
	retValues := make([]float64, end-begin)
	var wg sync.WaitGroup
	wg.Add(end - begin)
	for j := begin; j < end; j++ {
		go func(i int) {
			retValues[i] = worker(i)
			wg.Done()
		}(j)
	}
	wg.Wait()
	return floats.Sum(retValues)
}

// ParallelMean schedules and runs tasks in parallel, then returns the mean of returned values.
// nJob is the number of executors. worker is the executed function which passed a range of task
// Names (begin, end) and returns a double value.
func ParallelMean(nTask int, nJob int, worker func(begin, end int) (sum float64)) float64 {
	var wg sync.WaitGroup
	wg.Add(nJob)
	results := make([]float64, nJob)
	weights := make([]float64, nJob)
	for j := 0; j < nJob; j++ {
		go func(jobId int) {
			begin := nTask * jobId / nJob
			end := nTask * (jobId + 1) / nJob
			size := end - begin
			results[jobId] = worker(begin, end) / float64(size)
			weights[jobId] = float64(size) / float64(nTask)
			wg.Done()
		}(j)
	}
	wg.Wait()
	return stat.Mean(results, weights)
}
