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

package copier

import (
	"github.com/juju/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestPrimitives(t *testing.T) {
	var a = 1
	var b int
	err := Copy(&b, a)
	assert.NoError(t, err)
	assert.Equal(t, a, b)
	// not a pointer
	err = Copy(b, a)
	assert.True(t, errors.IsNotValid(err))
	// test type mismatch
	var c bool
	err = Copy(&c, a)
	assert.True(t, errors.IsNotValid(err))
	// copy to interface
	var d interface{}
	err = Copy(&d, a)
	assert.NoError(t, err)
	assert.Equal(t, a, d)
}

func TestSlice(t *testing.T) {
	a := [][]int{{1}, {2}, {3}, {4}}
	b := make([][]int, 0)
	err := Copy(&b, a)
	assert.NoError(t, err)
	assert.Equal(t, a, b)
	// test deep copy
	a[0][0] = 100
	assert.Equal(t, 1, b[0][0])
	// test reuse memory
	var integers = []int{10}
	c := [][]int{integers, {20}, {30}, {40}}
	err = Copy(&c, a)
	assert.NoError(t, err)
	integers[0] = 100
	assert.Equal(t, 100, c[0][0])
	// copy to interface
	var d interface{}
	err = Copy(&d, a)
	assert.NoError(t, err)
	assert.Equal(t, a, d)
	// copy empty slice
	var e interface{}
	err = Copy(&e, make([]int, 0))
	assert.NoError(t, err)
	assert.NotNil(t, e)
	// copy to larger slice
	var f = [][]int{{10}, {20}, {30}, {40}, {50}}
	err = Copy(&f, a)
	assert.NoError(t, err)
	assert.Equal(t, a, f)
	assert.Equal(t, 5, cap(f))
}

func TestMap(t *testing.T) {
	a := map[int64][]int64{1: {1}, 2: {1}, 3: {1}}
	b := map[int64][]int64{4: {100}, 5: {200}, 6: {300}}
	err := Copy(&b, a)
	assert.NoError(t, err)
	assert.Equal(t, a, b)
	// test deep copy
	a[1][0] = 100
	assert.Equal(t, int64(1), b[1][0])
	// copy to interface
	var d interface{}
	err = Copy(&d, a)
	assert.NoError(t, err)
	assert.Equal(t, a, d)
	// test no copy
	var integers = []int64{100}
	c := map[int64][]int64{1: integers, 2: {1}, 3: {1}}
	err = Copy(&c, a)
	assert.NoError(t, err)
	assert.Equal(t, a, c)
	integers[0] = 10
	assert.Equal(t, int64(10), c[1][0])
}

type Foo struct {
	A int64
	B []string
}

type Bar struct {
	A int64
}

func TestStruct(t *testing.T) {
	a := Foo{A: 3, B: []string{"3"}}
	b := Foo{A: 4, B: []string{"4"}}
	err := Copy(&b, a)
	assert.NoError(t, err)
	assert.Equal(t, a, b)
	// test deep copy
	a.B[0] = "100"
	assert.Equal(t, "3", b.B[0])
	// test type mismatch
	var c Bar
	err = Copy(&c, a)
	assert.True(t, errors.IsNotValid(err))
}

func TestPtr(t *testing.T) {
	a := &Foo{A: 3, B: []string{"3"}}
	b := &Foo{A: 4, B: []string{"4"}}
	err := Copy(&b, a)
	assert.NoError(t, err)
	assert.Equal(t, a, b)
}

func TestInterface(t *testing.T) {
	var a = []interface{}{&Foo{A: 3, B: []string{"3"}}, []int{100}, 1}
	var b []interface{}
	err := Copy(&b, a)
	assert.NoError(t, err)
	assert.Equal(t, a, b)
	// test reuse memory
	var strings = []string{"30"}
	var c = []interface{}{&Foo{A: 30, B: strings}, []int{1000}, 10}
	err = Copy(&c, a)
	assert.NoError(t, err)
	assert.Equal(t, a, c)
	strings[0] = "123"
	assert.Equal(t, "123", c[0].(*Foo).B[0])
}

type PrivateStruct struct {
	text *string
}

func (ps *PrivateStruct) MarshalBinary() (data []byte, err error) {
	return []byte(*ps.text), nil
}

func (ps *PrivateStruct) UnmarshalBinary(data []byte) error {
	ps.text = proto.String(string(data))
	return nil
}

func TestPrivate(t *testing.T) {
	var a = PrivateStruct{proto.String("hello")}
	var b PrivateStruct
	err := Copy(&b, a)
	assert.NoError(t, err)
	assert.Equal(t, a, b)
	// test deep copy
	*a.text = "world"
	assert.Equal(t, "hello", *b.text)
}
