package core

import (
	"github.com/gonum/stat"
	"math"
	"testing"
)

const EPSILON float64 = 0.05

func TestRandom(t *testing.T) {
	const expectRMSE = 1.514
	const expectMAE = 1.215
	// Cross validation
	random := NewRandom()
	data := LoadDataFromBuiltIn()
	results := CrossValidate(random, data, []Metrics{RootMeanSquareError, MeanAbsoluteError}, 5, 0)
	// Check RMSE
	rmse := stat.Mean(results[0], nil)
	if math.Abs(rmse-expectRMSE) > EPSILON {
		t.Fatalf("RMSE(%.3f) not in %.3f±%.3f", rmse, expectRMSE, EPSILON)
	}
	// Check MAE
	mae := stat.Mean(results[1], nil)
	if math.Abs(mae-expectMAE) > EPSILON {
		t.Fatalf("MAE(%.3f) not in %.3f±%.3f", mae, expectMAE, EPSILON)
	}
}
