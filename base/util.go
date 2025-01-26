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

package base

import (
	"fmt"
	"strings"

	"github.com/zhenghaoz/gorse/base/log"
	"go.uber.org/zap"
)

// RangeInt generate a slice [0, ..., n-1].
func RangeInt(n int) []int {
	a := make([]int, n)
	for i := range a {
		a[i] = i
	}
	return a
}

// RepeatFloat32s repeats value n times.
func RepeatFloat32s(n int, value float32) []float32 {
	a := make([]float32, n)
	for i := range a {
		a[i] = value
	}
	return a
}

// NewMatrix32 creates a 2D matrix of 32-bit floats.
func NewMatrix32(row, col int) [][]float32 {
	ret := make([][]float32, row)
	for i := range ret {
		ret[i] = make([]float32, col)
	}
	return ret
}

// NewTensor32 creates a 3D tensor of 32-bit floats.
func NewTensor32(a, b, c int) [][][]float32 {
	ret := make([][][]float32, a)
	for i := range ret {
		ret[i] = NewMatrix32(b, c)
	}
	return ret
}

// NewMatrixInt creates a 2D matrix of integers.
func NewMatrixInt(row, col int) [][]int {
	ret := make([][]int, row)
	for i := range ret {
		ret[i] = make([]int, col)
	}
	return ret
}

// CheckPanic catches panic.
func CheckPanic() {
	if r := recover(); r != nil {
		log.Logger().Error("panic recovered", zap.Any("panic", r))
	}
}

// ValidateId validates user/item id. Id cannot be empty and contain [/,].
func ValidateId(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("id cannot be empty")
	} else if strings.Contains(text, "/") {
		return fmt.Errorf("id cannot contain `/`")
	}
	return nil
}
