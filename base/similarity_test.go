package base

import (
	"math"
	"testing"
)

const simTestEpsilon = 1e-6

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
	if math.Abs(sim-0.978) > simTestEpsilon {
		t.Fatal(sim, "!=", 0.978)
	}
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
	if math.Abs(sim-0.1) > simTestEpsilon {
		t.Fatal(sim, "!=", 0.1)
	}
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
	if math.Abs(sim) > simTestEpsilon {
		t.Fatal(sim, "!=", 0.0)
	}
}
