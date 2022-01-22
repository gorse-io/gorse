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
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/ranking"
	"runtime"
	"testing"
)

func TestHNSW_Cosine(t *testing.T) {
	// load dataset
	trainSet, _, err := ranking.LoadDataFromBuiltIn("ml-1m")
	assert.NoError(t, err)
	values := make([]float32, trainSet.UserCount())
	for i := range values {
		values[i] = 1
	}
	var vectors []Vector
	for _, feedback := range trainSet.ItemFeedback {
		vectors = append(vectors, NewDictionaryVector(feedback, values))
	}

	// build vector index
	builder := NewHNSWBuilder(vectors, 10, 1000)
	_, recall := builder.Build(0.95, true)
	assert.Greater(t, recall, float32(0.95))
}

func TestHNSW_InnerProduct(t *testing.T) {
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
		vectors = append(vectors, NewDenseVector(itemFactor))
	}

	// build vector index
	builder := NewHNSWBuilder(vectors, 10, 1000)
	_, recall := builder.Build(0.95, false)
	assert.Greater(t, recall, float32(0.95))
}

func TestIVF_Cosine(t *testing.T) {
	// load dataset
	trainSet, _, err := ranking.LoadDataFromBuiltIn("ml-1m")
	assert.NoError(t, err)
	values := make([]float32, trainSet.UserCount())
	for i := range values {
		values[i] = 1
	}
	var vectors []Vector
	for _, feedback := range trainSet.ItemFeedback {
		vectors = append(vectors, NewDictionaryVector(feedback, values))
	}

	// build vector index
	builder := NewIVFBuilder(vectors, 10, 1000)
	_, recall := builder.Build(true)
	assert.Greater(t, recall, float32(0.9))
}
