// Copyright 2023 gorse Project Authors
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

package click

import (
	"bytes"
	"context"
	"fmt"
	"github.com/chewxy/math32"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/encoding"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/common/nn"
	"github.com/zhenghaoz/gorse/model"
	"go.uber.org/zap"
	"io"
	"modernc.org/mathutil"
	"runtime"
	"sync"
	"time"
)

type DeepFMV2 struct {
	BaseFactorizationMachine

	// runtime
	numCPU int
	mu     sync.RWMutex

	// dataset stats
	minTarget    float32
	maxTarget    float32
	numFeatures  int
	numDimension int

	// tuned parameters
	v          [][]float32
	w          []float32
	w0         [][]float32
	bData      []float32
	b0Data     []float32
	w1Data     [][]float32
	b1Data     [][]float32
	marshables []any

	// params and layers
	bias       *nn.Tensor
	embeddingW nn.Layer
	embeddingV nn.Layer
	linear     []nn.Layer

	// Hyper parameters
	batchSize    int
	nFactors     int
	nEpochs      int
	lr           float32
	reg          float32
	initMean     float32
	initStdDev   float32
	hiddenLayers []int
}

func NewDeepFMV2(params model.Params) *DeepFMV2 {
	fm := new(DeepFMV2)
	fm.SetParams(params)
	fm.numCPU = runtime.NumCPU()
	fm.marshables = []any{&fm.v, &fm.w, &fm.w0, &fm.bData, &fm.b0Data, &fm.w1Data, &fm.b1Data}
	return fm
}

func (fm *DeepFMV2) Clear() {
	fm.Index = nil
}

func (fm *DeepFMV2) Invalid() bool {
	return fm == nil ||
		fm.Index == nil
}

func (fm *DeepFMV2) SetParams(params model.Params) {
	fm.BaseFactorizationMachine.SetParams(params)
	fm.batchSize = fm.Params.GetInt(model.BatchSize, 1024)
	fm.nFactors = fm.Params.GetInt(model.NFactors, 16)
	fm.nEpochs = fm.Params.GetInt(model.NEpochs, 50)
	fm.lr = fm.Params.GetFloat32(model.Lr, 0.001)
	fm.reg = fm.Params.GetFloat32(model.Reg, 0.0)
	fm.initMean = fm.Params.GetFloat32(model.InitMean, 0)
	fm.initStdDev = fm.Params.GetFloat32(model.InitStdDev, 0.01)
	fm.hiddenLayers = fm.Params.GetIntSlice(model.HiddenLayers, []int{200, 200})
}

func (fm *DeepFMV2) GetParamsGrid(withSize bool) model.ParamsGrid {
	return model.ParamsGrid{
		model.NFactors:   lo.If(withSize, []interface{}{8, 16, 32, 64}).Else([]interface{}{16}),
		model.Lr:         []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
		model.Reg:        []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
		model.InitMean:   []interface{}{0},
		model.InitStdDev: []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
	}
}

func (fm *DeepFMV2) Predict(userId, itemId string, userFeatures, itemFeatures []Feature) float32 {
	panic("Predict is unsupported for deep learning models")
}

func (fm *DeepFMV2) InternalPredict(indices []int32, values []float32) float32 {
	panic("InternalPredict is unsupported for deep learning models")
}

func (fm *DeepFMV2) BatchInternalPredict(x []lo.Tuple2[[]int32, []float32]) []float32 {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	indicesTensor, valuesTensor, _ := fm.convertToTensors(x, nil)
	predictions := make([]float32, 0, len(x))
	for i := 0; i < len(x); i += fm.batchSize {
		output := fm.Forward(
			indicesTensor.Slice(i, i+fm.batchSize),
			valuesTensor.Slice(i, i+fm.batchSize))
		predictions = append(predictions, output.Data()...)
	}
	return predictions[:len(x)]
}

func (fm *DeepFMV2) BatchPredict(inputs []lo.Tuple4[string, string, []Feature, []Feature]) []float32 {
	x := make([]lo.Tuple2[[]int32, []float32], len(inputs))
	for i, input := range inputs {
		// encode user
		if userIndex := fm.Index.EncodeUser(input.A); userIndex != base.NotId {
			x[i].A = append(x[i].A, userIndex)
			x[i].B = append(x[i].B, 1)
		}
		// encode item
		if itemIndex := fm.Index.EncodeItem(input.B); itemIndex != base.NotId {
			x[i].A = append(x[i].A, itemIndex)
			x[i].B = append(x[i].B, 1)
		}
		// encode user labels
		for _, userFeature := range input.C {
			if userFeatureIndex := fm.Index.EncodeUserLabel(userFeature.Name); userFeatureIndex != base.NotId {
				x[i].A = append(x[i].A, userFeatureIndex)
				x[i].B = append(x[i].B, userFeature.Value)
			}
		}
		// encode item labels
		for _, itemFeature := range input.D {
			if itemFeatureIndex := fm.Index.EncodeItemLabel(itemFeature.Name); itemFeatureIndex != base.NotId {
				x[i].A = append(x[i].A, itemFeatureIndex)
				x[i].B = append(x[i].B, itemFeature.Value)
			}
		}
	}
	return fm.BatchInternalPredict(x)
}

