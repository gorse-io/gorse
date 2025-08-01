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

	"golang.org/x/sys/cpu"
)

//go:generate goat src/floats_rvv.c -O3 -march=rv64imafdv

type Feature uint64

const V Feature = 1

var feature Feature

func init() {
	if cpu.RISCV64.HasV {
		feature = feature | V
	}
}

func (feature Feature) String() string {
	if feature == V {
		return "RVV"
	} else {
		return "RV"
	}
}

func (feature Feature) mulConstAddTo(a []float32, b float32, c []float32, dst []float32) {
	if feature&V == V {
		vmul_const_add_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), unsafe.Pointer(&dst[0]), int64(len(a)))
	} else {
		mulConstAddTo(a, b, c, dst)
	}
}

func (feature Feature) mulConstAdd(a []float32, b float32, c []float32) {
	if feature&V == V {
		vmul_const_add(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), int64(len(a)))
	} else {
		mulConstAdd(a, b, c)
	}
}

func (feature Feature) mulConstTo(a []float32, b float32, c []float32) {
	if feature&V == V {
		vmul_const_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), int64(len(a)))
	} else {
		mulConstTo(a, b, c)
	}
}

func (feature Feature) addConst(a []float32, b float32) {
	if feature&V == V {
		vadd_const(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), int64(len(a)))
	} else {
		addConst(a, b)
	}
}

func (feature Feature) sub(a, b []float32) {
	if feature&V == V {
		vsub(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	} else {
		sub(a, b)
	}
}

func (feature Feature) subTo(a, b, c []float32) {
	if feature&V == V {
		vsub_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(len(a)))
	} else {
		subTo(a, b, c)
	}
}

func (feature Feature) mulTo(a, b, c []float32) {
	if feature&V == V {
		vmul_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(len(a)))
	} else {
		mulTo(a, b, c)
	}
}

func (feature Feature) mulConst(a []float32, b float32) {
	if feature&V == V {
		vmul_const(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), int64(len(a)))
	} else {
		mulConst(a, b)
	}
}

func (feature Feature) divTo(a, b, c []float32) {
	if feature&V == V {
		vdiv_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(len(a)))
	} else {
		divTo(a, b, c)
	}
}

func (feature Feature) sqrtTo(a, b []float32) {
	if feature&V == V {
		vsqrt_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	} else {
		sqrtTo(a, b)
	}
}

func (feature Feature) dot(a, b []float32) float32 {
	if feature&V == V {
		return vdot(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	} else {
		return dot(a, b)
	}
}

func (feature Feature) euclidean(a, b []float32) float32 {
	if feature&V == V {
		return veuclidean(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
	} else {
		return euclidean(a, b)
	}
}

func (feature Feature) mm(transA, transB bool, m, n, k int, a []float32, lda int, b []float32, ldb int, c []float32, ldc int) {
	if feature&V == V {
		vmm(transA, transB, int64(m), int64(n), int64(k), unsafe.Pointer(&a[0]), int64(lda), unsafe.Pointer(&b[0]), int64(ldb), unsafe.Pointer(&c[0]), int64(ldc))
	} else {
		mm(transA, transB, m, n, k, a, lda, b, ldb, c, ldc)
	}
}
