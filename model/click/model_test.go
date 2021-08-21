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
	"github.com/stretchr/testify/mock"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/model"
)

const (
	delta = 0.01
)

type mockTracker struct {
	mock.Mock
}

func (t *mockTracker) Start(total int) {
	t.Called(total)
}

func (t *mockTracker) Update(done int) {
	t.Called()
}

func (t *mockTracker) Finish() {
	t.Called()
}

func (t *mockTracker) Suspend(flag bool) {
	t.Called()
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
		Jobs:    1,
		Verbose: 1,
		Tracker: tracker,
	}, tracker
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
	assert.InDelta(t, 0.91684, score.Accuracy, delta)
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

func TestFM_Regression_Frappe(t *testing.T) {
	// LibFM command:
	// libfm.exe -train train.libfm -test test.libfm -task r \
	//   -method sgd -init_stdev 0.01 -dim 1,1,8 -iter 20 \
	//   -learn_rate 0.01 -regular 0,0,0.0001
	train, test, err := LoadDataFromBuiltIn("frappe")
	assert.NoError(t, err)
	m := NewFM(FMRegression, model.Params{
		model.InitStdDev: 0.01,
		model.NFactors:   8,
		model.NEpochs:    20,
		model.Lr:         0.01,
		model.Reg:        0.0001,
	})
	fitConfig, tracker := newFitConfigWithTestTracker(20)
	score := m.Fit(train, test, fitConfig)
	tracker.AssertExpectations(t)
	assert.InDelta(t, 0.494435, score.RMSE, delta)
}

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
