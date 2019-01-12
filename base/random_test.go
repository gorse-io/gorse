package base

import (
	"github.com/stretchr/testify/assert"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"math"
	"testing"
)

const randomEpsilon = 0.1

func TestRandomGenerator_MakeUniformVectorInt(t *testing.T) {
	rng := NewRandomGenerator(0)
	vec := rng.NewUniformVectorInt(1000, 10, 100)
	assert.False(t, Min(vec) < 10)
	assert.False(t, Max(vec) > 100)
}

func TestRandomGenerator_MakeNormalMatrix(t *testing.T) {
	rng := NewRandomGenerator(0)
	vec := rng.NewNormalMatrix(1, 1000, 1, 2)[0]
	assert.False(t, math.Abs(stat.Mean(vec, nil)-1) > randomEpsilon)
	assert.False(t, math.Abs(stat.StdDev(vec, nil)-2) > randomEpsilon)
}

func TestRandomGenerator_MakeUniformMatrix(t *testing.T) {
	rng := NewRandomGenerator(0)
	vec := rng.NewUniformMatrix(1, 1000, 1, 2)[0]
	assert.False(t, floats.Min(vec) < 1)
	assert.False(t, floats.Max(vec) > 2)
}
