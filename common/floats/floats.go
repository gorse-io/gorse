// Copyright 2020 gorse Project Authors
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
	"math"

	"github.com/chewxy/math32"
)

func dot(a, b []float32) (ret float32) {
	for i := range a {
		ret += a[i] * b[i]
	}
	return
}

func euclidean(a, b []float32) (ret float32) {
	for i := range a {
		ret += (a[i] - b[i]) * (a[i] - b[i])
	}
	return math32.Sqrt(ret)
}

func addConst(a []float32, c float32) {
	for i := range a {
		a[i] += c
	}
}

func sub(a, b []float32) {
	for i := range a {
		a[i] -= b[i]
	}
}

func subTo(a, b, c []float32) {
	for i := range a {
		c[i] = a[i] - b[i]
	}
}

func mulTo(a, b, c []float32) {
	for i := range a {
		c[i] = a[i] * b[i]
	}
}

func mulConstAddTo(a []float32, b float32, c []float32, dst []float32) {
	for i := range a {
		dst[i] = a[i]*b + c[i]
	}
}

func mulConstAdd(a []float32, c float32, dst []float32) {
	for i := range a {
		dst[i] += a[i] * c
	}
}

func mulConstTo(a []float32, b float32, c []float32) {
	for i := range a {
		c[i] = a[i] * b
	}
}

func mulConst(a []float32, b float32) {
	for i := range a {
		a[i] *= b
	}
}

func divTo(a, b, c []float32) {
	for i := range a {
		c[i] = a[i] / b[i]
	}
}

func sqrtTo(a, b []float32) {
	for i := range a {
		b[i] = math32.Sqrt(a[i])
	}
}

// MatZero fills zeros in a matrix of 32-bit floats.
func MatZero(x [][]float32) {
	for i := range x {
		for j := range x[i] {
			x[i][j] = 0
		}
	}
}

// Zero fills zeros in a slice of 32-bit floats.
func Zero(a []float32) {
	for i := range a {
		a[i] = 0
	}
}

// SubTo subtracts one vector by another and saves the result in dst: dst = a - b
func SubTo(a, b, dst []float32) {
	if len(dst) != len(b) || len(a) != len(b) {
		panic("floats: slice lengths do not match")
	}
	feature.subTo(a, b, dst)
}

// Add two vectors: dst = dst + s
func Add(dst, s []float32) {
	if len(dst) != len(s) {
		panic("floats: slice lengths do not match")
	}
	for i := range dst {
		dst[i] += s[i]
	}
}

// MulConst multiplies a vector with a const: dst = dst * c
func MulConst(dst []float32, c float32) {
	feature.mulConst(dst, c)
}

// Div one vectors by another: dst = dst / s
func Div(dst, s []float32) {
	if len(dst) != len(s) {
		panic("floats: slice lengths do not match")
	}
	for i := range dst {
		dst[i] /= s[i]
	}
}

func MulTo(a, b, c []float32) {
	if len(a) != len(b) || len(a) != len(c) {
		panic("floats: slice lengths do not match")
	}
	feature.mulTo(a, b, c)
}

// Sub one vector by another: dst = dst - s
func Sub(dst, s []float32) {
	if len(dst) != len(s) {
		panic("floats: slice lengths do not match")
	}
	feature.sub(dst, s)
}

// MulConstTo multiplies a vector and a const, then saves the result in dst: dst = a * c
func MulConstTo(a []float32, c float32, dst []float32) {
	if len(a) != len(dst) {
		panic("floats: slice lengths do not match")
	}
	feature.mulConstTo(a, c, dst)
}

func MulConstAddTo(a []float32, c float32, b, dst []float32) {
	if len(a) != len(b) || len(a) != len(dst) {
		panic("floats: slice lengths do not match")
	}
	feature.mulConstAddTo(a, c, b, dst)
}

// MulConstAddTo multiplies a vector and a const, then adds to dst: dst = dst + a * c
func MulConstAdd(a []float32, c float32, dst []float32) {
	if len(a) != len(dst) {
		panic("floats: slice lengths do not match")
	}
	feature.mulConstAdd(a, c, dst)
}

// MulAddTo multiplies a vector and a vector, then adds to a vector: c += a * b
func MulAddTo(a, b, c []float32) {
	if len(a) != len(b) || len(a) != len(c) {
		panic("floats: slice lengths do not match")
	}
	for i := range a {
		c[i] += a[i] * b[i]
	}
}

// AddTo adds two vectors and saves the result in dst: dst = a + b
func AddTo(a, b, dst []float32) {
	if len(a) != len(b) || len(a) != len(dst) {
		panic("floats: slice lengths do not match")
	}
	for i := range a {
		dst[i] = a[i] + b[i]
	}
}

func AddConst(dst []float32, c float32) {
	feature.addConst(dst, c)
}

func DivTo(a, b, c []float32) {
	if len(a) != len(b) || len(a) != len(c) {
		panic("floats: slice lengths do not match")
	}
	feature.divTo(a, b, c)
}

func SqrtTo(a, b []float32) {
	if len(a) != len(b) {
		panic("floats: slice lengths do not match")
	}
	feature.sqrtTo(a, b)
}

func Sqrt(a []float32) {
	for i := range a {
		a[i] = float32(math.Sqrt(float64(a[i])))
	}
}

// Dot two vectors.
func Dot(a, b []float32) (ret float32) {
	if len(a) != len(b) {
		panic("floats: slice lengths do not match")
	}
	return feature.dot(a, b)
}

func Euclidean(a, b []float32) float32 {
	if len(a) != len(b) {
		panic("floats: slice lengths do not match")
	}
	return feature.euclidean(a, b)
}

func MM(transA, transB bool, m, n, k int, a []float32, lda int, b []float32, ldb int, c []float32, ldc int) {
	feature.mm(transA, transB, m, n, k, a, lda, b, ldb, c, ldc)
}
