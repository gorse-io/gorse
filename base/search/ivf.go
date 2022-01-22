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
	"github.com/chewxy/math32"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/heap"
	"go.uber.org/zap"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

var _ VectorIndex = &IVF{}

type IVF struct {
	clusters  []ivfCluster
	data      []Vector
	k         int
	errorRate float32
	numProbe  int
	numJobs   int
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

func SetNumJobs(numJobs int) IVFConfig {
	return func(ivf *IVF) {
		ivf.numJobs = numJobs
	}
}

type ivfCluster struct {
	centroid     *dictionaryCentroidVector
	observations []int32
	mu           sync.Mutex
}

func NewIVF(vectors []Vector, configs ...IVFConfig) *IVF {
	idx := &IVF{
		data:      vectors,
		k:         int(math32.Sqrt(float32(len(vectors)))),
		errorRate: 0.05,
		numProbe:  1,
		numJobs:   runtime.NumCPU(),
	}
	for _, config := range configs {
		config(idx)
	}
	return idx
}

func (idx *IVF) Search(q Vector, n int, prune0 bool) (values []int32, scores []float32) {
	cq := heap.NewTopKFilter(idx.numProbe)
	for c := range idx.clusters {
		d := idx.clusters[c].centroid.Distance(q)
		cq.Push(int32(c), -d)
	}

	pq := heap.NewPriorityQueue(true)
	clusters, _ := cq.PopAll()
	for _, c := range clusters {
		for _, i := range idx.clusters[c].observations {
			pq.Push(int32(i), q.Distance(idx.data[i]))
			if pq.Len() > n {
				pq.Pop()
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
	cq := heap.NewTopKFilter(idx.numProbe)
	for c := range idx.clusters {
		d := idx.clusters[c].centroid.Distance(q)
		cq.Push(int32(c), -d)
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
			vec := idx.data[i]
			queues[""].Push(int32(i), q.Distance(vec))
			if queues[""].Len() > n {
				queues[""].Pop()
			}
			for _, term := range vec.Terms() {
				if _, match := queues[term]; match {
					queues[term].Push(int32(i), q.Distance(vec))
					if queues[term].Len() > n {
						queues[term].Pop()
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

func (idx *IVF) Build() {
	if idx.k > len(idx.data) {
		base.Logger().Fatal("the size of the observations set must at least equal k")
	}

	// initialize clusters
	clusters := make([]ivfCluster, idx.k)
	assignments := make([]int, len(idx.data))
	for i := range idx.data {
		c := rand.Intn(idx.k)
		clusters[c].observations = append(clusters[c].observations, int32(i))
		assignments[i] = c
	}
	for c := range clusters {
		clusters[c].centroid = newDictionaryCentroidVector(idx.data, clusters[c].observations)
	}

	for {
		errorCount := 0

		// reassign clusters
		nextClusters := make([]ivfCluster, idx.k)
		_ = base.Parallel(len(idx.data), idx.numJobs, func(_, i int) error {
			nextCluster, nextDistance := -1, float32(math32.MaxFloat32)
			for c := range clusters {
				d := clusters[c].centroid.Distance(idx.data[i])
				if d < nextDistance {
					nextCluster = c
					nextDistance = d
				}
			}
			if nextCluster != assignments[i] {
				errorCount++
			}
			nextClusters[nextCluster].mu.Lock()
			defer nextClusters[nextCluster].mu.Unlock()
			nextClusters[nextCluster].observations = append(nextClusters[nextCluster].observations, int32(i))
			assignments[i] = nextCluster
			return nil
		})

		base.Logger().Debug("spatial k means clustering",
			zap.Int("changes", errorCount))
		if float32(errorCount)/float32(len(idx.data)) < idx.errorRate {
			idx.clusters = clusters
			break
		}
		for c := range clusters {
			nextClusters[c].centroid = newDictionaryCentroidVector(idx.data, nextClusters[c].observations)
		}
		clusters = nextClusters
	}
}

type dictionaryCentroidVector struct {
	data map[int32]float32
	norm float32
}

func newDictionaryCentroidVector(vectors []Vector, indices []int32) *dictionaryCentroidVector {
	data := make(map[int32]float32)
	for _, i := range indices {
		vector, isDictVector := vectors[i].(*DictionaryVector)
		if !isDictVector {
			base.Logger().Fatal("vector type mismatch")
		}
		for _, i := range vector.indices {
			data[i] += math32.Sqrt(vector.values[i])
		}
	}
	var norm float32
	for _, val := range data {
		norm += val * val
	}
	norm = math32.Sqrt(norm)
	for i := range data {
		data[i] /= norm
	}
	return &dictionaryCentroidVector{
		data: data,
		norm: norm,
	}
}

func (v *dictionaryCentroidVector) Distance(vector Vector) float32 {
	var sum float32
	if dictVector, isDictVec := vector.(*DictionaryVector); !isDictVec {
		base.Logger().Fatal("vector type mismatch")
	} else {
		for _, i := range dictVector.indices {
			if val, exist := v.data[i]; exist {
				sum += val * math32.Sqrt(v.data[i])
			}
		}
	}
	return -sum
}

type IVFBuilder struct {
	bruteForce *Bruteforce
	data       []Vector
	testSize   int
	k          int
	rng        base.RandomGenerator
	configs    []IVFConfig
}

func NewIVFBuilder(data []Vector, k, testSize int, configs ...IVFConfig) *IVFBuilder {
	b := &IVFBuilder{
		bruteForce: NewBruteforce(data),
		data:       data,
		testSize:   testSize,
		k:          k,
		rng:        base.NewRandomGenerator(0),
		configs:    configs,
	}
	b.bruteForce.Build()
	return b
}

func (b *IVFBuilder) evaluate(idx *IVF, prune0 bool) float32 {
	testSize := base.Min(b.testSize, len(b.data))
	samples := b.rng.Sample(0, len(b.data), testSize)
	var result, count float32
	var mu sync.Mutex
	_ = base.Parallel(len(samples), idx.numJobs, func(_, i int) error {
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
	return result / count
}

func (b *IVFBuilder) Build(recall float32, numEpoch int, prune0 bool) (idx *IVF, score float32) {
	idx = NewIVF(b.data, b.configs...)
	start := time.Now()
	idx.Build()
	buildTime := time.Since(start)
	idx.numProbe = int(math32.Ceil(float32(b.k) / math32.Sqrt(float32(len(b.data)))))
	for i := 0; i < numEpoch; i++ {
		score = b.evaluate(idx, prune0)
		base.Logger().Info("try to build vector index",
			zap.String("index_type", "IVF"),
			zap.Float32("recall", score),
			zap.String("build_time", buildTime.String()))
		if score > recall {
			return
		} else {
			idx.numProbe <<= 1
		}
	}
	return
}

func (b *IVFBuilder) evaluateTermSearch(idx *IVF, prune0 bool, term string) float32 {
	testSize := base.Min(b.testSize, len(b.data))
	samples := b.rng.Sample(0, len(b.data), testSize)
	var result, count float32
	var mu sync.Mutex
	_ = base.Parallel(len(samples), idx.numJobs, func(_, i int) error {
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
