// Copyright 2025 gorse Project Authors
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
	"fmt"
	"math"
	"testing"

	"github.com/gorse-io/gorse/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestFloat16Conversion(t *testing.T) {
	values := []float32{
		0,
		float32(math.Copysign(0, -1)),
		1,
		-2,
		float32(math.Ldexp(1, -24)),
		float32(math.Ldexp(1, -25)),
		1 + float32(math.Ldexp(1, -11)),
		1 + float32(math.Ldexp(1, -10)),
		65504,
		float32(math.Inf(1)),
		float32(math.Inf(-1)),
	}
	encoded := []uint16{
		0x0000,
		0x8000,
		0x3c00,
		0xc000,
		0x0001,
		0x0000,
		0x3c00,
		0x3c01,
		0x7bff,
		0x7c00,
		0xfc00,
	}

	assert.Equal(t, encoded, FromFloat32(values))
	assert.Equal(t, []float32{
		0,
		float32(math.Copysign(0, -1)),
		1,
		-2,
		float32(math.Ldexp(1, -24)),
		0,
		1,
		1 + float32(math.Ldexp(1, -10)),
		65504,
		float32(math.Inf(1)),
		float32(math.Inf(-1)),
	}, ToFloat32(encoded))
	assert.Empty(t, FromFloat32(nil))
	assert.Empty(t, ToFloat32(nil))
}

