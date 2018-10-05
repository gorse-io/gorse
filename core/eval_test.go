package core

import (
	"math"
	"runtime"
	"testing"
)

func TestGridSearchCV(t *testing.T) {
	// Grid search
	paramGrid := ParameterGrid{
		"nEpochs": {5, 10},
		"reg":     {0.4, 0.6},
		"lr":      {0.002, 0.005},
	}
	out := GridSearchCV(NewBaseLine(nil), LoadDataFromBuiltIn("ml-100k"), paramGrid,
		[]Evaluator{RMSE, MAE}, 5, 0, runtime.NumCPU())
	// Check best parameters
	bestParams := out[0].BestParams
	if bestParams.GetInt("nEpochs", -1) != 10 {
		t.Fail()
	} else if bestParams.GetFloat64("reg", -1) != 0.4 {
		t.Fail()
	} else if bestParams.GetFloat64("lr", -1) != 0.005 {
		t.Fail()
	}
}

type TestEstimator struct {
	Base
	MaxUserId int
	MaxItemId int
	Matrix    [][]float64
}

func NewTestEstimator(users, items []int, ratings []float64) *TestEstimator {
	test := new(TestEstimator)
	test.MaxUserId = max(users)
	test.MaxItemId = max(items)
	test.Matrix = newZeroMatrix(test.MaxUserId+1, test.MaxItemId+1)
	for i := range ratings {
		userId := users[i]
		itemId := items[i]
		rating := ratings[i]
		test.Matrix[userId][itemId] = rating
	}
	return test
}

func (test *TestEstimator) Predict(userId, itemId int) float64 {
	if userId > test.MaxUserId {
		return 0
	} else if itemId > test.MaxItemId {
		return 0
	} else {
		return test.Matrix[userId][itemId]
	}
}

func TestRMSE(t *testing.T) {
	a := NewTestEstimator(nil, nil, nil)
	b := NewRawSet([]int{0, 1, 2}, []int{0, 1, 2}, []float64{-2.0, 0, 2.0})
	if math.Abs(RMSE(a, b)-1.63299) > 0.00001 {
		t.Fail()
	}
}

func TestMAE(t *testing.T) {
	a := NewTestEstimator(nil, nil, nil)
	b := NewRawSet([]int{0, 1, 2}, []int{0, 1, 2}, []float64{-2.0, 0, 2.0})
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
	b := NewRawSet([]int{0, 1, 2}, []int{0, 1, 2}, []float64{1.0, 0.5, 1.0})
	c := NewAUC(b)
	if c(a, b) != 1.0 {
		t.Fail()
	}
}
