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

type Order int

const RowMajor Order = 101

type Transpose int

const (
	NoTrans Transpose = 111
	Trans   Transpose = 112
)

func NewTranspose(transpose bool) Transpose {
	if transpose {
		return Trans
	} else {
		return NoTrans
	}
}
