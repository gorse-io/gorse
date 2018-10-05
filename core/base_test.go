package core

import (
	"gonum.org/v1/gonum/stat"
	"runtime"
	"testing"
)

const estimatorEpsilon float64 = 0.008

func Evaluate(t *testing.T, algo Estimator, dataSet DataSet,
	expectRMSE float64, expectMAE float64) {
	// Cross validation
	results := CrossValidate(algo, dataSet, []Evaluator{RMSE, MAE}, NewKFoldSplitter(5), 0, Parameters{
		"randState": 0,
	}, runtime.NumCPU())
	// Check RMSE
	rmse := stat.Mean(results[0].Tests, nil)
	if rmse > expectRMSE+estimatorEpsilon {
		t.Fatalf("RMSE(%.3f) > %.3f+%.3f", rmse, expectRMSE, estimatorEpsilon)
	}
	// Check MAE
	mae := stat.Mean(results[1].Tests, nil)
	if mae > expectMAE+estimatorEpsilon {
		t.Fatalf("MAE(%.3f) > %.3f+%.3f", mae, expectMAE, estimatorEpsilon)
	}
}

func TestRandom(t *testing.T) {
	Evaluate(t, NewRandom(nil), LoadDataFromBuiltIn("ml-100k"), 1.514, 1.215)
}

func TestBaseLine(t *testing.T) {
	Evaluate(t, NewBaseLine(nil), LoadDataFromBuiltIn("ml-100k"), 0.944, 0.748)
}

func TestSVD(t *testing.T) {
	Evaluate(t, NewSVD(nil), LoadDataFromBuiltIn("ml-100k"), 0.934, 0.737)
}

//func TestSVDPP(t *testing.T) {
//	Evaluate(t, NewSVDpp(), LoadDataFromBuiltIn(), 0.92, 0.722)
//}

func TestNMF(t *testing.T) {
	Evaluate(t, NewNMF(nil), LoadDataFromBuiltIn("ml-100k"), 0.963, 0.758)
}

func TestSlopeOne(t *testing.T) {
	Evaluate(t, NewSlopOne(nil), LoadDataFromBuiltIn("ml-100k"), 0.946, 0.743)
}

func TestKNN(t *testing.T) {
	Evaluate(t, NewKNN(nil), LoadDataFromBuiltIn("ml-100k"), 0.98, 0.774)
}

func TestKNNWithMean(t *testing.T) {
	Evaluate(t, NewKNNWithMean(nil), LoadDataFromBuiltIn("ml-100k"), 0.951, 0.749)
}

func TestNewKNNZScore(t *testing.T) {
	Evaluate(t, NewKNNWithZScore(nil), LoadDataFromBuiltIn("ml-100k"), 0.951, 0.746)
}

func TestKNNBaseLine(t *testing.T) {
	Evaluate(t, NewKNNBaseLine(nil), LoadDataFromBuiltIn("ml-100k"), 0.931, 0.733)
}

func TestCoClustering(t *testing.T) {
	Evaluate(t, NewCoClustering(nil), LoadDataFromBuiltIn("ml-100k"), 0.963, 0.753)
}
