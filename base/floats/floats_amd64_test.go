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

	"github.com/stretchr/testify/assert"
)

func TestAVX2_MulConstAddTo(t *testing.T) {
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	avx2{}.MulConstAddTo(a, 2, b)
	c := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	native{}.MulConstAddTo(a, 2, c)
	assert.Equal(t, c, b)
}

func TestAVX2_MulConstTo(t *testing.T) {
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	avx2{}.MulConstTo(a, 2, b)
	c := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	native{}.MulConstTo(a, 2, c)
	assert.Equal(t, c, b)
}

func TestAVX2_MulTo(t *testing.T) {
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	expected, actual := make([]float32, len(a)), make([]float32, len(a))
	avx2{}.MulTo(a, b, actual)
	native{}.MulTo(a, b, expected)
	assert.Equal(t, expected, actual)
}

func TestAVX2_MulConst(t *testing.T) {
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	avx2{}.MulConst(b, 2)
	c := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	native{}.MulConst(c, 2)
	assert.Equal(t, c, b)
}

func TestAVX2_Dot(t *testing.T) {
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	actual := avx2{}.Dot(a, b)
	expected := native{}.Dot(a, b)
	assert.Equal(t, expected, actual)
}

func initializeFloat32Array(n int) []float32 {
	x := make([]float32, n)
	for i := 0; i < n; i++ {
		x[i] = rand.Float32()
	}
	return x
}

func BenchmarkDot(b *testing.B) {
	for i := 16; i <= 128; i *= 2 {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			v1 := initializeFloat32Array(i)
			v2 := initializeFloat32Array(i)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				native{}.Dot(v1, v2)
			}
		})
	}
}

func BenchmarkDot_AXV2(b *testing.B) {
	for i := 16; i <= 128; i *= 2 {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			v1 := initializeFloat32Array(i)
			v2 := initializeFloat32Array(i)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				avx2{}.Dot(v1, v2)
			}
		})
	}
}

func BenchmarkMulConstAddTo(b *testing.B) {
	for i := 16; i <= 128; i *= 2 {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			v1 := initializeFloat32Array(i)
			v2 := initializeFloat32Array(i)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				native{}.MulConstAddTo(v1, 2, v2)
			}
		})
	}
}

func BenchmarkMulConstAddTo_AVX2(b *testing.B) {
	for i := 16; i <= 128; i *= 2 {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			v1 := initializeFloat32Array(i)
			v2 := initializeFloat32Array(i)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				avx2{}.MulConstAddTo(v1, 2, v2)
			}
		})
	}
}

func BenchmarkMulConstTo(b *testing.B) {
	for i := 16; i <= 128; i *= 2 {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			v1 := initializeFloat32Array(i)
			v2 := initializeFloat32Array(i)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				native{}.MulConstTo(v1, 2, v2)
			}
		})
	}
}

func BenchmarkMulConstTo_AVX2(b *testing.B) {
	for i := 16; i <= 128; i *= 2 {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			v1 := initializeFloat32Array(i)
			v2 := initializeFloat32Array(i)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				avx2{}.MulConstTo(v1, 2, v2)
			}
		})
	}
}

func BenchmarkMulConst(b *testing.B) {
	for i := 16; i <= 128; i *= 2 {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			v1 := initializeFloat32Array(i)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				native{}.MulConst(v1, 2)
			}
		})
	}
}

func BenchmarkMulConst_AVX2(b *testing.B) {
	for i := 16; i <= 128; i *= 2 {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			v1 := initializeFloat32Array(i)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				avx2{}.MulConst(v1, 2)
			}
		})
	}
}

func BenchmarkMulTo(b *testing.B) {
	for i := 16; i <= 128; i *= 2 {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			v1 := initializeFloat32Array(i)
			v2 := initializeFloat32Array(i)
			v3 := make([]float32, i)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				native{}.MulTo(v1, v2, v3)
			}
		})
	}
}

func BenchmarkMulTo_AXV2(b *testing.B) {
	for i := 16; i <= 128; i *= 2 {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			v1 := initializeFloat32Array(i)
			v2 := initializeFloat32Array(i)
			v3 := make([]float32, i)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				avx2{}.MulTo(v1, v2, v3)
			}
		})
	}
}
