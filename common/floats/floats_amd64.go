//go:build !noasm

// Copyright 2022 gorse Project Authors
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
	"strings"
	"unsafe"

	"github.com/klauspost/cpuid/v2"
)

//go:generate goat src/floats_avx.c -O3 -mavx
//go:generate goat src/floats_avx512.c -O3 -mavx -mfma -mavx512f

type Feature uint64

const (
	AVX Feature = 1 << iota
	FMA
	AVX512F
	CUDA
)

const AVX512 = AVX | FMA | AVX512F

var feature Feature

func init() {
	if cpuid.CPU.Supports(cpuid.AVX) {
		feature = feature | AVX
	}
	if cpuid.CPU.Supports(cpuid.FMA3) {
		feature = feature | FMA
	}
	if cpuid.CPU.Supports(cpuid.AVX512F) {
		feature = feature | AVX512F
	}
}

func (feature Feature) String() string {
	var features []string
	if feature&AVX512 == AVX512 {
		features = append(features, "AVX512")
	} else if feature&AVX == AVX {
		features = append(features, "AVX")
	}
	if len(features) == 0 {
		return "AMD64"
	}
	return strings.Join(features, "+")
}

func (feature Feature) mulConstAddTo(a []float32, b float32, c []float32) {
	if feature&AVX512 == AVX512 {
		_mm512_mul_const_add_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), int64(len(a)))
	} else if feature&AVX == AVX {
		_mm256_mul_const_add_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), int64(len(a)))
	} else {
		mulConstAddTo(a, b, c)
	}
}

func (feature Feature) mulConstTo(a []float32, b float32, c []float32) {
	if feature&AVX512 == AVX512 {
		_mm512_mul_const_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), int64(len(a)))
	} else if feature&AVX == AVX {
		_mm256_mul_const_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), int64(len(a)))
	} else {
		mulConstTo(a, b, c)
	}
}

func (feature Feature) mulTo(a, b, c []float32) {
	if feature&AVX512 == AVX512 {
		_mm512_mul_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(len(a)))
	} else if feature&AVX == AVX {
		_mm256_mul_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(len(a)))
	} else {
		mulTo(a, b, c)
	}
}

func (feature Feature) mulConst(a []float32, b float32) {
	if feature&AVX512 == AVX512 {
		_mm512_mul_const(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), int64(len(a)))
	} else if feature&AVX == AVX {
		_mm256_mul_const(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), int64(len(a)))
	} else {
		mulConst(a, b)
	}
}

func (feature Feature) dot(a, b []float32) float32 {
	if feature&AVX512 == AVX512 {
		return _mm512_dot(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	} else if feature&AVX == AVX {
		return _mm256_dot(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	} else {
		return dot(a, b)
	}
}

func (feature Feature) euclidean(a, b []float32) float32 {
	if feature&AVX512 == AVX512 {
		return _mm512_euclidean(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	} else if feature&AVX == AVX {
		return _mm256_euclidean(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	} else {
		return euclidean(a, b)
	}
}

func (feature Feature) mm(a, b, c []float32, m, n, k int, transA, transB bool) {
	// Bypass AVX512 optimizations when CUDA is enabled to avoid potential conflicts
	// between CUDA-based and AVX512-based matrix multiplication implementations.
	if feature&AVX512 == AVX512 && feature&CUDA == 0 {
		_mm512_mm(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(m), int64(n), int64(k), transA, transB)
	} else if feature&AVX == AVX && feature&CUDA == 0 {
		_mm256_mm(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(m), int64(n), int64(k), transA, transB)
	} else {
		mm(a, b, c, m, n, k, transA, transB)
	}
}
