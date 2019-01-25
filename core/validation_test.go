package core

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

func (model *CVTestModel) Predict(userId, itemId int) float64 {
	panic("Predict() not implemented")
}

func (model *CVTestModel) Fit(trainSet *DataSet, setters ...RuntimeOption) {}

func CVTestEvaluator(estimator Model, testSet *DataSet, excludeSet *DataSet) float64 {
	params := estimator.GetParams()
	a := params.GetFloat64(base.Lr, 0)
	b := params.GetFloat64(base.Reg, 0)
	c := params.GetFloat64(base.Alpha, 0)
	d := params.GetFloat64(base.InitMean, 0)
	return a + b + c + d
}

func TestCrossValidate(t *testing.T) {
	model := new(CVTestModel)
	model.SetParams(base.Params{
		base.Lr:    3,
		base.Reg:   5,
		base.Alpha: 7,
	})
	out := CrossValidate(model, nil, []Evaluator{CVTestEvaluator}, NewKFoldSplitter(5), 0)
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
	out := GridSearchCV(model, nil, paramGrid, []Evaluator{CVTestEvaluator}, NewKFoldSplitter(5), 0)
	// Check best parameters
	assert.Equal(t, 16.0, out[0].BestScore)
	assert.Equal(t, 26, out[0].BestIndex)
	assert.Equal(t, base.Params{base.Lr: 2, base.Reg: 3, base.Alpha: 1}, out[0].BestParams)
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
	out := RandomSearchCV(model, nil, paramGrid, []Evaluator{CVTestEvaluator}, NewKFoldSplitter(5), 100, 0)
	// Check best parameters
	assert.Equal(t, 16.0, out[0].BestScore)
	assert.Equal(t, base.Params{base.Lr: 2, base.Reg: 3, base.Alpha: 1}, out[0].BestParams)
}

func TestCrossValidateResult_MeanAndMargin(t *testing.T) {
	out := CrossValidateResult{TestScore: []float64{1, 2, 3, 4, 5}}
	mean, margin := out.MeanAndMargin()
	assert.Equal(t, 3.0, mean)
	assert.Equal(t, 2.0, margin)
}
