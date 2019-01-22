package floats

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAdd(t *testing.T) {
	a := []float64{1, 2, 3, 4}
	b := []float64{5, 6, 7, 8}
	Add(a, b)
	assert.Equal(t, []float64{6, 8, 10, 12}, a)
}

func TestSub(t *testing.T) {
	a := []float64{1, 2, 3, 4}
	b := []float64{5, 6, 7, 8}
	Sub(a, b)
	assert.Equal(t, []float64{-4, -4, -4, -4}, a)
}

func TestSubTo(t *testing.T) {
	a := []float64{1, 2, 3, 4}
	b := []float64{5, 6, 7, 8}
	c := make([]float64, 4)
	SubTo(a, b, c)
	assert.Equal(t, []float64{-4, -4, -4, -4}, c)
}

func TestMul(t *testing.T) {
	a := []float64{1, 2, 3, 4}
	b := []float64{5, 6, 7, 8}
	Mul(a, b)
	assert.Equal(t, []float64{5, 12, 21, 32}, a)
}

func TestMulConst(t *testing.T) {
	a := []float64{1, 2, 3, 4}
	MulConst(a, 2)
	assert.Equal(t, []float64{2, 4, 6, 8}, a)
}

func TestDiv(t *testing.T) {
	a := []float64{1, 4, 9, 16}
	b := []float64{1, 2, 3, 4}
	Div(a, b)
	assert.Equal(t, []float64{1, 2, 3, 4}, a)
}
