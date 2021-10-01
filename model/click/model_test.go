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
package click

import (
	"bytes"
	"github.com/stretchr/testify/mock"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/model"
)

const (
	regressionDelta     = 0.001
	classificationDelta = 0.01
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

func (t *mockTracker) Update(_ int) {
	if !t.notTracking {
		t.Called()
	}
}

func (t *mockTracker) Finish() {
	if !t.notTracking {
		t.Called()
	}
}

func (t *mockTracker) Suspend(_ bool) {
	if !t.notTracking {
		t.Called()
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
	cfg := NewFitConfig().
		SetVerbose(1).
		SetJobs(1).
		SetTracker(tracker)
	return cfg, tracker
}

func TestFM_Classification_Frappe(t *testing.T) {
	// LibFM command:
	// libfm.exe -train train.libfm -test test.libfm -task c \
	//   -method sgd -init_stdev 0.01 -dim 1,1,8 -iter 20 \
	//   -learn_rate 0.01 -regular 0,0,0.0001
	train, test, err := LoadDataFromBuiltIn("frappe")
	assert.NoError(t, err)
	m := NewFM(FMClassification, model.Params{
		model.InitStdDev: 0.01,
		model.NFactors:   8,
		model.NEpochs:    20,
		model.Lr:         0.01,
		model.Reg:        0.0001,
	})
	fitConfig, tracker := newFitConfigWithTestTracker(20)
	score := m.Fit(train, test, fitConfig)
	tracker.AssertExpectations(t)
	assert.InDelta(t, 0.91684, score.Accuracy, classificationDelta)
}

//func TestFM_Classification_MovieLens(t *testing.T) {
//	// LibFM command:
//	// libfm.exe -train train.libfm -test test.libfm -task r \
//	//   -method sgd -init_stdev 0.01 -dim 1,1,8 -iter 20 \
//	//   -learn_rate 0.01 -regular 0,0,0.0001
//	train, test, err := LoadDataFromBuiltIn("ml-tag")
//	assert.NoError(t, err)
//	m := NewFM(FMClassification, model.Params{
//		model.InitStdDev: 0.01,
//		model.NFactors:   8,
//		model.NEpochs:    20,
//		model.Lr:         0.01,
//		model.Reg:        0.0001,
//	})
//	score := m.Fit(train, test, fitConfig)
//	assertEpsilon(t, 0.901777, score.Precision)
//}

func TestFM_Regression_Criteo(t *testing.T) {
	// LibFM command:
	// libfm.exe -train train.libfm -test test.libfm -task r \
	//   -method sgd -init_stdev 0.01 -dim 1,1,8 -iter 20 \
	//   -learn_rate 0.001 -regular 0,0,0.0001
	train, test, err := LoadDataFromBuiltIn("criteo")
	assert.NoError(t, err)
	m := NewFM(FMRegression, model.Params{
		model.InitStdDev: 0.01,
		model.NFactors:   8,
		model.NEpochs:    20,
		model.Lr:         0.001,
		model.Reg:        0.0001,
	})
	fitConfig, tracker := newFitConfigWithTestTracker(20)
	score := m.Fit(train, test, fitConfig)
	tracker.AssertExpectations(t)
	assert.InDelta(t, 0.839194, score.RMSE, regressionDelta)

	// test prediction
	assert.Equal(t, m.InternalPredict([]int32{1, 2, 3, 4, 5, 6}, []float32{1, 1, 0.5, 0.5, 0.5, 0.5}),
		m.Predict("1", "2", []string{"3", "4"}, []string{"5", "6"}))

	// test increment test
	buf := bytes.NewBuffer(nil)
	err = MarshalModel(buf, m)
	assert.NoError(t, err)
	tmp, err := UnmarshalModel(buf)
	assert.NoError(t, err)
	m = tmp.(*FM)
	m.nEpochs = 1
	fitConfig, _ = newFitConfigWithTestTracker(1)
	scoreInc := m.Fit(train, test, fitConfig)
	assert.InDelta(t, 0.839194, scoreInc.RMSE, regressionDelta)

	// test clear
	m.Clear()
	assert.True(t, m.Invalid())
}

//func TestFM_Regression_Frappe(t *testing.T) {
//	// LibFM command:
//	// libfm.exe -train train.libfm -test test.libfm -task r \
//	//   -method sgd -init_stdev 0.01 -dim 1,1,8 -iter 20 \
//	//   -learn_rate 0.01 -regular 0,0,0.0001
//	train, test, err := LoadDataFromBuiltIn("frappe")
//	assert.NoError(t, err)
//	m := NewFM(FMRegression, model.Params{
//		model.InitStdDev: 0.01,
//		model.NFactors:   8,
//		model.NEpochs:    20,
//		model.Lr:         0.01,
//		model.Reg:        0.0001,
//	})
//	fitConfig, tracker := newFitConfigWithTestTracker(20)
//	score := m.Fit(train, test, fitConfig)
//	tracker.AssertExpectations(t)
//	assert.InDelta(t, 0.494435, score.RMSE, regressionDelta)
//}

//func TestFM_Regression_MovieLens(t *testing.T) {
//	// LibFM command:
//	// libfm.exe -train train.libfm -test test.libfm -task r \
//	//   -method sgd -init_stdev 0.01 -dim 1,1,8 -iter 20 \
//	//   -learn_rate 0.01 -regular 0,0,0.0001
//	train, test, err := LoadDataFromBuiltIn("ml-tag")
//	assert.NoError(t, err)
//	m := NewFM(FMRegression, model.Params{
//		model.InitStdDev: 0.01,
//		model.NFactors:   8,
//		model.NEpochs:    20,
//		model.Lr:         0.01,
//		model.Reg:        0.0001,
//	})
//	score := m.Fit(train, test, fitConfig)
//	assertEpsilon(t, 0.570648, score.RMSE)
//}
