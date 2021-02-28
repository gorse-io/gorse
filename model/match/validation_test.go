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
package match

import (
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"testing"
)

type mockMatrixFactorizationForSearch struct {
	model.BaseModel
}

func (m *mockMatrixFactorizationForSearch) GetUserIndex() base.Index {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForSearch) GetItemIndex() base.Index {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForSearch) Fit(trainSet *DataSet, validateSet *DataSet, config *FitConfig) Score {
	score := float32(0)
	score += m.Params.GetFloat32(model.NFactors, 0.0)
	score += m.Params.GetFloat32(model.NEpochs, 0.0)
	score += m.Params.GetFloat32(model.InitMean, 0.0)
	score += m.Params.GetFloat32(model.InitStdDev, 0.0)
	return Score{NDCG: score}
}

func (m *mockMatrixFactorizationForSearch) Predict(userId, itemId string) float32 {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForSearch) InternalPredict(userId, itemId int) float32 {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForSearch) Clear() {
	// do nothing
}

func (m *mockMatrixFactorizationForSearch) GetParamsGrid() model.ParamsGrid {
	return model.ParamsGrid{
		model.NFactors:   []interface{}{1, 2, 3, 4},
		model.InitMean:   []interface{}{4, 3, 2, 1},
		model.InitStdDev: []interface{}{4, 3, 2, 1},
	}
}

func TestGridSearchCV(t *testing.T) {
	m := &mockMatrixFactorizationForSearch{}
	r := GridSearchCV(m, nil, nil, m.GetParamsGrid(), 0, nil)
	assert.Equal(t, float32(12), r.BestScore.NDCG)
	assert.Equal(t, model.Params{
		model.NFactors:   4,
		model.InitMean:   4,
		model.InitStdDev: 4,
	}, r.BestParams)
}

func TestRandomSearchCV(t *testing.T) {
	m := &mockMatrixFactorizationForSearch{}
	r := RandomSearchCV(m, nil, nil, m.GetParamsGrid(), 100, 0, nil)
	assert.Equal(t, float32(12), r.BestScore.NDCG)
	assert.Equal(t, model.Params{
		model.NFactors:   4,
		model.InitMean:   4,
		model.InitStdDev: 4,
	}, r.BestParams)
}
