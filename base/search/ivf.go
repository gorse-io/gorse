// Copyright 2022 gorse Project Authors
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

package search

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/chewxy/math32"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/heap"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/parallel"
	"github.com/zhenghaoz/gorse/base/task"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"modernc.org/mathutil"
)

const (
	DefaultTestSize = 1000
	DefaultMaxIter  = 100
)

var _ VectorIndex = &IVF{}

type IVF struct {
	clusters []ivfCluster
	data     []Vector

	k         int
	errorRate float32
	maxIter   int
	numProbe  int
	jobsAlloc *task.JobsAllocator
}

type IVFConfig func(ivf *IVF)

func SetNumProbe(numProbe int) IVFConfig {
	return func(ivf *IVF) {
		ivf.numProbe = numProbe
	}
}

func SetClusterErrorRate(errorRate float32) IVFConfig {
	return func(ivf *IVF) {
		ivf.errorRate = errorRate
	}
}

func SetIVFJobsAllocator(jobsAlloc *task.JobsAllocator) IVFConfig {
	return func(ivf *IVF) {
		ivf.jobsAlloc = jobsAlloc
	}
}

func SetMaxIteration(maxIter int) IVFConfig {
	return func(ivf *IVF) {
		ivf.maxIter = maxIter
	}
}

type ivfCluster struct {
	centroid     CentroidVector
	observations []int32
	mu           sync.Mutex
}

func NewIVF(vectors []Vector, configs ...IVFConfig) *IVF {
	idx := &IVF{
		data:      vectors,
		k:         int(math32.Sqrt(float32(len(vectors)))),
		errorRate: 0.05,
		maxIter:   DefaultMaxIter,
		numProbe:  1,
	}
	for _, config := range configs {
		config(idx)
	}
	return idx
}

func (idx *IVF) Search(q Vector, n int, prune0 bool) (values []int32, scores []float32) {
	cq := heap.NewTopKFilter[int, float32](idx.numProbe)
	for c := range idx.clusters {
		d := idx.clusters[c].centroid.Distance(q)
		cq.Push(c, -d)
	}

	pq := heap.NewPriorityQueue(true)
	clusters, _ := cq.PopAll()
	for _, c := range clusters {
		for _, i := range idx.clusters[c].observations {
			if idx.data[i] != q {
				pq.Push(i, q.Distance(idx.data[i]))
				if pq.Len() > n {
					pq.Pop()
				}
			}
		}
	}
	pq = pq.Reverse()
	for pq.Len() > 0 {
		value, score := pq.Pop()
		if !prune0 || score < 0 {
			values = append(values, value)
			scores = append(scores, score)
		}
	}
	return
}

func (idx *IVF) MultiSearch(q Vector, terms []string, n int, prune0 bool) (values map[string][]int32, scores map[string][]float32) {
	cq := heap.NewTopKFilter[int, float32](idx.numProbe)
	for c := range idx.clusters {
		d := idx.clusters[c].centroid.Distance(q)
		cq.Push(c, -d)
	}

	// create priority queues
	queues := make(map[string]*heap.PriorityQueue)
	queues[""] = heap.NewPriorityQueue(true)
	for _, term := range terms {
		queues[term] = heap.NewPriorityQueue(true)
	}

	// search with terms
	clusters, _ := cq.PopAll()
	for _, c := range clusters {
		for _, i := range idx.clusters[c].observations {
			if idx.data[i] != q {
				vec := idx.data[i]
				queues[""].Push(i, q.Distance(vec))
				if queues[""].Len() > n {
					queues[""].Pop()
				}
				for _, term := range vec.Terms() {
					if _, match := queues[term]; match {
						queues[term].Push(i, q.Distance(vec))
						if queues[term].Len() > n {
							queues[term].Pop()
						}
					}
				}
			}
		}
	}

	// retrieve results
	values = make(map[string][]int32)
	scores = make(map[string][]float32)
	for term, pq := range queues {
		pq = pq.Reverse()
		for pq.Len() > 0 {
			value, score := pq.Pop()
			if !prune0 || score < 0 {
				values[term] = append(values[term], value)
				scores[term] = append(scores[term], score)
			}
		}
	}
	return
}

