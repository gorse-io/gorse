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

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// WeightEnv returns the environment for weight expression evaluation.
// It includes the Value variable and common math functions.
func WeightEnv() map[string]any {
	return map[string]any{
		"Value": 0.0,
		// Common math functions
		"abs":   math.Abs,
		"ceil":  math.Ceil,
		"floor": math.Floor,
		"round": math.Round,
		"sqrt":  math.Sqrt,
		"cbrt":  math.Cbrt,
		"log":   math.Log,
		"log2":  math.Log2,
		"log10": math.Log10,
		"log1p": math.Log1p,
		"exp":   math.Exp,
		"exp2":  math.Exp2,
		"expm1": math.Expm1,
		"pow":   math.Pow,
		"sin":   math.Sin,
		"cos":   math.Cos,
		"tan":   math.Tan,
		"asin":  math.Asin,
		"acos":  math.Acos,
		"atan":  math.Atan,
		"sinh":  math.Sinh,
		"cosh":  math.Cosh,
		"tanh":  math.Tanh,
		"max":   math.Max,
		"min":   math.Min,
	}
}

// CompileWeightExpression compiles a weight expression string.
func CompileWeightExpression(exprStr string) (*vm.Program, error) {
	return expr.Compile(exprStr, expr.Env(WeightEnv()))
}

// EvaluateWeight evaluates a compiled weight expression with the given value.
func EvaluateWeight(program *vm.Program, value float64) (float32, error) {
	env := WeightEnv()
	env["Value"] = value
	result, err := expr.Run(program, env)
	if err != nil {
		return 1.0, err
	}
	return ToFloat32(result)
}

// ToFloat32 converts various numeric types to float32.
func ToFloat32(v any) (float32, error) {
	switch val := v.(type) {
	case float32:
		return val, nil
	case float64:
		return float32(val), nil
	case int:
		return float32(val), nil
	case int8:
		return float32(val), nil
	case int16:
		return float32(val), nil
	case int32:
		return float32(val), nil
	case int64:
		return float32(val), nil
	case uint:
		return float32(val), nil
	case uint8:
		return float32(val), nil
	case uint16:
		return float32(val), nil
	case uint32:
		return float32(val), nil
	case uint64:
		return float32(val), nil
	default:
		return 1.0, nil
	}
}

// ComputeSampleWeights computes sample weights based on feedback_weight configuration.
// This function is used by both CTR training and item-to-item recommendation.
// If feedbackWeight is nil or empty, nil is returned (default weight 1.0 should be used).
func ComputeSampleWeights(feedbackWeight map[string]string, feedbackTypes []string, feedbackValues []float64) ([]float32, error) {
	if len(feedbackWeight) == 0 || len(feedbackTypes) == 0 {
		// No weight configuration or no feedback types, return nil for default weight 1.0
		return nil, nil
	}

	// Compile all weight expressions
	programs := make(map[string]*vm.Program, len(feedbackWeight))
	for fbType, exprStr := range feedbackWeight {
		program, err := CompileWeightExpression(exprStr)
		if err != nil {
			return nil, err
		}
		programs[fbType] = program
	}

	// Compute weight for each sample
	weights := make([]float32, len(feedbackTypes))
	for i, fbType := range feedbackTypes {
		if program, ok := programs[fbType]; ok {
			w, err := EvaluateWeight(program, feedbackValues[i])
			if err != nil {
				return nil, err
			}
			weights[i] = w
		} else {
			// Unknown feedback type, use default weight
			weights[i] = 1.0
		}
	}

	return weights, nil
}
