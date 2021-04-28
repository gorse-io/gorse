package pr

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestKNN_MovieLens(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("ml-1m")
	assert.Nil(t, err)
	m := NewKNN(nil)
	score := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.311, score.NDCG, benchEpsilon)
	assert.Equal(t, m.Predict([]string{"1"}, "1"), m.InternalPredict([]int{1}, 1))
}

func TestKNN_Pinterest(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("pinterest-20")
	assert.Nil(t, err)
	m := NewKNN(nil)
	score := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.570, score.NDCG, benchEpsilon)
}
