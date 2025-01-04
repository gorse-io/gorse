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
	"github.com/chewxy/math32"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/common/dataset"
	"github.com/zhenghaoz/gorse/common/util"
	"os"
	"path/filepath"
	"testing"
)

func TestLinearRegression(t *testing.T) {
	x := Rand(100, 1)
	y := Add(Rand(100, 1), NewScalar(5), Mul(NewScalar(2), x))

	w := Zeros(1, 1)
	b := Zeros(1)
	predict := func(x *Tensor) *Tensor { return Add(MatMul(x, w), b) }

	lr := float32(0.1)
	for i := 0; i < 100; i++ {
		yPred := predict(x)
		loss := MeanSquareError(y, yPred)

		w.grad = nil
		b.grad = nil
		loss.Backward()

		w.sub(w.grad.mul(NewScalar(lr)))
		b.sub(b.grad.mul(NewScalar(lr)))
	}

	assert.Equal(t, []int{1, 1}, w.shape)
	assert.InDelta(t, float64(2), w.data[0], 0.5)
	assert.Equal(t, []int{1}, b.shape)
	assert.InDelta(t, float64(5), b.data[0], 0.5)
}

func TestNeuralNetwork(t *testing.T) {
	x := Rand(100, 1)
	y := Add(Rand(100, 1), Sin(Mul(x, NewScalar(2*math32.Pi))))

	model := NewSequential(
		NewLinear(1, 10),
		NewSigmoid(),
		NewLinear(10, 1),
	)
	NormalInit(model.(*Sequential).layers[0].(*linearLayer).w, 0, 0.01)
	NormalInit(model.(*Sequential).layers[2].(*linearLayer).w, 0, 0.01)
	optimizer := NewSGD(model.Parameters(), 0.2)

	var l float32
	for i := 0; i < 10000; i++ {
		yPred := model.Forward(x)
		loss := MeanSquareError(y, yPred)

		optimizer.ZeroGrad()
		loss.Backward()

		optimizer.Step()
		l = loss.data[0]
	}
	assert.InDelta(t, float64(0), l, 0.1)
}

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
	x, y, err := iris()
	assert.NoError(t, err)

	model := NewSequential(
		NewLinear(4, 100),
		NewLinear(100, 100),
		NewLinear(100, 3),
	)
	optimizer := NewSGD(model.Parameters(), 0.0001)

	var l float32
	for i := 0; i < 100; i++ {
		yPred := model.Forward(x)
		loss := SoftmaxCrossEntropy(yPred, y)
		l = loss.data[0]
		fmt.Println(l)

		optimizer.ZeroGrad()
		loss.Backward()

		optimizer.Step()
	}
	fmt.Println(l)
}
