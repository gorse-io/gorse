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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base/task"
	"github.com/zhenghaoz/gorse/model"
)

const (
	regressionDelta     = 0.01
	classificationDelta = 0.01
)

func newFitConfigWithTestTracker(numEpoch int) *FitConfig {
	cfg := NewFitConfig().
		SetVerbose(1).
		SetJobsAllocator(task.NewConstantJobsAllocator(1))
	return cfg
}

func TestFM_Classification_Frappe(t *testing.T) {
	// LibFM command:
	// libfm.exe -train train.libfm -test test.libfm -task c \
	//   -method sgd -init_stdev 0.01 -dim 1,1,8 -iter 20 \
	//   -learn_rate 0.01 -regular 0,0,0.0001
	for _, optimizer := range []string{model.Adam, model.SGD} {
		t.Run(optimizer, func(t *testing.T) {
			train, test, err := LoadDataFromBuiltIn("frappe")
			assert.NoError(t, err)
			m := NewFM(FMClassification, model.Params{
				model.InitStdDev: 0.01,
				model.NFactors:   8,
				model.NEpochs:    20,
				model.Lr:         0.01,
				model.Reg:        0.0001,
				model.Optimizer:  optimizer,
			})
			fitConfig := newFitConfigWithTestTracker(20)
			score := m.Fit(context.Background(), train, test, fitConfig)
			assert.InDelta(t, 0.91684, score.Accuracy, classificationDelta)
		})
	}
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
	fitConfig := newFitConfigWithTestTracker(20)
	score := m.Fit(context.Background(), train, test, fitConfig)
	// TODO: Fix it back to 0.839194
	assert.Less(t, score.RMSE, float32(0.87))

	// test prediction
	assert.Equal(t, m.InternalPredict([]int32{1, 2, 3, 4, 5, 6}, []float32{1, 1, 0.3, 0.4, 0.5, 0.6}),
		m.Predict("1", "2",
			[]Feature{
				{Name: "3", Value: 0.3},
				{Name: "4", Value: 0.4},
			},
			[]Feature{
				{Name: "5", Value: 0.5},
				{Name: "6", Value: 0.6},
			}))

	// test increment test
	buf := bytes.NewBuffer(nil)
	err = MarshalModel(buf, m)
	assert.NoError(t, err)
	tmp, err := UnmarshalModel(buf)
	assert.NoError(t, err)
	m = tmp.(*FM)
	m.nEpochs = 1
	fitConfig = newFitConfigWithTestTracker(1)
	scoreInc := m.Fit(context.Background(), train, test, fitConfig)
	// TODO: Fix it back to 0.839194
	assert.Less(t, scoreInc.RMSE, float32(0.87))

	// test clear
	assert.False(t, m.Invalid())
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
