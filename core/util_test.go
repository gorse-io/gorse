package core

import (
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"math"
	"testing"
)

func EqualInt(a []int, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSelectFloat(t *testing.T) {
	a := []float64{-1.0, 0, 1.0}
	b := []float64{1.0, 0, 1.0}
	c := []int{2, 1, 2}
	if !floats.Equal(selectFloat(a, c), b) {
		t.Fail()
	}
}

func TestSelectInt(t *testing.T) {
	a := []int{1, 2, 3}
	b := []int{2, 3, 3}
	c := []int{1, 2, 2}
	if !EqualInt(selectInt(a, c), b) {
		t.Fail()
	}
}

func TestAbs(t *testing.T) {
	a := []float64{-1.0, 0, 1.0}
	b := []float64{1.0, 0, 1.0}
	abs(a)
	if !floats.Equal(a, b) {
		t.Fail()
	}
}

func TestMulConst(t *testing.T) {
	a := []float64{0.0, 1.0, 2.0}
	b := []float64{0.0, 2.0, 4.0}
	mulConst(2.0, a)
	if !floats.Equal(a, b) {
		t.Fail()
	}
}

func TestDivConst(t *testing.T) {
	a := []float64{0.0, 1.0, 2.0}
	b := []float64{0.0, 0.5, 1.0}
	divConst(2.0, a)
	if !floats.Equal(a, b) {
		t.Fail()
	}
}

func TestNewNormalVector(t *testing.T) {
	a := newNormalVector(1000, 1, 2)
	mean := stat.Mean(a, nil)
	stdDev := stat.StdDev(a, nil)
	if math.Abs(mean-1) > 0.2 {
		t.Fatalf("Mean(%.4f) doesn't match %.4f", mean, 1.0)
	} else if math.Abs(stdDev-2) > 0.2 {
		t.Fatalf("Std(%.4f) doesn't match %.4f", stdDev, 2.0)
	}
}

func TestNewUniformVector(t *testing.T) {
	a := newUniformVectorInt(100, 10, 100)
	for _, val := range a {
		if val < 10 {
			t.Fail()
		} else if val >= 100 {
			t.Fail()
		}
	}
}
