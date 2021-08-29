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
package floats

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMatZero(t *testing.T) {
	a := [][]float32{
		{3, 2, 5, 6, 0, 0},
		{1, 2, 3, 4, 5, 6},
	}
	MatZero(a)
	assert.Equal(t, [][]float32{
		{0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0},
	}, a)
}

func TestZero(t *testing.T) {
	a := []float32{3, 2, 5, 6, 0, 0}
	Zero(a)
	assert.Equal(t, []float32{0, 0, 0, 0, 0, 0}, a)
}

func TestMin(t *testing.T) {
	a := []float32{3, 2, 5, 6, 0, 0}
	assert.Equal(t, float32(0), Min(a))
	assert.Panics(t, func() { Min(nil) })
}

func TestMax(t *testing.T) {
	a := []float32{3, 2, 5, 6, 0, 0}
	assert.Equal(t, float32(6), Max(a))
	assert.Panics(t, func() { Max(nil) })
}

func TestAdd(t *testing.T) {
	a := []float32{1, 2, 3, 4}
	b := []float32{5, 6, 7, 8}
	Add(a, b)
	assert.Equal(t, []float32{6, 8, 10, 12}, a)
	assert.Panics(t, func() { Add([]float32{1}, nil) })
}

func TestSub(t *testing.T) {
	a := []float32{1, 2, 3, 4}
	b := []float32{5, 6, 7, 8}
	Sub(a, b)
	assert.Equal(t, []float32{-4, -4, -4, -4}, a)
	assert.Panics(t, func() { Sub([]float32{1}, nil) })
}

func TestSubTo(t *testing.T) {
	a := []float32{1, 2, 3, 4}
	b := []float32{5, 6, 7, 8}
	c := make([]float32, 4)
	SubTo(a, b, c)
	assert.Equal(t, []float32{-4, -4, -4, -4}, c)
	assert.Panics(t, func() { SubTo([]float32{1}, nil, nil) })
}

func TestMul(t *testing.T) {
	a := []float32{1, 2, 3, 4}
	b := []float32{5, 6, 7, 8}
	Mul(a, b)
	assert.Equal(t, []float32{5, 12, 21, 32}, a)
	assert.Panics(t, func() { Mul([]float32{1}, nil) })
}

func TestMulConst(t *testing.T) {
	a := []float32{1, 2, 3, 4}
	MulConst(a, 2)
	assert.Equal(t, []float32{2, 4, 6, 8}, a)
}

func TestDiv(t *testing.T) {
	a := []float32{1, 4, 9, 16}
	b := []float32{1, 2, 3, 4}
	Div(a, b)
	assert.Equal(t, []float32{1, 2, 3, 4}, a)
	assert.Panics(t, func() { Div([]float32{1}, nil) })
}

func TestMulConstTo(t *testing.T) {
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	dst := make([]float32, 11)
	target := []float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	MulConstTo(a, 2, dst)
	assert.Equal(t, target, dst)
	assert.Panics(t, func() { MulConstTo(nil, 2, dst) })
}

func TestMulConstAddTo(t *testing.T) {
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	dst := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	target := []float32{0, 3, 6, 9, 12, 15, 18, 21, 24, 27, 30}
	MulConstAddTo(a, 2, dst)
	assert.Equal(t, target, dst)
	assert.Panics(t, func() { MulConstAddTo(nil, 1, dst) })
}

func TestMulAddTo(t *testing.T) {
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	c := []float32{0, 3, 6, 9, 12, 15, 18, 21, 24, 27, 30}
	target := []float32{0, 5, 14, 27, 44, 65, 90, 119, 152, 189, 230}
	MulAddTo(a, b, c)
	assert.Equal(t, target, c)
	assert.Panics(t, func() { MulAddTo(nil, nil, c) })
}

func TestAddTo(t *testing.T) {
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	dst := make([]float32, 11)
	target := []float32{0, 3, 6, 9, 12, 15, 18, 21, 24, 27, 30}
	AddTo(a, b, dst)
	assert.Equal(t, target, dst)
	assert.Panics(t, func() { AddTo(nil, nil, dst) })
}

func TestDot(t *testing.T) {
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	assert.Equal(t, float32(770), Dot(a, b))
	assert.Panics(t, func() { Dot([]float32{1}, nil) })
}

func TestSum(t *testing.T) {
	a := []float32{3, 2, 5, 6, 0, 0}
	assert.Equal(t, float32(16), Sum(a))
}

func TestMean(t *testing.T) {
	a := []float32{3, 2, 5, 6, 0, 0, 0, 0}
	assert.Equal(t, float32(2), Mean(a))
}

func TestStdDev(t *testing.T) {
	a := []float32{1, 3, 5}
	assert.Equal(t, float32(2), StdDev(a))
}
