package model

import (
	"github.com/chewxy/math32"
	"github.com/zhenghaoz/gorse/config"
	"testing"
)

const (
	epsilon = 0.008
)

var fitConfig = &config.FitConfig{
	Jobs:         1,
	Verbose:      10,
	Candidates:   100,
	TopK:         10,
	NumTestUsers: 0,
}

func assertEpsilon(t *testing.T, expect float32, actual float32) {
	if math32.Abs(expect-actual) > epsilon {
		t.Fatalf("|%v - %v| > %v", expect, actual, epsilon)
	}
}

// He, Xiangnan, et al. "Neural collaborative filtering." Proceedings
// of the 26th international conference on world wide web. 2017.

func TestBPR_MovieLens(t *testing.T) {
	trainSet, testSet := LoadDataFromBuiltIn("ml-1m")
	model := NewBPR(Params{
		NFactors:   8,
		Reg:        0.01,
		Lr:         0.05,
		NEpochs:    30,
		InitMean:   0,
		InitStdDev: 0.001,
	})
	score := model.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.36, score.NDCG)
}

func TestBPR_Pinterest(t *testing.T) {
	trainSet, testSet := LoadDataFromBuiltIn("pinterest-20")
	model := NewBPR(Params{
		NFactors:   8,
		Reg:        0.005,
		Lr:         0.05,
		NEpochs:    50,
		InitMean:   0,
		InitStdDev: 0.001,
	})
	score := model.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.53, score.NDCG)
}

func TestALS_MovieLens(t *testing.T) {
	trainSet, testSet := LoadDataFromBuiltIn("ml-1m")
	model := NewALS(Params{
		NFactors: 8,
		Reg:      0.015,
		NEpochs:  10,
		Weight:   0.05,
	})
	score := model.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.36, score.NDCG)
}

func TestALS_Pinterest(t *testing.T) {
	trainSet, testSet := LoadDataFromBuiltIn("pinterest-20")
	model := NewALS(Params{
		NFactors:   8,
		Reg:        0.01,
		NEpochs:    10,
		InitStdDev: 0.01,
		Weight:     0.001,
	})
	score := model.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.52, score.NDCG)
}

// He, Xiangnan, et al. "Nais: Neural attentive item similarity model
// for recommendation." IEEE Transactions on Knowledge and Data
// Engineering 30.12 (2018): 2354-2366.

func TestFISM_MovieLens(t *testing.T) {
	// TODO: not implemented
}

func TestFISM_Pinterest(t *testing.T) {
	// TODO: not implemented
}
