package base

import (
	"math"
	"testing"
)

const epsilon = 0.01

func TestCosine(t *testing.T) {
	a := NewSortedIdRatings([]IdRating{
		{1, 4},
		{2, 5},
		{3, 6},
	})
	b := NewSortedIdRatings([]IdRating{
		{0, 0},
		{1, 1},
		{2, 2},
	})
	sim := Cosine(a, b)
	if math.Abs(sim-0.978) > epsilon {
		t.Fatal(sim, "!=", 0.978)
	}
}

func TestMSD(t *testing.T) {
	a := NewSortedIdRatings([]IdRating{
		{1, 4},
		{2, 5},
		{3, 6},
	})
	b := NewSortedIdRatings([]IdRating{
		{0, 0},
		{1, 1},
		{2, 2},
	})
	sim := MSD(a, b)
	if math.Abs(sim-0.1) > epsilon {
		t.Fatal(sim, "!=", 0.1)
	}
}

func TestPearson(t *testing.T) {
	a := NewSortedIdRatings([]IdRating{
		{1, 4},
		{2, 5},
		{3, 6},
	})
	b := NewSortedIdRatings([]IdRating{
		{0, 0},
		{1, 1},
		{2, 2},
	})
	sim := Pearson(a, b)
	if math.Abs(sim) > epsilon {
		t.Fatal(sim, "!=", 0.0)
	}
}
