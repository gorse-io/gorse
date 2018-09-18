package core

import (
	"math"
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
		[]Evaluator{RMSE, MAE}, 5, 0)
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

func TestRMSE(t *testing.T) {
	a := []float64{-2.0, 0, 2.0}
	b := []float64{0, 0, 0}
	if math.Abs(RMSE(a, b)-1.63299) > 0.00001 {
		t.Fail()
	}
}

func TestMAE(t *testing.T) {
	a := []float64{-2.0, 0, 2.0}
	b := []float64{0, 0, 0}
	if math.Abs(MAE(a, b)-1.33333) > 0.00001 {
		t.Fail()
	}
}
