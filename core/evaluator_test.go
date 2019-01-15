package core

import (
	"github.com/stretchr/testify/assert"
	. "github.com/zhenghaoz/gorse/base"
	"math"
	"testing"
)

const evalEpsilon = 0.00001

type EvaluatorTesterModel struct {
	MaxUserId int
	MaxItemId int
	Matrix    [][]float64
}

func NewEvaluatorTesterModel(users, items []int, ratings []float64) *EvaluatorTesterModel {
	test := new(EvaluatorTesterModel)
	if len(users) == 0 {
		test.MaxUserId = -1
	} else {
		test.MaxUserId = Max(users)
	}
	if len(items) == 0 {
		test.MaxItemId = -1
	} else {
		test.MaxItemId = Max(items)
	}
	test.Matrix = NewMatrix(test.MaxUserId+1, test.MaxItemId+1)
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
	// The mocked test dataset:
	// -2.0 NaN NaN
	//  NaN 0.0 NaN
	//  NaN NaN 2.0
	a := NewEvaluatorTesterModel(nil, nil, nil)
	b := NewDataSet(NewDataTable([]int{0, 1, 2}, []int{0, 1, 2}, []float64{-2.0, 0, 2.0}))
	if math.Abs(RMSE(a, b)-1.63299) > evalEpsilon {
		t.Fail()
	}
}

func TestMAE(t *testing.T) {
	// The mocked test dataset:
	// -2.0 NaN NaN
	//  NaN 0.0 NaN
	//  NaN NaN 2.0
	a := NewEvaluatorTesterModel(nil, nil, nil)
	b := NewDataSet(NewDataTable([]int{0, 1, 2}, []int{0, 1, 2}, []float64{-2.0, 0, 2.0}))
	if math.Abs(MAE(a, b)-1.33333) > evalEpsilon {
		t.Fail()
	}
}

func TestAUC(t *testing.T) {
	// The mocked test dataset:
	// 1.0 0.0 0.0
	// 0.0 0.5 0.0
	// 0.0 0.0 1.0
	a := NewEvaluatorTesterModel([]int{0, 0, 0, 1, 1, 1, 2, 2, 2},
		[]int{0, 1, 2, 0, 1, 2, 0, 1, 2},
		[]float64{1.0, 0.0, 0.0, 0.0, 0.5, 0.0, 0.0, 0.0, 1.0})
	b := NewDataSet(NewDataTable([]int{0, 1, 2}, []int{0, 1, 2}, []float64{1.0, 0.5, 1.0}))
	assert.Equal(t, 1.0, AUC(a, b))
}

func TestNDCG(t *testing.T) {

}

func TestPrecision(t *testing.T) {

}

func TestRecall(t *testing.T) {

}

func TestMAP(t *testing.T) {

}

func TestMRR(t *testing.T) {
	a := NewEvaluatorTesterModel(
		[]int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		[]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		[]float64{10, 9, 8, 7, 6, 5, 4, 3, 2, 1})
	b := NewDataSet(NewDataTable(
		[]int{1, 1, 0, 0, 0},
		[]int{0, 2, 4, 6, 8},
		[]float64{1, 1, 1, 1, 1}))
	//if math.Abs(MAE(a, b)-1.33333) > evalEpsilon {
	//	//	t.Fail()
	//	//}
	mrr := NewMRR(10)
	t.Log(mrr(a, b))
}
