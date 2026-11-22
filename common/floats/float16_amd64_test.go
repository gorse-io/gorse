//go:build amd64 && !noasm

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
	"testing"
)

// These oracles deliberately do not use the package conversion helpers.
func hSIMDDecode(h uint16) float32 {
	s := 1.0
	if h&0x8000 != 0 {
		s = -1
	}
	e, f := int(h>>10)&31, int(h&1023)
	if e == 31 {
		if f != 0 {
			return float32(math.NaN())
		}
		return float32(math.Inf(int(s)))
	}
	if e == 0 {
		return float32(s * math.Ldexp(float64(f), -24))
	}
	return float32(s * math.Ldexp(float64(1024+f), e-25))
}
func hSIMDEncode(x float32) uint16 {
	sign := uint16(0)
	if math.Signbit(float64(x)) {
		sign = 0x8000
	}
	v := math.Abs(float64(x))
	if math.IsNaN(v) {
		return sign | 0x7e00
	}
	if v >= 65520 {
		return sign | 0x7c00
	}
	lo, hi := 0, 0x7bff
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if float64(hSIMDDecode(uint16(mid))) <= v {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo < 0x7bff {
		a, b := float64(hSIMDDecode(uint16(lo))), float64(hSIMDDecode(uint16(lo+1)))
		if v-a > b-v || (v-a == b-v && lo&1 != 0) {
			lo++
		}
	}
	return sign | uint16(lo)
}
func hSIMDCheck(t *testing.T, got, want uint16) {
	t.Helper()
	if want&0x7c00 == 0x7c00 && want&1023 != 0 {
		if got&0x7c00 == 0x7c00 && got&1023 != 0 {
			return
		}
	}
	if got != want {
		t.Fatalf("half got %04x (%g), want %04x (%g)", got, hSIMDDecode(got), want, hSIMDDecode(want))
	}
}
func hSIMDFeatures(t *testing.T, fn func(*testing.T, Feature)) {
	for _, f := range []Feature{0, AVX, F16C, AVX | F16C, AVX | F16C | FMA, AVX512 | F16C} {
		t.Run(fmt.Sprintf("bits-%x", uint64(f)), func(t *testing.T) {
			if f&feature != f {
				t.Skip("unsupported CPU features")
			}
			fn(t, f)
		})
	}
}
func TestHFloat16AVX(t *testing.T) {
	t.Run("Elementwise", testHFloat16AMD64Elementwise)
	t.Run("MM", testHFloat16AMD64MM)
	t.Run("MMSpecial", testHFloat16AMD64MMSpecial)
}
func testHFloat16AMD64Elementwise(t *testing.T) {
	values := []uint16{0, 0x8000, 1, 0x8001, 0x3ff, 0x400, 0x3c00, 0xbc00, 0x7bff, 0xfbff, 0x7c00, 0xfc00, 0x7e01, 0x7c01, 0x3555, 0xb555}
	hSIMDFeatures(t, func(t *testing.T, f Feature) {
		for n := 0; n <= 65; n++ {
			for off := 0; off < 8; off++ {
				a, b := make([]uint16, n+off+1), make([]uint16, n+off+1)
				a, b = a[off:off+n], b[off:off+n]
				for i := range a {
					a[i] = values[i%len(values)]
					b[i] = values[(i*7+5)%len(values)]
				}
				orig := append([]uint16(nil), a...)
				f.hAddFloat(a, b)
				for i := range a {
					hSIMDCheck(t, a[i], hSIMDEncode(hSIMDDecode(orig[i])+hSIMDDecode(b[i])))
				}
				copy(a, orig)
				f.hAddFloat(a, a)
				for i := range a {
					hSIMDCheck(t, a[i], hSIMDEncode(hSIMDDecode(orig[i])+hSIMDDecode(orig[i])))
				}
				for _, c := range []float32{0, 0.00048828125, 0.0000000298023223876953125, -1, float32(math.Inf(1))} {
					copy(a, orig)
					f.hAddConst(a, c)
					for i := range a {
						hSIMDCheck(t, a[i], hSIMDEncode(hSIMDDecode(orig[i])+c))
					}
				}
				for _, alias := range []int{0, 1, 2} {
					copy(a, orig)
					dst := make([]uint16, n)
					if alias == 1 {
						dst = a
					}
					if alias == 2 {
						dst = b
					}
					savedB := append([]uint16(nil), b...)
					f.hMulTo(a, b, dst)
					for i := range dst {
						hSIMDCheck(t, dst[i], hSIMDEncode(hSIMDDecode(orig[i])*hSIMDDecode(savedB[i])))
					}
					copy(b, savedB)
				}
			}
		}
	})
}
func testHFloat16AMD64MMSpecial(t *testing.T) {
	hSIMDFeatures(t, func(t *testing.T, f Feature) {
		f.hmm(false, false, 0, 3, 1, nil, 1, nil, 3, nil, 3)
		f.hmm(false, false, 3, 0, 1, nil, 1, nil, 1, nil, 1)
		values := []uint16{0, 0x8000, 1, 0x8001, 0x3ff, 0x400, 0x3c00, 0xbc00, 0x7bff, 0xfbff, 0x7c00, 0xfc00, 0x7e01, 0x7c01}
		for _, ta := range []bool{false, true} {
			for _, tb := range []bool{false, true} {
				for _, av := range values {
					for _, k := range []int{1, 9, 17} {
						n := len(values)
						lda, ldb := k+2, n+3
						ar, br := 1, k
						if ta {
							lda = 3
							ar = k
						}
						if tb {
							ldb = k + 3
							br = n
						}
						a, b, c := make([]uint16, ar*lda), make([]uint16, br*ldb), make([]uint16, n)
						// Exactly one nonzero product isolates exceptional conversion/class
						// behavior in both the column-vector and contiguous-dot kernels.
						a[0] = av
						for j, v := range values {
							idx := j
							if tb {
								idx = j * ldb
							}
							b[idx] = v
						}
						f.hmm(ta, tb, 1, n, k, a, lda, b, ldb, c, n)
						for j, v := range values {
							hSIMDCheck(t, c[j], hSIMDEncode(float32(0)+hSIMDDecode(av)*hSIMDDecode(v)))
						}
					}
				}
			}
		}
	})
}

func testHFloat16AMD64MM(t *testing.T) {
	hSIMDFeatures(t, func(t *testing.T, f Feature) {
		for _, ta := range []bool{false, true} {
			for _, tb := range []bool{false, true} {
				for _, size := range [][3]int{{1, 1, 1}, {3, 17, 19}, {2, 65, 65}, {2, 9, 257}, {3, 7, 9}} {
					m, n, k := size[0], size[1], size[2]
					ar, ac, br, bc := m, k, k, n
					if ta {
						ar, ac = k, m
					}
					if tb {
						br, bc = n, k
					}
					lda, ldb, ldc := ac+3, bc+5, n+7
					a, b, c := make([]uint16, ar*lda+1), make([]uint16, br*ldb+1), make([]uint16, m*ldc+1)
					a, b, c = a[1:], b[1:], c[1:]
					for i := range a {
						a[i] = hSIMDEncode(float32(i%13-6) / 16)
					}
					for i := range b {
						b[i] = hSIMDEncode(float32(i%11-5) / 32)
					}
					for i := range c {
						c[i] = 0x7e11
					}
					f.hmm(ta, tb, m, n, k, a, lda, b, ldb, c, ldc)
					for i := 0; i < m; i++ {
						for j := 0; j < ldc; j++ {
							if j >= n {
								if c[i*ldc+j] != 0x7e11 {
									t.Fatalf("C padding overwritten at (%d,%d): %04x", i, j, c[i*ldc+j])
								}
								continue
							}
							var sum float32
							for p := 0; p < k; p++ {
								ai, bi := i*lda+p, p*ldb+j
								if ta {
									ai = p*lda + i
								}
								if tb {
									bi = j*ldb + p
								}
								product := float32(hSIMDDecode(a[ai]) * hSIMDDecode(b[bi]))
								sum = float32(sum + product)
							}
							hSIMDCheck(t, c[i*ldc+j], hSIMDEncode(sum))
						}
					}
				}
				// A result that is wrong if partial sums are rounded to half.
				k := 257
				a, b, c := make([]uint16, k), make([]uint16, k), []uint16{0x7e00}
				for p := range a {
					a[p] = 0x3c00
					b[p] = 0x1000
				}
				b[0] = 0x3c00
				f.hmm(ta, tb, 1, 1, k, a, func() int {
					if ta {
						return 1
					}
					return k
				}(), b, func() int {
					if tb {
						return k
					}
					return 1
				}(), c, 1)
				hSIMDCheck(t, c[0], hSIMDEncode(1+256*float32(hSIMDDecode(0x1000))))
			}
		}
	})
}
