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

package weight

import (
	"math"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// Env returns the environment for weight expression evaluation.
// It includes the Value variable and common math functions.
func Env() map[string]any {
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

// Compile compiles a weight expression string.
func Compile(exprStr string) (*vm.Program, error) {
	return expr.Compile(exprStr, expr.Env(Env()))
}

// Evaluate evaluates a compiled weight expression with the given value.
func Evaluate(program *vm.Program, value float64) (float32, error) {
	env := Env()
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
