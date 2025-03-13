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
			m := NewFM(model.Params{
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
