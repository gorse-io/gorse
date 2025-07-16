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
)

//go:generate goat src/floats_neon.c -O3

type Feature uint64

const (
	AMX Feature = 1 << iota // Apple matrix extension
	CUDA
)

var feature Feature

func (feature Feature) String() string {
	var features = []string{"ARM64"}
	if feature&AMX > 0 {
		features = append(features, "AMX")
	}
	return strings.Join(features, "+")
}

func (feature Feature) mulConstAddTo(a []float32, b float32, c, dst []float32) {
	vmul_const_add_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), unsafe.Pointer(&dst[0]), int64(len(a)))
}

func (feature Feature) mulConstAdd(a []float32, b float32, c []float32) {
	vmul_const_add(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), int64(len(a)))
}

func (feature Feature) mulConstTo(a []float32, b float32, c []float32) {
	vmul_const_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), unsafe.Pointer(&c[0]), int64(len(a)))
}

func (feature Feature) addConst(a []float32, b float32) {
	vadd_const(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), int64(len(a)))
}

func (feature Feature) sub(a, b []float32) {
	vsub(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
}

func (feature Feature) subTo(a, b, c []float32) {
	vsub_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(len(a)))
}

func (feature Feature) mulTo(a, b, c []float32) {
	vmul_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(len(a)))
}

func (feature Feature) divTo(a, b, c []float32) {
	vdiv_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(len(a)))
}

func (feature Feature) sqrtTo(a, b []float32) {
	vsqrt_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
}

func (feature Feature) mulConst(a []float32, b float32) {
	vmul_const(unsafe.Pointer(&a[0]), unsafe.Pointer(&b), int64(len(a)))
}

func (feature Feature) dot(a, b []float32) float32 {
	return vdot(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
}

func (feature Feature) euclidean(a, b []float32) float32 {
	return veuclidean(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int64(len(a)))
}

func (feature Feature) mm(a, b, c []float32, m, n, k int, transA, transB bool) {
	if feature&AMX == AMX || feature&CUDA == CUDA {
		mm(a, b, c, m, n, k, transA, transB)
	} else {
		vmm(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]), int64(m), int64(n), int64(k), transA, transB)
	}
}
