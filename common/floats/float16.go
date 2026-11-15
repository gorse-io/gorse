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

import "github.com/x448/float16"

// HAddFloat adds s to dst, both stored as IEEE 754 binary16 bits. Arithmetic
// uses FP32 and rounds each result to nearest FP16, ties to even. Exact aliasing
// of dst and s is supported; otherwise the slices must not overlap.
func HAddFloat(dst, s []uint16) {
	if len(dst) != len(s) {
		panic("floats: slice lengths do not match")
	}
	if len(dst) == 0 {
		return
	}
	feature.hAddFloat(dst, s)
}

// HAddConst adds the FP32 constant c to each IEEE 754 binary16 value in dst.
// Arithmetic uses FP32 and rounds each result to nearest FP16, ties to even;
// c is not rounded to FP16 before addition.
func HAddConst(dst []uint16, c float32) {
	if len(dst) == 0 {
		return
	}
	feature.hAddConst(dst, c)
}

// HMulTo stores a*b in dst, with IEEE 754 binary16 storage, FP32 arithmetic,
// and round-to-nearest, ties-to-even FP16 results. Exact aliasing of dst with
// either or both inputs is supported; otherwise the slices must not overlap.
func HMulTo(a, b, dst []uint16) {
	if len(a) != len(b) || len(a) != len(dst) {
		panic("floats: slice lengths do not match")
	}
	if len(dst) == 0 {
		return
	}
	feature.hMulTo(a, b, dst)
}

// HMM overwrites the m-by-n row-major matrix C with op(A)*op(B), using IEEE
// 754 binary16 storage. op(A) is m-by-k and op(B) is k-by-n. The transposition
// flags describe the stored inputs; lda, ldb and ldc are their row strides in
// elements. Strides must be nonnegative and at least the stored row widths.
// Buffers need only include the last logical element, not last-row padding.
// C must not overlap A or B. Padding is unchanged.
//
// Each output accumulates in FP32 and rounds to FP16 once, to nearest with ties
// to even. With k=0 it writes positive zeros; with m=0 or n=0 it does not access
// any buffers. Invalid dimensions, strides or buffers panic before C is changed.
func HMM(transA, transB bool, m, n, k int, a []uint16, lda int, b []uint16, ldb int, c []uint16, ldc int) {
	if m < 0 || n < 0 || k < 0 {
		panic("floats: negative matrix dimension")
	}
	ar, ac, br, bc := m, k, k, n
	if transA {
		ar, ac = k, m
	}
	if transB {
		br, bc = n, k
	}
	if lda < ac || ldb < bc || ldc < n {
		panic("floats: invalid matrix stride")
	}
	if m == 0 || n == 0 {
		return
	}
	if !hMatrixFits(ar, ac, lda, len(a)) || !hMatrixFits(br, bc, ldb, len(b)) || !hMatrixFits(m, n, ldc, len(c)) {
		panic("floats: matrix buffer too short")
	}
	if k == 0 {
		for i := range m {
			clear(c[i*ldc : i*ldc+n])
		}
		return
	}
	feature.hmm(transA, transB, m, n, k, a, lda, b, ldb, c, ldc)
}

// hMatrixFits checks (rows-1)*stride+cols <= size without overflowing int.
// Callers validate dimensions and strides before calling it.
func hMatrixFits(rows, cols, stride, size int) bool {
	if rows == 0 || cols == 0 {
		return true
	}
	return size >= cols && (rows == 1 || (stride > 0 && rows-1 <= (size-cols)/stride))
}

func hAddFloat(dst, s []uint16) {
	for i := range dst {
		dst[i] = float16.Fromfloat32(float16.Frombits(dst[i]).Float32() + float16.Frombits(s[i]).Float32()).Bits()
	}
}

func hAddConst(dst []uint16, c float32) {
	for i := range dst {
		dst[i] = float16.Fromfloat32(float16.Frombits(dst[i]).Float32() + c).Bits()
	}
}

func hMulTo(a, b, dst []uint16) {
	for i := range dst {
		dst[i] = float16.Fromfloat32(float16.Frombits(a[i]).Float32() * float16.Frombits(b[i]).Float32()).Bits()
	}
}

func hmm(transA, transB bool, m, n, k int, a []uint16, lda int, b []uint16, ldb int, c []uint16, ldc int) {
	for i := range m {
		for j := range n {
			var sum float32
			for l := range k {
				ai, bi := i*lda+l, l*ldb+j
				if transA {
					ai = l*lda + i
				}
				if transB {
					bi = j*ldb + l
				}
				// Explicit FP32 product rounding also prevents fused multiply-add.
				product := float32(float16.Frombits(a[ai]).Float32() * float16.Frombits(b[bi]).Float32())
				sum += product
			}
			c[i*ldc+j] = float16.Fromfloat32(sum).Bits()
		}
	}
}
