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
	"simd"

	"github.com/chewxy/math32"
)

func simdDot(a, b []float32) (ret float32) {
	var sum simd.Float32s
	vectorLen := sum.Len()
	for len(a) >= vectorLen {
		x := simd.LoadFloat32s(a)
		y := simd.LoadFloat32s(b)
		sum = x.MulAdd(y, sum)
		a = a[vectorLen:]
		b = b[vectorLen:]
	}

	var values [16]float32
	sum.Store(values[:vectorLen])
	for _, value := range values[:vectorLen] {
		ret += value
	}
	for i := range a {
		ret += a[i] * b[i]
	}
	return ret
}

func simdEuclidean(a, b []float32) (ret float32) {
	var sum simd.Float32s
	vectorLen := sum.Len()
	for len(a) >= vectorLen {
		diff := simd.LoadFloat32s(a).Sub(simd.LoadFloat32s(b))
		sum = diff.MulAdd(diff, sum)
		a = a[vectorLen:]
		b = b[vectorLen:]
	}

	var values [16]float32
	sum.Store(values[:vectorLen])
	for _, value := range values[:vectorLen] {
		ret += value
	}
	for i := range a {
		diff := a[i] - b[i]
		ret += diff * diff
	}
	return math32.Sqrt(ret)
}

func simdMulConstAddTo(a []float32, b float32, c []float32, dst []float32) {
	broadcast := simd.BroadcastFloat32s(b)
	vectorLen := broadcast.Len()
	for len(a) >= vectorLen {
		x := simd.LoadFloat32s(a)
		y := simd.LoadFloat32s(c)
		x.MulAdd(broadcast, y).Store(dst)
		a = a[vectorLen:]
		c = c[vectorLen:]
		dst = dst[vectorLen:]
	}
	if len(a) > 0 {
		x, n := simd.LoadFloat32sPart(a)
		y, _ := simd.LoadFloat32sPart(c)
		x.MulAdd(broadcast, y).StorePart(dst[:n])
	}
}

func simdMulConstAdd(a []float32, b float32, dst []float32) {
	broadcast := simd.BroadcastFloat32s(b)
	vectorLen := broadcast.Len()
	for len(a) >= vectorLen {
		x := simd.LoadFloat32s(a)
		y := simd.LoadFloat32s(dst)
		x.MulAdd(broadcast, y).Store(dst)
		a = a[vectorLen:]
		dst = dst[vectorLen:]
	}
	if len(a) > 0 {
		x, n := simd.LoadFloat32sPart(a)
		y, _ := simd.LoadFloat32sPart(dst)
		x.MulAdd(broadcast, y).StorePart(dst[:n])
	}
}

func simdMulConstTo(a []float32, b float32, dst []float32) {
	broadcast := simd.BroadcastFloat32s(b)
	vectorLen := broadcast.Len()
	for len(a) >= vectorLen {
		simd.LoadFloat32s(a).Mul(broadcast).Store(dst)
		a = a[vectorLen:]
		dst = dst[vectorLen:]
	}
	if len(a) > 0 {
		x, n := simd.LoadFloat32sPart(a)
		x.Mul(broadcast).StorePart(dst[:n])
	}
}

func simdMulConst(a []float32, b float32) {
	broadcast := simd.BroadcastFloat32s(b)
	vectorLen := broadcast.Len()
	for len(a) >= vectorLen {
		simd.LoadFloat32s(a).Mul(broadcast).Store(a)
		a = a[vectorLen:]
	}
	if len(a) > 0 {
		x, n := simd.LoadFloat32sPart(a)
		x.Mul(broadcast).StorePart(a[:n])
	}
}

func simdAddConst(a []float32, b float32) {
	broadcast := simd.BroadcastFloat32s(b)
	vectorLen := broadcast.Len()
	for len(a) >= vectorLen {
		simd.LoadFloat32s(a).Add(broadcast).Store(a)
		a = a[vectorLen:]
	}
	if len(a) > 0 {
		x, n := simd.LoadFloat32sPart(a)
		x.Add(broadcast).StorePart(a[:n])
	}
}

func simdSub(a, b []float32) {
	vectorLen := simd.VectorBitSize() / 32
	for len(a) >= vectorLen {
		simd.LoadFloat32s(a).Sub(simd.LoadFloat32s(b)).Store(a)
		a = a[vectorLen:]
		b = b[vectorLen:]
	}
	if len(a) > 0 {
		x, n := simd.LoadFloat32sPart(a)
		y, _ := simd.LoadFloat32sPart(b)
		x.Sub(y).StorePart(a[:n])
	}
}

func simdSubTo(a, b, dst []float32) {
	vectorLen := simd.VectorBitSize() / 32
	for len(a) >= vectorLen {
		simd.LoadFloat32s(a).Sub(simd.LoadFloat32s(b)).Store(dst)
		a = a[vectorLen:]
		b = b[vectorLen:]
		dst = dst[vectorLen:]
	}
	if len(a) > 0 {
		x, n := simd.LoadFloat32sPart(a)
		y, _ := simd.LoadFloat32sPart(b)
		x.Sub(y).StorePart(dst[:n])
	}
}

func simdMulTo(a, b, dst []float32) {
	vectorLen := simd.VectorBitSize() / 32
	for len(a) >= vectorLen {
		simd.LoadFloat32s(a).Mul(simd.LoadFloat32s(b)).Store(dst)
		a = a[vectorLen:]
		b = b[vectorLen:]
		dst = dst[vectorLen:]
	}
	if len(a) > 0 {
		x, n := simd.LoadFloat32sPart(a)
		y, _ := simd.LoadFloat32sPart(b)
		x.Mul(y).StorePart(dst[:n])
	}
}

func simdDivTo(a, b, dst []float32) {
	vectorLen := simd.VectorBitSize() / 32
	for len(a) >= vectorLen {
		simd.LoadFloat32s(a).Div(simd.LoadFloat32s(b)).Store(dst)
		a = a[vectorLen:]
		b = b[vectorLen:]
		dst = dst[vectorLen:]
	}
	if len(a) > 0 {
		x, n := simd.LoadFloat32sPart(a)
		y, _ := simd.LoadFloat32sPart(b)
		x.Div(y).StorePart(dst[:n])
	}
}

func simdSqrtTo(a, dst []float32) {
	vectorLen := simd.VectorBitSize() / 32
	for len(a) >= vectorLen {
		simd.LoadFloat32s(a).Sqrt().Store(dst)
		a = a[vectorLen:]
		dst = dst[vectorLen:]
	}
	if len(a) > 0 {
		x, n := simd.LoadFloat32sPart(a)
		x.Sqrt().StorePart(dst[:n])
	}
}

func simdMM(transA, transB bool, m, n, k int, a []float32, lda int, b []float32, ldb int, c []float32, ldc int) {
	if !transA && !transB {
		for i := range m {
			for l := range k {
				simdMulConstAdd(b[l*ldb:l*ldb+n], a[i*lda+l], c[i*ldc:i*ldc+n])
			}
		}
	} else if !transA && transB {
		for i := range m {
			for j := range n {
				c[i*ldc+j] = simdDot(a[i*lda:i*lda+k], b[j*ldb:j*ldb+k])
			}
		}
	} else if transA && !transB {
		for i := range m {
			for l := range k {
				simdMulConstAdd(b[l*ldb:l*ldb+n], a[l*lda+i], c[i*ldc:i*ldc+n])
			}
		}
	} else {
		for i := range m {
			for l := range k {
				for j := range n {
					c[i*ldc+j] += a[l*lda+i] * b[j*ldb+l]
				}
			}
		}
	}
}
