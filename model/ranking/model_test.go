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
	"github.com/stretchr/testify/mock"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/model"
)

const (
	benchDelta = 0.01
	incrDelta  = 1e-5
)

type mockTracker struct {
	mock.Mock
}

func (t *mockTracker) Start(total int) {
	t.Called(total)
}

func (t *mockTracker) Update(done int) {
	t.Called(done)
}

func (t *mockTracker) Finish() {
	t.Called()
}

func (t *mockTracker) Suspend(flag bool) {
	t.Called(flag)
}

func (t *mockTracker) SubTracker() model.Tracker {
	t.Called()
	return nil
}

func newFitConfigWithTestTracker(numEpoch int) (*FitConfig, *mockTracker) {
	tracker := new(mockTracker)
	tracker.On("Start", numEpoch)
	tracker.On("Update", mock.Anything)
	tracker.On("Finish")
	return &FitConfig{
		Jobs:       runtime.NumCPU(),
		Verbose:    1,
		Candidates: 100,
		TopK:       10,
		Tracker:    tracker,
	}, tracker
}

// He, Xiangnan, et al. "Neural collaborative filtering." Proceedings
// of the 26th international conference on world wide web. 2017.

func TestBPR_MovieLens(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("ml-1m")
	assert.NoError(t, err)
	m := NewBPR(model.Params{
		model.NFactors:   8,
		model.Reg:        0.01,
		model.Lr:         0.05,
		model.NEpochs:    30,
		model.InitMean:   0,
		model.InitStdDev: 0.001,
	})
	fitConfig, tracker := newFitConfigWithTestTracker(30)
	score := m.Fit(trainSet, testSet, fitConfig)
	tracker.AssertExpectations(t)
	assert.InDelta(t, 0.36, score.NDCG, benchDelta)

	// test predict
	assert.Equal(t, m.Predict("1", "1"), m.InternalPredict(1, 1))

	// test increment test
	m.nEpochs = 0
	tracker.On("Start", 0)
	scoreInc := m.Fit(trainSet, testSet, fitConfig)
	assert.InDelta(t, score.NDCG, scoreInc.NDCG, incrDelta)

	// test clear
	m.Clear()
	score = m.Fit(trainSet, testSet, fitConfig)
	assert.Less(t, score.NDCG, float32(0.2))
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

func TestALS_MovieLens(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("ml-1m")
	assert.NoError(t, err)
	m := NewALS(model.Params{
		model.NFactors: 8,
		model.Reg:      0.015,
		model.NEpochs:  10,
		model.Alpha:    0.05,
	})
	fitConfig, tracker := newFitConfigWithTestTracker(10)
	score := m.Fit(trainSet, testSet, fitConfig)
	tracker.AssertExpectations(t)
	assertEpsilon(t, 0.36, score.NDCG, benchEpsilon)
	assert.InDelta(t, 0.36, score.NDCG, benchDelta)

	// test predict
	assert.Equal(t, m.Predict("1", "1"), m.InternalPredict(1, 1))

	// test increment test
	m.nEpochs = 0
	tracker.On("Start", 0)
	scoreInc := m.Fit(trainSet, testSet, fitConfig)
	assert.InDelta(t, score.NDCG, scoreInc.NDCG, incrDelta)

	// test clear
	m.Clear()
	score = m.Fit(trainSet, testSet, fitConfig)
	assert.Less(t, score.NDCG, float32(0.2))
}

//func TestALS_Pinterest(t *testing.T) {
//	trainSet, testSet, err := LoadDataFromBuiltIn("pinterest-20")
//	assert.NoError(t, err)
//	m := NewALS(model.Params{
//		model.NFactors:   8,
//		model.Reg:        0.01,
//		model.NEpochs:    10,
//		model.InitStdDev: 0.01,
//		model.Alpha:      0.001,
//	})
//	score := m.Fit(trainSet, testSet, fitConfig)
//	assertEpsilon(t, 0.52, score.NDCG, benchDelta)
//}

func TestCCD_MovieLens(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("ml-1m")
	assert.NoError(t, err)
	m := NewCCD(model.Params{
		model.NFactors: 8,
		model.Reg:      0.015,
		model.NEpochs:  30,
		model.Alpha:    0.05,
	})
	fitConfig, tracker := newFitConfigWithTestTracker(30)
	score := m.Fit(trainSet, testSet, fitConfig)
	tracker.AssertExpectations(t)
	assertEpsilon(t, 0.36, score.NDCG, benchEpsilon)
	assert.InDelta(t, 0.36, score.NDCG, benchDelta)

	// test predict
	assert.Equal(t, m.Predict("1", "1"), m.InternalPredict(1, 1))

	// test increment test
	m.nEpochs = 0
	tracker.On("Start", 0)
	scoreInc := m.Fit(trainSet, testSet, fitConfig)
	assert.InDelta(t, score.NDCG, scoreInc.NDCG, incrDelta)

	// test clear
	m.Clear()
	score = m.Fit(trainSet, testSet, fitConfig)
	assert.Less(t, score.NDCG, float32(0.2))
}

//func TestCCD_Pinterest(t *testing.T) {
//	trainSet, testSet, err := LoadDataFromBuiltIn("pinterest-20")
//	assert.NoError(t, err)
//	m := NewCCD(model.Params{
//		model.NFactors:   8,
//		model.Reg:        0.01,
//		model.NEpochs:    20,
//		model.InitStdDev: 0.01,
//		model.Alpha:      0.001,
//	})
//	score := m.Fit(trainSet, testSet, fitConfig)
//	assertEpsilon(t, 0.52, score.NDCG, benchDelta)
//}
