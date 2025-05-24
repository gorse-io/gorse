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

func mm(a, b, c []float32, m, n, k int, transA, transB bool) {
	if !transA && !transB {
		for i := 0; i < m; i++ {
			for l := 0; l < k; l++ {
				// C_l += A_{il} * B_i
				MulConstAdd(b[l*n:(l+1)*n], a[i*k+l], c[i*n:(i+1)*n])
			}
		}
	} else if !transA && transB {
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				c[i*n+j] = Dot(a[i*k:i*k+k], b[j*k:j*k+k])
			}
		}
	} else if transA && !transB {
		for i := 0; i < m; i++ {
			for l := 0; l < k; l++ {
				// C_j += A_{ji} * B_i
				MulConstAdd(b[l*n:(l+1)*n], a[l*m+i], c[i*n:(i+1)*n])
			}
		}
	} else {
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				for l := 0; l < k; l++ {
					c[i*n+j] += a[l*m+i] * b[j*k+l]
				}
			}
		}
	}
}
