//go:build noasm || (!amd64 && !arm64)

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

func (Feature) hAddFloat(dst, s []uint16) {
	hAddFloat(dst, s)
}

func (Feature) hAddConst(dst []uint16, c float32) {
	hAddConst(dst, c)
}

func (Feature) hMulTo(a, b, dst []uint16) {
	hMulTo(a, b, dst)
}

func (Feature) hmm(transA, transB bool, m, n, k int, a []uint16, lda int, b []uint16, ldb int, c []uint16, ldc int) {
	hmm(transA, transB, m, n, k, a, lda, b, ldb, c, ldc)
}
