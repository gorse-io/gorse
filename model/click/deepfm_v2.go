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
	"github.com/zhenghaoz/gorse/common/nn"
	"github.com/zhenghaoz/gorse/common/nn/layers"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/chewxy/math32"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/google/uuid"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/encoding"
	"github.com/zhenghaoz/gorse/base/floats"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/progress"
	"github.com/zhenghaoz/gorse/model"
	"go.uber.org/zap"
	"gorgonia.org/gorgonia"
	"gorgonia.org/tensor"
	"modernc.org/mathutil"
)

type DeepFMV2 struct {
	BaseFactorizationMachine

	// runtime
	numCPU       int
	predictMutex sync.Mutex

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

	// gorgonia graph
	vm          gorgonia.VM
	g           *gorgonia.ExprGraph
	embeddingV  *gorgonia.Node
	embeddingW  *gorgonia.Node
	embeddingW0 *gorgonia.Node
	values      *gorgonia.Node
	output      *gorgonia.Node
	target      *gorgonia.Node
	cost        *gorgonia.Node
	b           *gorgonia.Node
	b0          *gorgonia.Node
	w1          []*gorgonia.Node
	b1          []*gorgonia.Node
	learnables  []*gorgonia.Node

	// layers
	embedding *layers.Embedding
	linear    []*layers.Linear

	// Adam optimizer variables
	m_v  [][]float32
	m_w  []float32
	m_w0 [][]float32
	v_v  [][]float32
	v_w  []float32
	v_w0 [][]float32
	t    int

	// preallocated arrays
	dataV  []float32
	dataW  []float32
	dataW0 []float32

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

func NewDeepFMV2(params model.Params) *DeepFM {
	fm := new(DeepFM)
	fm.SetParams(params)
	fm.numCPU = runtime.NumCPU()
	fm.g = gorgonia.NewGraph()
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
	fm.predictMutex.Lock()
	defer fm.predictMutex.Unlock()
	indicesTensor, valuesTensor, _ := fm.convertToTensors(x, nil)
	predictions := make([]float32, 0, len(x))
	for i := 0; i < len(x); i += fm.batchSize {
		v, w, w0 := fm.embedding(lo.Must1(indicesTensor.Slice(gorgonia.S(i, i+fm.batchSize))))
		lo.Must0(gorgonia.Let(fm.embeddingV, v))
		lo.Must0(gorgonia.Let(fm.embeddingW, w))
		lo.Must0(gorgonia.Let(fm.embeddingW0, w0))
		lo.Must0(gorgonia.Let(fm.values, lo.Must1(valuesTensor.Slice(gorgonia.S(i, i+fm.batchSize)))))
		lo.Must0(fm.vm.RunAll())
		predictions = append(predictions, fm.output.Value().Data().([]float32)...)
		fm.vm.Reset()
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
	indicesTensor, valuesTensor, targetTensor := fm.convertToTensors(x, y)

	solver := gorgonia.NewAdamSolver(gorgonia.WithBatchSize(float64(fm.batchSize)),
		gorgonia.WithL2Reg(float64(fm.reg)),
		gorgonia.WithLearnRate(float64(fm.lr)))

	_, span := progress.Start(ctx, "DeepFM.Fit", fm.nEpochs*trainSet.Count())
	for epoch := 1; epoch <= fm.nEpochs; epoch++ {
		fitStart := time.Now()
		cost := float32(0)
		for i := 0; i < trainSet.Count(); i += fm.batchSize {
			lo.Must0(gorgonia.Let(fm.values, lo.Must1(valuesTensor.Slice(gorgonia.S(i, i+fm.batchSize)))))
			lo.Must0(gorgonia.Let(fm.target, lo.Must1(targetTensor.Slice(gorgonia.S(i, i+fm.batchSize)))))
			lo.Must0(fm.vm.RunAll())

			fm.backward(lo.Must1(indicesTensor.Slice(gorgonia.S(i, i+fm.batchSize))))
			cost += fm.cost.Value().Data().(float32)
			lo.Must0(solver.Step(gorgonia.NodesToValueGrads(fm.learnables)))
			fm.vm.Reset()
			span.Add(mathutil.Min(fm.batchSize, trainSet.Count()-i))
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
	span.End()
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

	// init manually tuned parameters
	fm.v = fm.GetRandomGenerator().NormalMatrix(fm.numFeatures, fm.nFactors, fm.initMean, fm.initStdDev)
	fm.w = fm.GetRandomGenerator().NormalVector(fm.numFeatures, fm.initMean, fm.initStdDev)
	fm.w0 = fm.GetRandomGenerator().NormalMatrix(fm.numFeatures, fm.nFactors*fm.hiddenLayers[0], fm.initMean, fm.initStdDev)

	// init automatically tuned parameters
	fm.bData = make([]float32, 1)
	fm.b0Data = make([]float32, fm.hiddenLayers[0])
	fm.w1Data = make([][]float32, len(fm.hiddenLayers)-1)
	fm.b1Data = make([][]float32, len(fm.hiddenLayers)-1)
	for i := 1; i < len(fm.hiddenLayers); i++ {
		var (
			inputSize  int
			outputSize int
		)
		inputSize = fm.hiddenLayers[i]
		if i == len(fm.hiddenLayers)-1 {
			outputSize = 1
		} else {
			outputSize = fm.hiddenLayers[i+1]
		}
		fm.w1Data[i-1] = fm.GetRandomGenerator().NormalVector(inputSize*outputSize, fm.initMean, fm.initStdDev)
		fm.b1Data[i-1] = make([]float32, outputSize)
	}

	fm.build()
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
	var err error
	// read params
	if err := encoding.ReadGob(r, &fm.Params); err != nil {
		return errors.Trace(err)
	}
	fm.SetParams(fm.Params)
	// read index
	if fm.Index, err = UnmarshalIndex(r); err != nil {
		return errors.Trace(err)
	}
	// read dataset stats
	if err := encoding.ReadGob(r, &fm.minTarget); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.ReadGob(r, &fm.maxTarget); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.ReadGob(r, &fm.numFeatures); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.ReadGob(r, &fm.numDimension); err != nil {
		return errors.Trace(err)
	}
	// read weights
	for _, data := range fm.marshables {
		if err := encoding.ReadGob(r, data); err != nil {
			return errors.Trace(err)
		}
	}
	if !fm.Invalid() {
		fm.build()
	}
	return nil
}

func (fm *DeepFMV2) build() {
	// init Adam optimizer variables
	fm.m_v = zeros(fm.numFeatures, fm.nFactors)
	fm.m_w = make([]float32, fm.numFeatures)
	fm.m_w0 = zeros(fm.numFeatures, fm.nFactors*fm.hiddenLayers[0])
	fm.v_v = zeros(fm.numFeatures, fm.nFactors)
	fm.v_w = make([]float32, fm.numFeatures)
	fm.v_w0 = zeros(fm.numFeatures, fm.nFactors*fm.hiddenLayers[0])

	// init preallocated arrays
	fm.dataV = make([]float32, fm.batchSize*fm.numDimension*fm.nFactors)
	fm.dataW = make([]float32, fm.batchSize*fm.numDimension)
	fm.dataW0 = make([]float32, fm.batchSize*fm.numDimension*fm.nFactors*fm.hiddenLayers[0])

	fm.b = gorgonia.NewMatrix(fm.g, tensor.Float32,
		gorgonia.WithValue(tensor.New(tensor.WithShape(1, 1), tensor.WithBacking(fm.bData))),
		gorgonia.WithName("b"))
	fm.b0 = gorgonia.NewMatrix(fm.g, tensor.Float32,
		gorgonia.WithValue(tensor.New(tensor.WithShape(1, fm.hiddenLayers[0]), tensor.WithBacking(fm.b0Data))),
		gorgonia.WithName("b0"))
	for i := 1; i < len(fm.hiddenLayers); i++ {
		var (
			inputSize  int
			outputSize int
		)
		inputSize = fm.hiddenLayers[i]
		if i == len(fm.hiddenLayers)-1 {
			outputSize = 1
		} else {
			outputSize = fm.hiddenLayers[i+1]
		}
		fm.w1 = append(fm.w1, gorgonia.NewMatrix(fm.g, tensor.Float32,
			gorgonia.WithValue(tensor.New(tensor.WithShape(inputSize, outputSize), tensor.WithBacking(fm.w1Data[i-1]))),
			gorgonia.WithName(fmt.Sprintf("w%d", i))))
		fm.b1 = append(fm.b1, gorgonia.NewMatrix(fm.g, tensor.Float32,
			gorgonia.WithValue(tensor.New(tensor.WithShape(1, outputSize), tensor.WithBacking(fm.b1Data[i-1]))),
			gorgonia.WithName(fmt.Sprintf("b%d", i))))
	}
	fm.learnables = []*gorgonia.Node{fm.b, fm.b0}
	fm.learnables = append(fm.learnables, fm.w1...)
	fm.learnables = append(fm.learnables, fm.b1...)

	fm.forward(fm.batchSize)
	wrts := []*gorgonia.Node{fm.embeddingV, fm.embeddingW, fm.embeddingW0}
	wrts = append(wrts, fm.learnables...)
	lo.Must1(gorgonia.Grad(fm.cost, wrts...))

	fm.vm = gorgonia.NewTapeMachine(fm.g, gorgonia.BindDualValues(fm.learnables...))
}

func (fm *DeepFMV2) forward(batchSize int) {
	fm.embedding = layers.NewEmbedding(fm.numFeatures, fm.nFactors)
	fm.linear = []*layers.Linear{layers.NewLinear(fm.numDimension*fm.nFactors, fm.hiddenLayers[0])}
	for i := 0; i < len(fm.hiddenLayers); i++ {
		if i < len(fm.hiddenLayers)-1 {
			fm.linear = append(fm.linear, layers.NewLinear(fm.hiddenLayers[i], fm.hiddenLayers[i+1]))
		} else {
			fm.linear = append(fm.linear, layers.NewLinear(fm.hiddenLayers[i], 1))
		}
	}

	// input nodes
	fm.values = gorgonia.NodeFromAny(fm.g,
		tensor.New(tensor.WithShape(batchSize, fm.numDimension), tensor.WithBacking(make([]float32, batchSize*fm.numDimension))),
		gorgonia.WithName("values"))
	fm.target = gorgonia.NodeFromAny(fm.g,
		tensor.New(tensor.WithShape(batchSize), tensor.WithBacking(make([]float32, batchSize))),
		gorgonia.WithName("target"))

	// factorization machine
	x := gorgonia.Must(gorgonia.Reshape(fm.values, []int{batchSize, fm.numDimension, 1}))
	// [batchSize, numDimension, 1]
	vx := gorgonia.Must(gorgonia.ParallelBMM(gorgonia.Must(gorgonia.Transpose(fm.embeddingV, 0, 2, 1)), x, &fm.numCPU))
	// [batchSize, nFactors, 1] = [batchSize, nFactors, numDimension] * [batchSize, numDimension, 1]
	sumSquare := gorgonia.Must(gorgonia.Square(vx))
	// v2 = [numFeatures, nFactors]
	v2 := gorgonia.Must(gorgonia.Square(fm.embeddingV))
	x2 := gorgonia.Must(gorgonia.Square(x))
	squareSum := gorgonia.Must(gorgonia.ParallelBMM(gorgonia.Must(gorgonia.Transpose(v2, 0, 2, 1)), x2, &fm.numCPU))
	sum := gorgonia.Must(gorgonia.Sub(sumSquare, squareSum))
	sum = gorgonia.Must(gorgonia.Sum(sum, 1))
	sum = gorgonia.Must(gorgonia.Mul(sum, fm.nodeFromFloat64(0.5)))
	linear := gorgonia.Must(gorgonia.ParallelBMM(gorgonia.Must(gorgonia.Transpose(fm.embeddingW, 0, 2, 1)), x, &fm.numCPU))
	fm.output = gorgonia.Must(gorgonia.BroadcastAdd(
		gorgonia.Must(gorgonia.Reshape(linear, []int{batchSize})),
		fm.b,
		nil, []byte{0},
	))
	fmOutput := gorgonia.Must(gorgonia.Add(fm.output, gorgonia.Must(gorgonia.Reshape(sum, []int{batchSize}))))

	// output
	fm.output = gorgonia.Must(gorgonia.Add(fmOutput, dnnOutput))

	// loss function
	fm.cost = fm.bceWithLogits(fm.target, fm.output)
}

func (fm *DeepFMV2) Forward(indices, values *nn.Tensor) {
	// embedding
	e := fm.embedding.Forward(indices)

	// factorization machine
	x := nn.Reshape(values, fm.batchSize, fm.numDimension, 1)
	vx := nn.BMM(e, x, true)
	sumSquare := nn.Square(vx)
	e2 := nn.Square(e)
	x2 := nn.Square(x)
	squareSum := nn.BMM(e2, x2, true)
	sum := nn.Sub(sumSquare, squareSum)

	// deep network
	a := nn.Reshape(e, fm.batchSize, fm.numDimension*fm.nFactors)
	for i := 0; i < len(fm.hiddenLayers); i++ {
		a = fm.linear[i].Forward(a)
		if i < len(fm.hiddenLayers)-1 {
			a = nn.ReLu(a)
		} else {
			a = nn.Sigmoid(a)
		}
	}
}

func (fm *DeepFMV2) backward(indices tensor.View) {
	s := indices.Shape()
	if len(s) != 2 {
		panic("indices must be 2-dimensional")
	}
	batchSize, numDimension := s[0], s[1]

	gradEmbeddingV := lo.Must1(fm.embeddingV.Grad()).Data().([]float32)
	gradEmbeddingW := lo.Must1(fm.embeddingW.Grad()).Data().([]float32)
	gradEmbeddingW0 := lo.Must1(fm.embeddingW0.Grad()).Data().([]float32)
	indexSet := mapset.NewSet[int]()
	gradV := make([][]float32, fm.numFeatures)
	gradW := make([]float32, fm.numFeatures)
	gradW0 := make([][]float32, fm.numFeatures)

	for i := 0; i < batchSize; i++ {
		for j := 0; j < numDimension; j++ {
			index := int(lo.Must1(indices.At(i, j)).(float32))
			if index >= 0 && index < fm.numFeatures {
				if !indexSet.Contains(index) {
					indexSet.Add(index)
					gradV[index] = make([]float32, fm.nFactors)
					gradW0[index] = make([]float32, fm.nFactors*fm.hiddenLayers[0])
				}

				floats.Add(gradV[index], gradEmbeddingV[(i*numDimension+j)*fm.nFactors:(i*numDimension+j+1)*fm.nFactors])
				gradW[index] += gradEmbeddingW[i*numDimension+j]
				floats.Add(gradW0[index], gradEmbeddingW0[(i*numDimension+j)*fm.nFactors*fm.hiddenLayers[0]:(i*numDimension+j+1)*fm.nFactors*fm.hiddenLayers[0]])
			}
		}
	}

	fm.t++
	correction1 := 1 - math32.Pow(beta1, float32(fm.t))
	correction2 := 1 - math32.Pow(beta2, float32(fm.t))

	grad2 := make([]float32, fm.nFactors)
	mHat := make([]float32, fm.nFactors)
	vHat := make([]float32, fm.nFactors)
	for index := range indexSet.Iter() {
		grad := gradV[index]
		floats.MulConstAddTo(fm.v[index], fm.reg, grad)
		floats.MulConst(grad, 1/float32(batchSize))
		// m_t = beta_1 * m_{t-1} + (1 - beta_1) * g_t
		floats.MulConst(fm.m_v[index], beta1)
		floats.MulConstAddTo(grad, 1-beta1, fm.m_v[index])
		// v_t = beta_2 * v_{t-1} + (1 - beta_2) * g_t^2
		floats.MulConst(fm.v_v[index], beta2)
		floats.MulTo(grad, grad, grad2)
		floats.MulConstAddTo(grad2, 1-beta2, fm.v_v[index])
		// \hat{m}_t = m_t / (1 - beta_1^t)
		floats.MulConstTo(fm.m_v[index], 1/correction1, mHat)
		// \hat{v}_t = v_t / (1 - beta_2^t)
		floats.MulConstTo(fm.v_v[index], 1/correction2, vHat)
		// \theta_t = \theta_{t-1} + \eta * \hat{m}_t / (\sqrt{\hat{v}_t} + \epsilon)
		floats.Sqrt(vHat)
		floats.AddConst(vHat, eps)
		floats.Div(mHat, vHat)
		floats.MulConstAddTo(mHat, -fm.lr, fm.v[index])
	}

	for index := range indexSet.Iter() {
		grad := gradW[index]
		grad += fm.reg * fm.w[index]
		grad /= float32(batchSize)
		// m_t = beta_1 * m_{t-1} + (1 - beta_1) * g_t
		fm.m_w[index] = beta1*fm.m_w[index] + (1-beta1)*grad
		// v_t = beta_2 * v_{t-1} + (1 - beta_2) * g_t^2
		fm.v_w[index] = beta2*fm.v_w[index] + (1-beta2)*grad*grad
		// \hat{m}_t = m_t / (1 - beta_1^t)
		mHat := fm.m_w[index] / correction1
		// \hat{v}_t = v_t / (1 - beta_2^t)
		vHat := fm.v_w[index] / correction2
		// \theta_t = \theta_{t-1} + \eta * \hat{m}_t / (\sqrt{\hat{v}_t} + \epsilon)
		fm.w[index] -= fm.lr * mHat / (math32.Sqrt(vHat) + eps)
	}

	grad2 = make([]float32, fm.nFactors*fm.hiddenLayers[0])
	mHat = make([]float32, fm.nFactors*fm.hiddenLayers[0])
	vHat = make([]float32, fm.nFactors*fm.hiddenLayers[0])
	for index := range indexSet.Iter() {
		grad := gradW0[index]
		floats.MulConstAddTo(fm.w0[index], fm.reg, grad)
		floats.MulConst(grad, 1/float32(batchSize))
		// m_t = beta_1 * m_{t-1} + (1 - beta_1) * g_t
		floats.MulConst(fm.m_w0[index], beta1)
		floats.MulConstAddTo(grad, 1-beta1, fm.m_w0[index])
		// v_t = beta_2 * v_{t-1} + (1 - beta_2) * g_t^2
		floats.MulConst(fm.v_w0[index], beta2)
		floats.MulTo(grad, grad, grad2)
		floats.MulConstAddTo(grad2, 1-beta2, fm.v_w0[index])
		// \hat{m}_t = m_t / (1 - beta_1^t)
		floats.MulConstTo(fm.m_w0[index], 1/correction1, mHat)
		// \hat{v}_t = v_t / (1 - beta_2^t)
		floats.MulConstTo(fm.v_w0[index], 1/correction2, vHat)
		// \theta_t = \theta_{t-1} + \eta * \hat{m}_t / (\sqrt{\hat{v}_t} + \epsilon)
		floats.Sqrt(vHat)
		floats.AddConst(vHat, eps)
		floats.Div(mHat, vHat)
		floats.MulConstAddTo(mHat, -fm.lr, fm.w0[index])
	}
}

func (fm *DeepFMV2) convertToTensors(x []lo.Tuple2[[]int32, []float32], y []float32) (indicesTensor, valuesTensor, targetTensor *tensor.Dense) {
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

	indicesTensor = tensor.New(tensor.WithShape(alignedSize, fm.numDimension), tensor.WithBacking(alignedIndices))
	valuesTensor = tensor.New(tensor.WithShape(alignedSize, fm.numDimension), tensor.WithBacking(alignedValues))
	if y != nil {
		targetTensor = tensor.New(tensor.WithShape(alignedSize), tensor.WithBacking(alignedTarget))
	}
	return
}

// bceWithLogits is equivalent to:
//
//	(1 + target) * math32.Log(1+math32.Exp(-prediction)) / 2 + (1 - target) * math32.Log(1+math32.Exp(prediction)) / 2
func (fm *DeepFMV2) bceWithLogits(target, prediction *gorgonia.Node) *gorgonia.Node {
	// 1 + target
	onePlusTarget := gorgonia.Must(gorgonia.Add(fm.nodeFromFloat64(1), target))
	// math32.Exp(-prediction)
	expNegPrediction := gorgonia.Must(gorgonia.Exp(gorgonia.Must(gorgonia.Neg(prediction))))
	// 1+math32.Exp(-prediction)
	expNegPredictionPlusOne := gorgonia.Must(gorgonia.Add(fm.nodeFromFloat64(1), expNegPrediction))
	// math32.Log(1+math32.Exp(-prediction))
	logExpNegPredictionPlusOne := gorgonia.Must(gorgonia.Log(expNegPredictionPlusOne))
	// (1 + target) * math32.Log(1+math32.Exp(-prediction)) / 2
	positiveLoss := gorgonia.Must(gorgonia.Mul(onePlusTarget, logExpNegPredictionPlusOne))
	positiveLoss = gorgonia.Must(gorgonia.Div(positiveLoss, fm.nodeFromFloat64(2)))

	// 1 - target
	oneMinusTarget := gorgonia.Must(gorgonia.Sub(fm.nodeFromFloat64(1), target))
	// math32.Exp(prediction)
	expPrediction := gorgonia.Must(gorgonia.Exp(prediction))
	// 1+math32.Exp(prediction)
	expPredictionPlusOne := gorgonia.Must(gorgonia.Add(fm.nodeFromFloat64(1), expPrediction))
	// math32.Log(1+math32.Exp(prediction))
	logExpPredictionPlusOne := gorgonia.Must(gorgonia.Log(expPredictionPlusOne))
	// (1 - target) * math32.Log(1+math32.Exp(prediction)) / 2
	negativeLoss := gorgonia.Must(gorgonia.Mul(oneMinusTarget, logExpPredictionPlusOne))
	negativeLoss = gorgonia.Must(gorgonia.Div(negativeLoss, fm.nodeFromFloat64(2)))

	return gorgonia.Must(gorgonia.Add(positiveLoss, negativeLoss))
}

func (fm *DeepFMV2) nodeFromFloat64(any float32) *gorgonia.Node {
	return gorgonia.NodeFromAny(fm.g, any, gorgonia.WithName(uuid.NewString()))
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
