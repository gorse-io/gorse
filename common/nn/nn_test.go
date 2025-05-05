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
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chewxy/math32"
	"github.com/klauspost/cpuid/v2"
	"github.com/samber/lo"
	"github.com/schollz/progressbar/v3"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/common/datautil"
	"github.com/zhenghaoz/gorse/common/util"
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
	assert.InDelta(t, float64(2), w.data[0], 0.6)
	assert.Equal(t, []int{1}, b.shape)
	assert.InDelta(t, float64(5), b.data[0], 0.6)
}

func TestNeuralNetwork(t *testing.T) {
	x := Rand(100, 1)
	y := Add(Rand(100, 1), Sin(Mul(x, NewScalar(2*math32.Pi))))

	model := NewSequential(
		NewLinear(1, 10),
		NewSigmoid(),
		NewLinear(10, 1),
	)
	NormalInit(model.(*Sequential).Layers[0].(*LinearLayer).W, 0, 0.01)
	NormalInit(model.(*Sequential).Layers[2].(*LinearLayer).W, 0, 0.01)
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
	assert.InDelta(t, float64(0), l, 0.2)
}

func iris() (*Tensor, *Tensor, error) {
	// Download dataset
	path, err := datautil.DownloadAndUnzip("iris")
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
	optimizer := NewAdam(model.Parameters(), 0.01)

	var l float32
	for i := 0; i < 1000; i++ {
		yPred := model.Forward(x)
		loss := SoftmaxCrossEntropy(yPred, y)

		optimizer.ZeroGrad()
		loss.Backward()

		optimizer.Step()
		l = loss.data[0]
	}
	assert.InDelta(t, float32(0), l, 0.1)
}

func mnist() (lo.Tuple2[*Tensor, *Tensor], lo.Tuple2[*Tensor, *Tensor], error) {
	var train, test lo.Tuple2[*Tensor, *Tensor]
	// Download and unzip dataset
	path, err := datautil.DownloadAndUnzip("mnist")
	if err != nil {
		return train, test, err
	}
	// Open dataset
	train.A, train.B, err = openMNISTFile(filepath.Join(path, "train.libfm"))
	if err != nil {
		return train, test, err
	}
	test.A, test.B, err = openMNISTFile(filepath.Join(path, "test.libfm"))
	if err != nil {
		return train, test, err
	}
	return train, test, nil
}

func openMNISTFile(path string) (*Tensor, *Tensor, error) {
	// Open file
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	// Read data line by line
	var (
		images []float32
		labels []float32
	)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		splits := strings.Split(line, " ")
		// Parse label
		label, err := util.ParseFloat[float32](splits[0])
		if err != nil {
			return nil, nil, err
		}
		labels = append(labels, label)
		// Parse image
		image := make([]float32, 784)
		for _, split := range splits[1:] {
			kv := strings.Split(split, ":")
			index, err := strconv.Atoi(kv[0])
			if err != nil {
				return nil, nil, err
			}
			value, err := util.ParseFloat[float32](kv[1])
			if err != nil {
				return nil, nil, err
			}
			image[index] = value
		}
		images = append(images, image...)
	}
	return NewTensor(images, len(labels), 784), NewTensor(labels, len(labels)), nil
}

func accuracy(prediction, target *Tensor) float32 {
	var precision float32
	for i, gt := range target.data {
		if prediction.Slice(i, i+1).argmax()[1] == int(gt) {
			precision += 1
		}
	}
	precision /= float32(len(target.data))
	return precision
}

func TestMNIST(t *testing.T) {
	if cpuid.CPU.VendorString != "Apple" && !cpuid.CPU.Supports(cpuid.AVX512F, cpuid.AVX512DQ) {
		// Since the test takes a long time, we run the test only in development environment.
		// 1. Mac with Apple Silicon.
		// 2. x86 CPU with AVX512 support.
		t.Skip("Skip test on non-development environment.")
	}

	train, test, err := mnist()
	assert.NoError(t, err)

	model := NewSequential(
		NewLinear(784, 1000),
		NewReLU(),
		NewLinear(1000, 10),
	)
	optimizer := NewAdam(model.Parameters(), 0.001)

	const (
		batchSize = 1000
		numEpoch  = 5
	)
	for i := 0; i < numEpoch; i++ {
		startTime := time.Now()
		sumLoss, sumAcc := float32(0), float32(0)
		bar := progressbar.Default(int64(train.A.shape[0]), fmt.Sprintf("Epoch %v/%v", i+1, numEpoch))
		for j := 0; j < train.A.shape[0]; j += batchSize {
			xBatch := train.A.Slice(j, j+batchSize)
			yBatch := train.B.Slice(j, j+batchSize)

			yPred := model.Forward(xBatch)
			loss := SoftmaxCrossEntropy(yPred, yBatch)

			optimizer.ZeroGrad()
			loss.Backward()

			optimizer.Step()
			sumLoss += loss.data[0]
			sumAcc += accuracy(yPred, yBatch)
			assert.NoError(t, bar.Add(batchSize))
		}
		sumLoss /= float32(train.A.shape[0] / batchSize)
		sumAcc /= float32(train.A.shape[0] / batchSize)
		assert.NoError(t, bar.Finish())
		fmt.Println("Duration:", time.Since(startTime), "Loss:", sumLoss, "Accuracy:", sumAcc)
	}

	testAcc := accuracy(model.Forward(test.A), test.B)
	fmt.Println("Test Accuracy:", testAcc)
	assert.Greater(t, float64(testAcc), 0.95)
}

func spiral() (*Tensor, *Tensor, error) {
	numData, numClass, inputDim := 100, 3, 2
	dataSize := numClass * numData
	x := Zeros(dataSize, inputDim)
	t := Zeros(dataSize)

	for j := 0; j < numClass; j++ {
		for i := 0; i < numData; i++ {
			rate := float32(i) / float32(numData)
			radius := 1.0 * rate
			theta := float32(j)*4.0 + 4.0*rate + float32(rand.NormFloat64())*0.2
			ix := numData*j + i
			x.data[ix*inputDim] = radius * math32.Sin(theta)
			x.data[ix*inputDim+1] = radius * math32.Cos(theta)
			t.data[ix] = float32(j)
		}
	}

	indices := rand.Perm(dataSize)
	x = x.SliceIndices(indices...)
	t = t.SliceIndices(indices...)
	return x, t, nil
}

func TestSaveAndLoad(t *testing.T) {
	x, y, err := spiral()
	assert.NoError(t, err)

	model := NewSequential(
		NewLinear(2, 10),
		NewSigmoid(),
		NewLinear(10, 3),
	)
	optimizer := NewAdam(model.Parameters(), 0.01)

	var expected float32
	for i := 0; i < 300; i++ {
		yPred := model.Forward(x)
		loss := SoftmaxCrossEntropy(yPred, y)

		optimizer.ZeroGrad()
		loss.Backward()

		optimizer.Step()
		expected = loss.data[0]
	}

	buffer := bytes.NewBuffer(nil)
	err = Save(model, buffer)
	assert.NoError(t, err)
	modelLoaded := NewSequential(
		NewLinear(2, 10),
		NewSigmoid(),
		NewLinear(10, 3),
	)
	err = Load(modelLoaded, buffer)
	assert.NoError(t, err)
	yPred := modelLoaded.Forward(x)
	loss := SoftmaxCrossEntropy(yPred, y)
	assert.InDelta(t, float64(expected), float64(loss.data[0]), 0.01)
}
