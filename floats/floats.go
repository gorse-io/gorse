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
	"github.com/chewxy/math32"
)

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
	for i := range dst {
		dst[i] *= c
	}
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

// Mul two vectors: dst = dst * s
func Mul(dst, s []float32) {
	if len(dst) != len(s) {
		panic("floats: slice lengths do not match")
	}
	for i := range dst {
		dst[i] *= s[i]
	}
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
	for i := range a {
		dst[i] = a[i] * c
	}
}

// MulConstAddTo multiplies a vector and a const, then adds to dst: dst = dst + a * c
func MulConstAddTo(a []float32, c float32, dst []float32) {
	if len(a) != len(dst) {
		panic("floats: slice lengths do not match")
	}
	for i := range a {
		dst[i] += a[i] * c
	}
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

// Dot two vectors.
func Dot(a, b []float32) (ret float32) {
	if len(a) != len(b) {
		panic("floats: slice lengths do not match")
	}
	for i := range a {
		ret += a[i] * b[i]
	}
	return
}

// Min element of a slice of 32-bit floats.
func Min(x []float32) float32 {
	if len(x) == 0 {
		panic("floats: zero slice length")
	}
	min := x[0]
	for _, v := range x[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

// Max element of a slice of 32-bit floats.
func Max(x []float32) float32 {
	if len(x) == 0 {
		panic("floats: zero slice length")
	}
	max := x[0]
	for _, v := range x[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

// Sum of a slice of 32-bit floats.
func Sum(x []float32) float32 {
	sum := float32(0)
	for _, v := range x {
		sum += v
	}
	return sum
}

// Mean of a slice of 32-bit floats.
func Mean(x []float32) float32 {
	return Sum(x) / float32(len(x))
}

// StdDev returns the sample standard deviation.
func StdDev(x []float32) float32 {
	_, variance := MeanVariance(x)
	return math32.Sqrt(variance)
}

// MeanVariance computes the sample mean and unbiased variance, where the mean and variance are
//  \sum_i w_i * x_i / (sum_i w_i)
//  \sum_i w_i (x_i - mean)^2 / (sum_i w_i - 1)
// respectively.
// If weights is nil then all of the weights are 1. If weights is not nil, then
// len(x) must equal len(weights).
// When weights sum to 1 or less, a biased variance estimator should be used.
func MeanVariance(x []float32) (mean, variance float32) {
	// This uses the corrected two-pass algorithm (1.7), from "Algorithms for computing
	// the sample variance: Analysis and recommendations" by Chan, Tony F., Gene H. Golub,
	// and Randall J. LeVeque.

	// note that this will panic if the slice lengths do not match
	mean = Mean(x)
	var (
		ss           float32
		compensation float32
	)
	for _, v := range x {
		d := v - mean
		ss += d * d
		compensation += d
	}
	variance = (ss - compensation*compensation/float32(len(x))) / float32(len(x)-1)
	return
}
