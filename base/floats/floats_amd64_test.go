//go:build !noasm

// Copyright 2022 gorse Project Authors
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
	"math/rand"
	"strconv"
	"testing"

	"github.com/klauspost/cpuid/v2"
	"github.com/stretchr/testify/assert"
)

func TestAVX_MulConstAddTo(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.AVX) || !cpuid.CPU.Supports(cpuid.FMA3) {
		t.Skip("AVX and FMA3 are not supported in the current CPU")
	}
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	AVX.mulConstAddTo(a, 2, b)
	c := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	Default.mulConstAddTo(a, 2, c)
	assert.Equal(t, c, b)
}

func TestAVX_MulConstTo(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.AVX) || !cpuid.CPU.Supports(cpuid.FMA3) {
		t.Skip("AVX and FMA3 are not supported in the current CPU")
	}
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	AVX.mulConstTo(a, 2, b)
	c := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	Default.mulConstTo(a, 2, c)
	assert.Equal(t, c, b)
}

func TestAVX_MulTo(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.AVX) || !cpuid.CPU.Supports(cpuid.FMA3) {
		t.Skip("AVX and FMA3 are not supported in the current CPU")
	}
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	expected, actual := make([]float32, len(a)), make([]float32, len(a))
	AVX.mulTo(a, b, actual)
	Default.mulTo(a, b, expected)
	assert.Equal(t, expected, actual)
}

func TestAVX_MulConst(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.AVX) || !cpuid.CPU.Supports(cpuid.FMA3) {
		t.Skip("AVX and FMA3 are not supported in the current CPU")
	}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	AVX.mulConst(b, 2)
	c := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	Default.mulConst(c, 2)
	assert.Equal(t, c, b)
}

func TestAVX_Dot(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.AVX) || !cpuid.CPU.Supports(cpuid.FMA3) {
		t.Skip("AVX and FMA3 are not supported in the current CPU")
	}
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	actual := AVX.dot(a, b)
	expected := Default.dot(a, b)
	assert.Equal(t, expected, actual)
}

func TestAVX_Euclidean(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.AVX) || !cpuid.CPU.Supports(cpuid.FMA3) {
		t.Skip("AVX and FMA3 are not supported in the current CPU")
	}
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	b := []float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18}
	actual := AVX.euclidean(a, b)
	expected := Default.euclidean(a, b)
	assert.InDelta(t, expected, actual, 1e-6)
}

func TestAVX512_MulConstAddTo(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.AVX512F) || !cpuid.CPU.Supports(cpuid.AVX512DQ) {
		t.Skip("AVX512F and AVX512DQ are not supported in the current CPU")
	}
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	AVX512.mulConstAddTo(a, 2, b)
	c := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	Default.mulConstAddTo(a, 2, c)
	assert.Equal(t, c, b)
}

func TestAVX512_MulConstTo(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.AVX512F) || !cpuid.CPU.Supports(cpuid.AVX512DQ) {
		t.Skip("AVX512F and AVX512DQ are not supported in the current CPU")
	}
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	AVX512.mulConstTo(a, 2, b)
	c := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	Default.mulConstTo(a, 2, c)
	assert.Equal(t, c, b)
}

func TestAVX512_MulTo(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.AVX512F) || !cpuid.CPU.Supports(cpuid.AVX512DQ) {
		t.Skip("AVX512F and AVX512DQ are not supported in the current CPU")
	}
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	expected, actual := make([]float32, len(a)), make([]float32, len(a))
	AVX512.mulTo(a, b, actual)
	Default.mulTo(a, b, expected)
	assert.Equal(t, expected, actual)
}

func TestAVX512_MulConst(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.AVX512F) || !cpuid.CPU.Supports(cpuid.AVX512DQ) {
		t.Skip("AVX512F and AVX512DQ are not supported in the current CPU")
	}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	AVX512.mulConst(b, 2)
	c := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	Default.mulConst(c, 2)
	assert.Equal(t, c, b)
}

func TestAVX512_Dot(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.AVX512F) || !cpuid.CPU.Supports(cpuid.AVX512DQ) {
		t.Skip("AVX512F and AVX512DQ are not supported in the current CPU")
	}
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	actual := AVX512.dot(a, b)
	expected := Default.dot(a, b)
	assert.Equal(t, expected, actual)
}

func TestAVX512_Euclidean(t *testing.T) {
	if !cpuid.CPU.Supports(cpuid.AVX512F) || !cpuid.CPU.Supports(cpuid.AVX512DQ) {
		t.Skip("AVX512F and AVX512DQ are not supported in the current CPU")
	}
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	actual := AVX512.euclidean(a, b)
	expected := Default.euclidean(a, b)
	assert.InDelta(t, expected, actual, 1e-6)
}

func initializeFloat32Array(n int) []float32 {
	x := make([]float32, n)
	for i := 0; i < n; i++ {
		x[i] = rand.Float32()
	}
	return x
}

func BenchmarkDot(b *testing.B) {
	for _, impl := range []implementation{Default, AVX, AVX512} {
		b.Run(impl.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := initializeFloat32Array(i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						impl.dot(v1, v2)
					}
				})
			}
		})
	}
}

func BenchmarkEuclidean(b *testing.B) {
	for _, impl := range []implementation{Default, AVX, AVX512} {
		b.Run(impl.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := initializeFloat32Array(i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						impl.euclidean(v1, v2)
					}
				})
			}
		})
	}
}

func BenchmarkMulConstAddTo(b *testing.B) {
	for _, impl := range []implementation{Default, AVX, AVX512} {
		b.Run(impl.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := initializeFloat32Array(i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						impl.mulConstAddTo(v1, 2, v2)
					}
				})
			}
		})
	}
}

func BenchmarkMulConst(b *testing.B) {
	for _, impl := range []implementation{Default, AVX, AVX512} {
		b.Run(impl.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						impl.mulConst(v1, 2)
					}
				})
			}
		})
	}
}

func BenchmarkMulConstTo(b *testing.B) {
	for _, impl := range []implementation{Default, AVX, AVX512} {
		b.Run(impl.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := initializeFloat32Array(i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						impl.mulConstTo(v1, 2, v2)
					}
				})
			}
		})
	}
}

func BenchmarkMulTo(b *testing.B) {
	for _, impl := range []implementation{Default, AVX, AVX512} {
		b.Run(impl.String(), func(b *testing.B) {
			for i := 16; i <= 128; i *= 2 {
				b.Run(strconv.Itoa(i), func(b *testing.B) {
					v1 := initializeFloat32Array(i)
					v2 := initializeFloat32Array(i)
					v3 := make([]float32, i)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						impl.mulTo(v1, v2, v3)
					}
				})
			}
		})
	}
}
