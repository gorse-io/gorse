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
package click

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
)

type mockFactorizationMachineForSearch struct {
	model.BaseModel
}

func (m *mockFactorizationMachineForSearch) Invalid() bool {
	panic("implement me")
}

func (m *mockFactorizationMachineForSearch) GetUserIndex() base.Index {
	panic("don't call me")
}

func (m *mockFactorizationMachineForSearch) GetItemIndex() base.Index {
	panic("don't call me")
}

func (m *mockFactorizationMachineForSearch) Fit(trainSet, testSet *Dataset, config *FitConfig) Score {
	score := float32(0)
	score += m.Params.GetFloat32(model.NFactors, 0.0)
	score += m.Params.GetFloat32(model.NEpochs, 0.0)
	score += m.Params.GetFloat32(model.InitMean, 0.0)
	score += m.Params.GetFloat32(model.InitStdDev, 0.0)
	return Score{Task: FMClassification, Precision: score}
}

func (m *mockFactorizationMachineForSearch) Predict(userId, itemId string, labels []string) float32 {
	panic("don't call me")
}

func (m *mockFactorizationMachineForSearch) InternalPredict(x []int) float32 {
	panic("don't call me")
}

func (m *mockFactorizationMachineForSearch) Clear() {
	// do nothing
}

func (m *mockFactorizationMachineForSearch) GetParamsGrid() model.ParamsGrid {
	return model.ParamsGrid{
		model.NFactors:   []interface{}{1, 2, 3, 4},
		model.InitMean:   []interface{}{4, 3, 2, 1},
		model.InitStdDev: []interface{}{4, 4, 4, 4},
	}
}

func TestGridSearchCV(t *testing.T) {
	m := &mockFactorizationMachineForSearch{}
	r := GridSearchCV(m, nil, nil, m.GetParamsGrid(), 0, nil)
	assert.Equal(t, float32(12), r.BestScore.Precision)
	assert.Equal(t, model.Params{
		model.NFactors:   4,
		model.InitMean:   4,
		model.InitStdDev: 4,
	}, r.BestParams)
}

func TestRandomSearchCV(t *testing.T) {
	m := &mockFactorizationMachineForSearch{}
	r := RandomSearchCV(m, nil, nil, m.GetParamsGrid(), 63, 0, nil)
	assert.Equal(t, float32(12), r.BestScore.Precision)
	assert.Equal(t, model.Params{
		model.NFactors:   4,
		model.InitMean:   4,
		model.InitStdDev: 4,
	}, r.BestParams)
}
