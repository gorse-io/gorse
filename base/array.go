// Copyright 2021 gorse Project Authors
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

package base

const batchSize = 1024 * 1024

type Array[T any] struct {
	Data [][]T
}

func (a *Array[T]) Len() int {
	if len(a.Data) == 0 {
		return 0
	}
	return len(a.Data)*batchSize - batchSize + len(a.Data[len(a.Data)-1])
}

func (a *Array[T]) Get(index int) T {
	return a.Data[index/batchSize][index%batchSize]
}

func (a *Array[T]) Append(val T) {
	if len(a.Data) == 0 || len(a.Data[len(a.Data)-1]) == batchSize {
		a.Data = append(a.Data, make([]T, 0, batchSize))
	}
	a.Data[len(a.Data)-1] = append(a.Data[len(a.Data)-1], val)
}
