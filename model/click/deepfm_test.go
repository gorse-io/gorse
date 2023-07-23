// Copyright 2023 gorse Project Authors
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/model"
)

func TestDeepFM_Classification_Frappe(t *testing.T) {
	train, test, err := LoadDataFromBuiltIn("frappe")
	assert.NoError(t, err)
	m := NewDeepFM(model.Params{
		model.InitStdDev: 0.01,
		model.NFactors:   8,
		model.NEpochs:    10,
		model.Lr:         0.01,
		model.Reg:        0.0001,
	})
	fitConfig := newFitConfigWithTestTracker(20)
	score := m.Fit(train, test, fitConfig)
	assert.InDelta(t, 0.9439709, score.Accuracy, classificationDelta)
}

func TestDeepFM_Classification_Criteo(t *testing.T) {
	train, test, err := LoadDataFromBuiltIn("criteo")
	assert.NoError(t, err)
	m := NewDeepFM(model.Params{
		model.InitStdDev: 0.01,
		model.NFactors:   8,
		model.NEpochs:    10,
		model.Lr:         0.01,
		model.Reg:        0.0001,
	})
	fitConfig := newFitConfigWithTestTracker(10)
	score := m.Fit(train, test, fitConfig)
	assert.InDelta(t, 0.77, score.Accuracy, classificationDelta)

	// test increment test
	buf := bytes.NewBuffer(nil)
	err = MarshalModel(buf, m)
	assert.NoError(t, err)
	tmp, err := UnmarshalModel(buf)
	assert.NoError(t, err)
	m = tmp.(*DeepFM)
	m.nEpochs = 1
	scoreInc := m.Fit(train, test, fitConfig)
	assert.InDelta(t, 0.77, scoreInc.RMSE, regressionDelta)

	// test clear
	assert.False(t, m.Invalid())
	m.Clear()
	assert.True(t, m.Invalid())
}
