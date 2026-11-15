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

import "unsafe"

//go:generate go tool goat src/float16_avx.c --target amd64 -O3 -mavx -mf16c -e=-ffp-contract=off

// F16C is set only when runtime AVX (including OS register support) is usable.
// These kernels require neither AVX2 nor FMA, even on AVX512 machines.
func (feature Feature) hAddFloat(dst, s []uint16) {
	if len(dst) == 0 {
		return
	}
	if feature&F16C != 0 {
		_h256_add(unsafe.Pointer(&dst[0]), unsafe.Pointer(&s[0]), int64(len(dst)))
		return
	}
	hAddFloat(dst, s)
}
func (feature Feature) hAddConst(dst []uint16, c float32) {
	if len(dst) == 0 {
		return
	}
	if feature&F16C != 0 {
		_h256_add_const(unsafe.Pointer(&dst[0]), unsafe.Pointer(&c), int64(len(dst)))
		return
	}
	hAddConst(dst, c)
}
func (feature Feature) hMulTo(a, b, dst []uint16) {
	if len(dst) == 0 {
		return
	}
	if feature&F16C != 0 {
		_h256_mul_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&dst[0]), int64(len(dst)))
		return
	}
	hMulTo(a, b, dst)
}
func (feature Feature) hmm(transA, transB bool, m, n, k int, a []uint16, lda int, b []uint16, ldb int, c []uint16, ldc int) {
	if m == 0 || n == 0 {
		return
	}
	if k == 0 {
		hmm(transA, transB, m, n, k, a, lda, b, ldb, c, ldc)
		return
	}
	if feature&F16C != 0 {
		var ta, tb int64
		if transA {
			ta = 1
		}
		if transB {
			tb = 1
		}
		_h256_mm(ta, tb, int64(m), int64(n), int64(k), unsafe.Pointer(&a[0]), int64(lda), unsafe.Pointer(&b[0]), int64(ldb), unsafe.Pointer(&c[0]), int64(ldc))
		return
	}
	hmm(transA, transB, m, n, k, a, lda, b, ldb, c, ldc)
}
