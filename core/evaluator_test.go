package core

import (
	"github.com/stretchr/testify/assert"
	. "github.com/zhenghaoz/gorse/base"
	"math"
	"testing"
)

type EvaluatorTesterModel struct {
	MaxUserId int
	MaxItemId int
	Matrix    [][]float64
}

func NewEvaluatorTesterModel(users, items []int, ratings []float64) *EvaluatorTesterModel {
	test := new(EvaluatorTesterModel)
	test.MaxUserId = Max(users)
	test.MaxItemId = Max(items)
	test.Matrix = MakeMatrix(test.MaxUserId+1, test.MaxItemId+1)
	for i := range ratings {
		userId := users[i]
		itemId := items[i]
		rating := ratings[i]
		test.Matrix[userId][itemId] = rating
	}
	return test
}

func (tester *EvaluatorTesterModel) Predict(userId, itemId int) float64 {
	if userId > tester.MaxUserId {
		return 0
	} else if itemId > tester.MaxItemId {
		return 0
	} else {
		return tester.Matrix[userId][itemId]
	}
}

func (tester *EvaluatorTesterModel) GetParams() Params {
	panic("EvaluatorTesterModel.GetParams() should never be called.")
}

func (tester *EvaluatorTesterModel) SetParams(params Params) {
	panic("EvaluatorTesterModel.SetParams() should never be called.")
}

func (tester *EvaluatorTesterModel) Fit(set DataSet, options ...FitOption) {
	panic("EvaluatorTesterModel.Fit() should never be called.")
}

func TestRMSE(t *testing.T) {
	a := NewEvaluatorTesterModel(nil, nil, nil)
	b := NewDataSet(NewDataTable([]int{0, 1, 2}, []int{0, 1, 2}, []float64{-2.0, 0, 2.0}))
	if math.Abs(RMSE(a, b, b)-1.63299) > 0.00001 {
		t.Fail()
	}
}

func TestMAE(t *testing.T) {
	a := NewEvaluatorTesterModel(nil, nil, nil)
	b := NewDataSet(NewDataTable([]int{0, 1, 2}, []int{0, 1, 2}, []float64{-2.0, 0, 2.0}))
	if math.Abs(MAE(a, b, b)-1.33333) > 0.00001 {
		t.Fail()
	}
}

func TestAUC(t *testing.T) {
	// 1.0 0.0 0.0
	// 0.0 0.5 0.0
	// 0.0 0.0 1.0
	a := NewEvaluatorTesterModel([]int{0, 0, 0, 1, 1, 1, 2, 2, 2},
		[]int{0, 1, 2, 0, 1, 2, 0, 1, 2},
		[]float64{1.0, 0.0, 0.0, 0.0, 0.5, 0.0, 0.0, 0.0, 1.0})
	b := NewDataSet(NewDataTable([]int{0, 1, 2}, []int{0, 1, 2}, []float64{1.0, 0.5, 1.0}))
	assert.Equal(t, 1.0, AUC(a, b, b))
}
