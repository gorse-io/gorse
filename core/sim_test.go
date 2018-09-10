package core

import (
	"math"
	"testing"
)

const epsion = 0.01

func TestCosine(t *testing.T) {
	a := []float64{3, 4, 5, math.NaN()}
	b := []float64{math.NaN(), 1, 2, 3}
	sim := Cosine(a, b)
	if math.Abs(sim-0.978) > epsion {
		t.Fatal(sim, "!=", 0.978)
	}
}

func TestMSD(t *testing.T) {
	a := []float64{3, 4, 5, math.NaN()}
	b := []float64{math.NaN(), 1, 2, 3}
	sim := MSD(a, b)
	if math.Abs(sim-0.1) > epsion {
		t.Fatal(sim, "!=", 0.1)
	}
}

func TestPearson(t *testing.T) {
	a := []float64{3, 4, 5, math.NaN()}
	b := []float64{math.NaN(), 1, 2, 3}
	sim := Pearson(a, b)
	if math.Abs(sim) > epsion {
		t.Fatal(sim, "!=", 0.0)
	}
}