func (fm *DeepFMV2) Fit(ctx context.Context, trainSet *Dataset, testSet *Dataset, config *FitConfig) Score {
	fm.Init(trainSet)
	evalStart := time.Now()
	score := EvaluateClassification(fm, testSet)
	evalTime := time.Since(evalStart)
	fields := append([]zap.Field{zap.String("eval_time", evalTime.String())}, score.ZapFields()...)
	log.Logger().Info(fmt.Sprintf("fit DeepFM %v/%v", 0, fm.nEpochs), fields...)

	var x []lo.Tuple2[[]int32, []float32]
	var y []float32
	for i := 0; i < trainSet.Target.Len(); i++ {
		fm.minTarget = math32.Min(fm.minTarget, trainSet.Target.Get(i))
		fm.maxTarget = math32.Max(fm.maxTarget, trainSet.Target.Get(i))
		indices, values, target := trainSet.Get(i)
		x = append(x, lo.Tuple2[[]int32, []float32]{A: indices, B: values})
		y = append(y, target)
	}
	indices, values, target := fm.convertToTensors(x, y)

	optimizer := nn.NewAdam(fm.Parameters(), fm.lr)
	for epoch := 1; epoch <= fm.nEpochs; epoch++ {
		fitStart := time.Now()
		cost := float32(0)
		for i := 0; i < trainSet.Count(); i += fm.batchSize {
			batchIndices := indices.Slice(i, i+fm.batchSize)
			batchValues := values.Slice(i, i+fm.batchSize)
			batchTarget := target.Slice(i, i+fm.batchSize)
			batchOutput := fm.Forward(batchIndices, batchValues)
			batchLoss := nn.BCEWithLogits(batchTarget, batchOutput)
			cost += batchLoss.Data()[0]
			optimizer.ZeroGrad()
			batchLoss.Backward()
			optimizer.Step()
		}

		fitTime := time.Since(fitStart)
		// Cross validation
		if epoch%config.Verbose == 0 || epoch == fm.nEpochs {
			evalStart = time.Now()
			score = EvaluateClassification(fm, testSet)
			evalTime = time.Since(evalStart)
			fields = append([]zap.Field{
				zap.String("fit_time", fitTime.String()),
				zap.String("eval_time", evalTime.String()),
				zap.Float32("loss", cost),
			}, score.ZapFields()...)
			log.Logger().Info(fmt.Sprintf("fit DeepFM %v/%v", epoch, fm.nEpochs), fields...)
			// check NaN
			if math32.IsNaN(cost) || math32.IsNaN(score.GetValue()) {
				log.Logger().Warn("model diverged", zap.Float32("lr", fm.lr))
				break
			}
		}
	}
	return score
}

// Init parameters for DeepFM.
func (fm *DeepFMV2) Init(trainSet *Dataset) {
	fm.numFeatures = trainSet.ItemCount() + trainSet.UserCount() + len(trainSet.UserFeatures) + len(trainSet.ItemFeatures) + len(trainSet.ContextFeatures)
	fm.numDimension = 0
	for i := 0; i < trainSet.Count(); i++ {
		_, x, _ := trainSet.Get(i)
		fm.numDimension = mathutil.MaxVal(fm.numDimension, len(x))
	}
	fm.bias = nn.Zeros()
	fm.embeddingW = nn.NewEmbedding(fm.numFeatures, 1)
	fm.embeddingV = nn.NewEmbedding(fm.numFeatures, fm.nFactors)
	fm.linear = []nn.Layer{nn.NewLinear(fm.numDimension*fm.nFactors, fm.hiddenLayers[0])}
	for i := 0; i < len(fm.hiddenLayers); i++ {
		if i < len(fm.hiddenLayers)-1 {
			fm.linear = append(fm.linear, nn.NewLinear(fm.hiddenLayers[i], fm.hiddenLayers[i+1]))
		} else {
			fm.linear = append(fm.linear, nn.NewLinear(fm.hiddenLayers[i], 1))
		}
	}
	fm.BaseFactorizationMachine.Init(trainSet)
}

