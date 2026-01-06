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
	"context"
	"sync"

	"github.com/gorse-io/gorse/common/util"
	"github.com/juju/errors"
	"github.com/samber/lo"
)

const chanSize = 1024

/* Parallel Schedulers */

// Parallel schedules and runs tasks in parallel. nTask is the number of tasks. nJob is
// the number of executors. worker is the executed function which passed a range of task
// Names (begin, end). The ctx argument allows callers to cancel outstanding work.
func Parallel(ctx context.Context, nJobs, nWorkers int, worker func(workerId, jobId int) error) error {
	if nWorkers <= 1 {
		for i := 0; i < nJobs; i++ {
			if err := ctx.Err(); err != nil {
				return errors.Trace(err)
			}
			if err := worker(0, i); err != nil {
				return errors.Trace(err)
			}
		}
	} else {
		c := make(chan int, chanSize)
		// producer
		go func() {
			defer close(c)
			for i := 0; i < nJobs; i++ {
				select {
				case <-ctx.Done():
					return
				case c <- i:
				}
			}
		}()
		// consumer
		var wg sync.WaitGroup
		errs := make([]error, nJobs)
		for j := 0; j < nWorkers; j++ {
			// start workers
			workerId := j
			wg.Go(func() {
				defer util.CheckPanic()
				for {
					select {
					case <-ctx.Done():
						return
					case jobId, ok := <-c:
						if !ok {
							return
						}
						if err := ctx.Err(); err != nil {
							errs[jobId] = err
							return
						}
						// run job
						if err := worker(workerId, jobId); err != nil {
							errs[jobId] = err
							return
						}
					}
				}
			})
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

func For(nJobs, nWorkers int, worker func(int)) {
	if nWorkers <= 1 {
		for i := 0; i < nJobs; i++ {
			worker(i)
		}
	} else {
		c := make(chan int, chanSize)
		// producer
		go func() {
			for i := 0; i < nJobs; i++ {
				c <- i
			}
			close(c)
		}()
		// consumer
		var wg sync.WaitGroup
		for j := 0; j < nWorkers; j++ {
			// start workers
			wg.Go(func() {
				for jobId := range c {
					worker(jobId)
				}
			})
		}
		wg.Wait()
	}
}

func ForEach[T any](a []T, nWorkers int, worker func(int, T)) {
	if nWorkers <= 1 {
		for i, v := range a {
			worker(i, v)
		}
	} else {
		c := make(chan lo.Tuple2[int, T], chanSize)
		// producer
		go func() {
			for i, v := range a {
				c <- lo.Tuple2[int, T]{A: i, B: v}
			}
			close(c)
		}()
		// consumer
		var wg sync.WaitGroup
		for j := 0; j < nWorkers; j++ {
			// start workers
			wg.Go(func() {
				for job := range c {
					worker(job.A, job.B)
				}
			})
		}
		wg.Wait()
	}
}

// Split a slice into n slices and keep the order of elements.
func Split[T any](a []T, n int) [][]T {
	if len(a) == 0 {
		return nil
	}
	if n > len(a) {
		n = len(a)
	}
	minChunkSize := len(a) / n
	maxChunkNum := len(a) % n
	chunks := make([][]T, n)
	for i, j := 0, 0; i < n; i++ {
		chunkSize := minChunkSize
		if i < maxChunkNum {
			chunkSize++
		}
		chunks[i] = a[j : j+chunkSize]
		j += chunkSize
	}
	return chunks
}

type Context struct {
	sem         chan struct{}
	detachedSem chan struct{}
	detached    bool
}

func (ctx *Context) Detach() {
	if ctx == nil || ctx.detached {
		return
	}
	ctx.detachedSem <- struct{}{}
	ctx.detached = true
	<-ctx.sem
}

func (ctx *Context) Attach() {
	if ctx == nil || !ctx.detached {
		return
	}
	ctx.detached = false
	<-ctx.detachedSem
	ctx.sem <- struct{}{}
}

func Detachable(ctx context.Context, nJobs, nWorkers, nMaxDetached int, worker func(*Context, int)) error {
	sem := make(chan struct{}, nWorkers)
	detachedSem := make(chan struct{}, nMaxDetached)
	var wg sync.WaitGroup
	for i := 0; i < nJobs; i++ {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case sem <- struct{}{}:
		}

		wg.Go(func() {
			if ctx.Err() != nil {
				<-sem
				return
			}
			c := &Context{sem: sem, detachedSem: detachedSem}
			worker(c, i)
			if c.detached {
				<-c.detachedSem
			} else {
				<-sem
			}
		})
	}
	wg.Wait()
	return ctx.Err()
}
