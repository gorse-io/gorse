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

func checkRank(t *testing.T, model ModelInterface, dataSet DataSetInterface, splitter Splitter, evalNames []string,
	expectations []float64, evaluators ...Evaluator) {
	// Cross validation
	results := CrossValidate(model, dataSet, splitter, 0, &RuntimeOptions{CVJobs: runtime.NumCPU()}, evaluators...)
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

// LibRec Benchmarks: https://www.librec.net/release/v1.3/example.html

func TestItemPop(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	checkRank(t, NewItemPop(nil), data, NewKFoldSplitter(5),
		[]string{"Prec@5", "Recall@5", "Prec@10", "Recall@10", "MAP", "NDCG", "MRR"},
		[]float64{0.211, 0.070, 0.190, 0.116, 0.135, 0.477, 0.417},
		NewEvaluator(5, 0, Precision, Recall),
		NewEvaluator(10, 0, Precision, Recall),
		NewEvaluator(math.MaxInt32, 0, MAP, NDCG, MRR))
}

func TestBPR(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	checkRank(t, NewBPR(Params{
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
		NewEvaluator(5, 0, Precision, Recall),
		NewEvaluator(10, 0, Precision, Recall),
		NewEvaluator(math.MaxInt32, 0, MAP, NDCG))
}

func TestALS(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	checkRank(t, NewALS(Params{
		NFactors: 20,
		Reg:      0.015,
		Alpha:    1.0,
		NEpochs:  10,
	}), data, NewKFoldSplitter(5),
		[]string{"Prec@5", "Recall@5", "Prec@10", "Recall@10", "MAP", "NDCG"},
		[]float64{0.416, 0.142, 0.353, 0.227, 0.287, 0.624},
		NewEvaluator(5, 0, Precision, Recall),
		NewEvaluator(10, 0, Precision, Recall),
		NewEvaluator(math.MaxInt32, 0, MAP, NDCG))
}

func TestKNN(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	// KNN should perform better than ItemPop.
	checkRank(t, NewKNN(nil), data, NewKFoldSplitter(5),
		[]string{"Prec@5", "Recall@5", "Prec@10", "Recall@10", "MAP", "NDCG", "MRR"},
		[]float64{0.211, 0.070, 0.190, 0.116, 0.135, 0.477, 0.417},
		NewEvaluator(5, 0, Precision, Recall),
		NewEvaluator(10, 0, Precision, Recall),
		NewEvaluator(math.MaxInt32, 0, MAP, NDCG, MRR))
}

func TestFM_BPR(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	checkRank(t, NewFM(Params{
		Optimizer:  BPROptimizer,
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
		NewEvaluator(5, 0, Precision, Recall),
		NewEvaluator(10, 0, Precision, Recall),
		NewEvaluator(math.MaxInt32, 0, MAP, NDCG))
}
