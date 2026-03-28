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
	"github.com/expr-lang/expr/vm"
	"github.com/gorse-io/gorse/common/weight"
)

// WeightEnv returns the environment for weight expression evaluation.
// Deprecated: Use weight.Env() instead.
func WeightEnv() map[string]any {
	return weight.Env()
}

// CompileWeightExpression compiles a weight expression string.
// Deprecated: Use weight.Compile() instead.
func CompileWeightExpression(exprStr string) (*vm.Program, error) {
	return weight.Compile(exprStr)
}

// EvaluateWeight evaluates a compiled weight expression with the given value.
// Deprecated: Use weight.Evaluate() instead.
func EvaluateWeight(program *vm.Program, value float64) (float32, error) {
	return weight.Evaluate(program, value)
}
