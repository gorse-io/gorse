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

package master

import (
	"fmt"
	"github.com/bits-and-blooms/bitset"
	"github.com/chewxy/math32"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base/search"
	"reflect"
)

type VectorsInterface interface {
	Distance(i, j int) float32
	Neighbors(i int) []int32
}

type Vectors struct {
	connections [][]int32
	connected   [][]int32
	weights     []float32
}

func NewVectors(connections, connected [][]int32, weights []float32) *Vectors {
	if len(connected) != len(weights) {
		panic("the length of connected and weights doesn't match")
	}
	return &Vectors{
		connections: connections,
		connected:   connected,
		weights:     weights,
	}
}

func (v *Vectors) Distance(i, j int) float32 {
	commonSum, commonCount := commonElements(v.connections[i], v.connections[j], v.weights)
	if commonCount > 0 {
		return commonSum * commonCount /
			math32.Sqrt(weightedSum(v.connections[i], v.weights)) /
			math32.Sqrt(weightedSum(v.connections[j], v.weights)) /
			(commonCount + similarityShrink)
	} else {
		return 0
	}
}

func (v *Vectors) Neighbors(i int) []int32 {
	connections := v.connections[i]
	bitSet := bitset.New(uint(len(connections)))
	var adjacent []int32
	for _, p := range connections {
		for _, neighbor := range v.connected[p] {
			if !bitSet.Test(uint(neighbor)) {
				bitSet.Set(uint(neighbor))
				adjacent = append(adjacent, neighbor)
			}
		}
	}
	return adjacent
}

type DualVectors struct {
	first  *Vectors
	second *Vectors
}

func NewDualVectors(first, second *Vectors) *DualVectors {
	if len(first.connections) != len(second.connections) {
		panic("the number of connections mismatch")
	}
	return &DualVectors{
		first:  first,
		second: second,
	}
}

func (v *DualVectors) Distance(i, j int) float32 {
	return (v.first.Distance(i, j) + v.second.Distance(i, j)) / 2
}

func (v *DualVectors) Neighbors(i int) []int32 {
	connections := v.first.connections[i]
	bitSet := bitset.New(uint(len(connections)))
	var adjacent []int32
	// iterate the first group
	for _, p := range connections {
		for _, neighbor := range v.first.connected[p] {
			if !bitSet.Test(uint(neighbor)) {
				bitSet.Set(uint(neighbor))
				adjacent = append(adjacent, neighbor)
			}
		}
	}
	// iterate the second group
	for _, p := range v.second.connections[i] {
		for _, neighbor := range v.second.connected[p] {
			if !bitSet.Test(uint(neighbor)) {
				bitSet.Set(uint(neighbor))
				adjacent = append(adjacent, neighbor)
			}
		}
	}
	return adjacent
}

type DualDictionaryVector struct {
	first  *search.DictionaryVector
	second *search.DictionaryVector
}

func NewDualDictionaryVector(
	indices1 []int32, values1 []float32,
	indices2 []int32, values2 []float32,
	terms []string, isHidden bool) *DualDictionaryVector {
	return &DualDictionaryVector{
		first:  search.NewDictionaryVector(indices1, values1, terms, isHidden),
		second: search.NewDictionaryVector(indices2, values2, terms, isHidden),
	}
}

func (v *DualDictionaryVector) Distance(vector search.Vector) float32 {
	switch typedVector := vector.(type) {
	case *DualDictionaryVector:
		return (v.first.Distance(typedVector.first) + v.second.Distance(typedVector.second)) / 2
	default:
		panic(fmt.Sprintf("unexpected vector type: %v", reflect.TypeOf(vector)))
	}
}

func (v *DualDictionaryVector) Terms() []string {
	return v.first.Terms()
}

func (v *DualDictionaryVector) IsHidden() bool {
	return v.first.IsHidden()
}

func (v *DualDictionaryVector) Centroid(vectors []search.Vector, indices []int32) search.CentroidVector {
	return &DualDictionaryCentroidVector{
		first: search.DictionaryVector{}.Centroid(lo.Map(vectors, func(v search.Vector, _ int) search.Vector {
			switch typedVector := v.(type) {
			case *DualDictionaryVector:
				return typedVector.first
			default:
				panic(fmt.Sprintf("unexpected vector type: %v", reflect.TypeOf(v)))
			}
		}), indices),
		second: search.DictionaryVector{}.Centroid(lo.Map(vectors, func(v search.Vector, _ int) search.Vector {
			switch typedVector := v.(type) {
			case *DualDictionaryVector:
				return typedVector.second
			default:
				panic(fmt.Sprintf("unexpected vector type: %v", reflect.TypeOf(v)))
			}
		}), indices),
	}
}

type DualDictionaryCentroidVector struct {
	first  search.CentroidVector
	second search.CentroidVector
}

func (d *DualDictionaryCentroidVector) Distance(vector search.Vector) float32 {
	switch typedVector := vector.(type) {
	case *DualDictionaryVector:
		return (d.first.Distance(typedVector.first) + d.second.Distance(typedVector.second)) / 2
	default:
		panic(fmt.Sprintf("unexpected vector type: %v", reflect.TypeOf(vector)))
	}
}
