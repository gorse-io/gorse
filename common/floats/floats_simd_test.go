//go:build goexperiment.simd

// Copyright 2026 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package floats

import (
	"fmt"
	"simd"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeSIMDTestVectors() ([]float32, []float32) {
	n := 2*simd.VectorBitSize()/32 + 1
	a := make([]float32, n)
	b := make([]float32, n)
	for i := range n {
		a[i] = float32(i + 1)
		b[i] = float32((i + 1) * 2)
	}
	return a, b
}

func TestSIMDMulConstAddTo(t *testing.T) {
	a, b := makeSIMDTestVectors()
	expected := make([]float32, len(a))
	actual := make([]float32, len(a))
	mulConstAddTo(a, 2, b, expected)
	simdMulConstAddTo(a, 2, b, actual)
	assert.Equal(t, expected, actual)
}

func TestSIMDDot(t *testing.T) {
	a, b := makeSIMDTestVectors()
	assert.Equal(t, dot(a, b), simdDot(a, b))
}

func TestSIMDEuclidean(t *testing.T) {
	a, b := makeSIMDTestVectors()
	assert.InDelta(t, euclidean(a, b), simdEuclidean(a, b), 1e-5)
}

func TestSIMDMulConstAdd(t *testing.T) {
	a, b := makeSIMDTestVectors()
	expected := append([]float32(nil), b...)
	actual := append([]float32(nil), b...)
	mulConstAdd(a, 2, expected)
	simdMulConstAdd(a, 2, actual)
	assert.Equal(t, expected, actual)
}

func TestSIMDMulConstTo(t *testing.T) {
	a, _ := makeSIMDTestVectors()
	expected := make([]float32, len(a))
	actual := make([]float32, len(a))
	mulConstTo(a, 2, expected)
	simdMulConstTo(a, 2, actual)
	assert.Equal(t, expected, actual)
}

func TestSIMDMulConst(t *testing.T) {
	a, _ := makeSIMDTestVectors()
	expected := append([]float32(nil), a...)
	actual := append([]float32(nil), a...)
	mulConst(expected, 2)
	simdMulConst(actual, 2)
	assert.Equal(t, expected, actual)
}

func TestSIMDAddConst(t *testing.T) {
	a, _ := makeSIMDTestVectors()
	expected := append([]float32(nil), a...)
	actual := append([]float32(nil), a...)
	addConst(expected, 2)
	simdAddConst(actual, 2)
	assert.Equal(t, expected, actual)
}

func TestSIMDSub(t *testing.T) {
	a, b := makeSIMDTestVectors()
	expected := append([]float32(nil), a...)
	actual := append([]float32(nil), a...)
	sub(expected, b)
	simdSub(actual, b)
	assert.Equal(t, expected, actual)
}

func TestSIMDSubTo(t *testing.T) {
	a, b := makeSIMDTestVectors()
	expected := make([]float32, len(a))
	actual := make([]float32, len(a))
	subTo(a, b, expected)
	simdSubTo(a, b, actual)
	assert.Equal(t, expected, actual)
}

func TestSIMDMulTo(t *testing.T) {
	a, b := makeSIMDTestVectors()
	expected := make([]float32, len(a))
	actual := make([]float32, len(a))
	mulTo(a, b, expected)
	simdMulTo(a, b, actual)
	assert.Equal(t, expected, actual)
}

func TestSIMDDivTo(t *testing.T) {
	a, b := makeSIMDTestVectors()
	expected := make([]float32, len(a))
	actual := make([]float32, len(a))
	divTo(b, a, expected)
	simdDivTo(b, a, actual)
	assert.Equal(t, expected, actual)
}

func TestSIMDSqrtTo(t *testing.T) {
	n := 2*simd.VectorBitSize()/32 + 1
	squares := make([]float32, n)
	for i := range n {
		squares[i] = float32((i + 1) * (i + 1))
	}
	expected := make([]float32, n)
	actual := make([]float32, n)
	sqrtTo(squares, expected)
	simdSqrtTo(squares, actual)
	assert.Equal(t, expected, actual)
}

func TestSIMDMM(t *testing.T) {
	a := []float32{1, 2, 3, 4, 5, 6}
	b := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	tests := []struct {
		name           string
		transA, transB bool
		lda, ldb       int
		expected       []float32
	}{
		{"NN", false, false, 3, 4, []float32{38, 44, 50, 56, 83, 98, 113, 128}},
		{"NT", false, true, 3, 3, []float32{14, 32, 50, 68, 32, 77, 122, 167}},
		{"TN", true, false, 2, 4, []float32{61, 70, 79, 88, 76, 88, 100, 112}},
		{"TT", true, true, 2, 3, []float32{22, 49, 76, 103, 28, 64, 100, 136}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := make([]float32, 8)
			simdMM(test.transA, test.transB, 2, 4, 3, a, test.lda, b, test.ldb, actual, 4)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func BenchmarkDotSIMD(b *testing.B) {
	for n := 16; n <= 128; n *= 2 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			a := initializeFloat32Array(n)
			c := initializeFloat32Array(n)
			b.ResetTimer()
			for range b.N {
				simdDot(a, c)
			}
		})
	}
}

func BenchmarkEuclideanSIMD(b *testing.B) {
	for n := 16; n <= 128; n *= 2 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			a := initializeFloat32Array(n)
			c := initializeFloat32Array(n)
			b.ResetTimer()
			for range b.N {
				simdEuclidean(a, c)
			}
		})
	}
}

