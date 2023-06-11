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
	"fmt"
	"io"
	"time"

	"github.com/chewxy/math32"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/model"
	"go.uber.org/zap"
	"gorgonia.org/gorgonia"
	"gorgonia.org/tensor"
	"modernc.org/mathutil"
)

type DeepFM struct {
	BaseFactorizationMachine

	vm     gorgonia.VM
	g      *gorgonia.ExprGraph
	v_i    []*gorgonia.Node
	w_i    []*gorgonia.Node
	x_i    []*gorgonia.Node
	y      *gorgonia.Node
	target *gorgonia.Node
	cost   *gorgonia.Node

	learnables   []*gorgonia.Node
	v            *gorgonia.Node
	w            *gorgonia.Node
	b            *gorgonia.Node
	B            float32
	MinTarget    float32
	MaxTarget    float32
	Task         FMTask
	numFeatures  int
	numDimension int
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
	fm.B = 0.0
	fm.Index = nil
}

func (fm *DeepFM) Invalid() bool {
	return fm == nil ||
		fm.Index == nil
}

func (fm *DeepFM) SetParams(params model.Params) {
	fm.batchSize = 128
	fm.nFactors = fm.Params.GetInt(model.NFactors, 16)
	fm.nEpochs = fm.Params.GetInt(model.NEpochs, 200)
	fm.lr = fm.Params.GetFloat32(model.Lr, 0.01)
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
	fm.vm.Reset()
	for i := range indices {
		lo.Must0(gorgonia.UnsafeLet(fm.v_i[i], gorgonia.S(int(indices[i]))))
		lo.Must0(gorgonia.UnsafeLet(fm.w_i[i], gorgonia.S(int(indices[i]))))
		lo.Must0(gorgonia.Let(fm.x_i[i], values[i]))
	}
	lo.Must0(fm.vm.RunAll())
	return fm.y.Value().Data().(float32)
}

