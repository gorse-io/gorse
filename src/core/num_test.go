package core

import (
	"github.com/gonum/floats"
	"github.com/gonum/stat"
	"math"
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

func TestNewNormalVector(t *testing.T) {
	a := NewNormalVector(100, 1, 2)
	mean := stat.Mean(a, nil)
	stdDev := stat.StdDev(a, nil)
	if math.Abs(mean-1) > 0.25 {
		t.Fail()
	} else if math.Abs(stdDev-2) > 0.25 {
		t.Fail()
	}
}

func TestNewUniformVector(t *testing.T) {
	a := NewUniformVector(100, 10, 100)
	for _, val := range a {
		if val < 10 {
			t.Fail()
		} else if val > 100 {
			t.Fail()
		}
	}
}

func TestRootMeanSquareError(t *testing.T) {
	a := []float64{-2.0, 0, 2.0}
	b := []float64{0, 0, 0}
	if math.Abs(RootMeanSquareError(a, b)-1.63299) > 0.00001 {
		t.Fail()
	}
}

func TestMeanAbsoluteError(t *testing.T) {
	a := []float64{-2.0, 0, 2.0}
	b := []float64{0, 0, 0}
	if math.Abs(MeanAbsoluteError(a, b)-1.33333) > 0.00001 {
		t.Fail()
	}
}
