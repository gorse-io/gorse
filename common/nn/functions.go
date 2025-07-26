// Copyright 2024 gorse Project Authors
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

package nn

import (
	"fmt"
)

func Neg(x *Tensor) *Tensor {
	return apply(&neg{}, x)
}

// Add returns the element-wise sum of two tensors. The shape of the second tensor must be a suffix sequence of the shape of the first tensor.
func Add(x0 *Tensor, x ...*Tensor) *Tensor {
	output := x0
	for _, x1 := range x {
		if len(x0.shape) < len(x1.shape) {
			output, x1 = x1, output
		}
		for i := 0; i < len(x1.shape); i++ {
			if x0.shape[len(x0.shape)-len(x1.shape)+i] != x1.shape[i] {
				panic(fmt.Sprintf("the shape of one tensor %v must be a suffix sequence of the shape of the other tensor %v", x0.shape, x1.shape))
			}
		}
		output = apply(&add{}, output, x1)
	}
	return output
}

// Sub returns the element-wise difference of two tensors. The shape of the second tensor must be a suffix sequence of the shape of the first tensor.
func Sub(x0, x1 *Tensor) *Tensor {
	if len(x0.shape) < len(x1.shape) {
		panic(fmt.Sprintf("the shape of the second tensor %v must be a suffix sequence of the shape of the first tensor %v", x1.shape, x0.shape))
	}
	for i := 0; i < len(x1.shape); i++ {
		if x0.shape[len(x0.shape)-len(x1.shape)+i] != x1.shape[i] {
			panic("the shape of the second tensor must be a suffix sequence of the shape of the first tensor")
		}
	}
	return apply(&sub{}, x0, x1)
}

// Mul returns the element-wise product of two tensors. The shape of the second tensor must be a suffix sequence of the shape of the first tensor.
func Mul(x0, x1 *Tensor) *Tensor {
	if len(x0.shape) < len(x1.shape) {
		x0, x1 = x1, x0
	}
	for i := 0; i < len(x1.shape); i++ {
		if x0.shape[len(x0.shape)-len(x1.shape)+i] != x1.shape[i] {
			panic(fmt.Sprintf("the shape of the second tensor %v must be a suffix sequence of the shape of the first tensor %v", x1.shape, x0.shape))
		}
	}
	return apply(&mul{}, x0, x1)
}

// Div returns the element-wise division of two tensors. The shape of the second tensor must be a suffix sequence of the shape of the first tensor.
func Div(x0, x1 *Tensor) *Tensor {
	if len(x0.shape) < len(x1.shape) {
		panic(fmt.Sprintf("the shape of the second tensor %v must be a suffix sequence of the shape of the first tensor %v", x1.shape, x0.shape))
	}
	for i := 0; i < len(x1.shape); i++ {
		if x0.shape[len(x0.shape)-len(x1.shape)+i] != x1.shape[i] {
			panic("the shape of the second tensor must be a suffix sequence of the shape of the first tensor")
		}
	}
	return apply(&div{}, x0, x1)
}

// Square returns the element-wise square of a tensor.
func Square(x *Tensor) *Tensor {
	return apply(&square{}, x)
}

// Pow returns the element-wise power of a tensor. The shape of the second tensor must be a suffix sequence of the shape of the first tensor.
func Pow(x *Tensor, n *Tensor) *Tensor {
	if len(x.shape) < len(n.shape) {
		panic("the shape of the second tensor must be a suffix sequence of the shape of the first tensor")
	}
	for i := 0; i < len(n.shape); i++ {
		if n.shape[len(n.shape)-len(x.shape)+i] != x.shape[i] {
			panic("the shape of the second tensor must be a suffix sequence of the shape of the first tensor")
		}
	}
	return apply(&pow{}, x, n)
}

// Exp returns the element-wise exponential of a tensor.
func Exp(x *Tensor) *Tensor {
	return apply(&exp{}, x)
}

// Log returns the element-wise natural logarithm of a tensor.
func Log(x *Tensor) *Tensor {
	return apply(&log{}, x)
}

// Sin returns the element-wise sine of a tensor.
func Sin(x *Tensor) *Tensor {
	return apply(&sin{}, x)
}

func Cos(x *Tensor) *Tensor {
	return apply(&cos{}, x)
}

// Sum returns the sum of all elements in a tensor.
func Sum(x *Tensor, along ...int) *Tensor {
	if len(along) > 1 {
		panic("only one along is allowed")
	} else if len(along) == 1 {
		return apply(&partialSum{along: int64(along[0])}, x)
	}
	return apply(&sum{}, x)
}

// Mean returns the mean of all elements in a tensor.
func Mean(x *Tensor) *Tensor {
	return apply(&mean{}, x)
}

func MatMul(x, y *Tensor, transpose1, transpose2 bool, jobs int) *Tensor {
	op := &matMul{
		transpose1: transpose1,
		transpose2: transpose2,
		jobs:       jobs,
	}
	return apply(op, x, y)
}

func BMM(x, y *Tensor, transpose1, transpose2 bool, jobs int) *Tensor {
	op := &batchMatMul{
		transpose1: transpose1,
		transpose2: transpose2,
		jobs:       jobs,
	}
	return apply(op, x, y)
}

func Broadcast(x *Tensor, shape ...int) *Tensor {
	return apply(&broadcast{shape: shape}, x)
}

func Flatten(x *Tensor) *Tensor {
	return apply(&flatten{}, x)
}

func Reshape(x *Tensor, shape ...int) *Tensor {
	size1 := 1
	for i := range x.shape {
		size1 *= x.shape[i]
	}
	size2 := 1
	for i := range shape {
		size2 *= shape[i]
	}
	if size1 != size2 {
		panic("the size of the tensor must be equal to the size of the new shape")
	}
	return apply(&reshape{shape: shape}, x)
}

func Embedding(w, x *Tensor) *Tensor {
	return apply(&embedding{}, w, x)
}

func Sigmoid(x *Tensor) *Tensor {
	return apply(&sigmoid{}, x)
}

func ReLu(x *Tensor) *Tensor {
	return apply(&relu{}, x)
}

func Softmax(x *Tensor, axis int) *Tensor {
	return apply(&softmax{axis: axis}, x)
}

func MeanSquareError(x, y *Tensor) *Tensor {
	return Mean(Square(Sub(x, y)))
}

func SoftmaxCrossEntropy(x, y *Tensor) *Tensor {
	if len(x.shape) != 2 {
		panic("the shape of the first tensor must be 2-D")
	}
	if len(y.shape) != 1 {
		panic("the shape of the second tensor must be 1-D")
	}
	if x.shape[0] != y.shape[0] {
		panic("the size of the first tensor must be equal to the size of the second tensor")
	}
	return apply(&softmaxCrossEntropy{}, x, y)
}

// BCEWithLogits is equivalent to:
//
//	(1 + target) * math32.Log(1+math32.Exp(-prediction)) / 2 + (1 - target) * math32.Log(1+math32.Exp(prediction)) / 2
func BCEWithLogits(target, prediction *Tensor) *Tensor {
	return Mean(Add(
		Div(
			Mul(
				Add(NewScalar(1), target),
				Log(Add(NewScalar(1), Exp(Neg(prediction))))),
			NewScalar(2)),
		Div(
			Mul(
				Sub(Ones(target.shape...), target),
				Log(Add(NewScalar(1), Exp(prediction)))),
			NewScalar(2))))
}
