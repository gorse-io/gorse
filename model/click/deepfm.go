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
	"context"
	"fmt"
	"io"
	"time"

	"github.com/chewxy/math32"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/progress"
	"github.com/zhenghaoz/gorse/model"
	"go.uber.org/zap"
	"gorgonia.org/gorgonia"
	"gorgonia.org/tensor"
	"modernc.org/mathutil"
)

const (
	beta1 float32 = 0.9
	beta2 float32 = 0.999
	eps   float32 = 1e-8
)

type DeepFM struct {
	BaseFactorizationMachine

	MinTarget    float32
	MaxTarget    float32
	Task         FMTask
	numFeatures  int
	numDimension int

	vm         gorgonia.VM
	g          *gorgonia.ExprGraph
	embeddingV *gorgonia.Node
	embeddingW *gorgonia.Node
	values     *gorgonia.Node
	output     *gorgonia.Node
	target     *gorgonia.Node
	cost       *gorgonia.Node
	b          *gorgonia.Node
	learnables []*gorgonia.Node
	v          [][]float32
	w          []float32
	m_v        [][]float32
	m_w        []float32
	v_v        [][]float32
	v_w        []float32
	t          int

	// Hyper parameters
	batchSize  int
	nFactors   int
	nEpochs    int
	lr         float32
	reg        float32
	initMean   float32
	initStdDev float32
}

func NewDeepFM(params model.Params) FactorizationMachine {
	fm := new(DeepFM)
	fm.g = gorgonia.NewGraph()
	fm.SetParams(params)
	return fm
}

func (fm *DeepFM) Clear() {
	fm.Index = nil
}

func (fm *DeepFM) Invalid() bool {
	return fm == nil ||
		fm.Index == nil
}

func (fm *DeepFM) SetParams(params model.Params) {
	fm.BaseFactorizationMachine.SetParams(params)
	fm.batchSize = fm.Params.GetInt(model.BatchSize, 1024)
	fm.nFactors = fm.Params.GetInt(model.NFactors, 16)
	fm.nEpochs = fm.Params.GetInt(model.NEpochs, 200)
	fm.lr = fm.Params.GetFloat32(model.Lr, 0.001)
	fm.reg = fm.Params.GetFloat32(model.Reg, 0.0)
	fm.initMean = fm.Params.GetFloat32(model.InitMean, 0)
	fm.initStdDev = fm.Params.GetFloat32(model.InitStdDev, 0.01)
}

func (fm *DeepFM) GetParamsGrid(withSize bool) model.ParamsGrid {
	return model.ParamsGrid{
		model.NFactors:   lo.If(withSize, []interface{}{8, 16, 32, 64}).Else([]interface{}{16}),
		model.Lr:         []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
		model.Reg:        []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
		model.InitMean:   []interface{}{0},
		model.InitStdDev: []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
	}
}

func (fm *DeepFM) Predict(userId, itemId string, userFeatures, itemFeatures []Feature) float32 {
	var (
		indices []int32
		values  []float32
	)
	// encode user
	if userIndex := fm.Index.EncodeUser(userId); userIndex != base.NotId {
		indices = append(indices, userIndex)
		values = append(values, 1)
	}
	// encode item
	if itemIndex := fm.Index.EncodeItem(itemId); itemIndex != base.NotId {
		indices = append(indices, itemIndex)
		values = append(values, 1)
	}
	// encode user labels
	for _, userFeature := range userFeatures {
		if userFeatureIndex := fm.Index.EncodeUserLabel(userFeature.Name); userFeatureIndex != base.NotId {
			indices = append(indices, userFeatureIndex)
			values = append(values, userFeature.Value)
		}
	}
	// encode item labels
	for _, itemFeature := range itemFeatures {
		if itemFeatureIndex := fm.Index.EncodeItemLabel(itemFeature.Name); itemFeatureIndex != base.NotId {
			indices = append(indices, itemFeatureIndex)
			values = append(values, itemFeature.Value)
		}
	}
	return fm.InternalPredict(indices, values)
}

