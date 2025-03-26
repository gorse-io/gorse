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

func mulTo(a, b, c []float32) {
	for i := range a {
		c[i] = a[i] * b[i]
	}
}

func mulConstAddTo(a []float32, c float32, dst []float32) {
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
	for i := range dst {
		dst[i] = a[i] - b[i]
	}
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
	impl.mulConst(dst, c)
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
	impl.mulTo(a, b, c)
}

// Sub one vector by another: dst = dst - s
func Sub(dst, s []float32) {
	if len(dst) != len(s) {
		panic("floats: slice lengths do not match")
	}
	for i := range dst {
		dst[i] -= s[i]
	}
}

// MulConstTo multiplies a vector and a const, then saves the result in dst: dst = a * c
func MulConstTo(a []float32, c float32, dst []float32) {
	if len(a) != len(dst) {
		panic("floats: slice lengths do not match")
	}
	impl.mulConstTo(a, c, dst)
}

// MulConstAddTo multiplies a vector and a const, then adds to dst: dst = dst + a * c
func MulConstAddTo(a []float32, c float32, dst []float32) {
	if len(a) != len(dst) {
		panic("floats: slice lengths do not match")
	}
	impl.mulConstAddTo(a, c, dst)
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
	for i := range dst {
		dst[i] += c
	}
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
	return impl.dot(a, b)
}

func Euclidean(a, b []float32) float32 {
	if len(a) != len(b) {
		panic("floats: slice lengths do not match")
	}
	return impl.euclidean(a, b)
}
