//go:build cgo

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

package floats

import "github.com/gorse-io/gorse/common/blas"

func init() {
	feature = feature | AMX
}

func mm(transA, transB bool, m, n, k int, a []float32, lda int, b []float32, ldb int, c []float32, ldc int) {
	blas.SGEMM(blas.RowMajor, blas.NewTranspose(transA), blas.NewTranspose(transB),
		m, n, k, 1.0, a, lda, b, ldb, 0, c, ldc)
}
