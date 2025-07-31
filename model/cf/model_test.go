// Copyright 2021 gorse Project Authors
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
package cf

import (
	"bytes"
	"context"
	"math"
	"runtime"
	"testing"

	"github.com/gorse-io/gorse/common/floats"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model"
	"github.com/stretchr/testify/assert"
)

const benchDelta = 0.01

func newFitConfig(_ int) *FitConfig {
	cfg := NewFitConfig().SetVerbose(1).SetJobs(runtime.NumCPU())
	return cfg
}

func TestBPR_MovieLens(t *testing.T) {
	trainSet, testSet, err := dataset.LoadDataFromBuiltIn("ml-1m")
	assert.NoError(t, err)
	m := NewBPR(model.Params{
		model.NFactors:   8,
		model.Reg:        0.01,
		model.Lr:         0.05,
		model.NEpochs:    30,
		model.InitMean:   0,
		model.InitStdDev: 0.001,
	})
	fitConfig := newFitConfig(30)
	score := m.Fit(context.Background(), trainSet, testSet, fitConfig)
	assert.InDelta(t, 0.36, score.NDCG, benchDelta)
	assert.Equal(t, trainSet.GetUserDict(), m.GetUserIndex())
	assert.Equal(t, testSet.GetItemDict(), m.GetItemIndex())

	// test predict
	assert.Equal(t, m.Predict("1", "1"), m.internalPredict(1, 1))
	assert.Equal(t, m.internalPredict(1, 1), floats.Dot(m.GetUserFactor(1), m.GetItemFactor(1)))
	assert.True(t, m.IsUserPredictable(1))
	assert.True(t, m.IsItemPredictable(1))
	assert.False(t, m.IsUserPredictable(math.MaxInt32))
	assert.False(t, m.IsItemPredictable(math.MaxInt32))

	// test encode/decode model and increment training
	buf := bytes.NewBuffer(nil)
	err = MarshalModel(buf, m)
	assert.NoError(t, err)
	tmp, err := UnmarshalModel(buf)
	assert.NoError(t, err)
	assert.Equal(t, m.Params, tmp.GetParams())
	assert.Equal(t, m.Predict("1", "1"), tmp.Predict("1", "1"))
	assert.True(t, m.IsUserPredictable(1))
	assert.True(t, m.IsItemPredictable(1))
	assert.False(t, m.IsUserPredictable(math.MaxInt32))
	assert.False(t, m.IsItemPredictable(math.MaxInt32))

	// test clear
	m.Clear()
	assert.True(t, m.Invalid())
}

//func TestBPR_Pinterest(t *testing.T) {
//	trainSet, testSet, err := LoadDataFromBuiltIn("pinterest-20")
//	assert.NoError(t, err)
//	m := NewBPR(model.Params{
//		model.NFactors:   8,
//		model.Reg:        0.005,
//		model.Lr:         0.05,
//		model.NEpochs:    50,
//		model.InitMean:   0,
//		model.InitStdDev: 0.001,
//	})
//	score := m.Fit(trainSet, testSet, fitConfig)
//	assertEpsilon(t, 0.53, score.NDCG, benchDelta)
//}

func TestCCD_MovieLens(t *testing.T) {
	trainSet, testSet, err := dataset.LoadDataFromBuiltIn("ml-1m")
	assert.NoError(t, err)
	m := NewALS(model.Params{
		model.NFactors: 8,
		model.Reg:      0.015,
		model.NEpochs:  30,
		model.Alpha:    0.05,
	})
	fitConfig := newFitConfig(30)
	score := m.Fit(context.Background(), trainSet, testSet, fitConfig)
	assert.InDelta(t, 0.36, score.NDCG, benchDelta)

	// test predict
	assert.Equal(t, m.Predict("1", "1"), m.internalPredict(1, 1))
	assert.Equal(t, m.internalPredict(1, 1), floats.Dot(m.GetUserFactor(1), m.GetItemFactor(1)))

	// test encode/decode model and increment training
	buf := bytes.NewBuffer(nil)
	err = MarshalModel(buf, m)
	assert.NoError(t, err)
	tmp, err := UnmarshalModel(buf)
	assert.NoError(t, err)
	assert.Equal(t, m.Params, tmp.GetParams())
	assert.Equal(t, m.Predict("1", "1"), tmp.Predict("1", "1"))

	// test clear
	m.Clear()
	assert.True(t, m.Invalid())
}

//func TestCCD_Pinterest(t *testing.T) {
//	trainSet, testSet, err := LoadDataFromBuiltIn("pinterest-20")
//	assert.NoError(t, err)
//	m := NewALS(model.Params{
//		model.NFactors:   8,
//		model.Reg:        0.01,
//		model.NEpochs:    20,
//		model.InitStdDev: 0.01,
//		model.Alpha:      0.001,
//	})
//	score := m.Fit(trainSet, testSet, fitConfig)
//	assertEpsilon(t, 0.52, score.NDCG, benchDelta)
//}
