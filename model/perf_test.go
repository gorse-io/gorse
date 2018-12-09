package model

import (
	. "github.com/zhenghaoz/gorse/base"
	. "github.com/zhenghaoz/gorse/core"
	"gonum.org/v1/gonum/stat"
	"testing"
)

const performanceEpsilon float64 = 0.006

func EvaluateKFold(t *testing.T, algo Model, dataSet DataSet,
	expectRMSE float64, expectMAE float64) {
	// Cross validation
	results := CrossValidate(algo, dataSet, []Evaluator{RMSE, MAE}, NewKFoldSplitter(5))
	// Check RMSE
	rmse := stat.Mean(results[0].Tests, nil)
	if rmse > expectRMSE+performanceEpsilon {
		t.Fatalf("RMSE(%.3f) > %.3f+%.3f", rmse, expectRMSE, performanceEpsilon)
	}
	// Check MAE
	mae := stat.Mean(results[1].Tests, nil)
	if mae > expectMAE+performanceEpsilon {
		t.Fatalf("MAE(%.3f) > %.3f+%.3f", mae, expectMAE, performanceEpsilon)
	}
}

func EvaluateRatio(t *testing.T, algo Model, dataSet DataSet,
	expectRMSE float64, expectMAE float64) {
	// Cross validation
	results := CrossValidate(algo, dataSet, []Evaluator{RMSE, MAE}, NewRatioSplitter(1, 0.2))
	// Check RMSE
	rmse := stat.Mean(results[0].Tests, nil)
	if rmse > expectRMSE+performanceEpsilon {
		t.Fatalf("RMSE(%.3f) > %.3f+%.3f", rmse, expectRMSE, performanceEpsilon)
	}
	// Check MAE
	mae := stat.Mean(results[1].Tests, nil)
	if mae > expectMAE+performanceEpsilon {
		t.Fatalf("MAE(%.3f) > %.3f+%.3f", mae, expectMAE, performanceEpsilon)
	}
}

// Surprise Benchmark: https://github.com/NicolasHug/Surprise#benchmarks

func TestRandom(t *testing.T) {
	EvaluateKFold(t, NewRandom(nil), LoadDataFromBuiltIn("ml-100k"), 1.514, 1.215)
}

func TestBaseLine(t *testing.T) {
	EvaluateKFold(t, NewBaseLine(nil), LoadDataFromBuiltIn("ml-100k"), 0.944, 0.748)
}

func TestSVD(t *testing.T) {
	EvaluateKFold(t, NewSVD(nil), LoadDataFromBuiltIn("ml-100k"), 0.934, 0.737)
}

func TestSVDPP(t *testing.T) {
	EvaluateRatio(t, NewSVDpp(nil), LoadDataFromBuiltIn("ml-100k"), 0.92, 0.722)
}

func TestNMF(t *testing.T) {
	EvaluateKFold(t, NewNMF(nil), LoadDataFromBuiltIn("ml-100k"), 0.963, 0.758)
}

func TestSlopeOne(t *testing.T) {
	EvaluateKFold(t, NewSlopOne(nil), LoadDataFromBuiltIn("ml-100k"), 0.946, 0.743)
}

func TestKNN(t *testing.T) {
	EvaluateKFold(t, NewKNN(nil), LoadDataFromBuiltIn("ml-100k"), 0.98, 0.774)
}

func TestKNNWithMean(t *testing.T) {
	EvaluateKFold(t, NewKNN(Params{KNNType: Centered}), LoadDataFromBuiltIn("ml-100k"), 0.951, 0.749)
}

func TestNewKNNZScore(t *testing.T) {
	EvaluateKFold(t, NewKNN(Params{KNNType: ZScore}), LoadDataFromBuiltIn("ml-100k"), 0.951, 0.746)
}

func TestKNNBaseLine(t *testing.T) {
	EvaluateKFold(t, NewKNN(Params{KNNType: Baseline}), LoadDataFromBuiltIn("ml-100k"), 0.931, 0.733)
}

func TestCoClustering(t *testing.T) {
	EvaluateKFold(t, NewCoClustering(nil), LoadDataFromBuiltIn("ml-100k"), 0.963, 0.753)
}

// LibRec Benchmarks: https://www.librec.net/release/v1.3/example.html
