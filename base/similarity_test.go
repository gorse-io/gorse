package base

import (
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
)

const simTestEpsilon = 1e-3

func TestCosine(t *testing.T) {
	indexer := NewTestIndexer([]int{0, 1, 2, 3})
	a := NewMarginalSubSet(indexer, []int{1, 2, 3}, []float64{4, 5, 6}, []int{0, 1, 2})
	b := NewMarginalSubSet(indexer, []int{0, 1, 2}, []float64{0, 1, 2}, []int{0, 1, 2})
	sim := CosineSimilarity(a, b)
	assert.False(t, math.Abs(sim-0.978) > simTestEpsilon)
}

func TestMSD(t *testing.T) {
	indexer := NewTestIndexer([]int{0, 1, 2, 3})
	a := NewMarginalSubSet(indexer, []int{1, 2, 3}, []float64{4, 5, 6}, []int{0, 1, 2})
	b := NewMarginalSubSet(indexer, []int{0, 1, 2}, []float64{0, 1, 2}, []int{0, 1, 2})
	sim := MSDSimilarity(a, b)
	assert.False(t, math.Abs(sim-0.1) > simTestEpsilon)
}

func TestPearson(t *testing.T) {
	indexer := NewTestIndexer([]int{0, 1, 2, 3})
	a := NewMarginalSubSet(indexer, []int{1, 2, 3}, []float64{4, 5, 6}, []int{0, 1, 2})
	b := NewMarginalSubSet(indexer, []int{0, 1, 2}, []float64{0, 1, 2}, []int{0, 1, 2})
	sim := PearsonSimilarity(a, b)
	assert.False(t, math.Abs(sim) > simTestEpsilon)
}
