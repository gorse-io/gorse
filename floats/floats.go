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

// SubTo subtracts one vector by another and saves the result in dst: dst = a - b
func SubTo(a, b, dst []float32) {
	if len(dst) != len(b) || len(dst) != len(b) {
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