func (fm *DeepFMV2) Marshal(w io.Writer) error {
	// write params
	if err := encoding.WriteGob(w, fm.Params); err != nil {
		return errors.Trace(err)
	}
	// write index
	if err := MarshalIndex(w, fm.Index); err != nil {
		return errors.Trace(err)
	}
	// write dataset stats
	if err := encoding.WriteGob(w, fm.minTarget); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.WriteGob(w, fm.maxTarget); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.WriteGob(w, fm.numFeatures); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.WriteGob(w, fm.numDimension); err != nil {
		return errors.Trace(err)
	}
	// write weights
	for _, data := range fm.marshables {
		if err := encoding.WriteGob(w, data); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (fm *DeepFMV2) Unmarshal(r io.Reader) error {
	return nil
}

func (fm *DeepFMV2) Forward(indices, values *nn.Tensor) *nn.Tensor {
	// embedding
	e := fm.embeddingV.Forward(indices)

	// factorization machine
	x := nn.Reshape(values, fm.batchSize, fm.numDimension, 1)
	vx := nn.BMM(e, x, true)
	sumSquare := nn.Square(vx)
	e2 := nn.Square(e)
	x2 := nn.Square(x)
	squareSum := nn.BMM(e2, x2, true)
	sum := nn.Sub(sumSquare, squareSum)
	sum = nn.Sum(sum, 1)
	sum = nn.Mul(sum, nn.NewScalar(0.5))
	w := fm.embeddingW.Forward(indices)
	linear := nn.BMM(w, x, true)
	fmOutput := nn.Add(nn.Reshape(linear, fm.batchSize), nn.Reshape(sum, fm.batchSize), fm.bias)
	fmOutput = nn.Flatten(fmOutput)

	// deep network
	a := nn.Reshape(e, fm.batchSize, fm.numDimension*fm.nFactors)
	for i := 0; i < len(fm.linear); i++ {
		a = fm.linear[i].Forward(a)
		if i < len(fm.linear)-1 {
			a = nn.ReLu(a)
		} else {
			a = nn.Sigmoid(a)
		}
	}
	dnnOutput := nn.Flatten(a)

	// output
	return nn.Add(fmOutput, dnnOutput)
}

func (fm *DeepFMV2) Parameters() []*nn.Tensor {
	var params []*nn.Tensor
	params = append(params, fm.bias)
	params = append(params, fm.embeddingV.Parameters()...)
	params = append(params, fm.embeddingW.Parameters()...)
	for _, layer := range fm.linear {
		params = append(params, layer.Parameters()...)
	}
	return params
}

func (fm *DeepFMV2) convertToTensors(x []lo.Tuple2[[]int32, []float32], y []float32) (indicesTensor, valuesTensor, targetTensor *nn.Tensor) {
	if y != nil && len(x) != len(y) {
		panic("length of x and y must be equal")
	}

	numBatch := (len(x) + fm.batchSize - 1) / fm.batchSize
	alignedSize := numBatch * fm.batchSize
	alignedIndices := make([]float32, alignedSize*fm.numDimension)
	alignedValues := make([]float32, alignedSize*fm.numDimension)
	alignedTarget := make([]float32, alignedSize)
	for i := range x {
		if len(x[i].A) != len(x[i].B) {
			panic("length of indices and values must be equal")
		}
		for j := range x[i].A {
			alignedIndices[i*fm.numDimension+j] = float32(x[i].A[j])
			alignedValues[i*fm.numDimension+j] = x[i].B[j]
		}
		if y != nil {
			alignedTarget[i] = y[i]
		}
	}

	indicesTensor = nn.NewTensor(alignedIndices, alignedSize, fm.numDimension)
	valuesTensor = nn.NewTensor(alignedValues, alignedSize, fm.numDimension)
	if y != nil {
		targetTensor = nn.NewTensor(alignedTarget, alignedSize)
	}
	return
}

func (fm *DeepFMV2) Clone() FactorizationMachine {
	buf := bytes.NewBuffer(nil)
	if err := MarshalModel(buf, fm); err != nil {
		panic(err)
	}
	if copied, err := UnmarshalModel(buf); err != nil {
		panic(err)
	} else {
		copied.SetParams(copied.GetParams())
		return copied
	}
}

func (fm *DeepFMV2) Spawn() FactorizationMachine {
	return fm.Clone()
}