func (idx *IVF) Build(_ context.Context) {
	if idx.k > len(idx.data) {
		panic("the size of the observations set must greater than or equal to k")
	} else if len(idx.data) == 0 {
		log.Logger().Warn("no vectors for building IVF")
		return
	}

	// initialize clusters
	clusters := make([]ivfCluster, idx.k)
	assignments := make([]int, len(idx.data))
	for i := range idx.data {
		if !idx.data[i].IsHidden() {
			c := rand.Intn(idx.k)
			clusters[c].observations = append(clusters[c].observations, int32(i))
			assignments[i] = c
		}
	}
	for c := range clusters {
		clusters[c].centroid = idx.data[0].Centroid(idx.data, clusters[c].observations)
	}

	for it := 0; it < idx.maxIter; it++ {
		errorCount := atomic.NewInt32(0)

		// reassign clusters
		nextClusters := make([]ivfCluster, idx.k)
		_ = parallel.Parallel(len(idx.data), idx.jobsAlloc.AvailableJobs(), func(_, i int) error {
			if !idx.data[i].IsHidden() {
				nextCluster, nextDistance := -1, float32(math32.MaxFloat32)
				for c := range clusters {
					d := clusters[c].centroid.Distance(idx.data[i])
					if d < nextDistance {
						nextCluster = c
						nextDistance = d
					}
				}
				if nextCluster == -1 {
					return nil
				}
				if nextCluster != assignments[i] {
					errorCount.Inc()
				}
				nextClusters[nextCluster].mu.Lock()
				defer nextClusters[nextCluster].mu.Unlock()
				nextClusters[nextCluster].observations = append(nextClusters[nextCluster].observations, int32(i))
				assignments[i] = nextCluster
			}
			return nil
		})

		log.Logger().Debug("spatial k means clustering",
			zap.Int32("changes", errorCount.Load()))
		if float32(errorCount.Load())/float32(len(idx.data)) < idx.errorRate {
			idx.clusters = clusters
			break
		}
		for c := range clusters {
			nextClusters[c].centroid = idx.data[0].Centroid(idx.data, nextClusters[c].observations)
		}
		clusters = nextClusters
	}
}

type IVFBuilder struct {
	bruteForce *Bruteforce
	data       []Vector
	testSize   int
	k          int
	rng        base.RandomGenerator
	configs    []IVFConfig
}

func NewIVFBuilder(data []Vector, k int, configs ...IVFConfig) *IVFBuilder {
	b := &IVFBuilder{
		bruteForce: NewBruteforce(data),
		data:       data,
		testSize:   DefaultTestSize,
		k:          k,
		rng:        base.NewRandomGenerator(0),
		configs:    configs,
	}
	b.bruteForce.Build(context.Background())
	return b
}

func (b *IVFBuilder) evaluate(idx *IVF, prune0 bool) float32 {
	testSize := mathutil.Min(b.testSize, len(b.data))
	samples := b.rng.Sample(0, len(b.data), testSize)
	var result, count float32
	var mu sync.Mutex
	_ = parallel.Parallel(len(samples), idx.jobsAlloc.AvailableJobs(), func(_, i int) error {
		sample := samples[i]
		expected, _ := b.bruteForce.Search(b.data[sample], b.k, prune0)
		if len(expected) > 0 {
			actual, _ := idx.Search(b.data[sample], b.k, prune0)
			mu.Lock()
			defer mu.Unlock()
			result += recall(expected, actual)
			count++
		}
		return nil
	})
	if count == 0 {
		return 0
	}
	return result / count
}

func (b *IVFBuilder) Build(recall float32, numEpoch int, prune0 bool) (idx *IVF, score float32) {
	idx = NewIVF(b.data, b.configs...)
	start := time.Now()
	idx.Build(context.Background())

	buildTime := time.Since(start)
	idx.numProbe = int(math32.Ceil(float32(b.k) / math32.Sqrt(float32(len(b.data)))))
	for i := 0; i < numEpoch; i++ {
		score = b.evaluate(idx, prune0)
		log.Logger().Info("try to build vector index",
			zap.String("index_type", "IVF"),
			zap.Int("num_probe", idx.numProbe),
			zap.Float32("recall", score),
			zap.String("build_time", buildTime.String()))
		if score >= recall {
			return
		} else {
			idx.numProbe <<= 1
		}
	}
	return
}

func (b *IVFBuilder) evaluateTermSearch(idx *IVF, prune0 bool, term string) float32 {
	testSize := mathutil.Min(b.testSize, len(b.data))
	samples := b.rng.Sample(0, len(b.data), testSize)
	var result, count float32
	var mu sync.Mutex
	_ = parallel.Parallel(len(samples), idx.jobsAlloc.AvailableJobs(), func(_, i int) error {
		sample := samples[i]
		expected, _ := b.bruteForce.MultiSearch(b.data[sample], []string{term}, b.k, prune0)
		if len(expected) > 0 {
			actual, _ := idx.MultiSearch(b.data[sample], []string{term}, b.k, prune0)
			mu.Lock()
			defer mu.Unlock()
			result += recall(expected[term], actual[term])
			count++
		}
		return nil
	})
	return result / count
}

func EstimateIVFBuilderComplexity(num, numEpoch int) int {
	// clustering complexity
	complexity := DefaultMaxIter * num * int(math.Sqrt(float64(num)))
	// search complexity
	complexity += num * DefaultTestSize * numEpoch
	return complexity
}
