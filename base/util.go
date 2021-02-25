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
	"log"
	"time"
)

// Max finds the maximum in a vector of integers. Panic if the slice is empty.
func Max(a ...int) int {
	if len(a) == 0 {
		log.Panicf("Can't get the maximum from empty vec")
	}
	maximum := a[0]
	for _, m := range a {
		if m > maximum {
			maximum = m
		}
	}
	return maximum
}

func Min(a ...int) int {
	if len(a) == 0 {
		log.Panicf("Can't get the minimum from empty vec")
	}
	minimum := a[0]
	for _, m := range a {
		if m < minimum {
			minimum = m
		}
	}
	return minimum
}

func NewMatrix32(row, col int) [][]float32 {
	ret := make([][]float32, row)
	for i := range ret {
		ret[i] = make([]float32, col)
	}
	return ret
}

func NewMatrixInt(row, col int) [][]int {
	ret := make([][]int, row)
	for i := range ret {
		ret[i] = make([]int, col)
	}
	return ret
}

func Now() string {
	return time.Now().Format("2006-01-02 3:4:5")
}
