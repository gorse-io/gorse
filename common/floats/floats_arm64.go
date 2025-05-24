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

//go:generate goat src/floats_neon.c -O3

type Feature uint64

const (
	ASIMD Feature = 1 << iota
	AMX           // Apple matrix extension
	CUDA
)

var feature Feature

func init() {
	if cpuid.CPU.Supports(cpuid.ASIMD) {
		feature = feature | ASIMD
	}
}

func (feature Feature) String() string {
	var features []string
	if feature&ASIMD > 0 {
		features = append(features, "ASIMD")
	}
	if len(features) == 0 {
		return "ARM64"
	}
	return strings.Join(features, "+")
}

func (feature Feature) mulConstAddTo(a []float32, b float32, c, dst []float32) {
	if feature&ASIMD == ASIMD {
		vmul_const_add_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), unsafe.Pointer(&dst[0]), int64(len(a)))
	} else {
		mulConstAddTo(a, b, c, dst)
	}
}

func (feature Feature) mulConstAdd(a []float32, b float32, c []float32) {
	if feature&ASIMD == ASIMD {
		vmul_const_add(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), int64(len(a)))
	} else {
		mulConstAdd(a, b, c)
	}
}

func (feature Feature) mulConstTo(a []float32, b float32, c []float32) {
	if feature&ASIMD == ASIMD {
		vmul_const_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), int64(len(a)))
	} else {
		mulConstTo(a, b, c)
	}
}

func (feature Feature) addConst(a []float32, b float32) {
	if feature&ASIMD == ASIMD {
		vadd_const(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), int64(len(a)))
	} else {
		addConst(a, b)
	}
}

func (feature Feature) sub(a, b []float32) {
	if feature&ASIMD == ASIMD {
		vsub(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	} else {
		sub(a, b)
	}
}

func (feature Feature) subTo(a, b, c []float32) {
	if feature&ASIMD == ASIMD {
		vsub_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(len(a)))
	} else {
		subTo(a, b, c)
	}
}

func (feature Feature) mulTo(a, b, c []float32) {
	if feature&ASIMD == ASIMD {
		vmul_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(len(a)))
	} else {
		mulTo(a, b, c)
	}
}

func (feature Feature) divTo(a, b, c []float32) {
	if feature&ASIMD == ASIMD {
		vdiv_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(len(a)))
	} else {
		divTo(a, b, c)
	}
}

func (feature Feature) sqrtTo(a, b []float32) {
	if feature&ASIMD == ASIMD {
		vsqrt_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	} else {
		sqrtTo(a, b)
	}
}

func (feature Feature) mulConst(a []float32, b float32) {
	if feature&ASIMD == ASIMD {
		vmul_const(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), int64(len(a)))
	} else {
		mulConst(a, b)
	}
}

func (feature Feature) dot(a, b []float32) float32 {
	if feature&ASIMD == ASIMD {
		return vdot(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	} else {
		return dot(a, b)
	}
}

func (feature Feature) euclidean(a, b []float32) float32 {
	if feature&ASIMD == ASIMD {
		return veuclidean(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	} else {
		return euclidean(a, b)
	}
}

func (feature Feature) mm(a, b, c []float32, m, n, k int, transA, transB bool) {
	if feature&ASIMD == ASIMD && feature&AMX == 0 && feature&CUDA == 0 {
		vmm(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(m), int64(n), int64(k), transA, transB)
	} else {
		mm(a, b, c, m, n, k, transA, transB)
	}
}
