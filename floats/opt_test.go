package floats

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMulConstTo(t *testing.T) {
	a := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	dst := make([]float64, 11)
	target := []float64{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	MulConstTo(a, 2, dst)
	assert.Equal(t, target, dst)
}

func TestMulConstAddTo(t *testing.T) {
	a := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	dst := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	target := []float64{0, 3, 6, 9, 12, 15, 18, 21, 24, 27, 30}
	MulConstAddTo(a, 2, dst)
	assert.Equal(t, target, dst)
}

func TestAddTo(t *testing.T) {
	a := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float64{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	dst := make([]float64, 11)
	target := []float64{0, 3, 6, 9, 12, 15, 18, 21, 24, 27, 30}
	AddTo(a, b, dst)
	assert.Equal(t, target, dst)
}

func TestDot(t *testing.T) {
	a := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float64{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	assert.Equal(t, 770.0, Dot(a, b))
}
