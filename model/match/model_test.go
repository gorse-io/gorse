package match

import (
	"github.com/chewxy/math32"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/model"
	"runtime"
	"testing"
)

const (
	epsilon = 0.008
)

var fitConfig = &FitConfig{
	Jobs:       runtime.NumCPU(),
	Verbose:    1,
	Candidates: 100,
	TopK:       10,
}

func assertEpsilon(t *testing.T, expect float32, actual float32) {
	if math32.Abs(expect-actual) > epsilon {
		t.Fatalf("|%v - %v| > %v", expect, actual, epsilon)
	}
}

// He, Xiangnan, et al. "Neural collaborative filtering." Proceedings
// of the 26th international conference on world wide web. 2017.

func TestBPR_MovieLens(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("ml-1m")
	assert.Nil(t, err)
	m := NewBPR(model.Params{
		model.NFactors:   8,
		model.Reg:        0.01,
		model.Lr:         0.05,
		model.NEpochs:    30,
		model.InitMean:   0,
		model.InitStdDev: 0.001,
	})
	score := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.36, score.NDCG)

	// test predict
	assert.Equal(t, m.Predict("1", "1"), m.InternalPredict(1, 1))

	// test increment test
	m.nEpochs = 0
	score = m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.36, score.NDCG)

	// test clear
	m.Clear()
	score = m.Fit(trainSet, testSet, fitConfig)
	assert.Less(t, score.NDCG, float32(0.2))
}

func TestBPR_Pinterest(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("pinterest-20")
	assert.Nil(t, err)
	m := NewBPR(model.Params{
		model.NFactors:   8,
		model.Reg:        0.005,
		model.Lr:         0.05,
		model.NEpochs:    50,
		model.InitMean:   0,
		model.InitStdDev: 0.001,
	})
	score := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.53, score.NDCG)
}

func TestALS_MovieLens(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("ml-1m")
	assert.Nil(t, err)
	m := NewALS(model.Params{
		model.NFactors: 8,
		model.Reg:      0.015,
		model.NEpochs:  10,
		model.Alpha:    0.05,
	})
	score := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.36, score.NDCG)

	// test predict
	assert.Equal(t, m.Predict("1", "1"), m.InternalPredict(1, 1))

	// test increment test
	m.nEpochs = 0
	score = m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.36, score.NDCG)

	// test clear
	m.Clear()
	score = m.Fit(trainSet, testSet, fitConfig)
	assert.Less(t, score.NDCG, float32(0.2))
}

func TestALS_Pinterest(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("pinterest-20")
	assert.Nil(t, err)
	m := NewALS(model.Params{
		model.NFactors:   8,
		model.Reg:        0.01,
		model.NEpochs:    10,
		model.InitStdDev: 0.01,
		model.Alpha:      0.001,
	})
	score := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.52, score.NDCG)
}

func TestCCD_MovieLens(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("ml-1m")
	assert.Nil(t, err)
	m := NewCCD(model.Params{
		model.NFactors: 8,
		model.Reg:      0.015,
		model.NEpochs:  30,
		model.Alpha:    0.05,
	})
	score := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.36, score.NDCG)

	// test predict
	assert.Equal(t, m.Predict("1", "1"), m.InternalPredict(1, 1))

	// test increment test
	m.nEpochs = 0
	score = m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.36, score.NDCG)

	// test clear
	m.Clear()
	score = m.Fit(trainSet, testSet, fitConfig)
	assert.Less(t, score.NDCG, float32(0.2))
}

func TestCCD_Pinterest(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("pinterest-20")
	assert.Nil(t, err)
	m := NewCCD(model.Params{
		model.NFactors:   8,
		model.Reg:        0.01,
		model.NEpochs:    20,
		model.InitStdDev: 0.01,
		model.Alpha:      0.001,
	})
	score := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.52, score.NDCG)
}
