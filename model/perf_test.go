package model

import (
	. "github.com/zhenghaoz/gorse/base"
	. "github.com/zhenghaoz/gorse/core"
	"gonum.org/v1/gonum/stat"
	"math"
	"testing"
)

const perfEpsilon float64 = 0.005

func EvaluateRegression(t *testing.T, algo Model, dataSet Table, splitter Splitter, evalNames []string,
	evaluators []Evaluator, expectations []float64) {
	// Cross validation
	results := CrossValidate(algo, dataSet, evaluators, splitter)
	// Check accuracy
	for i := range evalNames {
		accuracy := stat.Mean(results[i].TestScore, nil)
		if accuracy > expectations[i]+perfEpsilon {
			t.Fatalf("%s: %.3f > %.3f+%.3f", evalNames[i], accuracy, expectations[i], perfEpsilon)
		} else {
			t.Logf("%s: %.3f = %.3f%+.3f", evalNames[i], accuracy, expectations[i], accuracy-expectations[i])
		}
	}
}

func EvaluateRank(t *testing.T, algo Model, dataSet Table, splitter Splitter, evalNames []string,
	evaluators []Evaluator, expectations []float64) {
	// Cross validation
	results := CrossValidate(algo, dataSet, evaluators, splitter)
	// Check accuracy
	for i := range evalNames {
		accuracy := stat.Mean(results[i].TestScore, nil)
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

func TestSVDpp(t *testing.T) {
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
	EvaluateRegression(t, NewKNN(Params{Type: Basic}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.98, 0.774})
}

func TestKNNWithMean(t *testing.T) {
	EvaluateRegression(t, NewKNN(Params{Type: Centered}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.951, 0.749})
}

func TestNewKNNZScore(t *testing.T) {
	EvaluateRegression(t, NewKNN(Params{Type: ZScore}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.951, 0.746})
}

func TestKNNBaseLine(t *testing.T) {
	EvaluateRegression(t, NewKNN(Params{Type: Baseline}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.931, 0.733})
}

func TestCoClustering(t *testing.T) {
	EvaluateRegression(t, NewCoClustering(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.963, 0.753})
}

// LibRec Benchmarks: https://www.librec.net/release/v1.3/example.html

func TestKNN_UserBased_LibRec(t *testing.T) {
	EvaluateRegression(t, NewKNN(Params{
		Type:       Centered,
		Similarity: Pearson,
		UserBased:  true,
		Shrinkage:  25,
		K:          60,
	}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.944, 0.737})
}

func TestKNN_ItemBased_LibRec(t *testing.T) {
	EvaluateRegression(t, NewKNN(Params{
		Type:       Centered,
		Similarity: Pearson,
		UserBased:  false,
		Shrinkage:  2500,
		K:          40,
	}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.924, 0.723})
}

func TestSlopeOne_LibRec(t *testing.T) {
	EvaluateRegression(t, NewSlopOne(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.940, 0.739})
}

func TestSVD_LibRec(t *testing.T) {
	EvaluateRegression(t, NewSVD(Params{
		Lr:       0.007,
		NEpochs:  100,
		NFactors: 80,
		Reg:      0.1,
	}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.911, 0.718})
}

func TestNMF_LibRec(t *testing.T) {
	EvaluateRegression(t, NewNMF(nil), LoadDataFromBuiltIn("filmtrust"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.859, 0.643})
}

func TestSVDpp_LibRec(t *testing.T) {
	// factors=20, reg=0.1, learn.rate=0.01, max.iter=100
	EvaluateRegression(t, NewSVDpp(Params{
		Lr:         0.01,
		NEpochs:    100,
		NFactors:   20,
		Reg:        0.1,
		InitMean:   0,
		InitStdDev: 0.001,
	}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []Evaluator{RMSE, MAE}, []float64{0.911, 0.718})
}

func TestItemPop(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	EvaluateRank(t, NewItemPop(nil), data, NewKFoldSplitter(5),
		[]string{"Prec@5", "Prec@10", "Recall@5", "Recall@10", "MAP", "NDCG", "MRR"},
		[]Evaluator{
			NewPrecision(5),
			NewPrecision(10),
			NewRecall(5),
			NewRecall(10),
			NewMAP(math.MaxInt32),
			NewNDCG(math.MaxInt32),
			NewMRR(math.MaxInt32),
		},
		[]float64{0.211, 0.190, 0.070, 0.116, 0.135, 0.477, 0.417})
}

func TestItemPop2(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	EvaluateRank(t, NewItemPop(nil), data, NewKFoldSplitter(5),
		[]string{"AUC"},
		[]Evaluator{NewAUCEvaluator()},
		[]float64{0.857})
}

func TestSVD_BPR(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	EvaluateRank(t, NewSVD(Params{
		Target:   BPR,
		NFactors: 10,
		Reg:      0.01,
		Lr:       0.05,
		NEpochs:  30,
	}), data, NewKFoldSplitter(5),
		[]string{"Prec@5", "Prec@10"}, []Evaluator{NewPrecision(5), NewPrecision(10)}, []float64{-0.378, 0.321})
}

func TestWRMF(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	EvaluateRank(t, NewWRMF(Params{
		NFactors: 20,
		Reg:      0.015,
		Alpha:    1.0,
		NEpochs:  10,
	}), data, NewKFoldSplitter(5),
		[]string{"Prec@5"}, []Evaluator{NewPrecision(5)}, []float64{0.424})
}
