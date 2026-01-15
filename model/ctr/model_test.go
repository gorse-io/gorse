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
package ctr

import (
	"bytes"
	"context"
	"runtime"
	"testing"

	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

const classificationDelta = 0.01

func newFitConfigWithTestTracker() *FitConfig {
	cfg := NewFitConfig().SetVerbose(1).SetJobs(runtime.NumCPU())
	return cfg
}

func TestFactorizationMachines_Classification_Frappe(t *testing.T) {
	// python .\model.py frappe -dim 8 -iter 10 -learn_rate 0.01 -regular 0.0001
	train, test, err := LoadDataFromBuiltIn("frappe")
	assert.NoError(t, err)
	m := NewAFM(model.Params{
		model.NFactors:  8,
		model.NEpochs:   10,
		model.Lr:        0.01,
		model.Reg:       0.0001,
		model.BatchSize: 1024,
	})
	fitConfig := newFitConfigWithTestTracker()
	score := m.Fit(context.Background(), train, test, fitConfig)
	assert.InDelta(t, 0.919, score.Accuracy, classificationDelta)
}

func TestFactorizationMachines_Classification_MovieLens(t *testing.T) {
	t.Skip("Skip time-consuming test")
	// python .\model.py ml-tag -dim 8 -iter 10 -learn_rate 0.01 -regular 0.0001
	train, test, err := LoadDataFromBuiltIn("ml-tag")
	assert.NoError(t, err)
	m := NewAFM(model.Params{
		model.InitStdDev: 0.01,
		model.NFactors:   8,
		model.NEpochs:    10,
		model.Lr:         0.001,
		model.Reg:        0.0001,
		model.BatchSize:  1024,
	})
	fitConfig := newFitConfigWithTestTracker()
	score := m.Fit(context.Background(), train, test, fitConfig)
	assert.InDelta(t, 0.815, score.Accuracy, classificationDelta)
}

func TestFactorizationMachines_Classification_Criteo(t *testing.T) {
	// python .\model.py criteo -dim 8 -iter 10 -learn_rate 0.01 -regular 0.0001
	train, test, err := LoadDataFromBuiltIn("criteo")
	assert.NoError(t, err)
	m := NewAFM(model.Params{
		model.NFactors:  8,
		model.NEpochs:   10,
		model.Lr:        0.01,
		model.Reg:       0.0001,
		model.BatchSize: 1024,
	})
	fitConfig := newFitConfigWithTestTracker()
	score := m.Fit(context.Background(), train, test, fitConfig)
	assert.InDelta(t, 0.77, score.Accuracy, 0.025)

	// test prediction
	assert.Equal(t, m.BatchInternalPredict(
		[]lo.Tuple2[[]int32, []float32]{{A: []int32{1, 2, 3, 4, 5, 6}, B: []float32{1, 1, 0.3, 0.4, 0.5, 0.6}}},
		nil, fitConfig.Jobs),
		m.BatchPredict([]lo.Tuple4[string, string, []Label, []Label]{{
			A: "1",
			B: "2",
			C: []Label{
				{Name: "3", Value: 0.3},
				{Name: "4", Value: 0.4},
			},
			D: []Label{
				{Name: "5", Value: 0.5},
				{Name: "6", Value: 0.6},
			}}}, nil, fitConfig.Jobs))

	// test marshal and unmarshal
	buf := bytes.NewBuffer(nil)
	err = MarshalModel(buf, m)
	assert.NoError(t, err)
	tmp, err := UnmarshalModel(buf)
	assert.NoError(t, err)
	scoreClone := EvaluateClassification(tmp, test, fitConfig.Jobs)
	assert.InDelta(t, 0.77, scoreClone.Accuracy, 0.02)

	// test clear
	assert.False(t, m.Invalid())
	m.Clear()
	assert.True(t, m.Invalid())
}

func newSynthesisDataset() *Dataset {
	builder := dataset.NewUnifiedMapIndexBuilder()
	builder.AddUser("u0")
	builder.AddUser("u1")
	builder.AddUserLabel("ul0")
	builder.AddUserLabel("ul1")
	builder.AddUserLabel("ul2")
	builder.AddItem("i0")
	builder.AddItem("i1")
	builder.AddItemLabel("il0")
	builder.AddItemLabel("il1")
	builder.AddItemLabel("il2")

	dataSet := NewMapIndexDataset()
	dataSet.Index = builder.Build()
	dataSet.UserLabels = [][]lo.Tuple2[int32, float32]{
		{{A: 0, B: 1.0}, {A: 1, B: 0.5}, {A: 2, B: -1.0}},
		{{A: 0, B: -1.0}, {A: 1, B: -0.5}, {A: 2, B: 1.0}},
	}
	dataSet.ItemLabels = [][]lo.Tuple2[int32, float32]{
		{{A: 0, B: 1.0}, {A: 1, B: 0.5}, {A: 2, B: -1.0}},
		{{A: 0, B: -1.0}, {A: 1, B: -0.5}, {A: 2, B: 1.0}},
	}
	dataSet.ItemEmbeddingIndex = dataset.NewMapIndex()
	dataSet.ItemEmbeddingIndex.Add("e1")
	dataSet.ItemEmbeddingIndex.Add("e2")
	dataSet.ItemEmbeddingDimension = []int{3, 4}
	dataSet.ItemEmbeddings = [][][]float32{
		{{0.8, 0.8, 0.8}, {0.1, 0.1, 0.1, 0.1}},
		{{-0.8, -0.8, -0.8}, {-0.1, -0.1, -0.1, -0.1}},
	}

	dataSet.Users = []int32{0, 0, 1, 1}
	dataSet.Items = []int32{0, 1, 0, 1}
	dataSet.Target = []float32{1, -1, -1, 1}
	dataSet.PositiveCount = 2
	dataSet.NegativeCount = 2
	return dataSet
}

func TestFactorizationMachines_Classification_Synthesis(t *testing.T) {
	dataSet := newSynthesisDataset()
	fitConfig := newFitConfigWithTestTracker()
	m := NewAFM(nil)
	score := m.Fit(context.Background(), dataSet, dataSet, fitConfig)
	assert.GreaterOrEqual(t, score.Accuracy, float32(0.5))

	buf := bytes.NewBuffer(nil)
	err := MarshalModel(buf, m)
	assert.NoError(t, err)
	clone, err := UnmarshalModel(buf)
	assert.NoError(t, err)
	cloneScore := EvaluateClassification(clone, dataSet, fitConfig.Jobs)
	assert.InDelta(t, score.Accuracy, cloneScore.Accuracy, 0.05)

	indicesPos, valuesPos, embeddingsPos, _ := dataSet.Get(0)
	indicesNeg, valuesNeg, embeddingsNeg, _ := dataSet.Get(1)
	internalPrediction := m.BatchInternalPredict(
		[]lo.Tuple2[[]int32, []float32]{
			{A: indicesPos, B: valuesPos},
			{A: indicesNeg, B: valuesNeg},
		},
		[][][]float32{embeddingsPos, embeddingsNeg},
		fitConfig.Jobs,
	)
	batchedPrediction := m.BatchPredict(
		[]lo.Tuple4[string, string, []Label, []Label]{
			{
				A: "u0",
				B: "i0",
				C: []Label{{Name: "ul0", Value: 1.0}, {Name: "ul1", Value: 0.5}, {Name: "ul2", Value: -1.0}},
				D: []Label{{Name: "il0", Value: 1.0}, {Name: "il1", Value: 0.5}, {Name: "il2", Value: -1.0}},
			},
			{
				A: "u0",
				B: "i1",
				C: []Label{{Name: "ul0", Value: 1.0}, {Name: "ul1", Value: 0.5}, {Name: "ul2", Value: -1.0}},
				D: []Label{{Name: "il0", Value: -1.0}, {Name: "il1", Value: -0.5}, {Name: "il2", Value: 1.0}},
			},
		},
		[][]Embedding{
			{{Name: "e1", Value: embeddingsPos[0]}, {Name: "e2", Value: embeddingsPos[1]}},
			{{Name: "e1", Value: embeddingsNeg[0]}, {Name: "e2", Value: embeddingsNeg[1]}},
		},
		fitConfig.Jobs,
	)
	assert.Len(t, internalPrediction, 2)
	assert.Len(t, batchedPrediction, 2)
	assert.InDeltaSlice(t, internalPrediction, batchedPrediction, 1e-6)
}
