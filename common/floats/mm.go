//go:build !cgo || (!(darwin && arm64) && !cuda)

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

func mm(transA, transB bool, m, n, k int, a []float32, lda int, b []float32, ldb int, c []float32, ldc int) {
	if !transA && !transB {
		for i := 0; i < m; i++ {
			for l := 0; l < k; l++ {
				// C_l += A_{il} * B_i
				MulConstAdd(b[l*ldb:(l+1)*ldb], a[i*lda+l], c[i*ldc:(i+1)*ldc])
			}
		}
	} else if !transA && transB {
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				c[i*ldc+j] = Dot(a[i*lda:(i+1)*lda], b[j*ldb:(j+1)*ldb])
			}
		}
	} else if transA && !transB {
		for i := 0; i < m; i++ {
			for l := 0; l < k; l++ {
				// C_j += A_{ji} * B_i
				MulConstAdd(b[l*ldb:(l+1)*ldb], a[l*lda+i], c[i*ldc:(i+1)*ldc])
			}
		}
	} else {
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				for l := 0; l < k; l++ {
					c[i*ldc+j] += a[l*lda+i] * b[j*ldb+l]
				}
			}
		}
	}
}
