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

package hnsw

import (
	"github.com/bits-and-blooms/bitset"
	"github.com/chewxy/math32"
	"github.com/scylladb/go-set/i32set"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/floats"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/ranking"
	"runtime"
	"testing"
	"time"
)

type cosineVector struct {
	data *bitset.BitSet
}

func (f cosineVector) Distance(vector Vector) float32 {
	if feedbackVector, isFeedback := vector.(*cosineVector); !isFeedback {
		panic("vector type mismatch")
	} else {
		common := f.data.IntersectionCardinality(feedbackVector.data)
		if common == 0 {
			return 0
		}
		return -float32(common) / math32.Sqrt(float32(f.data.Count())) / math32.Sqrt(float32(feedbackVector.data.Count()))
	}
}

type innerProductVector struct {
	data []float32
}

func (f innerProductVector) Distance(vector Vector) float32 {
	if feedbackVector, isFeedback := vector.(*innerProductVector); !isFeedback {
		panic("vector type mismatch")
	} else {
		return -floats.Dot(f.data, feedbackVector.data)
	}
}

func recall(expected, actual []int32) float32 {
	var result float32
	truth := i32set.New(expected...)
	for _, v := range actual {
		if truth.Has(v) {
			result++
		}
	}
	return result / float32(len(actual))
}

func TestCosine(t *testing.T) {
	// load dataset
	trainSet, _, err := ranking.LoadDataFromBuiltIn("ml-1m")
	assert.NoError(t, err)
	var vectors []Vector
	for _, feedback := range trainSet.ItemFeedback {
		bits := bitset.New(uint(trainSet.UserCount()))
		for _, userId := range feedback {
			bits.Set(uint(userId))
		}
		vectors = append(vectors, &cosineVector{data: bits})
	}

	// generate ground truth
	start := time.Now()
	bf := NewBruteforce(vectors)
	expected := make([][]int32, len(vectors))
	for itemId := range vectors {
		expected[itemId], _ = bf.Search(vectors[itemId], 10)
	}
	bruteforceTime := time.Since(start)

	// test vector index
	start = time.Now()
	hnsw := NewHierarchicalNSW(vectors,
		SetMaxConnection(16),
		SetEFConstruction(100))
	var result float32
	for itemId := range vectors {
		approximate, _ := hnsw.Search(vectors[itemId], 10)
		result += recall(expected[itemId], approximate)
	}
	result /= float32(len(vectors))
	hnswTime := time.Since(start)
	assert.Less(t, hnswTime, bruteforceTime)
	assert.Greater(t, result, float32(0.96))
}

func TestInnerProduct(t *testing.T) {
	// load dataset
	trainSet, testSet, err := ranking.LoadDataFromBuiltIn("ml-1m")
	assert.NoError(t, err)
	m := ranking.NewBPR(model.Params{
		model.NFactors:   8,
		model.Reg:        0.01,
		model.Lr:         0.05,
		model.NEpochs:    30,
		model.InitMean:   0,
		model.InitStdDev: 0.001,
	})
	fitConfig := ranking.NewFitConfig().SetVerbose(1).SetJobs(runtime.NumCPU())
	m.Fit(trainSet, testSet, fitConfig)
	var vectors []Vector
	for _, itemFactor := range m.ItemFactor {
		vectors = append(vectors, &innerProductVector{data: itemFactor})
	}

	// generate ground truth
	start := time.Now()
	bf := NewBruteforce(vectors)
	expected := make([][]int32, len(m.UserFactor))
	for userId, userFactor := range m.UserFactor {
		expected[userId], _ = bf.Search(&innerProductVector{data: userFactor}, 10)
	}
	bruteforceTime := time.Since(start)

	// test vector index
	start = time.Now()
	hnsw := NewHierarchicalNSW(vectors,
		SetMaxConnection(16),
		SetEFConstruction(100))
	var result float32
	for userId, userFactor := range m.UserFactor {
		approximate, _ := hnsw.Search(&innerProductVector{data: userFactor}, 10)
		result += recall(expected[userId], approximate)
	}
	result /= float32(len(m.UserFactor))
	hnswTime := time.Since(start)
	assert.Less(t, hnswTime, bruteforceTime)
	assert.Greater(t, result, float32(0.99))
}
