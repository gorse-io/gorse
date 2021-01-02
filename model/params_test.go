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
package model

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParams_Copy(t *testing.T) {
	// Create parameters
	a := Params{
		NFactors:    1,
		Lr:          0.1,
		RandomState: 0,
	}
	// Create copy
	b := a.Copy()
	b[NFactors] = 2
	b[Lr] = 0.2
	b[RandomState] = 1
	// Check original parameters
	assert.Equal(t, 1, a.GetInt(NFactors, -1))
	assert.Equal(t, 0.1, a.GetFloat64(Lr, -0.1))
	assert.Equal(t, int64(0), a.GetInt64(RandomState, -1))
	// Check copy parameters
	assert.Equal(t, 2, b.GetInt(NFactors, -1))
	assert.Equal(t, 0.2, b.GetFloat64(Lr, -0.1))
	assert.Equal(t, int64(1), b.GetInt64(RandomState, -1))
}

func TestParams_GetFloat64(t *testing.T) {
	p := Params{}
	// Empty case
	assert.Equal(t, 0.1, p.GetFloat64(Lr, 0.1))
	// Normal case
	p[Lr] = 1.0
	assert.Equal(t, 1.0, p.GetFloat64(Lr, 0.1))
	// Wrong type case
	p[Lr] = 1
	assert.Equal(t, 1.0, p.GetFloat64(Lr, 0.1))
	p[Lr] = "hello"
	assert.Equal(t, 0.1, p.GetFloat64(Lr, 0.1))
}

func TestParams_GetInt(t *testing.T) {
	p := Params{}
	// Empty case
	assert.Equal(t, -1, p.GetInt(NFactors, -1))
	// Normal case
	p[NFactors] = 0
	assert.Equal(t, 0, p.GetInt(NFactors, -1))
	// Wrong type case
	p[NFactors] = "hello"
	assert.Equal(t, -1, p.GetInt(NFactors, -1))
}

func TestParams_GetInt64(t *testing.T) {
	p := Params{}
	// Empty case
	assert.Equal(t, int64(-1), p.GetInt64(RandomState, -1))
	// Normal case
	p[RandomState] = int64(0)
	assert.Equal(t, int64(0), p.GetInt64(RandomState, -1))
	// Wrong type case
	p[RandomState] = 0
	assert.Equal(t, int64(0), p.GetInt64(RandomState, -1))
	p[RandomState] = "hello"
	assert.Equal(t, int64(-1), p.GetInt64(RandomState, -1))
}
