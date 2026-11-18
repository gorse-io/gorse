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
)

// Independent numerical oracle: decode the binary16 exponent/significand and
// choose the nearest finite encoding (even significand on a tie).
func hTestDecode(h uint16) float64 {
	e, f := int(h>>10)&31, int(h&1023)
	var v float64
	switch e {
	case 31:
		if f != 0 {
			return math.NaN()
		}
		v = math.Inf(1)
	case 0:
		v = math.Ldexp(float64(f), -24)
	default:
		v = math.Ldexp(float64(1024+f), e-25)
	}
	if h&0x8000 != 0 {
		v = -v
	}
	return v
}

func hTestEncode(v float32) uint16 {
	x := float64(v)
	sign := uint16(0)
	if math.Signbit(x) {
		sign = 0x8000
		x = -x
	}
	if math.IsNaN(x) {
		return sign | 0x7e00
	}
	if x >= 65520 {
		return sign | 0x7c00
	}
	lo, hi := 0, 0x7bff
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if hTestDecode(uint16(mid)) <= x {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo < 0x7bff {
		down, up := x-hTestDecode(uint16(lo)), hTestDecode(uint16(lo+1))-x
		if up < down || (up == down && lo&1 != 0) {
			lo++
		}
	}
	return sign | uint16(lo)
}

func hTestEqual(t *testing.T, got, want []uint16) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("lengths: %d != %d", len(got), len(want))
	}
	for i := range got {
		if math.IsNaN(hTestDecode(want[i])) && math.IsNaN(hTestDecode(got[i])) {
			continue
		}
		if got[i] != want[i] {
			t.Fatalf("index %d: got %#04x (%g), want %#04x (%g)", i, got[i], hTestDecode(got[i]), want[i], hTestDecode(want[i]))
		}
	}
}

func TestHElementwise(t *testing.T) {
	bits := []uint16{0, 0x8000, 1, 0x8001, 0x3ff, 0x400, 0x3c00, 0x3c01, 0xbc00, 0x7bff, 0xfbff, 0x7c00, 0xfc00, 0x7e01}
	for _, size := range []int{0, 1, 7, 8, 15, 16, 17, 31, 32, 33, 129} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			a, b := make([]uint16, size), make([]uint16, size)
			for i := range a {
				a[i], b[i] = bits[i%len(bits)], bits[(i*3+6)%len(bits)]
			}
			want := make([]uint16, size)
			for i := range a {
				want[i] = hTestEncode(float32(hTestDecode(a[i])) + float32(hTestDecode(b[i])))
			}
			got := slices.Clone(a)
			HAddFloat(got, b)
			hTestEqual(t, got, want)
			for i := range a {
				want[i] = hTestEncode(float32(hTestDecode(a[i])) * float32(hTestDecode(b[i])))
			}
			HMulTo(a, b, got)
			hTestEqual(t, got, want)
			for _, c := range []float32{0, 0.00048828125, 0.0000000298023223876953125, 1.0003, float32(math.Inf(1)), float32(math.NaN())} {
				copy(got, a)
				for i := range a {
					want[i] = hTestEncode(float32(hTestDecode(a[i])) + c)
				}
				HAddConst(got, c)
				hTestEqual(t, got, want)
			}
		})
	}
}

func TestHExactAlias(t *testing.T) {
	a := []uint16{0x3c00, 0x4000, 0xc200}
	HAddFloat(a, a)
	hTestEqual(t, a, []uint16{0x4000, 0x4400, 0xc600})
	for _, which := range []int{0, 1, 2} {
		a, b := []uint16{0x3c00, 0x4000, 0xc200}, []uint16{0x4000, 0xc200, 0x4400}
		if which == 0 {
			HMulTo(a, b, a)
			hTestEqual(t, a, []uint16{0x4000, 0xc600, 0xca00})
		}
		if which == 1 {
			HMulTo(a, b, b)
			hTestEqual(t, b, []uint16{0x4000, 0xc600, 0xca00})
		}
		if which == 2 {
			HMulTo(a, a, a)
			hTestEqual(t, a, []uint16{0x3c00, 0x4400, 0x4880})
		}
	}
	HAddFloat(nil, nil)
	HAddConst(nil, 1)
	HMulTo(nil, nil, nil)
}

func TestHMM(t *testing.T) {
	const m, n, k = 3, 5, 7
	for _, ta := range []bool{false, true} {
		for _, tb := range []bool{false, true} {
			t.Run(fmt.Sprintf("%t/%t", ta, tb), func(t *testing.T) {
				ar, ac, br, bc := m, k, k, n
				if ta {
					ar, ac = k, m
				}
				if tb {
					br, bc = n, k
				}
				lda, ldb, ldc := ac+2, bc+3, n+4
				a, b, c := make([]uint16, (ar-1)*lda+ac), make([]uint16, (br-1)*ldb+bc), make([]uint16, (m-1)*ldc+n)
				for i := range a {
					a[i] = 0x7e01
				}
				for i := range b {
					b[i] = 0x7e01
				}
				for i := range c {
					c[i] = 0x7e01
				}
				for i := range m {
					for l := range k {
						idx := i*lda + l
						if ta {
							idx = l*lda + i
						}
						a[idx] = hTestEncode(float32(i*3 - l + 1))
					}
				}
				for l := range k {
					for j := range n {
						idx := l*ldb + j
						if tb {
							idx = j*ldb + l
						}
						b[idx] = hTestEncode(float32(l - j*2 + 2))
					}
				}
				want := slices.Clone(c)
				for i := range m {
					for j := range n {
						sum := 0
						for l := range k {
							sum += (i*3 - l + 1) * (l - j*2 + 2)
						}
						want[i*ldc+j] = hTestEncode(float32(sum))
					}
				}
				beforeA, beforeB := slices.Clone(a), slices.Clone(b)
				HMM(ta, tb, m, n, k, a, lda, b, ldb, c, ldc)
				hTestEqual(t, c, want)
				hTestEqual(t, a, beforeA)
				hTestEqual(t, b, beforeB)
			})
		}
	}
}

