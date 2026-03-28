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

package logics

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompileWeightExpression(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"constant", "1", false},
		{"value", "Value", false},
		{"expression", "Value * 2 + 1", false},
		{"math function", "log(Value)", false},
		{"complex", "sqrt(Value) / 10", false},
		{"invalid", "invalid++", true},
		{"unknown function", "unknown(Value)", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CompileWeightExpression(tt.expr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEvaluateWeight(t *testing.T) {
	tests := []struct {
		name      string
		expr      string
		value     float64
		want      float32
		tolerance float32
	}{
		{"constant", "5", 1.0, 5.0, 0},
		{"value", "Value", 3.5, 3.5, 0},
		{"expression", "Value * 2", 5.0, 10.0, 0},
		{"log", "log(Value)", math.E, 1.0, 0.001},
		{"sqrt", "sqrt(Value)", 16.0, 4.0, 0},
		{"abs", "abs(Value)", -5.0, 5.0, 0},
		{"complex", "log10(Value)", 1000.0, 3.0, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileWeightExpression(tt.expr)
			require.NoError(t, err)
			got, err := EvaluateWeight(program, tt.value)
			require.NoError(t, err)
			if tt.tolerance > 0 {
				assert.InDelta(t, tt.want, got, tt.tolerance)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestEvaluateWeight_EdgeCases(t *testing.T) {
	t.Run("negative value", func(t *testing.T) {
		program, err := CompileWeightExpression("abs(Value)")
		require.NoError(t, err)
		got, err := EvaluateWeight(program, -10.0)
		require.NoError(t, err)
		assert.Equal(t, float32(10.0), got)
	})

	t.Run("zero value", func(t *testing.T) {
		program, err := CompileWeightExpression("Value + 1")
		require.NoError(t, err)
		got, err := EvaluateWeight(program, 0.0)
		require.NoError(t, err)
		assert.Equal(t, float32(1.0), got)
	})
}

func TestComputeSampleWeights(t *testing.T) {
	t.Run("no weight config", func(t *testing.T) {
		weights, err := ComputeSampleWeights(nil, []string{"click"}, []float64{1.0})
		assert.NoError(t, err)
		assert.Nil(t, weights)
	})

	t.Run("empty weight config", func(t *testing.T) {
		weights, err := ComputeSampleWeights(map[string]string{}, []string{"click"}, []float64{1.0})
		assert.NoError(t, err)
		assert.Nil(t, weights)
	})

	t.Run("no feedback types", func(t *testing.T) {
		weights, err := ComputeSampleWeights(map[string]string{"click": "1"}, nil, nil)
		assert.NoError(t, err)
		assert.Nil(t, weights)
	})

	t.Run("constant weights", func(t *testing.T) {
		weights, err := ComputeSampleWeights(
			map[string]string{"click": "1", "purchase": "5"},
			[]string{"click", "purchase", "click"},
			[]float64{1.0, 1.0, 1.0},
		)
		assert.NoError(t, err)
		assert.Equal(t, []float32{1.0, 5.0, 1.0}, weights)
	})

	t.Run("value-based weights", func(t *testing.T) {
		weights, err := ComputeSampleWeights(
			map[string]string{"rating": "Value"},
			[]string{"rating", "rating", "rating"},
			[]float64{3.0, 4.0, 5.0},
		)
		assert.NoError(t, err)
		assert.Equal(t, []float32{3.0, 4.0, 5.0}, weights)
	})

	t.Run("expression weights", func(t *testing.T) {
		weights, err := ComputeSampleWeights(
			map[string]string{"view_time": "log1p(Value)"},
			[]string{"view_time", "view_time", "view_time"},
			[]float64{99.0, 999.0, 9.0},
		)
		assert.NoError(t, err)
		require.NotNil(t, weights)
		assert.InDelta(t, float32(math.Log1p(99.0)), weights[0], 0.001)
		assert.InDelta(t, float32(math.Log1p(999.0)), weights[1], 0.001)
		assert.InDelta(t, float32(math.Log1p(9.0)), weights[2], 0.001)
	})

	t.Run("unknown feedback type uses default", func(t *testing.T) {
		weights, err := ComputeSampleWeights(
			map[string]string{"click": "1", "purchase": "5"},
			[]string{"click", "unknown", "purchase"},
			[]float64{1.0, 1.0, 1.0},
		)
		assert.NoError(t, err)
		assert.Equal(t, []float32{1.0, 1.0, 5.0}, weights)
	})

	t.Run("invalid expression returns error", func(t *testing.T) {
		_, err := ComputeSampleWeights(
			map[string]string{"click": "invalid++"},
			[]string{"click"},
			[]float64{1.0},
		)
		assert.Error(t, err)
	})
}
