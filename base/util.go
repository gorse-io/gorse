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
	"go.uber.org/zap"
	"time"
)

var logger *zap.Logger

func init() {
	SetProductionLogger()
}

// Logger get current logger
func Logger() *zap.Logger {
	return logger
}

// SetProductionLogger set current logger in production mode.
func SetProductionLogger() {
	logger, _ = zap.NewProduction()
}

// SetDevelopmentLogger set current logger in development mode.
func SetDevelopmentLogger() {
	logger, _ = zap.NewDevelopment()
}

// Max finds the maximum in a vector of integers. Panic if the slice is empty.
func Max(a int32, b ...int32) int32 {
	for _, value := range b {
		if value > a {
			a = value
		}
	}
	return a
}

// Min finds the minimum in a vector of integers. Panic if the slice is empty.
func Min(a int, b ...int) int {
	for _, value := range b {
		if value < a {
			a = value
		}
	}
	return a
}

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

// NewMatrixInt creates a 2D matrix of integers.
func NewMatrixInt(row, col int) [][]int {
	ret := make([][]int, row)
	for i := range ret {
		ret[i] = make([]int, col)
	}
	return ret
}

// Now returns the current time in the format of `2006-01-02T15:04:05Z07:00`.
func Now() string {
	return time.Now().Format("2006-01-02T15:04:05Z07:00")
}

func DateNow() time.Time {
	timestamp := time.Now()
	return time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, time.UTC)
}

// CheckPanic catches panic.
func CheckPanic() {
	if r := recover(); r != nil {
		Logger().Error("panic recovered", zap.Any("panic", r))
	}
}

// Hex returns the hex form of a 64-bit integer.
func Hex(v int64) string {
	return fmt.Sprintf("%x", v)
}
