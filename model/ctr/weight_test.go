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

package ctr

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
		{"constant float", "2.5", false},
		{"value variable", "Value", false},
		{"value multiplication", "Value * 2", false},
		{"value division", "Value / 5", false},
		{"log function", "log(Value)", false},
		{"log1p function", "log1p(Value)", false},
		{"sqrt function", "sqrt(Value)", false},
		{"abs function", "abs(Value)", false},
		{"pow function", "pow(Value, 2)", false},
		{"max function", "max(Value, 1)", false},
		{"min function", "min(Value, 10)", false},
		{"complex expression", "log(Value + 1) * 2", false},
		{"invalid expression", "Value ++", true},
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
		{"constant 1", "1", 100.0, 1.0, 0.001},
		{"constant 2.5", "2.5", 100.0, 2.5, 0.001},
		{"value", "Value", 5.0, 5.0, 0.001},
		{"value * 2", "Value * 2", 3.0, 6.0, 0.001},
		{"value / 5", "Value / 5", 10.0, 2.0, 0.001},
		{"log(100)", "log(Value)", 100.0, float32(math.Log(100)), 0.001},
		{"log1p(99)", "log1p(Value)", 99.0, float32(math.Log1p(99)), 0.001},
		{"sqrt(16)", "sqrt(Value)", 16.0, 4.0, 0.001},
		{"abs(-5)", "abs(Value)", -5.0, 5.0, 0.001},
		{"pow(3, 2)", "pow(Value, 2)", 3.0, 9.0, 0.001},
		{"max(5, 10)", "max(Value, 10)", 5.0, 10.0, 0.001},
		{"min(5, 1)", "min(Value, 1)", 5.0, 1.0, 0.001},
		{"log(Value + 1)", "log(Value + 1)", 99.0, float32(math.Log(100)), 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileWeightExpression(tt.expr)
			require.NoError(t, err)

			got, err := EvaluateWeight(program, tt.value)
			require.NoError(t, err)

			assert.InDelta(t, tt.want, got, float64(tt.tolerance))
		})
	}
}

func TestEvaluateWeight_EdgeCases(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		program, err := CompileWeightExpression("Value + 1")
		require.NoError(t, err)

		got, err := EvaluateWeight(program, 0.0)
		require.NoError(t, err)
		assert.Equal(t, float32(1.0), got)
	})

	t.Run("negative value", func(t *testing.T) {
		program, err := CompileWeightExpression("abs(Value)")
		require.NoError(t, err)

		got, err := EvaluateWeight(program, -10.0)
		require.NoError(t, err)
		assert.Equal(t, float32(10.0), got)
	})

	t.Run("large value", func(t *testing.T) {
		program, err := CompileWeightExpression("log10(Value)")
		require.NoError(t, err)

		got, err := EvaluateWeight(program, 1000000.0)
		require.NoError(t, err)
		assert.InDelta(t, float32(6.0), got, 0.001)
	})
}
