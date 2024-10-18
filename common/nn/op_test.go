package nn

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAdd(t *testing.T) {
	// (2,3) + (2,3) -> (2,3)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := NewTensor([]float32{2, 3, 4, 5, 6, 7}, 2, 3)
	z := Add(x, y)
	assert.Equal(t, []float32{3, 5, 7, 9, 11, 13}, z.data)

	// (2,3) + () -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2})
	z = Add(x, y)
	assert.Equal(t, []float32{3, 4, 5, 6, 7, 8}, z.data)

	// (2,3) + (3) -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2, 3, 4}, 3)
	z = Add(x, y)
	assert.Equal(t, []float32{3, 5, 7, 6, 8, 10}, z.data)
}

func TestSub(t *testing.T) {
	// (2,3) - (2,3) -> (2,3)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := NewTensor([]float32{2, 3, 4, 5, 6, 7}, 2, 3)
	z := Sub(x, y)
	assert.Equal(t, []float32{-1, -1, -1, -1, -1, -1}, z.data)

	// (2,3) - () -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2})
	z = Sub(x, y)
	assert.Equal(t, []float32{-1, 0, 1, 2, 3, 4}, z.data)

	// (2,3) - (3) -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2, 3, 4}, 3)
	z = Sub(x, y)
	assert.Equal(t, []float32{-1, -1, -1, 2, 2, 2}, z.data)
}

func TestMul(t *testing.T) {
	// (2,3) * (2,3) -> (2,3)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y := NewTensor([]float32{2, 3, 4, 5, 6, 7}, 2, 3)
	z := Mul(x, y)
	assert.Equal(t, []float32{2, 6, 12, 20, 30, 42}, z.data)

	// (2,3) * () -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2})
	z = Mul(x, y)
	assert.Equal(t, []float32{2, 4, 6, 8, 10, 12}, z.data)

	// (2,3) * (3) -> (2,3)
	x = NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	y = NewTensor([]float32{2, 3, 4}, 3)
	z = Mul(x, y)
	assert.Equal(t, []float32{2, 6, 12, 8, 15, 24}, z.data)
}

func TestPow(t *testing.T) {
	// (2,3) ** 2 -> (2,3)
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	z := Pow(x, 2)
	assert.Equal(t, []float32{1, 4, 9, 16, 25, 36}, z.data)
}

func TestSum(t *testing.T) {
	// (2,3) -> ()
	x := NewTensor([]float32{1, 2, 3, 4, 5, 6}, 2, 3)
	z := Sum(x)
	assert.Equal(t, []float32{21}, z.data)
}
