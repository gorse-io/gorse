// Copyright 2020 Zhenghao Zhang
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
package model

import (
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"gonum.org/v1/gonum/stat"
	"testing"
)

type CVTestModel struct {
	Params base.Params
}

func (model *CVTestModel) SetParams(params base.Params) {
	model.Params = params
}

func (model *CVTestModel) GetParams() base.Params {
	return model.Params
}

func (model *CVTestModel) Predict(userId, itemId string) float64 {
	panic("Predict() not implemented")
}

func (model *CVTestModel) Fit(trainSet DataSetInterface, setters *base.RuntimeOptions) {}

func CVTestEvaluator(estimator ModelInterface, testSet, excludeSet DataSetInterface) []float64 {
	params := estimator.GetParams()
	a := params.GetFloat64(base.Lr, 0)
	b := params.GetFloat64(base.Reg, 0)
	c := params.GetFloat64(base.Alpha, 0)
	d := params.GetFloat64(base.InitMean, 0)
	return []float64{a + b + c + d}
}

func TestCrossValidate(t *testing.T) {
	model := new(CVTestModel)
	model.SetParams(base.Params{
		base.Lr:    3,
		base.Reg:   5,
		base.Alpha: 7,
	})
	out := CrossValidate(model, nil, NewUserLOOSplitter(5), 0, nil, CVTestEvaluator)
	assert.Equal(t, 15.0, stat.Mean(out[0].TestScore, nil))
}

func TestGridSearchCV(t *testing.T) {
	// Grid search
	paramGrid := ParameterGrid{
		base.Lr:    {6, 4, 2},
		base.Reg:   {7, 5, 3},
		base.Alpha: {3, 2, 1},
	}
	model := new(CVTestModel)
	model.SetParams(base.Params{base.InitMean: 10})
	out := GridSearchCV(model, nil, paramGrid, NewUserLOOSplitter(5), 0, nil, CVTestEvaluator)
	// Check best parameters
	assert.Equal(t, 26.0, out[0].BestScore)
	assert.Equal(t, 0, out[0].BestIndex)
	assert.Equal(t, base.Params{base.Lr: 6, base.Reg: 7, base.Alpha: 3}, out[0].BestParams)
}

func TestRandomSearchCV(t *testing.T) {
	// Grid search
	paramGrid := ParameterGrid{
		base.Lr:    {6, 4, 2},
		base.Reg:   {7, 5, 3},
		base.Alpha: {3, 2, 1},
	}
	model := new(CVTestModel)
	model.SetParams(base.Params{base.InitMean: 10})
	out := RandomSearchCV(model, nil, paramGrid, NewUserLOOSplitter(5), 100, 0, nil, CVTestEvaluator)
	// Check best parameters
	assert.Equal(t, 26.0, out[0].BestScore)
	assert.Equal(t, base.Params{base.Lr: 6, base.Reg: 7, base.Alpha: 3}, out[0].BestParams)
}

func TestCrossValidateResult_MeanAndMargin(t *testing.T) {
	out := CrossValidateResult{TestScore: []float64{1, 2, 3, 4, 5}}
	mean, margin := out.MeanAndMargin()
	assert.Equal(t, 3.0, mean)
	assert.Equal(t, 2.0, margin)
}
