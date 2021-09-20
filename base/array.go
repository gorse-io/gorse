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

type Integers struct {
	Data [][]int32
}

func (i *Integers) Len() int {
	if len(i.Data) == 0 {
		return 0
	}
	return len(i.Data)*batchSize - batchSize + len(i.Data[len(i.Data)-1])
}

func (i *Integers) Get(index int) int32 {
	return i.Data[index/batchSize][index%batchSize]
}

func (i *Integers) Append(val int32) {
	if len(i.Data) == 0 || len(i.Data[len(i.Data)-1]) == batchSize {
		i.Data = append(i.Data, make([]int32, 0, batchSize))
	}
	i.Data[len(i.Data)-1] = append(i.Data[len(i.Data)-1], val)
}

type Floats struct {
	Data [][]float32
}

func (i *Floats) Len() int {
	if len(i.Data) == 0 {
		return 0
	}
	return len(i.Data)*batchSize - batchSize + len(i.Data[len(i.Data)-1])
}

func (i *Floats) Get(index int) float32 {
	return i.Data[index/batchSize][index%batchSize]
}

func (i *Floats) Append(val float32) {
	if len(i.Data) == 0 || len(i.Data[len(i.Data)-1]) == batchSize {
		i.Data = append(i.Data, make([]float32, 0, batchSize))
	}
	i.Data[len(i.Data)-1] = append(i.Data[len(i.Data)-1], val)
}
