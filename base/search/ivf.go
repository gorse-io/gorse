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
	"time"
)

var _ VectorIndex = &IVF{}

type IVF struct {
	clusters  []ivfCluster
	data      []Vector
	k         int
	errorRate float32
	numProbe  int
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

type ivfCluster struct {
	centroid     *dictionaryCentroidVector
	observations []int32
}

func NewInvertedFile(vectors []Vector, configs ...IVFConfig) *IVF {
	idx := &IVF{
		data:      vectors,
		k:         int(math32.Sqrt(float32(len(vectors)))),
		errorRate: 0.05,
		numProbe:  8,
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
		for i := range idx.data {
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
			nextClusters[nextCluster].observations = append(nextClusters[nextCluster].observations, int32(i))
			assignments[i] = nextCluster
		}

		if float32(errorCount)/float32(len(idx.data)) < idx.errorRate {
			idx.clusters = clusters
			break
		} else {
			for c := range clusters {
				nextClusters[c].centroid = newDictionaryCentroidVector(idx.data, nextClusters[c].observations)
			}
			clusters = nextClusters
		}
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
}

func NewIVFBuilder(data []Vector, k, testSize int) *IVFBuilder {
	b := &IVFBuilder{
		bruteForce: NewBruteforce(data),
		data:       data,
		testSize:   testSize,
		k:          k,
		rng:        base.NewRandomGenerator(0),
	}
	b.bruteForce.Build()
	return b
}

func (b *IVFBuilder) evaluate(idx VectorIndex, prune0 bool) float32 {
	testSize := base.Min(b.testSize, len(b.data))
	samples := b.rng.Sample(0, len(b.data), testSize)
	var result, count float32
	for _, i := range samples {
		expected, _ := b.bruteForce.Search(b.data[i], b.k, prune0)
		if len(expected) > 0 {
			actual, _ := idx.Search(b.data[i], b.k, prune0)
			result += recall(expected, actual)
			count++
		}
	}
	return result / count
}

func (b *IVFBuilder) Build(prune0 bool) (*IVF, float32) {
	idx := NewInvertedFile(b.data)
	start := time.Now()
	idx.Build()
	buildTime := time.Since(start)
	score := b.evaluate(idx, prune0)
	base.Logger().Info("try to build vector index",
		zap.String("index_type", "IVF"),
		zap.Float32("recall", score),
		zap.String("build_time", buildTime.String()))
	return idx, score
}
