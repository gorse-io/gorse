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

package search

import (
	"github.com/chewxy/math32"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/floats"
	"go.uber.org/zap"
	"modernc.org/sortutil"
	"reflect"
	"sort"
)

type Vector interface {
	Distance(vector Vector) float32
	Terms() []string
}

type DenseVector struct {
	data []float32
}

func NewDenseVector(data []float32) *DenseVector {
	return &DenseVector{
		data: data,
	}
}

func (v *DenseVector) Distance(vector Vector) float32 {
	feedbackVector, isFeedback := vector.(*DenseVector)
	if !isFeedback {
		base.Logger().Fatal("vector type mismatch",
			zap.String("expect", reflect.TypeOf(v).String()),
			zap.String("actual", reflect.TypeOf(vector).String()))
	}
	return -floats.Dot(v.data, feedbackVector.data)
}

func (v *DenseVector) Terms() []string {
	return nil
}

type DictionaryVector struct {
	terms   []string
	indices []int32
	values  []float32
	norm    float32
}

func NewDictionaryVector(indices []int32, values []float32, terms []string) *DictionaryVector {
	sort.Sort(sortutil.Int32Slice(indices))
	var norm float32
	for _, i := range indices {
		norm += values[i]
	}
	norm = math32.Sqrt(norm)
	return &DictionaryVector{
		terms:   terms,
		indices: indices,
		values:  values,
		norm:    norm,
	}
}

func (v *DictionaryVector) Dot(vector *DictionaryVector) float32 {
	i, j, sum := 0, 0, float32(0)
	for i < len(v.indices) && j < len(vector.indices) {
		if v.indices[i] == vector.indices[j] {
			sum += v.values[v.indices[i]]
			i++
			j++
		} else if v.indices[i] < vector.indices[j] {
			i++
		} else if v.indices[i] > vector.indices[j] {
			j++
		}
	}
	return sum
}

func (v *DictionaryVector) Distance(vector Vector) float32 {
	var score float32
	if dictVec, isDictVec := vector.(*DictionaryVector); !isDictVec {
		base.Logger().Fatal("vector type mismatch",
			zap.String("expect", reflect.TypeOf(v).String()),
			zap.String("actual", reflect.TypeOf(vector).String()))
	} else {
		dot := v.Dot(dictVec)
		if dot > 0 {
			score = -dot / v.norm / dictVec.norm
		}
	}
	return score
}

func (v *DictionaryVector) Terms() []string {
	return v.terms
}

type VectorIndex interface {
	Build()
	Search(q Vector, n int, prune0 bool) ([]int32, []float32)
	MultiSearch(q Vector, terms []string, n int, prune0 bool) (map[string][]int32, map[string][]float32)
}