func BenchmarkMulConstAddToSIMD(b *testing.B) {
	for n := 16; n <= 128; n *= 2 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			a := initializeFloat32Array(n)
			c := initializeFloat32Array(n)
			dst := make([]float32, n)
			b.ResetTimer()
			for range b.N {
				simdMulConstAddTo(a, 2, c, dst)
			}
		})
	}
}

func BenchmarkMulConstAddSIMD(b *testing.B) {
	for n := 16; n <= 128; n *= 2 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			a := initializeFloat32Array(n)
			dst := initializeFloat32Array(n)
			b.ResetTimer()
			for range b.N {
				simdMulConstAdd(a, 2, dst)
			}
		})
	}
}

func BenchmarkMulConstSIMD(b *testing.B) {
	for n := 16; n <= 128; n *= 2 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			a := initializeFloat32Array(n)
			b.ResetTimer()
			for range b.N {
				simdMulConst(a, 2)
			}
		})
	}
}

func BenchmarkMulConstToSIMD(b *testing.B) {
	for n := 16; n <= 128; n *= 2 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			a := initializeFloat32Array(n)
			dst := make([]float32, n)
			b.ResetTimer()
			for range b.N {
				simdMulConstTo(a, 2, dst)
			}
		})
	}
}

func BenchmarkAddConstSIMD(b *testing.B) {
	for n := 16; n <= 128; n *= 2 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			a := initializeFloat32Array(n)
			b.ResetTimer()
			for range b.N {
				simdAddConst(a, 2)
			}
		})
	}
}

func BenchmarkSubSIMD(b *testing.B) {
	for n := 16; n <= 128; n *= 2 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			a := initializeFloat32Array(n)
			c := initializeFloat32Array(n)
			b.ResetTimer()
			for range b.N {
				simdSub(a, c)
			}
		})
	}
}

func BenchmarkSubToSIMD(b *testing.B) {
	for n := 16; n <= 128; n *= 2 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			a := initializeFloat32Array(n)
			c := initializeFloat32Array(n)
			dst := make([]float32, n)
			b.ResetTimer()
			for range b.N {
				simdSubTo(a, c, dst)
			}
		})
	}
}

func BenchmarkMulToSIMD(b *testing.B) {
	for n := 16; n <= 128; n *= 2 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			a := initializeFloat32Array(n)
			c := initializeFloat32Array(n)
			dst := make([]float32, n)
			b.ResetTimer()
			for range b.N {
				simdMulTo(a, c, dst)
			}
		})
	}
}

func BenchmarkDivToSIMD(b *testing.B) {
	for n := 16; n <= 128; n *= 2 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			a := initializeFloat32Array(n)
			c := initializeFloat32Array(n)
			dst := make([]float32, n)
			b.ResetTimer()
			for range b.N {
				simdDivTo(a, c, dst)
			}
		})
	}
}

func BenchmarkSqrtToSIMD(b *testing.B) {
	for n := 16; n <= 128; n *= 2 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			a := initializeFloat32Array(n)
			dst := make([]float32, n)
			b.ResetTimer()
			for range b.N {
				simdSqrtTo(a, dst)
			}
		})
	}
}

func BenchmarkMMSIMD(b *testing.B) {
	for _, transA := range []bool{false, true} {
		for _, transB := range []bool{false, true} {
			b.Run(fmt.Sprintf("(%v,%v)", transA, transB), func(b *testing.B) {
				for n := 16; n <= 128; n *= 2 {
					b.Run(strconv.Itoa(n), func(b *testing.B) {
						matA := initializeFloat32Array(n * n)
						matB := initializeFloat32Array(n * n)
						matC := make([]float32, n*n)
						b.ResetTimer()
						for range b.N {
							simdMM(transA, transB, n, n, n, matA, n, matB, n, matC, n)
						}
					})
				}
			})
		}
	}
}