func (fm *DeepFM) InternalPredict(indices []int32, values []float32) float32 {
	panic("InternalPredict is unsupported for deep learning models")
}

func (fm *DeepFM) BatchPredict(x []lo.Tuple2[[]int32, []float32]) []float32 {
	indicesTensor, valuesTensor, _ := fm.convertToTensors(x, nil)
	predictions := make([]float32, 0, len(x))
	for i := 0; i < len(x); i += fm.batchSize {
		v, w := fm.embedding(lo.Must1(indicesTensor.Slice(gorgonia.S(i, i+fm.batchSize))))
		lo.Must0(gorgonia.Let(fm.embeddingV, v))
		lo.Must0(gorgonia.Let(fm.embeddingW, w))
		lo.Must0(gorgonia.Let(fm.values, lo.Must1(valuesTensor.Slice(gorgonia.S(i, i+fm.batchSize)))))
		lo.Must0(fm.vm.RunAll())
		predictions = append(predictions, fm.output.Value().Data().([]float32)...)
		fm.vm.Reset()
	}
	return predictions[:len(x)]
}

func (fm *DeepFM) Fit(ctx context.Context, trainSet *Dataset, testSet *Dataset, config *FitConfig) Score {
	fm.Init(trainSet)
	evalStart := time.Now()
	score := EvaluateClassification(fm, testSet)
	evalTime := time.Since(evalStart)
	fields := append([]zap.Field{zap.String("eval_time", evalTime.String())}, score.ZapFields()...)
	log.Logger().Info(fmt.Sprintf("fit fm %v/%v", 0, fm.nEpochs), fields...)

	var x []lo.Tuple2[[]int32, []float32]
	var y []float32
	for i := 0; i < trainSet.Target.Len(); i++ {
		fm.MinTarget = math32.Min(fm.MinTarget, trainSet.Target.Get(i))
		fm.MaxTarget = math32.Max(fm.MaxTarget, trainSet.Target.Get(i))
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
			v, w := fm.embedding(lo.Must1(indicesTensor.Slice(gorgonia.S(i, i+fm.batchSize))))
			lo.Must0(gorgonia.Let(fm.embeddingV, v))
			lo.Must0(gorgonia.Let(fm.embeddingW, w))
			lo.Must0(gorgonia.Let(fm.values, lo.Must1(valuesTensor.Slice(gorgonia.S(i, i+fm.batchSize)))))
			lo.Must0(gorgonia.Let(fm.target, lo.Must1(targetTensor.Slice(gorgonia.S(i, i+fm.batchSize)))))
			lo.Must0(fm.vm.RunAll())

			fm.backward(lo.Must1(indicesTensor.Slice(gorgonia.S(i, i+fm.batchSize))), epoch)
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
			log.Logger().Info(fmt.Sprintf("fit fm %v/%v", epoch, fm.nEpochs), fields...)
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
func (fm *DeepFM) Init(trainSet *Dataset) {
	fm.numFeatures = trainSet.ItemCount() + trainSet.UserCount() + len(trainSet.UserFeatures) + len(trainSet.ItemFeatures) + len(trainSet.ContextFeatures)
	fm.numDimension = 0
	for i := 0; i < trainSet.Count(); i++ {
		_, x, _ := trainSet.Get(i)
		fm.numDimension = mathutil.MaxVal(fm.numDimension, len(x))
	}

	fm.v = fm.GetRandomGenerator().NormalMatrix(fm.numFeatures, fm.nFactors, fm.initMean, fm.initStdDev)
	fm.w = fm.GetRandomGenerator().NormalVector(fm.numFeatures, fm.initMean, fm.initStdDev)
	fm.m_v = zeros(fm.numFeatures, fm.nFactors)
	fm.m_w = make([]float32, fm.numFeatures)
	fm.v_v = zeros(fm.numFeatures, fm.nFactors)
	fm.v_w = make([]float32, fm.numFeatures)
	fm.b = gorgonia.NewMatrix(fm.g, tensor.Float32,
		gorgonia.WithShape(1, 1),
		gorgonia.WithName("b"),
		gorgonia.WithInit(gorgonia.Zeroes()))

	fm.forward(fm.batchSize)
	fm.learnables = []*gorgonia.Node{fm.b, fm.embeddingV, fm.embeddingW}
	lo.Must1(gorgonia.Grad(fm.cost, fm.learnables...))

	fm.vm = gorgonia.NewTapeMachine(fm.g, gorgonia.BindDualValues(fm.learnables...))
	fm.BaseFactorizationMachine.Init(trainSet)
}

func (fm *DeepFM) Marshal(w io.Writer) error {
	return nil
}

func (fm *DeepFM) Bytes() int {
	return 0
}

func (fm *DeepFM) forward(batchSize int) {
	// input nodes
	fm.embeddingV = gorgonia.NodeFromAny(fm.g,
		tensor.New(tensor.WithShape(batchSize, fm.numDimension, fm.nFactors), tensor.WithBacking(make([]float32, batchSize*fm.numDimension*fm.nFactors))),
		gorgonia.WithName("embeddingV"))
	fm.embeddingW = gorgonia.NodeFromAny(fm.g,
		tensor.New(tensor.WithShape(batchSize, fm.numDimension, 1), tensor.WithBacking(make([]float32, batchSize*fm.numDimension))),
		gorgonia.WithName("embeddingW"))
	fm.values = gorgonia.NodeFromAny(fm.g,
		tensor.New(tensor.WithShape(batchSize, fm.numDimension), tensor.WithBacking(make([]float32, batchSize*fm.numDimension))),
		gorgonia.WithName("values"))
	fm.target = gorgonia.NodeFromAny(fm.g,
		tensor.New(tensor.WithShape(batchSize), tensor.WithBacking(make([]float32, batchSize))),
		gorgonia.WithName("target"))

	// factorization machine
	x := gorgonia.Must(gorgonia.Reshape(fm.values, []int{batchSize, fm.numDimension, 1}))
	vx := gorgonia.Must(gorgonia.BatchedMatMul(fm.embeddingV, x, true))
	sumSquare := gorgonia.Must(gorgonia.Square(vx))
	v2 := gorgonia.Must(gorgonia.Square(fm.embeddingV))
	x2 := gorgonia.Must(gorgonia.Square(x))
	squareSum := gorgonia.Must(gorgonia.BatchedMatMul(v2, x2, true))
	sum := gorgonia.Must(gorgonia.Sub(sumSquare, squareSum))
	sum = gorgonia.Must(gorgonia.Sum(sum, 1))
	sum = gorgonia.Must(gorgonia.Mul(sum, fm.nodeFromFloat64(0.5)))
	linear := gorgonia.Must(gorgonia.BatchedMatMul(fm.embeddingW, x, true, false))
	fm.output = gorgonia.Must(gorgonia.BroadcastAdd(
		gorgonia.Must(gorgonia.Reshape(linear, []int{batchSize})),
		fm.b,
		nil, []byte{0},
	))
	fm.output = gorgonia.Must(gorgonia.Add(fm.output, gorgonia.Must(gorgonia.Reshape(sum, []int{batchSize}))))

	// loss function
	fm.cost = fm.bceWithLogits(fm.target, fm.output)
}

func (fm *DeepFM) embedding(indices tensor.View) (v, w *tensor.Dense) {
	s := indices.Shape()
	if len(s) != 2 {
		panic("indices must be 2-dimensional")
	}
	batchSize, numDimension := s[0], s[1]

	dataV := make([]float32, batchSize*numDimension*fm.nFactors)
	dataW := make([]float32, batchSize*numDimension)
	for i := 0; i < batchSize; i++ {
		for j := 0; j < numDimension; j++ {
			index := lo.Must1(indices.At(i, j)).(float32)
			for k := 0; k < fm.nFactors; k++ {
				dataV[i*numDimension*fm.nFactors+j*fm.nFactors+k] = fm.v[int(index)][k]
			}
			dataW[i*numDimension+j] = fm.w[int(index)]
		}
	}

	v = tensor.New(tensor.WithShape(batchSize, numDimension, fm.nFactors), tensor.WithBacking(dataV))
	w = tensor.New(tensor.WithShape(batchSize, numDimension, 1), tensor.WithBacking(dataW))
	return
}

func (fm *DeepFM) backward(indices tensor.View, t int) {
	s := indices.Shape()
	if len(s) != 2 {
		panic("indices must be 2-dimensional")
	}
	batchSize, numDimension := s[0], s[1]

	gradEmbeddingV := lo.Must1(fm.embeddingV.Grad()).Data().([]float32)
	gradEmbeddingW := lo.Must1(fm.embeddingW.Grad()).Data().([]float32)
	gradV := make(map[int][]float32)
	gradW := make(map[int]float32)

	for i := 0; i < batchSize; i++ {
		for j := 0; j < numDimension; j++ {
			index := int(lo.Must1(indices.At(i, j)).(float32))

			if _, exist := gradV[index]; !exist {
				gradV[index] = make([]float32, fm.nFactors)
			}
			for k := 0; k < fm.nFactors; k++ {
				gradV[index][k] += gradEmbeddingV[i*numDimension*fm.nFactors+j*fm.nFactors+k]
			}

			if _, exist := gradW[index]; !exist {
				gradW[index] = 0
			}
			gradW[index] += gradEmbeddingW[i*numDimension+j]
		}
	}

	fm.t++
	correction1 := float32(1 - math32.Pow(beta1, float32(fm.t)))
	correction2 := float32(1 - math32.Pow(beta2, float32(fm.t)))

	for index, grad := range gradV {
		for k := 0; k < fm.nFactors; k++ {
			grad[k] += fm.reg * fm.v[index][k]
			grad[k] /= float32(batchSize)
			// m_t = beta_1 * m_{t-1} + (1 - beta_1) * g_t
			fm.m_v[index][k] = beta1*fm.m_v[index][k] + (1-beta1)*grad[k]
			// v_t = beta_2 * v_{t-1} + (1 - beta_2) * g_t^2
			fm.v_v[index][k] = beta2*fm.v_v[index][k] + (1-beta2)*grad[k]*grad[k]
			// \hat{m}_t = m_t / (1 - beta_1^t)
			mHat := fm.m_v[index][k] / correction1
			// \hat{v}_t = v_t / (1 - beta_2^t)
			vHat := fm.v_v[index][k] / correction2
			// \theta_t = \theta_{t-1} + \eta * \hat{m}_t / (\sqrt{\hat{v}_t} + \epsilon)
			fm.v[index][k] -= fm.lr * mHat / (math32.Sqrt(vHat) + eps)
		}
	}

	for index, grad := range gradW {
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
}

func (fm *DeepFM) convertToTensors(x []lo.Tuple2[[]int32, []float32], y []float32) (indicesTensor, valuesTensor, targetTensor *tensor.Dense) {
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
func (fm *DeepFM) bceWithLogits(target, prediction *gorgonia.Node) *gorgonia.Node {
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

func (fm *DeepFM) nodeFromFloat64(any float32) *gorgonia.Node {
	return gorgonia.NodeFromAny(fm.g, any, gorgonia.WithName(uuid.NewString()))
}

func zeros(a, b int) [][]float32 {
	retVal := make([][]float32, a)
	for i := range retVal {
		retVal[i] = make([]float32, b)
	}
	return retVal
}
