package core

import (
	"github.com/gonum/floats"
	"testing"
)

func TestAbs(t *testing.T) {
	a := []float64{-1.0, 0, 1.0}
	b := []float64{1.0, 0, 1.0}
	Abs(a)
	if !floats.Equal(a, b) {
		t.Fail()
	}
}

func TestMulConst(t *testing.T) {
	a := []float64{0.0, 1.0, 2.0}
	b := []float64{0.0, 2.0, 4.0}
	MulConst(2.0, a)
	if !floats.Equal(a, b) {
		t.Fail()
	}
}

func TestDivConst(t *testing.T) {
	a := []float64{0.0, 1.0, 2.0}
	b := []float64{0.0, 0.5, 1.0}
	DivConst(2.0, a)
	if !floats.Equal(a, b) {
		t.Fail()
	}
}
