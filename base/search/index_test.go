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
	"math/big"
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
	for i, itemFactor := range m.ItemFactor {
		var terms []string
		if big.NewInt(int64(i)).ProbablyPrime(0) {
			terms = append(terms, "prime")
		}
		vectors = append(vectors, NewDenseVector(itemFactor, terms, false))
	}

	// build vector index
	builder := NewHNSWBuilder(vectors, 10, runtime.NumCPU())
	idx, recall := builder.Build(context.Background(), 0.9, 5, false)
	assert.Greater(t, recall, float32(0.9))
	recall = builder.evaluateTermSearch(idx, true, "prime")
	assert.Greater(t, recall, float32(0.8))
}
