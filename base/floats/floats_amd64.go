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
	"github.com/klauspost/cpuid/v2"
	"unsafe"
)

func init() {
	if cpuid.CPU.Supports(cpuid.AVX2) {
		impl = avx2{}
	}
}

type avx2 struct{}

func (avx2) MulConstAddTo(a []float32, b float32, c []float32) {
	__mm256_mul_const_add_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
}

func (avx2) MulConstTo(a []float32, b float32, c []float32) {
	__mm256_mul_const_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
}

func (avx2) MulTo(a, b, c []float32) {
	__mm256_mul_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
}

func (avx2) MulConst(a []float32, b float32) {
	__mm256_mul_const(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(uintptr(len(a))))
}

func (avx2) Dot(a, b []float32) float32 {
	var ret float32
	__mm256_dot(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(uintptr(len(a))), unsafe.Pointer(&ret))
	return ret
}

//go:noescape
func __mm256_mul_const_add_to(a, b, c, n unsafe.Pointer)

//go:noescape
func __mm256_mul_const_to(a, b, c, n unsafe.Pointer)

//go:noescape
func __mm256_mul_const(a, b, n unsafe.Pointer)

//go:noescape
func __mm256_mul_to(a, b, c, n unsafe.Pointer)

//go:noescape
func __mm256_dot(a, b, n, ret unsafe.Pointer)
