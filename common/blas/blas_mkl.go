//go:build cgo && mkl

// Copyright 2025 gorse Project Authors
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

package blas

// #cgo CFLAGS: -I/opt/intel/oneapi/mkl/latest/include
// #cgo LDFLAGS: -L$/opt/intel/oneapi/mkl/latest/lib/intel64 -lmkl_intel_lp64 -lmkl_sequential -lmkl_core -lpthread -lm -ldl
// #include "mkl.h"
import "C"

func SGEMM(order Order, transA, transB Transpose, m, n, k int, alpha float32, a []float32, lda int, b []float32, ldb int, beta float32, c []float32, ldc int) {
	C.cblas_sgemm(C.CBLAS_LAYOUT(order), C.CBLAS_TRANSPOSE(transA), C.CBLAS_TRANSPOSE(transB), C.int(m), C.int(n), C.int(k), C.float(alpha),
		(*C.float)(&a[0]), C.int(lda), (*C.float)(&b[0]), C.int(ldb), C.float(beta), (*C.float)(&c[0]), C.int(ldc))
}
