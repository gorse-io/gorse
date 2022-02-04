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
)

func init() {
	impl = neon{}
}

type neon struct{}

func (neon) MulConstAddTo(a []float32, b float32, c []float32) {
	vmul_const_add_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
}

func (neon) MulConstTo(a []float32, b float32, c []float32) {
	vmul_const_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
}

func (neon) MulTo(a, b, c []float32) {
	vmul_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
}

func (neon) MulConst(a []float32, b float32) {
	vmul_const(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(uintptr(len(a))))
}

func (neon) Dot(a, b []float32) float32 {
	var ret float32
	vdot(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(uintptr(len(a))), unsafe.Pointer(&ret))
	return ret
}

//go:noescape
func vmul_const_add_to(a, b, c, n unsafe.Pointer)

//go:noescape
func vmul_const_to(a, b, c, n unsafe.Pointer)

//go:noescape
func vmul_const(a, b, n unsafe.Pointer)

//go:noescape
func vmul_to(a, b, c, n unsafe.Pointer)

//go:noescape
func vdot(a, b, n, ret unsafe.Pointer)
