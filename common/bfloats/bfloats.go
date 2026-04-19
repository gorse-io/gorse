// Copyright 2020 gorse Project Authors
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

package bfloats

import (
	"math"

	"github.com/chewxy/math32"
)

func FromFloat32(a []float32) (ret []uint16) {
	ret = make([]uint16, len(a))
	for i := range a {
		ret[i] = uint16(math.Float32bits(a[i]) >> 16)
	}
	return
}

func ToFloat32(a []uint16) (ret []float32) {
	ret = make([]float32, len(a))
	for i := range a {
		ret[i] = math.Float32frombits(uint32(a[i]) << 16)
	}
	return
}

func euclidean(a, b []uint16) (ret float32) {
	for i := range a {
		ai := math.Float32frombits(uint32(a[i]) << 16)
		bi := math.Float32frombits(uint32(b[i]) << 16)
		ret += (ai - bi) * (ai - bi)
	}
	return math32.Sqrt(ret)
}

func Euclidean(a, b []uint16) float32 {
	if len(a) != len(b) {
		panic("floats: slice lengths do not match")
	}
	if len(a) == 0 {
		return 0
	}
	return feature.euclidean(a, b)
}
