//go:build cgo && cuda

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

import "github.com/zhenghaoz/gorse/common/blas"

func init() {
	feature = feature | CUDA
}

func mm(a, b, c []float32, m, n, k int, transA, transB bool) {
	var err *blas.Error
	if !transA && !transB {
		err = blas.SGEMM(blas.RowMajor, blas.NoTrans, blas.NoTrans, m, n, k, 1.0,
			a, k, b, n, 0, c, n)
	} else if !transA && transB {
		err = blas.SGEMM(blas.RowMajor, blas.NoTrans, blas.Trans, m, n, k, 1.0,
			a, k, b, k, 0, c, n)
	} else if transA && !transB {
		err = blas.SGEMM(blas.RowMajor, blas.Trans, blas.NoTrans, m, n, k, 1.0,
			a, m, b, n, 0, c, n)
	} else {
		err = blas.SGEMM(blas.RowMajor, blas.Trans, blas.Trans, m, n, k, 1.0,
			a, m, b, k, 0, c, n)
	}
	if err != nil {
		panic(err)
	}
}
