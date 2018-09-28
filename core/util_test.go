package core

import (
	"gonum.org/v1/gonum/floats"
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
