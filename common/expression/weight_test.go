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

package expression

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFeedbackWeightExpression(t *testing.T) {
	t.Run("valid expressions", func(t *testing.T) {
		weightExpr, err := NewFeedbackWeightExpression(map[string]string{
			"click":  "1",
			"rating": "Value * 2",
		})
		require.NoError(t, err)
		require.NotNil(t, weightExpr)
		assert.Len(t, weightExpr.programs, 2)
	})

	t.Run("invalid expression", func(t *testing.T) {
		weightExpr, err := NewFeedbackWeightExpression(map[string]string{
			"click": "invalid++",
		})
		assert.Error(t, err)
		assert.Nil(t, weightExpr)
	})
}

func TestFeedbackWeightExpression_Evaluate(t *testing.T) {
	weightExpr, err := NewFeedbackWeightExpression(map[string]string{
		"click":     "1",
		"rating":    "Value * 2",
		"view_time": "log1p(Value)",
	})
	require.NoError(t, err)

	t.Run("constant expression", func(t *testing.T) {
		got, err := weightExpr.Evaluate("click", 123)
		require.NoError(t, err)
		assert.Equal(t, float32(1), got)
	})

	t.Run("value expression", func(t *testing.T) {
		got, err := weightExpr.Evaluate("rating", 3.5)
		require.NoError(t, err)
		assert.Equal(t, float32(7), got)
	})

	t.Run("math expression", func(t *testing.T) {
		got, err := weightExpr.Evaluate("view_time", 99)
		require.NoError(t, err)
		assert.InDelta(t, float32(math.Log1p(99)), got, 0.001)
	})

	t.Run("missing feedback type returns default", func(t *testing.T) {
		got, err := weightExpr.Evaluate("purchase", 5)
		require.NoError(t, err)
		assert.Equal(t, float32(1), got)
	})
}

func TestToFloat32(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  float32
	}{
		{"float32", float32(1.5), float32(1.5)},
		{"float64", float64(2.5), float32(2.5)},
		{"int", int(3), float32(3)},
		{"int8", int8(4), float32(4)},
		{"int16", int16(5), float32(5)},
		{"int32", int32(6), float32(6)},
		{"int64", int64(7), float32(7)},
		{"uint", uint(8), float32(8)},
		{"uint8", uint8(9), float32(9)},
		{"uint16", uint16(10), float32(10)},
		{"uint32", uint32(11), float32(11)},
		{"uint64", uint64(12), float32(12)},
		{"unsupported type", "invalid", float32(1.0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToFloat32(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
