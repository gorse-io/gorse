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

package hnsw

type Vector interface {
	Distance(vector Vector) float32
	Match(condition string) bool
}

type VectorIndex interface {
	Search(q Vector, n int) ([]int32, []float32)
	SearchConditional(q Vector, condition string, n int) ([]int32, []float32)
}
