package model

import (
	. "github.com/zhenghaoz/gorse/base"
	. "github.com/zhenghaoz/gorse/core"
	"gonum.org/v1/gonum/stat"
	"math"
	"runtime"
	"testing"
)

const (
	ratingEpsilon  = 0.005
	rankingEpsilon = 0.008
)

func checkRegression(t *testing.T, model ModelInterface, dataSet DataSetInterface, splitter Splitter, evalNames []string,
	expectations []float64, evaluators ...CrossValidationEvaluator) {
	// Cross validation
	results := CrossValidate(model, dataSet, splitter, 0, &RuntimeOptions{Verbose: false, NJobs: runtime.NumCPU()}, evaluators...)
	// Check accuracy
	for i := range evalNames {
		accuracy := stat.Mean(results[i].TestScore, nil)
		if math.IsNaN(accuracy) {
			t.Fatalf("%s: NaN", evalNames[i])
		} else if accuracy > expectations[i]+ratingEpsilon {
			t.Fatalf("%s: %.3f > %.3f+%.3f", evalNames[i], accuracy, expectations[i], ratingEpsilon)
		} else {
			t.Logf("%s: %.3f = %.3f%+.3f", evalNames[i], accuracy, expectations[i], accuracy-expectations[i])
		}
	}
}

func checkRank(t *testing.T, model ModelInterface, dataSet DataSetInterface, splitter Splitter, evalNames []string,
	expectations []float64, evaluators ...CrossValidationEvaluator) {
	// Cross validation
	results := CrossValidate(model, dataSet, splitter, 0, &RuntimeOptions{Verbose: false, NJobs: runtime.NumCPU()}, evaluators...)
	// Check accuracy
	for i := range evalNames {
		accuracy := stat.Mean(results[i].TestScore, nil)
		if accuracy < expectations[i]-rankingEpsilon {
			t.Fatalf("%s: %.3f < %.3f-%.3f", evalNames[i], accuracy, expectations[i], ratingEpsilon)
		} else {
			t.Logf("%s: %.3f = %.3f%+.3f", evalNames[i], accuracy, expectations[i], accuracy-expectations[i])
		}
	}
}

// Surprise Benchmark: https://github.com/NicolasHug/Surprise#benchmarks

func TestBaseLine(t *testing.T) {
	checkRegression(t, NewBaseLine(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.944, 0.748}, NewRatingEvaluator(RMSE, MAE))
}

func TestSVD(t *testing.T) {
	checkRegression(t, NewSVD(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.934, 0.737}, NewRatingEvaluator(RMSE, MAE))
}

func TestNMF(t *testing.T) {
	checkRegression(t, NewNMF(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.963, 0.758}, NewRatingEvaluator(RMSE, MAE))
}

func TestSlopeOne(t *testing.T) {
	checkRegression(t, NewSlopOne(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.946, 0.743}, NewRatingEvaluator(RMSE, MAE))
}

func TestKNN(t *testing.T) {
	checkRegression(t, NewKNN(Params{Type: Basic}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.98, 0.774}, NewRatingEvaluator(RMSE, MAE))
}

func TestKNNWithMean(t *testing.T) {
	checkRegression(t, NewKNN(Params{Type: Centered}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.951, 0.749}, NewRatingEvaluator(RMSE, MAE))
}

func TestKNNZScore(t *testing.T) {
	checkRegression(t, NewKNN(Params{Type: ZScore}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.951, 0.746}, NewRatingEvaluator(RMSE, MAE))
}

func TestKNNBaseLine(t *testing.T) {
	checkRegression(t, NewKNN(Params{Type: Baseline}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.931, 0.733}, NewRatingEvaluator(RMSE, MAE))
}

func TestCoClustering(t *testing.T) {
	checkRegression(t, NewCoClustering(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.963, 0.753}, NewRatingEvaluator(RMSE, MAE))
}

// LibRec Benchmarks: https://www.librec.net/release/v1.3/example.html

func TestKNN_UserBased_LibRec(t *testing.T) {
	checkRegression(t, NewKNN(Params{
		Type:       Centered,
		Similarity: Pearson,
		UserBased:  true,
		Shrinkage:  25,
		K:          60,
	}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.944, 0.737}, NewRatingEvaluator(RMSE, MAE))
}

func TestKNN_ItemBased_LibRec(t *testing.T) {
	checkRegression(t, NewKNN(Params{
		Type:       Centered,
		Similarity: Pearson,
		UserBased:  false,
		Shrinkage:  2500,
		K:          40,
	}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.924, 0.723}, NewRatingEvaluator(RMSE, MAE))
}