func TestFromAny(t *testing.T) {
	floatSlice := []float32{0.1, 0.2, 0.3}
	testCases := []struct {
		name     string
		input    any
		expected []uint16
		ok       bool
	}{
		{name: "float32 slice", input: floatSlice, expected: FromFloat32(floatSlice), ok: true},
		{name: "fp16 slice", input: FromFloat32(floatSlice), expected: FromFloat32(floatSlice), ok: true},
		{name: "float64 slice", input: []float64{0.1, 0.2, 0.3}, expected: FromFloat32(floatSlice), ok: true},
		{name: "int slice", input: []int{-1, 0, 2}, expected: FromFloat32([]float32{-1, 0, 2}), ok: true},
		{name: "int32 slice", input: []int32{-1, 0, 2}, expected: FromFloat32([]float32{-1, 0, 2}), ok: true},
		{name: "int64 slice", input: []int64{-1, 0, 2}, expected: FromFloat32([]float32{-1, 0, 2}), ok: true},
		{name: "mixed any slice", input: []any{float32(0.1), float64(0.2), int(0), int32(1), int64(2)}, expected: FromFloat32([]float32{0.1, 0.2, 0, 1, 2}), ok: true},
		{name: "empty any slice", input: []any{}, expected: []uint16{}, ok: true},
		{name: "invalid element", input: []any{float32(0.1), "string"}, ok: false},
		{name: "nil element", input: []any{float32(0.1), nil}, ok: false},
		{name: "nil", input: nil, ok: false},
		{name: "scalar", input: 1.0, ok: false},
		{name: "non-numeric slice", input: []string{"1"}, ok: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, ok := FromAny(tc.input)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

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

func TestMulTo(t *testing.T) {
	a := []float32{1, 2, 3, 4}
	b := []float32{5, 6, 7, 8}
	c := make([]float32, 4)
	MulTo(a, b, c)
	assert.Equal(t, []float32{5, 12, 21, 32}, c)
	assert.Panics(t, func() { MulTo([]float32{1}, nil, nil) })
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

func TestMulConstAdd(t *testing.T) {
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	dst := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	target := []float32{0, 3, 6, 9, 12, 15, 18, 21, 24, 27, 30}
	MulConstAdd(a, 2, dst)
	assert.Equal(t, target, dst)
	assert.Panics(t, func() { MulConstAdd(nil, 1, dst) })
}

func TestMulConstAddTo(t *testing.T) {
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	dst := make([]float32, 11)
	target := []float32{0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50}
	MulConstAddTo(a, 3, b, dst)
	assert.Equal(t, target, dst)
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

func TestAddConst(t *testing.T) {
	a := []float32{1, 2, 3, 4}
	AddConst(a, 2)
	assert.Equal(t, []float32{3, 4, 5, 6}, a)
}

func TestDivTo(t *testing.T) {
	a := []float32{1, 4, 9, 16}
	b := []float32{1, 2, 3, 4}
	c := make([]float32, 4)
	DivTo(a, b, c)
	assert.Equal(t, []float32{1, 2, 3, 4}, c)
	assert.Panics(t, func() { DivTo([]float32{1}, nil, nil) })
}

func TestSqrtTo(t *testing.T) {
	a := []float32{1, 4, 9, 16}
	b := make([]float32, 4)
	SqrtTo(a, b)
	assert.Equal(t, []float32{1, 2, 3, 4}, b)
	assert.Panics(t, func() { SqrtTo([]float32{1}, nil) })
}

func TestSqrt(t *testing.T) {
	a := []float32{1, 4, 9, 16}
	Sqrt(a)
	assert.Equal(t, []float32{1, 2, 3, 4}, a)
}

func TestDot(t *testing.T) {
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	assert.Equal(t, float32(770), Dot(a, b))
	assert.Panics(t, func() { Dot([]float32{1}, nil) })
}

func TestEuclidean(t *testing.T) {
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	assert.Equal(t, float32(19.621416), Euclidean(a, b))
	assert.Panics(t, func() { Euclidean([]float32{1}, nil) })
}

type NativeTestSuite struct {
	suite.Suite
}

func (suite *NativeTestSuite) SetupTest() {
	log.SetTestLogger(suite.T())
}

func (suite *NativeTestSuite) SetupSuite() {
	log.SetTestLogger(suite.T())
}

func (suite *NativeTestSuite) TestDot() {
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	suite.Equal(float32(770), dot(a, b))
}

func (suite *NativeTestSuite) TestEuclidean() {
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	suite.InDelta(float32(19.621416), euclidean(a, b), 1e-6)
}

func (suite *NativeTestSuite) TestMulConstAddTo() {
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	dst := make([]float32, 11)
	mulConstAddTo(a, 3, b, dst)
	suite.Equal([]float32{0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50}, dst)
}

func (suite *NativeTestSuite) TestMulConstAdd() {
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	dst := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	target := []float32{0, 3, 6, 9, 12, 15, 18, 21, 24, 27, 30}
	mulConstAdd(a, 2, dst)
	suite.Equal(target, dst)
}

func (suite *NativeTestSuite) TestMulConstTo() {
	a := []float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	dst := make([]float32, 11)
	target := []float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	mulConstTo(a, 2, dst)
	suite.Equal(target, dst)
}

func (suite *NativeTestSuite) TestAddConst() {
	a := []float32{1, 2, 3, 4}
	addConst(a, 2)
	suite.Equal([]float32{3, 4, 5, 6}, a)
}

func (suite *NativeTestSuite) TestSub() {
	a := []float32{1, 2, 3, 4}
	b := []float32{5, 6, 7, 8}
	sub(a, b)
	suite.Equal([]float32{-4, -4, -4, -4}, a)
}

func (suite *NativeTestSuite) TestSubTo() {
	a := []float32{1, 2, 3, 4}
	b := []float32{5, 6, 7, 8}
	c := make([]float32, 4)
	subTo(a, b, c)
	suite.Equal([]float32{-4, -4, -4, -4}, c)
}

func (suite *NativeTestSuite) TestMulTo() {
	a := []float32{1, 2, 3, 4}
	b := []float32{5, 6, 7, 8}
	c := make([]float32, 4)
	mulTo(a, b, c)
	suite.Equal([]float32{5, 12, 21, 32}, c)
}

func (suite *NativeTestSuite) TestDivTo() {
	a := []float32{1, 4, 9, 16}
	b := []float32{1, 2, 3, 4}
	c := make([]float32, 4)
	divTo(a, b, c)
	suite.Equal([]float32{1, 2, 3, 4}, c)
}

func (suite *NativeTestSuite) TestSqrtTo() {
	a := []float32{1, 4, 9, 16}
	b := make([]float32, 4)
	sqrtTo(a, b)
	suite.Equal([]float32{1, 2, 3, 4}, b)
}

func (suite *NativeTestSuite) TestMulConst() {
	a := []float32{1, 2, 3, 4}
	mulConst(a, 2)
	suite.Equal([]float32{2, 4, 6, 8}, a)
}

func (suite *NativeTestSuite) TestMM() {
	a := []float32{1, 2, 3, 4, 5, 6}
	b := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	c := make([]float32, 8)
	target := []float32{38, 44, 50, 56, 83, 98, 113, 128}
	mm(false, false, 2, 4, 3, a, 3, b, 4, c, 4)
	suite.Equal(target, c)

	c = make([]float32, 8)
	target = []float32{14, 32, 50, 68, 32, 77, 122, 167}
	mm(false, true, 2, 4, 3, a, 3, b, 3, c, 4)
	suite.Equal(target, c)

	c = make([]float32, 8)
	target = []float32{61, 70, 79, 88, 76, 88, 100, 112}
	mm(true, false, 2, 4, 3, a, 2, b, 4, c, 4)
	suite.Equal(target, c)

	c = make([]float32, 8)
	target = []float32{22, 49, 76, 103, 28, 64, 100, 136}
	mm(true, true, 2, 4, 3, a, 2, b, 3, c, 4)
	suite.Equal(target, c)
}

func TestNativeTestSuite(t *testing.T) {
	suite.Run(t, new(NativeTestSuite))
}

type SIMDTestSuite struct {
	suite.Suite
	Feature
	MMOverwritesC bool
}

func (suite *SIMDTestSuite) SetupSuite() {
	if feature&suite.Feature != suite.Feature {
		suite.T().Skipf("%s is not supported", (suite.Feature - (feature & suite.Feature)).String())
	}
}

func (suite *SIMDTestSuite) TestFromFloat32() {
	a := []float32{
		0,
		float32(math.Copysign(0, -1)),
		1,
		-2,
		float32(math.Ldexp(1, -24)),
		float32(math.Ldexp(1, -25)),
		1 + float32(math.Ldexp(1, -11)),
		1 + float32(math.Ldexp(1, -10)),
		65504,
		0.1,
		-0.2,
		1.5,
		2.5,
		3.5,
		4.5,
		5.5,
		6.5,
	}
	expected := make([]uint16, len(a))
	fromFloat32(a, expected)
	actual := make([]uint16, len(a))
	suite.Feature.fromFloat32(a, actual)
	suite.Equal(expected, actual)
}

func (suite *SIMDTestSuite) TestToFloat32() {
	a := []uint16{0x0000, 0x8000, 0x0001, 0x03ff, 0x0400, 0x3c00, 0xc000, 0x2e66, 0xb266, 0x7bff, 0x7c00, 0xfc00, 0x3e00, 0x4100, 0x4300, 0x4480, 0x4580}
	expected := make([]float32, len(a))
	toFloat32(a, expected)
	actual := make([]float32, len(a))
	suite.Feature.toFloat32(a, actual)
	suite.Equal(expected, actual)
}

func (suite *SIMDTestSuite) TestMulConstAddTo() {
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	dst := make([]float32, len(a))
	suite.mulConstAddTo(a, 2, b, dst)
	c := make([]float32, len(a))
	mulConstAddTo(a, 2, b, c)
	assert.Equal(suite.T(), c, dst)
}

func (suite *SIMDTestSuite) TestMulConstAdd() {
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	suite.mulConstAdd(a, 2, b)
	c := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	mulConstAdd(a, 2, c)
	assert.Equal(suite.T(), c, b)
}

func (suite *SIMDTestSuite) TestMulConstTo() {
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	suite.mulConstTo(a, 2, b)
	c := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	mulConstTo(a, 2, c)
	assert.Equal(suite.T(), c, b)
}

func (suite *SIMDTestSuite) TestAddConst() {
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	suite.addConst(a, 2)
	c := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	addConst(c, 2)
	assert.Equal(suite.T(), c, a)
}

func (suite *SIMDTestSuite) TestSub() {
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	suite.sub(a, b)
	c := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	sub(c, b)
	suite.Equal(c, a)
}

func (suite *SIMDTestSuite) TestSubTo() {
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	c := make([]float32, len(a))
	suite.subTo(a, b, c)
	d := make([]float32, len(a))
	subTo(a, b, d)
	assert.Equal(suite.T(), c, d)
}

func (suite *SIMDTestSuite) TestMulTo() {
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	expected, actual := make([]float32, len(a)), make([]float32, len(a))
	suite.mulTo(a, b, actual)
	mulTo(a, b, expected)
	assert.Equal(suite.T(), expected, actual)
}

func (suite *SIMDTestSuite) TestMulConst() {
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	suite.mulConst(b, 2)
	c := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	mulConst(c, 2)
	assert.Equal(suite.T(), c, b)
}

func (suite *SIMDTestSuite) TestDivTo() {
	a := []float32{1, 4, 9, 16, 25, 36, 49, 64, 81, 100, 121, 144, 169, 196, 225, 256, 289, 324, 361, 400}
	b := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	c := make([]float32, len(a))
	suite.divTo(a, b, c)
	d := make([]float32, len(a))
	divTo(a, b, d)
	assert.Equal(suite.T(), c, d)
}

func (suite *SIMDTestSuite) TestSqrtTo() {
	a := []float32{1, 4, 9, 16, 25, 36, 49, 64, 81, 100, 121, 144, 169, 196, 225, 256, 289, 324, 361, 400}
	b := make([]float32, len(a))
	suite.sqrtTo(a, b)
	c := make([]float32, len(a))
	sqrtTo(a, c)
	assert.Equal(suite.T(), b, c)
}

func (suite *SIMDTestSuite) TestDot() {
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	actual := suite.dot(a, b)
	expected := dot(a, b)
	assert.Equal(suite.T(), expected, actual)
}

func (suite *SIMDTestSuite) TestEuclidean() {
	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	b := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	actual := suite.euclidean(a, b)
	expected := euclidean(a, b)
	assert.Equal(suite.T(), expected, actual)
}

func (suite *SIMDTestSuite) TestMM() {
	a := []float32{1, 2, 3, 4, 5, 6}
	b := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	c := make([]float32, 8)
	target := []float32{38, 44, 50, 56, 83, 98, 113, 128}
	suite.mm(false, false, 2, 4, 3, a, 3, b, 4, c, 4)
	suite.Equal(target, c)

	c = make([]float32, 8)
	target = []float32{14, 32, 50, 68, 32, 77, 122, 167}
	suite.mm(false, true, 2, 4, 3, a, 3, b, 3, c, 4)
	suite.Equal(target, c)

	c = make([]float32, 8)
	target = []float32{61, 70, 79, 88, 76, 88, 100, 112}
	suite.mm(true, false, 2, 4, 3, a, 2, b, 4, c, 4)
	suite.Equal(target, c)

	c = make([]float32, 8)
	target = []float32{22, 49, 76, 103, 28, 64, 100, 136}
	suite.mm(true, true, 2, 4, 3, a, 2, b, 3, c, 4)
	suite.Equal(target, c)
}

func (suite *SIMDTestSuite) TestMMNoTransposeB() {
	for _, transA := range []bool{false, true} {
		for _, shape := range []struct {
			m, n, k int
		}{
			{m: 3, n: 7, k: 5},
			{m: 4, n: 16, k: 5},
			{m: 5, n: 19, k: 7},
		} {
			suite.Run(fmt.Sprintf("transA=%v/%dx%dx%d", transA, shape.m, shape.n, shape.k), func() {
				lda := shape.k + 2
				aRows := shape.m
				if transA {
					lda = shape.m + 2
					aRows = shape.k
				}
				ldb := shape.n + 3
				ldc := shape.n + 4
				a := make([]float32, aRows*lda)
				b := make([]float32, shape.k*ldb)
				actual := make([]float32, shape.m*ldc)
				for i := range a {
					a[i] = float32(i%11+1) / 8
				}
				for i := range b {
					b[i] = float32(i%13+1) / 16
				}
				for i := range actual {
					actual[i] = float32(i%7+1) / 32
				}
				expected := append([]float32(nil), actual...)
				for i := range shape.m {
					if suite.MMOverwritesC {
						clear(expected[i*ldc : i*ldc+shape.n])
					}
					for l := range shape.k {
						av := a[i*lda+l]
						if transA {
							av = a[l*lda+i]
						}
						for j := range shape.n {
							expected[i*ldc+j] += av * b[l*ldb+j]
						}
					}
				}
				suite.mm(transA, false, shape.m, shape.n, shape.k, a, lda, b, ldb, actual, ldc)
				for i := range expected {
					suite.InDelta(expected[i], actual[i], 1e-6)
				}
			})
		}
	}
}
