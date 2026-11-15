//go:build !noasm

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
	"math"
	"slices"
	"testing"

	"github.com/x448/float16"
	"golang.org/x/sys/cpu"
)

func requireHNEON(t *testing.T) {
	t.Helper()
	if !cpu.ARM64.HasASIMD {
		t.Skip("ASIMD is unavailable")
	}
}

func checkHNEON(t *testing.T, got, want []uint16) {
	t.Helper()
	for i := range want {
		if got[i] != want[i] && !(math.IsNaN(float64(float16.Frombits(got[i]).Float32())) && math.IsNaN(float64(float16.Frombits(want[i]).Float32()))) {
			t.Fatalf("element %d: got %#04x want %#04x", i, got[i], want[i])
		}
	}
}

func TestHFloat16ASIMD(t *testing.T) {
	t.Run("Vectors", testHNEONVectors)
	t.Run("MM", testHNEONMM)
	t.Run("MMRoundOnce", testHNEONMMRoundOnce)
}

func testHNEONVectors(t *testing.T) {
	requireHNEON(t)
	values := []uint16{0, 0x8000, 1, 0x8001, 0x3ff, 0x400, 0x3c00, 0xbc00, 0x7bff, 0xfbff, 0x7c00, 0xfc00, 0x7e01, 0x3555, 0x3c01, 0x0401, 0x3800}
	for _, feat := range []Feature{0, AMX, OPENBLAS, AMX | OPENBLAS} {
		for n := 1; n <= 33; n++ {
			t.Run(fmt.Sprintf("%d/%d", feat, n), func(t *testing.T) {
				a, b := make([]uint16, n), make([]uint16, n)
				for i := range a {
					a[i] = values[i%len(values)]
					b[i] = values[(i*7+3)%len(values)]
				}
				want := make([]uint16, n)
				for _, alias := range []bool{false, true} {
					dst := slices.Clone(a)
					src := b
					if alias {
						src = dst
					}
					for i := range want {
						want[i] = float16.Fromfloat32(float16.Frombits(dst[i]).Float32() + float16.Frombits(src[i]).Float32()).Bits()
					}
					feat.hAddFloat(dst, src)
					checkHNEON(t, dst, want)
				}
				for _, c := range []float32{0, 0.0000000298023223876953125, 0.00048828125, 1.0001, -2, float32(math.Inf(1)), float32(math.NaN())} {
					dst := slices.Clone(a)
					for i := range want {
						want[i] = float16.Fromfloat32(float16.Frombits(a[i]).Float32() + c).Bits()
					}
					feat.hAddConst(dst, c)
					checkHNEON(t, dst, want)
				}
				for _, alias := range []int{0, 1, 2} {
					x, y := slices.Clone(a), slices.Clone(b)
					dst := make([]uint16, n)
					if alias == 1 {
						dst = x
					}
					if alias == 2 {
						dst = y
					}
					for i := range want {
						want[i] = float16.Fromfloat32(float16.Frombits(a[i]).Float32() * float16.Frombits(b[i]).Float32()).Bits()
					}
					feat.hMulTo(x, y, dst)
					checkHNEON(t, dst, want)
				}
			})
		}
	}
}

func testHNEONMM(t *testing.T) {
	requireHNEON(t)
	for _, transA := range []bool{false, true} {
		for _, transB := range []bool{false, true} {
			for _, dims := range [][3]int{{3, 7, 11}, {5, 1, 7}, {1, 9, 3}, {2, 5, 257}} {
				t.Run(fmt.Sprintf("%v/%v/%v", transA, transB, dims), func(t *testing.T) {
					m, n, k := dims[0], dims[1], dims[2]
					ar, ac, br, bc := m, k, k, n
					if transA {
						ar, ac = k, m
					}
					if transB {
						br, bc = n, k
					}
					lda, ldb, ldc := ac+3, bc+5, n+2
					a, b, c := make([]uint16, ar*lda), make([]uint16, br*ldb), make([]uint16, m*ldc)
					for i := range a {
						a[i] = 0x7e00
					}
					for i := range b {
						b[i] = 0x7e00
					}
					for i := range c {
						c[i] = 0xdead
					}
					for i := 0; i < ar; i++ {
						for j := 0; j < ac; j++ {
							a[i*lda+j] = float16.Fromfloat32(float32((i*13+j*7)%31-15) / 16).Bits()
						}
					}
					for i := 0; i < br; i++ {
						for j := 0; j < bc; j++ {
							b[i*ldb+j] = float16.Fromfloat32(float32((i*11+j*3)%29-14) / 32).Bits()
						}
					}
					want := slices.Clone(c)
					for i := 0; i < m; i++ {
						for j := 0; j < n; j++ {
							var sum float32
							for l := 0; l < k; l++ {
								ai, bi := i*lda+l, l*ldb+j
								if transA {
									ai = l*lda + i
								}
								if transB {
									bi = j*ldb + l
								}
								product := float32(float16.Frombits(a[ai]).Float32() * float16.Frombits(b[bi]).Float32())
								sum = float32(sum + product)
							}
							want[i*ldc+j] = float16.Fromfloat32(sum).Bits()
						}
					}
					for _, feat := range []Feature{0, AMX, OPENBLAS, AMX | OPENBLAS} {
						feat.hmm(transA, transB, m, n, k, a, lda, b, ldb, c, ldc)
						checkHNEON(t, c, want)
					}
				})
			}
		}
	}
}

func testHNEONMMRoundOnce(t *testing.T) {
	requireHNEON(t)
	// Repeated increments disappear if the accumulator is rounded to FP16.
	a, b := make([]uint16, 2050), make([]uint16, 2050*5)
	for i := range a {
		a[i] = 0x3c00
		for j := range 5 {
			b[i*5+j] = 0x1000
		}
	}
	for j := range 5 {
		b[j] = 0x3c00
	}
	c := []uint16{0x7e00, 0x7e00, 0x7e00, 0x7e00, 0x7e00}
	Feature(0).hmm(false, false, 1, 5, len(a), a, len(a), b, 5, c, 5)
	var sum float32
	for i := range a {
		sum = float32(sum + float16.Frombits(b[i*5]).Float32())
	}
	want := make([]uint16, 5)
	for i := range want {
		want[i] = float16.Fromfloat32(sum).Bits()
	}
	checkHNEON(t, c, want)
}
