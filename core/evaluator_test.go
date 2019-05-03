package core

import (
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"math"
	"testing"
)

const evalEpsilon = 0.00001

func EqualEpsilon(t *testing.T, expect float64, actual float64, epsilon float64) {
	if math.Abs(expect-actual) > evalEpsilon {
		t.Fatalf("Expect %f ± %f, Actual: %f\n", expect, epsilon, actual)
	}
}

type EvaluatorTesterModel struct {
	MaxUserId int
	MaxItemId int
	Matrix    [][]float64
}

func NewEvaluatorTesterModel(users, items []int, ratings []float64) *EvaluatorTesterModel {
	test := new(EvaluatorTesterModel)
	if len(users) == 0 {
		test.MaxUserId = -1
	} else {
		test.MaxUserId = base.Max(users)
	}
	if len(items) == 0 {
		test.MaxItemId = -1
	} else {
		test.MaxItemId = base.Max(items)
	}
	test.Matrix = base.NewMatrix(test.MaxUserId+1, test.MaxItemId+1)
	for i := range ratings {
		userId := users[i]
		itemId := items[i]
		rating := ratings[i]
		test.Matrix[userId][itemId] = rating
	}
	return test
}

func (tester *EvaluatorTesterModel) Predict(userId, itemId int) float64 {
	if userId > tester.MaxUserId {
		return 0
	} else if itemId > tester.MaxItemId {
		return 0
	} else {
		return tester.Matrix[userId][itemId]
	}
}

func (tester *EvaluatorTesterModel) GetParams() base.Params {
	panic("EvaluatorTesterModel.GetParams() should never be called.")
}

func (tester *EvaluatorTesterModel) SetParams(params base.Params) {
	panic("EvaluatorTesterModel.SetParams() should never be called.")
}

func (tester *EvaluatorTesterModel) Fit(set DataSetInterface, options *base.RuntimeOptions) {
	panic("EvaluatorTesterModel.Fit() should never be called.")
}

func NewTestIndexer(id []int) *base.Indexer {
	indexer := base.NewIndexer()
	for _, v := range id {
		indexer.Add(v)
	}
	return indexer
}

func TestRMSE(t *testing.T) {
	a := []float64{0, 0, 0}
	b := []float64{-2.0, 0, 2.0}
	if math.Abs(RMSE(a, b)-1.63299) > evalEpsilon {
		t.Fail()
	}
}

func TestMAE(t *testing.T) {
	a := []float64{0, 0, 0}
	b := []float64{-2.0, 0, 2.0}
	if math.Abs(MAE(a, b)-1.33333) > evalEpsilon {
		t.Fail()
	}
}

func NewTestTargetSet(ids []int) *base.MarginalSubSet {
	indexer := NewTestIndexer(ids)
	values := make([]float64, len(ids))
	subset := make([]int, len(ids))
	for i := range subset {
		subset[i] = i
	}
	return base.NewMarginalSubSet(indexer, subset, values, subset)
}

func TestNDCG(t *testing.T) {
	targetSet := NewTestTargetSet([]int{1, 3, 5, 7})
	rankList := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	EqualEpsilon(t, NDCG(targetSet, rankList), 0.6766372989, evalEpsilon)
}

func TestPrecision(t *testing.T) {
	targetSet := NewTestTargetSet([]int{1, 3, 5, 7})
	rankList := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	EqualEpsilon(t, Precision(targetSet, rankList), 0.4, evalEpsilon)
}

func TestRecall(t *testing.T) {
	targetSet := NewTestTargetSet([]int{1, 3, 15, 17, 19})
	rankList := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	EqualEpsilon(t, Recall(targetSet, rankList), 0.4, evalEpsilon)
}

func TestAP(t *testing.T) {
	targetSet := NewTestTargetSet([]int{1, 3, 7, 9})
	rankList := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	EqualEpsilon(t, MAP(targetSet, rankList), 0.44375, evalEpsilon)
}

func TestRR(t *testing.T) {
	targetSet := NewTestTargetSet([]int{3})
	rankList := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	EqualEpsilon(t, MRR(targetSet, rankList), 0.25, evalEpsilon)
}

func TestAUC(t *testing.T) {
	// The mocked test dataset:
	// 1.0 0.0 0.0
	// 0.0 0.5 0.0
	// 0.0 0.0 1.0
	a := NewEvaluatorTesterModel([]int{0, 0, 0, 1, 1, 1, 2, 2, 2},
		[]int{0, 1, 2, 0, 1, 2, 0, 1, 2},
		[]float64{1.0, 0.0, 0.0, 0.0, 0.5, 0.0, 0.0, 0.0, 1.0})
	b := NewDataSet([]int{0, 1, 2}, []int{0, 1, 2}, []float64{1.0, 0.5, 1.0})
	assert.Equal(t, 1.0, EvaluateAUC(a, b, nil))
}