func TestSlopeOne_LibRec(t *testing.T) {
	checkRegression(t, NewSlopOne(nil), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.940, 0.739}, NewRatingEvaluator(RMSE, MAE))
}

func TestSVD_LibRec(t *testing.T) {
	checkRegression(t, NewSVD(Params{
		Lr:       0.007,
		NEpochs:  100,
		NFactors: 80,
		Reg:      0.1,
	}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.911, 0.718}, NewRatingEvaluator(RMSE, MAE))
}

func TestNMF_LibRec(t *testing.T) {
	checkRegression(t, NewNMF(Params{
		NFactors: 10,
		NEpochs:  100,
		InitLow:  0,
		InitHigh: 0.01,
	}), LoadDataFromBuiltIn("filmtrust"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.859, 0.643}, NewRatingEvaluator(RMSE, MAE))
}

func TestSVDpp_LibRec(t *testing.T) {
	// factors=20, reg=0.1, learn.rate=0.01, max.iter=100
	checkRegression(t, NewSVDpp(Params{
		Lr:         0.01,
		NEpochs:    100,
		NFactors:   20,
		Reg:        0.1,
		InitMean:   0,
		InitStdDev: 0.001,
	}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.911, 0.718}, NewRatingEvaluator(RMSE, MAE))
}

func TestItemPop(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	checkRank(t, NewItemPop(nil), data, NewKFoldSplitter(5),
		[]string{"Prec@5", "Recall@5", "Prec@10", "Recall@10", "MAP", "NDCG", "MRR"},
		[]float64{0.211, 0.070, 0.190, 0.116, 0.135, 0.477, 0.417},
		NewRankEvaluator(5, Precision, Recall),
		NewRankEvaluator(10, Precision, Recall),
		NewRankEvaluator(math.MaxInt32, MAP, NDCG, MRR))
}

func TestSVD_BPR(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	checkRank(t, NewSVD(Params{
		Optimizer:  BPR,
		NFactors:   10,
		Reg:        0.01,
		Lr:         0.05,
		NEpochs:    100,
		InitMean:   0,
		InitStdDev: 0.001,
	}),
		data, NewKFoldSplitter(5),
		[]string{"Prec@5", "Recall@5", "Prec@10", "Recall@10", "MAP", "NDCG"},
		[]float64{0.378, 0.129, 0.321, 0.209, 0.260, 0.601},
		NewRankEvaluator(5, Precision, Recall),
		NewRankEvaluator(10, Precision, Recall),
		NewRankEvaluator(math.MaxInt32, MAP, NDCG))
}

func TestWRMF(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	checkRank(t, NewWRMF(Params{
		NFactors: 20,
		Reg:      0.015,
		Alpha:    1.0,
		NEpochs:  10,
	}), data, NewKFoldSplitter(5),
		[]string{"Prec@5", "Recall@5", "Prec@10", "Recall@10", "MAP", "NDCG"},
		[]float64{0.416, 0.142, 0.353, 0.227, 0.287, 0.624},
		NewRankEvaluator(5, Precision, Recall),
		NewRankEvaluator(10, Precision, Recall),
		NewRankEvaluator(math.MaxInt32, MAP, NDCG))
}

func TestFM_SVD(t *testing.T) {
	checkRegression(t, NewFM(Params{
		Lr:       0.007,
		NEpochs:  100,
		NFactors: 80,
		Reg:      0.1,
	}), LoadDataFromBuiltIn("ml-100k"), NewKFoldSplitter(5),
		[]string{"RMSE", "MAE"}, []float64{0.911, 0.718}, NewRatingEvaluator(RMSE, MAE))
}

func TestFM_BPR(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	checkRank(t, NewFM(Params{
		Optimizer:  BPR,
		NFactors:   10,
		Reg:        0.01,
		Lr:         0.05,
		NEpochs:    100,
		InitMean:   0,
		InitStdDev: 0.001,
	}),
		data, NewKFoldSplitter(5),
		[]string{"Prec@5", "Recall@5", "Prec@10", "Recall@10", "MAP", "NDCG"},
		[]float64{0.378, 0.129, 0.321, 0.209, 0.260, 0.601},
		NewRankEvaluator(5, Precision, Recall),
		NewRankEvaluator(10, Precision, Recall),
		NewRankEvaluator(math.MaxInt32, MAP, NDCG))
}
