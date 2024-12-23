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
	"unsafe"

	"github.com/klauspost/cpuid/v2"
)

//go:generate goat src/floats_avx.c -O3 -mavx
//go:generate goat src/floats_avx512.c -O3 -mavx -mfma -mavx512f -mavx512dq

var impl = Default

func init() {
	if cpuid.CPU.Supports(cpuid.AVX512F, cpuid.AVX512DQ) {
		impl = AVX512
	} else if cpuid.CPU.Supports(cpuid.AVX) {
		impl = AVX
	}
}

type implementation int

const (
	Default implementation = iota
	AVX
	AVX512
)

func (i implementation) String() string {
	switch i {
	case AVX:
		return "avx"
	case AVX512:
		return "avx512"
	default:
		return "default"
	}
}

func (i implementation) mulConstAddTo(a []float32, b float32, c []float32) {
	switch i {
	case AVX:
		_mm256_mul_const_add_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
	case AVX512:
		_mm512_mul_const_add_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
	default:
		mulConstAddTo(a, b, c)
	}
}

func (i implementation) mulConstTo(a []float32, b float32, c []float32) {
	switch i {
	case AVX:
		_mm256_mul_const_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
	case AVX512:
		_mm512_mul_const_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
	default:
		mulConstTo(a, b, c)
	}
}

func (i implementation) mulTo(a, b, c []float32) {
	switch i {
	case AVX:
		_mm256_mul_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
	case AVX512:
		_mm512_mul_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
	default:
		mulTo(a, b, c)
	}
}

func (i implementation) mulConst(a []float32, b float32) {
	switch i {
	case AVX:
		_mm256_mul_const(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(uintptr(len(a))))
	case AVX512:
		_mm512_mul_const(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(uintptr(len(a))))
	default:
		mulConst(a, b)
	}
}

func (i implementation) dot(a, b []float32) float32 {
	switch i {
	case AVX:
		var ret float32
		_mm256_dot(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(uintptr(len(a))), unsafe.Pointer(&ret))
		return ret
	case AVX512:
		var ret float32
		_mm512_dot(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(uintptr(len(a))), unsafe.Pointer(&ret))
		return ret
	default:
		return dot(a, b)
	}
}

func (i implementation) euclidean(a, b []float32) float32 {
	switch i {
	case AVX:
		var ret float32
		_mm256_euclidean(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(uintptr(len(a))), unsafe.Pointer(&ret))
		return ret
	case AVX512:
		var ret float32
		_mm512_euclidean(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(uintptr(len(a))), unsafe.Pointer(&ret))
		return ret
	default:
		return euclidean(a, b)
	}
}