func (fm *DeepFM) Fit(trainSet *Dataset, testSet *Dataset, config *FitConfig) Score {
	fm.Init(trainSet)
	evalStart := time.Now()
	score := EvaluateClassification(fm, testSet)
	evalTime := time.Since(evalStart)
	fields := append([]zap.Field{zap.String("eval_time", evalTime.String())}, score.ZapFields()...)
	log.Logger().Info(fmt.Sprintf("fit fm %v/%v", 0, fm.nEpochs), fields...)

	lo.Must1(gorgonia.Grad(fm.cost, fm.learnables...))
	solver := gorgonia.NewAdamSolver()
	vm := gorgonia.NewTapeMachine(fm.g, gorgonia.BindDualValues(fm.learnables...))

	for epoch := 1; epoch <= fm.nEpochs; epoch++ {
		for i := 0; i < trainSet.Target.Len(); i++ {
			fm.MinTarget = math32.Min(fm.MinTarget, trainSet.Target.Get(i))
			fm.MaxTarget = math32.Max(fm.MaxTarget, trainSet.Target.Get(i))
		}
		fitStart := time.Now()
		cost := float32(0)

		for i := 0; i < trainSet.Count(); i++ {
			indices, values, y := trainSet.Get(i)
			for i := range indices {
				lo.Must0(gorgonia.UnsafeLet(fm.v_i[i], gorgonia.S(int(indices[i]))))
				lo.Must0(gorgonia.UnsafeLet(fm.w_i[i], gorgonia.S(int(indices[i]))))
				lo.Must0(gorgonia.Let(fm.x_i[i], values[i]))
			}
			lo.Must0(gorgonia.Let(fm.target, y))
			lo.Must0(vm.RunAll())

			cost += fm.cost.Value().Data().(float32)
			lo.Must0(solver.Step(gorgonia.NodesToValueGrads(fm.learnables)))
			vm.Reset()
			fmt.Printf("\r[%d/%d] %v", i+1, trainSet.Count(), cost/float32(i+1))
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
		config.Task.Add(1)
	}
	return score
}

func (fm *DeepFM) Init(trainSet *Dataset) {
	fm.numFeatures = trainSet.ItemCount() + trainSet.UserCount() + len(trainSet.UserFeatures) + len(trainSet.ItemFeatures) + len(trainSet.ContextFeatures)
	fm.numDimension = 0
	for i := 0; i < trainSet.Count(); i++ {
		_, x, _ := trainSet.Get(i)
		fm.numDimension = mathutil.MaxVal(fm.numDimension, len(x))
	}

	fm.v = gorgonia.NewMatrix(fm.g, tensor.Float32,
		gorgonia.WithShape(fm.numFeatures, fm.nFactors),
		gorgonia.WithName("v"),
		gorgonia.WithInit(gorgonia.Gaussian(float64(fm.initMean), float64(fm.initStdDev))))
	fm.w = gorgonia.NewVector(fm.g, tensor.Float32,
		gorgonia.WithShape(fm.numFeatures),
		gorgonia.WithName("w"),
		gorgonia.WithInit(gorgonia.Gaussian(float64(fm.initMean), float64(fm.initStdDev))))
	fm.b = gorgonia.NewScalar(fm.g, tensor.Float32,
		gorgonia.WithName("b"),
		gorgonia.WithInit(gorgonia.Zeroes()))
	fm.learnables = []*gorgonia.Node{fm.v}

	fm.v_i = make([]*gorgonia.Node, fm.numDimension)
	fm.w_i = make([]*gorgonia.Node, fm.numDimension)
	fm.x_i = make([]*gorgonia.Node, fm.numDimension)
	for i := 0; i < fm.numDimension; i++ {
		fm.v_i[i] = gorgonia.Must(gorgonia.Slice(fm.v, gorgonia.S(i)))
		fm.w_i[i] = gorgonia.Must(gorgonia.Slice(fm.w, gorgonia.S(i)))
		fm.x_i[i] = gorgonia.NodeFromAny(fm.g, float32(0), gorgonia.WithName(fmt.Sprintf("x_%d", i)))
	}
	fm.target = gorgonia.NodeFromAny(fm.g, float32(0), gorgonia.WithName("target"))

	var interactions []*gorgonia.Node
	for i := 0; i < fm.numDimension; i++ {
		interactions = append(interactions, gorgonia.Must(gorgonia.Mul(fm.w_i[i], fm.x_i[i])))
		for j := i + 1; j < fm.numDimension; j++ {
			v1v2 := gorgonia.Must(gorgonia.Mul(fm.v_i[i], fm.v_i[j]))
			x1x2 := gorgonia.Must(gorgonia.Mul(fm.x_i[i], fm.x_i[j]))
			interactions = append(interactions, gorgonia.Must(gorgonia.Mul(v1v2, x1x2)))
		}
	}
	fm.y = gorgonia.Must(gorgonia.ReduceAdd(interactions))
	// fm.y = gorgonia.Must(gorgonia.Add(fm.y, fm.b))

	fm.cost = gorgonia.Must(gorgonia.Div(gorgonia.Must(gorgonia.Add(
		gorgonia.Must(gorgonia.Mul(
			gorgonia.Must(gorgonia.Add(gorgonia.NodeFromAny(fm.g, float32(1)), fm.target)),
			gorgonia.Must(gorgonia.Log(
				gorgonia.Must(gorgonia.Add(gorgonia.NodeFromAny(fm.g, float32(1)),
					gorgonia.Must(gorgonia.Exp(gorgonia.Must(gorgonia.Neg(fm.y)))))))))),
		gorgonia.Must(gorgonia.Mul(
			gorgonia.Must(gorgonia.Sub(gorgonia.NodeFromAny(fm.g, float32(1)), fm.target)),
			gorgonia.Must(gorgonia.Log(
				gorgonia.Must(gorgonia.Add(gorgonia.NodeFromAny(fm.g, float32(1)),
					gorgonia.Must(gorgonia.Exp(fm.y)))))))))),
		gorgonia.NodeFromAny(fm.g, float32(2))))

	fm.vm = gorgonia.NewTapeMachine(fm.g)
	fm.BaseFactorizationMachine.Init(trainSet)
}

func (fm *DeepFM) Marshal(w io.Writer) error {
	return nil
}

func (fm *DeepFM) Bytes() int {
	return 0
}

func (fm *DeepFM) Complexity() int {
	return 0
}
