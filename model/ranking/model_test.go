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
package ranking

import (
	"runtime"
	"testing"

	"github.com/chewxy/math32"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/model"
)

const (
	benchEpsilon = 0.01
	incrEpsilon  = 1e-5
)

var fitConfig = &FitConfig{
	Jobs:       runtime.NumCPU(),
	Verbose:    1,
	Candidates: 100,
	TopK:       10,
}

func assertEpsilon(t *testing.T, expect, actual, eps float32) {
	if math32.Abs(expect-actual) > eps {
		t.Fatalf("|%v - %v| > %v", expect, actual, eps)
	}
}

// He, Xiangnan, et al. "Neural collaborative filtering." Proceedings
// of the 26th international conference on world wide web. 2017.

func TestBPR_MovieLens(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("ml-1m")
	assert.Nil(t, err)
	m := NewBPR(model.Params{
		model.NFactors:   8,
		model.Reg:        0.01,
		model.Lr:         0.05,
		model.NEpochs:    30,
		model.InitMean:   0,
		model.InitStdDev: 0.001,
	})
	score := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.36, score.NDCG, benchEpsilon)

	// test predict
	assert.Equal(t, m.Predict("1", "1"), m.InternalPredict(1, 1))

	// test increment test
	m.nEpochs = 0
	scoreInc := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, score.NDCG, scoreInc.NDCG, incrEpsilon)

	// test clear
	m.Clear()
	score = m.Fit(trainSet, testSet, fitConfig)
	assert.Less(t, score.NDCG, float32(0.2))
}

//func TestBPR_Pinterest(t *testing.T) {
//	trainSet, testSet, err := LoadDataFromBuiltIn("pinterest-20")
//	assert.Nil(t, err)
//	m := NewBPR(model.Params{
//		model.NFactors:   8,
//		model.Reg:        0.005,
//		model.Lr:         0.05,
//		model.NEpochs:    50,
//		model.InitMean:   0,
//		model.InitStdDev: 0.001,
//	})
//	score := m.Fit(trainSet, testSet, fitConfig)
//	assertEpsilon(t, 0.53, score.NDCG, benchEpsilon)
//}

func TestALS_MovieLens(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("ml-1m")
	assert.Nil(t, err)
	m := NewALS(model.Params{
		model.NFactors: 8,
		model.Reg:      0.015,
		model.NEpochs:  10,
		model.Alpha:    0.05,
	})
	score := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.36, score.NDCG, benchEpsilon)

	// test predict
	assert.Equal(t, m.Predict("1", "1"), m.InternalPredict(1, 1))

	// test increment test
	m.nEpochs = 0
	scoreInc := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, score.NDCG, scoreInc.NDCG, incrEpsilon)

	// test clear
	m.Clear()
	score = m.Fit(trainSet, testSet, fitConfig)
	assert.Less(t, score.NDCG, float32(0.2))
}

//func TestALS_Pinterest(t *testing.T) {
//	trainSet, testSet, err := LoadDataFromBuiltIn("pinterest-20")
//	assert.Nil(t, err)
//	m := NewALS(model.Params{
//		model.NFactors:   8,
//		model.Reg:        0.01,
//		model.NEpochs:    10,
//		model.InitStdDev: 0.01,
//		model.Alpha:      0.001,
//	})
//	score := m.Fit(trainSet, testSet, fitConfig)
//	assertEpsilon(t, 0.52, score.NDCG, benchEpsilon)
//}

func TestCCD_MovieLens(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("ml-1m")
	assert.Nil(t, err)
	m := NewCCD(model.Params{
		model.NFactors: 8,
		model.Reg:      0.015,
		model.NEpochs:  30,
		model.Alpha:    0.05,
	})
	score := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.36, score.NDCG, benchEpsilon)

	// test predict
	assert.Equal(t, m.Predict("1", "1"), m.InternalPredict(1, 1))

	// test increment test
	m.nEpochs = 0
	scoreInc := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, score.NDCG, scoreInc.NDCG, incrEpsilon)

	// test clear
	m.Clear()
	score = m.Fit(trainSet, testSet, fitConfig)
	assert.Less(t, score.NDCG, float32(0.2))
}

//func TestCCD_Pinterest(t *testing.T) {
//	trainSet, testSet, err := LoadDataFromBuiltIn("pinterest-20")
//	assert.Nil(t, err)
//	m := NewCCD(model.Params{
//		model.NFactors:   8,
//		model.Reg:        0.01,
//		model.NEpochs:    20,
//		model.InitStdDev: 0.01,
//		model.Alpha:      0.001,
//	})
//	score := m.Fit(trainSet, testSet, fitConfig)
//	assertEpsilon(t, 0.52, score.NDCG, benchEpsilon)
//}
