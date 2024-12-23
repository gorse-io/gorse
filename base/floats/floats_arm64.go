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

import "unsafe"

//go:generate goat src/floats_neon.c -O3

var impl = Neon

type implementation int

const (
	Default implementation = iota
	Neon
)

func (i implementation) String() string {
	switch i {
	case Neon:
		return "neon"
	default:
		return "default"
	}
}

func (i implementation) mulConstAddTo(a []float32, b float32, c []float32) {
	if i == Neon {
		vmul_const_add_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
	} else {
		mulConstAddTo(a, b, c)
	}
}

func (i implementation) mulConstTo(a []float32, b float32, c []float32) {
	if i == Neon {
		vmul_const_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
	} else {
		mulConstTo(a, b, c)
	}
}

func (i implementation) mulTo(a, b, c []float32) {
	if i == Neon {
		vmul_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), unsafe.Pointer(uintptr(len(a))))
	} else {
		mulTo(a, b, c)
	}
}

func (i implementation) mulConst(a []float32, b float32) {
	if i == Neon {
		vmul_const(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(uintptr(len(a))))
	} else {
		mulConst(a, b)
	}
}

func (i implementation) dot(a, b []float32) float32 {
	if i == Neon {
		var ret float32
		vdot(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(uintptr(len(a))), unsafe.Pointer(&ret))
		return ret
	} else {
		return dot(a, b)
	}
}

func (i implementation) euclidean(a, b []float32) float32 {
	if i == Neon {
		var ret float32
		veuclidean(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(uintptr(len(a))), unsafe.Pointer(&ret))
		return ret
	} else {
		return euclidean(a, b)
	}
}
