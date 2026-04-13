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

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// FeedbackWeightExpression wraps compiled weight expressions by feedback type.
type FeedbackWeightExpression struct {
	programs map[string]*vm.Program
}

func env(value float64) map[string]any {
	return map[string]any{
		"Value": value,
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

// NewFeedbackWeightExpression compiles weight expressions and wraps them by feedback type.
func NewFeedbackWeightExpression(feedbackWeight map[string]string) (*FeedbackWeightExpression, error) {
	programs := make(map[string]*vm.Program, len(feedbackWeight))
	for feedbackType, exprStr := range feedbackWeight {
		program, err := expr.Compile(exprStr, expr.Env(env(0.0)))
		if err != nil {
			return nil, err
		}
		programs[feedbackType] = program
	}
	return &FeedbackWeightExpression{programs: programs}, nil
}

// Evaluate evaluates the weight for the given feedback type and value.
// If there is no expression for the feedback type, the default weight 1.0 is returned.
func (weightExpr *FeedbackWeightExpression) Evaluate(feedbackType string, value float64) (float32, error) {
	program, ok := weightExpr.programs[feedbackType]
	if !ok {
		return 1.0, nil
	}
	result, err := expr.Run(program, env(value))
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