func TestHMMFinalRounding(t *testing.T) {
	// Rounding each partial sum to FP16 would discard both small terms.
	a := []uint16{0x3c00, 0x1000, 0x1000}
	b := []uint16{0x3c00, 0x3c00, 0x3c00}
	c := []uint16{0x7e01}
	HMM(false, false, 1, 1, 3, a, 3, b, 1, c, 1)
	hTestEqual(t, c, []uint16{0x3c01})
	// FP32 (not FP64) accumulation loses the middle term.
	HMM(false, false, 1, 1, 3, []uint16{0x7bff, 1, 0xfbff}, 3, b, 1, c, 1)
	hTestEqual(t, c, []uint16{0})
	HMM(false, false, 1, 1, 1, []uint16{0x7c00}, 1, []uint16{0}, 1, c, 1)
	if !math.IsNaN(hTestDecode(c[0])) {
		t.Fatal("infinity times zero must be NaN")
	}
}

func TestHMMZeroSizes(t *testing.T) {
	for _, ta := range []bool{false, true} {
		for _, tb := range []bool{false, true} {
			c := []uint16{0x7e01, 0x7e01, 0x5555, 0x7e01, 0x7e01}
			HMM(ta, tb, 2, 2, 0, nil, 2, nil, 2, c, 3)
			hTestEqual(t, c, []uint16{0, 0, 0x5555, 0, 0})
			HMM(ta, tb, 0, 2, 3, nil, 3, nil, 3, nil, 2)
			HMM(ta, tb, 2, 0, 3, nil, 3, nil, 3, nil, 0)
		}
	}
}

func TestHValidation(t *testing.T) {
	assertPanic := func(t *testing.T, f func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Error("expected panic")
			}
		}()
		f()
	}
	for _, f := range []func([]uint16){
		func(d []uint16) { HAddFloat(d, nil) },
		func(d []uint16) { HMulTo(d, nil, d) },
		func(d []uint16) { HMulTo(d, d, nil) },
		func(d []uint16) { HMulTo(nil, d, d) },
	} {
		d := []uint16{0x3c00}
		assertPanic(t, func() { f(d) })
		hTestEqual(t, d, []uint16{0x3c00})
	}
	maxInt := int(^uint(0) >> 1)
	cases := []struct{ m, n, k, lda, ldb, ldc, alen, blen, clen int }{
		{-1, 2, 2, 2, 2, 2, 4, 4, 4}, {2, -1, 2, 2, 2, 2, 4, 4, 4}, {2, 2, -1, 2, 2, 2, 4, 4, 4},
		{2, 2, 2, 1, 2, 2, 4, 4, 4}, {2, 2, 2, 2, 1, 2, 4, 4, 4}, {2, 2, 2, 2, 2, 1, 4, 4, 4},
		{2, 2, 2, -1, 2, 2, 4, 4, 4}, {2, 2, 2, 2, -1, 2, 4, 4, 4}, {2, 2, 2, 2, 2, -1, 4, 4, 4},
		{2, 2, 2, 2, 2, 2, 3, 4, 4}, {2, 2, 2, 2, 2, 2, 4, 3, 4}, {2, 2, 2, 2, 2, 2, 4, 4, 3},
		{2, 2, 2, maxInt, 2, 2, 4, 4, 4}, {2, 2, 2, 2, maxInt, 2, 4, 4, 4}, {2, 2, 2, 2, 2, maxInt, 4, 4, 4},
		{maxInt, 2, 2, 2, 2, 2, 4, 4, 4},
	}
	for _, ta := range []bool{false, true} {
		for _, tb := range []bool{false, true} {
			for i, tc := range cases {
				t.Run(fmt.Sprintf("%t/%t/%d", ta, tb, i), func(t *testing.T) {
					a, b, c := make([]uint16, tc.alen), make([]uint16, tc.blen), make([]uint16, tc.clen)
					for i := range c {
						c[i] = 0x3555
					}
					before := slices.Clone(c)
					assertPanic(t, func() { HMM(ta, tb, tc.m, tc.n, tc.k, a, tc.lda, b, tc.ldb, c, tc.ldc) })
					hTestEqual(t, c, before)
				})
			}
		}
	}
}

func TestHNoAllocations(t *testing.T) {
	a, b, c := make([]uint16, 256), make([]uint16, 256), make([]uint16, 256)
	for name, f := range map[string]func(){
		"add": func() { HAddFloat(a, b) }, "const": func() { HAddConst(a, 0) },
		"mul": func() { HMulTo(a, b, c) }, "mm": func() { HMM(false, false, 16, 16, 16, a, 16, b, 16, c, 16) },
	} {
		t.Run(name, func(t *testing.T) {
			if n := testing.AllocsPerRun(100, f); n != 0 {
				t.Fatalf("allocations: %g", n)
			}
		})
	}
}
