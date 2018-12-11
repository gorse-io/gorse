package model

import (
	. "github.com/zhenghaoz/gorse/base"
	. "github.com/zhenghaoz/gorse/core"
	"gonum.org/v1/gonum/stat"
	"testing"
)

const perfEpsilon float64 = 0.006

func EvaluateRegression(t *testing.T, algo Model, dataSet DataSet, splitter Splitter, evalNames []string,
	evaluators []Evaluator, expectations []float64) {
	// Cross validation
	results := CrossValidate(algo, dataSet, evaluators, splitter)
	// Check accuracy
	for i := range evalNames {
		accuracy := stat.Mean(results[i].Tests, nil)
		if accuracy > expectations[i]+perfEpsilon {
			t.Fatalf("%s: %.3f > %.3f+%.3f", evalNames[i], accuracy, expectations[i], perfEpsilon)
		} else {
			t.Logf("%s: %.3f = %.3f%+.3f", evalNames[i], accuracy, expectations[i], accuracy-expectations[i])
		}
	}
}

func EvaluateRank(t *testing.T, algo Model, dataSet DataSet, splitter Splitter, evalNames []string,
	evaluators []Evaluator, expectations []float64) {
	// Cross validation
	results := CrossValidate(algo, dataSet, evaluators, splitter)
	// Check accuracy
	for i := range evalNames {
		accuracy := stat.Mean(results[i].Tests, nil)
		if accuracy < expectations[i]-perfEpsilon {
			t.Fatalf("%s: %.3f < %.3f-%.3f", evalNames[i], accuracy, expectations[i], perfEpsilon)
		} else {
			t.Logf("%s: %.3f = %.3f%+.3f", evalNames[i], accuracy, expectations[i], accuracy-expectations[i])
		}
	}
}

// Surprise Benchmark: https://github.com/NicolasHug/Surprise#benchmarks

func TestRandom(t *testing.T) {
	EvaluateRegression(t, NewRandom(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{1.514, 1.215})
}

func TestBaseLine(t *testing.T) {
	EvaluateRegression(t, NewBaseLine(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.944, 0.748})
}

func TestSVD(t *testing.T) {
	EvaluateRegression(t, NewSVD(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.934, 0.737})
}

func TestSVDPP(t *testing.T) {
	EvaluateRegression(t, NewSVDpp(nil), LoadDataFromBuiltIn("ml-100k"), NewRatioSplitter(1, 0.2),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.92, 0.722})
}

func TestNMF(t *testing.T) {
	EvaluateRegression(t, NewNMF(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.963, 0.758})
}

func TestSlopeOne(t *testing.T) {
	EvaluateRegression(t, NewSlopOne(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.946, 0.743})
}

func TestKNN(t *testing.T) {
	EvaluateRegression(t, NewKNN(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.98, 0.774})
}

func TestKNNWithMean(t *testing.T) {
	EvaluateRegression(t, NewKNN(Params{KNNType: Centered}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.951, 0.749})
}

func TestNewKNNZScore(t *testing.T) {
	EvaluateRegression(t, NewKNN(Params{KNNType: ZScore}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.951, 0.746})
}

func TestKNNBaseLine(t *testing.T) {
	EvaluateRegression(t, NewKNN(Params{KNNType: Baseline}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.931, 0.733})
}

func TestCoClustering(t *testing.T) {
	EvaluateRegression(t, NewCoClustering(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.963, 0.753})
}

// LibRec Benchmarks: https://www.librec.net/release/v1.3/example.html

func TestItemPop(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	EvaluateRank(t, NewItemPop(nil), data, NewUserLOOSplitter(5),
		[]string{"AUC"}, []Evaluator{NewAUCEvaluator(data)}, []float64{0.857})
}

func TestSVD_BPR(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	EvaluateRank(t, NewSVD(Params{
		Target:   BPR,
		NFactors: 10,
		Reg:      0.01,
		Lr:       0.05,
		NEpochs:  30,
	}), data, NewUserLOOSplitter(5),
		[]string{"AUC"}, []Evaluator{NewAUCEvaluator(data)}, []float64{0.933})
}
