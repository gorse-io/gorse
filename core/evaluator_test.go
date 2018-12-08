package core

import (
	. "github.com/zhenghaoz/gorse/base"
	"math"
	"testing"
)

type TestEstimator struct {
	MaxUserId int
	MaxItemId int
	Matrix    [][]float64
}

func NewTestEstimator(users, items []int, ratings []float64) *TestEstimator {
	test := new(TestEstimator)
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

func (tester *TestEstimator) Predict(userId, itemId int) float64 {
	if userId > tester.MaxUserId {
		return 0
	} else if itemId > tester.MaxItemId {
		return 0
	} else {
		return tester.Matrix[userId][itemId]
	}
}

func (tester *TestEstimator) GetParams() Params {
	panic("TestEstimator.GetParams() should never be called.")
}

func (tester *TestEstimator) SetParams(params Params) {
	panic("TestEstimator.SetParams() should never be called.")
}

func (tester *TestEstimator) Fit(set TrainSet, options ...RuntimeOption) {
	panic("TestEstimator.Fit() should never be called.")
}

func TestRMSE(t *testing.T) {
	a := NewTestEstimator(nil, nil, nil)
	b := NewRawDataSet([]int{0, 1, 2}, []int{0, 1, 2}, []float64{-2.0, 0, 2.0})
	if math.Abs(RMSE(a, b)-1.63299) > 0.00001 {
		t.Fail()
	}
}

func TestMAE(t *testing.T) {
	a := NewTestEstimator(nil, nil, nil)
	b := NewRawDataSet([]int{0, 1, 2}, []int{0, 1, 2}, []float64{-2.0, 0, 2.0})
	if math.Abs(MAE(a, b)-1.33333) > 0.00001 {
		t.Fail()
	}
}

func TestAUC(t *testing.T) {
	// 1.0 0.0 0.0
	// 0.0 0.5 0.0
	// 0.0 0.0 1.0
	a := NewTestEstimator([]int{0, 0, 0, 1, 1, 1, 2, 2, 2},
		[]int{0, 1, 2, 0, 1, 2, 0, 1, 2},
		[]float64{1.0, 0.0, 0.0, 0.0, 0.5, 0.0, 0.0, 0.0, 1.0})
	b := NewRawDataSet([]int{0, 1, 2}, []int{0, 1, 2}, []float64{1.0, 0.5, 1.0})
	c := NewAUCEvaluator(b)
	if c(a, b) != 1.0 {
		t.Fail()
	}
}
