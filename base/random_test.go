package base

import (
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"math"
	"testing"
)

const randomEpsilon = 0.1

func TestRandomGenerator_MakeNormalVector(t *testing.T) {
	rng := NewRandomGenerator(0)
	vec := rng.MakeNormalVector(1000, 1, 2)
	if mean := stat.Mean(vec, nil); math.Abs(mean-1) > randomEpsilon {
		t.Fatal("mean is not 1 but", mean)
	}
	if std := stat.StdDev(vec, nil); math.Abs(std-2) > randomEpsilon {
		t.Fatal("stddev is not 2 but", std)
	}
}

func TestRandomGenerator_MakeUniformVector(t *testing.T) {
	rng := NewRandomGenerator(0)
	vec := rng.MakeUniformVector(100, 1, 2)
	if floats.Min(vec) < 1 {
		t.Fatal("minimum is less than 1")
	}
	if floats.Max(vec) > 2 {
		t.Fatal("maximum is greater than 2")
	}
}
