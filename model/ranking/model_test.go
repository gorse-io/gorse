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
	incrDelta  = 0.05
)

type mockTracker struct {
	mock.Mock
	notTracking bool
}

func (t *mockTracker) Start(total int) {
	if !t.notTracking {
		t.Called(total)
	}
}

func (t *mockTracker) Update(done int) {
	if !t.notTracking {
		t.Called(done)
	}
}

func (t *mockTracker) Finish() {
	if !t.notTracking {
		t.Called()
	}
}

func (t *mockTracker) Suspend(flag bool) {
	if !t.notTracking {
		t.Called(flag)
	}
}

func (t *mockTracker) SubTracker() model.Tracker {
	if !t.notTracking {
		t.Called()
	}
	return &mockTracker{notTracking: true}
}

func newFitConfigWithTestTracker(numEpoch int) (*FitConfig, *mockTracker) {
	tracker := new(mockTracker)
	tracker.On("Start", numEpoch)
	tracker.On("Update", mock.Anything)
	tracker.On("Finish")
	cfg := NewFitConfig().SetVerbose(1).SetJobs(runtime.NumCPU()).SetTracker(tracker)
	return cfg, tracker
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
	assert.Equal(t, trainSet.UserIndex, m.GetUserIndex())
	assert.Equal(t, testSet.ItemIndex, m.GetItemIndex())

	// test predict
	assert.Equal(t, m.Predict("1", "1"), m.InternalPredict(1, 1))

	// test encode/decode model and increment training
	buf, err := EncodeModel(m)
	assert.NoError(t, err)
	tmp, err := DecodeModel(buf)
	assert.NoError(t, err)
	m = tmp.(*BPR)
	m.nEpochs = 1
	fitConfig, _ = newFitConfigWithTestTracker(1)
	scoreInc := m.Fit(trainSet, testSet, fitConfig)
	assert.InDelta(t, score.NDCG, scoreInc.NDCG, incrDelta)

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
	assert.InDelta(t, 0.36, score.NDCG, benchDelta)

	// test predict
	assert.Equal(t, m.Predict("1", "1"), m.InternalPredict(1, 1))

	// test encode/decode model and increment training
	buf, err := EncodeModel(m)
	assert.NoError(t, err)
	tmp, err := DecodeModel(buf)
	assert.NoError(t, err)
	m = tmp.(*ALS)
	m.nEpochs = 1
	fitConfig, _ = newFitConfigWithTestTracker(1)
	scoreInc := m.Fit(trainSet, testSet, fitConfig)
	assert.InDelta(t, score.NDCG, scoreInc.NDCG, incrDelta)

	// test clear
	m.Clear()
	assert.True(t, m.Invalid())
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
	assert.InDelta(t, 0.36, score.NDCG, benchDelta)

	// test predict
	assert.Equal(t, m.Predict("1", "1"), m.InternalPredict(1, 1))

	// test encode/decode model and increment training
	buf, err := EncodeModel(m)
	assert.NoError(t, err)
	tmp, err := DecodeModel(buf)
	assert.NoError(t, err)
	m = tmp.(*CCD)
	m.nEpochs = 1
	fitConfig, _ = newFitConfigWithTestTracker(1)
	scoreInc := m.Fit(trainSet, testSet, fitConfig)
	assert.InDelta(t, score.NDCG, scoreInc.NDCG, incrDelta)

	// test clear
	m.Clear()
	assert.True(t, m.Invalid())
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
