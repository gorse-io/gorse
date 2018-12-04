package base

import (
	"github.com/stretchr/testify/assert"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"math"
	"testing"
)

const randomEpsilon = 0.1

func TestRandomGenerator_MakeNormalVector(t *testing.T) {
	rng := NewRandomGenerator(0)
	vec := rng.MakeNormalVector(1000, 1, 2)
	assert.False(t, math.Abs(stat.Mean(vec, nil)-1) > randomEpsilon)
	assert.False(t, math.Abs(stat.StdDev(vec, nil)-2) > randomEpsilon)
}

func TestRandomGenerator_MakeUniformVector(t *testing.T) {
	rng := NewRandomGenerator(0)
	vec := rng.MakeUniformVector(100, 1, 2)
	assert.False(t, floats.Min(vec) < 1)
	assert.False(t, floats.Max(vec) > 2)
}
