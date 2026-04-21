// Copyright 2026 gorse Project Authors
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEuclidean(t *testing.T) {
	a := FromFloat32([]float32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	b := FromFloat32([]float32{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20})
	assert.Equal(t, float32(19.621416), Euclidean(a, b))
	assert.Panics(t, func() { Euclidean([]uint16{1}, nil) })
}

func TestFromAny(t *testing.T) {
	floatSlice := []float32{0.1, 0.2, 0.3}
	testCases := []struct {
		name     string
		input    any
		expected []uint16
		ok       bool
	}{
		{
			name:     "float32 slice",
			input:    floatSlice,
			expected: FromFloat32([]float32{0.1, 0.2, 0.3}),
			ok:       true,
		},
		{
			name:     "bf16 slice",
			input:    FromFloat32(floatSlice),
			expected: FromFloat32([]float32{0.1, 0.2, 0.3}),
			ok:       true,
		},
		{
			name:     "float64 slice",
			input:    []float64{0.1, 0.2, 0.3},
			expected: FromFloat32([]float32{0.1, 0.2, 0.3}),
			ok:       true,
		},
		{
			name:     "int slice",
			input:    []int{-1, 0, 2},
			expected: FromFloat32([]float32{-1, 0, 2}),
			ok:       true,
		},
		{
			name:     "int32 slice",
			input:    []int32{-1, 0, 2},
			expected: FromFloat32([]float32{-1, 0, 2}),
			ok:       true,
		},
		{
			name:     "int64 slice",
			input:    []int64{-1, 0, 2},
			expected: FromFloat32([]float32{-1, 0, 2}),
			ok:       true,
		},
		{
			name:     "mixed any slice",
			input:    []any{float32(0.1), float64(0.2), int(0), int32(1), int64(2)},
			expected: FromFloat32([]float32{0.1, 0.2, 0.0, 1.0, 2.0}),
			ok:       true,
		},
		{
			name:     "empty any slice",
			input:    []any{},
			expected: []uint16{},
			ok:       true,
		},
		{
			name:  "invalid element in any slice",
			input: []any{float32(0.1), "string"},
			ok:    false,
		},
		{
			name:  "nil element in any slice",
			input: []any{float32(0.1), nil},
			ok:    false,
		},
		{
			name:  "invalid input type",
			input: "string",
			ok:    false,
		},
		{
			name:  "nil input",
			input: nil,
			ok:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, ok := FromAny(tc.input)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.expected, result)
		})
	}
}
