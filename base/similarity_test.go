package base

import (
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
)

const simTestEpsilon = 1e-3

func TestCosine(t *testing.T) {
	a := NewSparseVector()
	a.Add(1, 4)
	a.Add(2, 5)
	a.Add(3, 6)
	b := NewSparseVector()
	b.Add(0, 0)
	b.Add(1, 1)
	b.Add(2, 2)
	sim := Cosine(a, b)
	assert.False(t, math.Abs(sim-0.978) > simTestEpsilon)
}

func TestMSD(t *testing.T) {
	a := NewSparseVector()
	a.Add(1, 4)
	a.Add(2, 5)
	a.Add(3, 6)
	b := NewSparseVector()
	b.Add(0, 0)
	b.Add(1, 1)
	b.Add(2, 2)
	sim := MSD(a, b)
	assert.False(t, math.Abs(sim-0.1) > simTestEpsilon)
}

func TestPearson(t *testing.T) {
	a := NewSparseVector()
	a.Add(1, 4)
	a.Add(2, 5)
	a.Add(3, 6)
	b := NewSparseVector()
	b.Add(0, 0)
	b.Add(1, 1)
	b.Add(2, 2)
	sim := Pearson(a, b)
	assert.False(t, math.Abs(sim) > simTestEpsilon)
}
