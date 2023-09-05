// Copyright 2023 gorse Project Authors
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
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base/task"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/ranking"
)

func TestHNSW_InnerProduct(t *testing.T) {
	// load dataset
	trainSet, testSet, err := ranking.LoadDataFromBuiltIn("ml-100k")
	assert.NoError(t, err)
	m := ranking.NewBPR(model.Params{
		model.NFactors:   8,
		model.Reg:        0.01,
		model.Lr:         0.05,
		model.NEpochs:    30,
		model.InitMean:   0,
		model.InitStdDev: 0.001,
	})
	fitConfig := ranking.NewFitConfig().SetVerbose(1).SetJobsAllocator(task.NewConstantJobsAllocator(runtime.NumCPU()))
	m.Fit(context.Background(), trainSet, testSet, fitConfig)
	var vectors []Vector
	for _, itemFactor := range m.ItemFactor {
		vectors = append(vectors, NewDenseVector(itemFactor))
	}

	// build vector index
	embeddingIndex := NewHNSW(Dot)
	embeddingIndex.Add(context.Background(), vectors...)
	assert.Greater(t, embeddingIndex.Evaluate(100), 0.9)
}

func TestHNSW_Cosine(t *testing.T) {
	// load dataset
	trainSet, _, err := ranking.LoadDataFromBuiltIn("ml-100k")
	assert.NoError(t, err)
	var vectors []Vector
	for _, feedback := range trainSet.ItemFeedback {
		values := make([]float32, len(feedback))
		for i := range feedback {
			values[i] = 1
		}
		vectors = append(vectors, NewSparseVector(feedback, values))
	}

	// build vector index
	embeddingIndex := NewHNSW(Euclidean)
	embeddingIndex.Add(context.Background(), vectors...)
	assert.Greater(t, embeddingIndex.Evaluate(100), 0.8)
}

func TestHNSW_HasNil(t *testing.T) {
	vectors := []Vector{nil, nil, NewDenseVector([]float32{0, 0, 0, 0, 0, 1}), nil}
	index := NewHNSW(Euclidean)
	index.Add(context.Background(), vectors...)
	results := index.Search(NewDenseVector([]float32{0, 0, 0, 0, 0, 1}), 3)
	assert.Equal(t, []Result{{Index: 2, Distance: 0}}, results)
}

func TestHNSW_HasInf(t *testing.T) {
	index := NewHNSW(Euclidean)
	index.Add(context.Background(), NewSparseVector([]int32{1, 3, 5}, []float32{1, 2, 3}))
	index.Add(context.Background(), NewSparseVector([]int32{2, 4, 6}, []float32{1, 2, 3}))
	results := index.Search(NewSparseVector([]int32{1, 3, 5}, []float32{1, 2, 3}), 3)
	assert.Equal(t, []Result{{Index: 0, Distance: 0}}, results)
}
