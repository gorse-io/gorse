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
	"encoding/csv"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/common/dataset"
	"github.com/zhenghaoz/gorse/common/util"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func iris() (*Tensor, *Tensor, error) {
	// Download dataset
	path, err := dataset.DownloadAndUnzip("iris")
	if err != nil {
		return nil, nil, err
	}
	dataFile := filepath.Join(path, "iris.data")
	// Load data
	f, err := os.Open(dataFile)
	if err != nil {
		return nil, nil, err
	}
	reader := csv.NewReader(f)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	// Parse data
	data := make([]float32, len(rows)*4)
	target := make([]float32, len(rows))
	types := make(map[string]int)
	for i, row := range rows {
		for j, cell := range row[:4] {
			data[i*4+j], err = util.ParseFloat[float32](cell)
			if err != nil {
				return nil, nil, err
			}
		}
		if _, exist := types[row[4]]; !exist {
			types[row[4]] = len(types)
		}
		target[i] = float32(types[row[4]])
	}
	return NewTensor(data, len(rows), 4), NewTensor(target, len(rows)), nil
}

func TestIris(t *testing.T) {
	// Load data
	x, _, err := iris()
	assert.NoError(t, err)

	// Build model
	model := NewSequential(
		NewLinear(4, 100),
		NewLinear(100, 100),
		NewLinear(100, 3),
		//NewSoftmax(1),
	)
	predY := model.Forward(x)
	fmt.Println(predY.data)
}

func TestSine(t *testing.T) {
	// Create Tensors to hold input and outputs.
	x := LinSpace(-math.Pi, math.Pi, 100)
	y := Sin(x)

	// Prepare the input tensor (x, x^2, x^3).
	p := NewTensor([]float32{1, 2, 3}, 1, 3)
	xx := Pow(Broadcast(x, 3), p)

	// Use the nn package to define our model and loss function.
	model := NewSequential(
		NewLinear(3, 1),
		NewFlatten(),
	)

	// Use the optim package to define an Optimizer that will update the weights of
	// the model for us. Here we will use RMSprop; the optim package contains many other
	// optimization algorithms. The first argument to the RMSprop constructor tells the
	// optimizer which Tensors it should update.
	learningRate := float32(1e-3)
	optimizer := NewSGD(model.Parameters(), learningRate)
	for i := 0; i < 2000; i++ {
		// Forward pass: compute predicted y by passing x to the model.
		predY := model.Forward(xx)

		// Compute and print loss.
		loss := MSE(predY, y)
		if i%100 == 99 {
			fmt.Println(i, loss.data)
		}

		// Before the backward pass, use the optimizer object to zero all of the
		// gradients for the variables it will update (which are the learnable
		// weights of the model). This is because by default, gradients are
		// accumulated in buffers( i.e, not overwritten) whenever .backward()
		// is called. Checkout docs of torch.autograd.backward for more details.
		optimizer.ZeroGrad()

		// Backward pass: compute gradient of the loss with respect to model
		// parameters
		loss.Backward()

		// Calling the step function on an Optimizer makes an update to its
		// parameters
		optimizer.Step()
	}

	// Result: y = -0.00037327734753489494 + 0.8557199835777283 x + -0.0004013392608612776 x^2 + -0.09375270456075668 x^3
	fmt.Println(model.Parameters())
}
