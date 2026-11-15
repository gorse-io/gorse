//go:build !noasm

// Copyright 2026 gorse Project Authors
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

//go:generate go tool goat src/float16_neon.c --target arm64 -O3

// ASIMD includes FP16 storage conversions on baseline ARM64. These kernels
// never use the optional ARMv8.2 native FP16 arithmetic instructions.
func (feature Feature) hAddFloat(dst, s []uint16) {
	if !cpu.ARM64.HasASIMD {
		hAddFloat(dst, s)
		return
	}
	vhadd_float(unsafe.Pointer(&dst[0]), unsafe.Pointer(&s[0]), int64(len(dst)))
}

func (feature Feature) hAddConst(dst []uint16, c float32) {
	if !cpu.ARM64.HasASIMD {
		hAddConst(dst, c)
		return
	}
	vhadd_const(unsafe.Pointer(&dst[0]), unsafe.Pointer(&c), int64(len(dst)))
}

func (feature Feature) hMulTo(a, b, dst []uint16) {
	if !cpu.ARM64.HasASIMD {
		hMulTo(a, b, dst)
		return
	}
	vhmul_to(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&dst[0]), int64(len(dst)))
}

func (feature Feature) hmm(transA, transB bool, m, n, k int, a []uint16, lda int, b []uint16, ldb int, c []uint16, ldc int) {
	if !cpu.ARM64.HasASIMD {
		hmm(transA, transB, m, n, k, a, lda, b, ldb, c, ldc)
		return
	}
	vhmm(transA, transB, int64(m), int64(n), int64(k), unsafe.Pointer(&a[0]), int64(lda), unsafe.Pointer(&b[0]), int64(ldb), unsafe.Pointer(&c[0]), int64(ldc))
}
