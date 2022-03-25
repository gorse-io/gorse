// Copyright 2020 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parallel

import (
	"github.com/juju/errors"
	"modernc.org/mathutil"
	"sync"
)

/* Parallel Schedulers */

// Parallel schedules and runs tasks in parallel. nTask is the number of tasks. nJob is
// the number of executors. worker is the executed function which passed a range of task
// Names (begin, end).
func Parallel(nJobs, nWorkers int, worker func(workerId, jobId int) error) error {
	if nWorkers == 1 {
		for i := 0; i < nJobs; i++ {
			if err := worker(0, i); err != nil {
				return errors.Trace(err)
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
				return errors.Trace(err)
			}
		}
	}
	return nil
}

type batchJob struct {
	beginId int
	endId   int
}

// BatchParallel run parallel jobs in batches to reduce the cost of context switch.
func BatchParallel(nJobs, nWorkers, batchSize int, worker func(workerId, beginJobId, endJobId int) error) error {
	if nWorkers == 1 {
		return worker(0, 0, nJobs)
	}
	const chanSize = 64
	const chanEOF = -1
	c := make(chan batchJob, chanSize)
	// producer
	go func() {
		// send jobs
		for i := 0; i < nJobs; i += batchSize {
			c <- batchJob{beginId: i, endId: mathutil.Min(i+batchSize, nJobs)}
		}
		// send EOF
		for i := 0; i < nWorkers; i++ {
			c <- batchJob{beginId: chanEOF, endId: chanEOF}
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
				job := <-c
				if job.beginId == chanEOF {
					return
				}
				// run job
				if err := worker(workerId, job.beginId, job.endId); err != nil {
					errs[job.beginId] = err
					return
				}
			}
		}(j)
	}
	wg.Wait()
	// check errors
	for _, err := range errs {
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}
