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
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"gorgonia.org/gorgonia"
	"gorgonia.org/tensor"
	"io"
	"modernc.org/mathutil"
)

type DeepFM struct {
	BaseFactorizationMachine
	// Model parameters
	g *gorgonia.ExprGraph
	x *gorgonia.Node
	y *gorgonia.Node

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
	nFactors   int
	nEpochs    int
	lr         float32
	reg        float32
	initMean   float32
	initStdDev float32
}

func NewDeepFM() FactorizationMachine {
	fm := new(DeepFM)
	fm.g = gorgonia.NewGraph()
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
	fm.nFactors = fm.Params.GetInt(model.NFactors, 16)
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
	x := make([]float32, fm.numFeatures)
	for i := range indices {
		x[indices[i]] = values[i]
	}
	vm := gorgonia.NewTapeMachine(fm.g)
	lo.Must0(vm.Let(fm.x, tensor.NewDense(tensor.Float32, []int{fm.numFeatures}, tensor.WithBacking(x))))
	lo.Must0(vm.RunAll())
	return fm.y.Value().Data().(float32)
}

func (fm *DeepFM) Fit(trainSet *Dataset, testSet *Dataset, config *FitConfig) Score {
	fm.Init(trainSet)
	return Score{}
}

func (fm *DeepFM) Init(trainSet *Dataset) {
	fm.numFeatures = trainSet.ItemCount() + trainSet.UserCount() + len(trainSet.UserFeatures) + len(trainSet.ItemFeatures) + len(trainSet.ContextFeatures)
	fm.numDimension = 0
	for i := 0; i < trainSet.Count(); i++ {
		x, _, _ := trainSet.Get(i)
		fm.numDimension = mathutil.MaxVal(fm.numDimension, len(x))
	}
	fmt.Println(fm.numDimension)
	fm.x = gorgonia.NewVector(fm.g, tensor.Float32,
		gorgonia.WithShape(fm.numFeatures),
		gorgonia.WithName("x"),
		gorgonia.WithInit(gorgonia.Zeroes()))
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
	var interactions []*gorgonia.Node
	//interactions = append(interactions, fm.b)
	interactions = append(interactions, gorgonia.Must(gorgonia.Mul(fm.x, fm.w)))
	a := gorgonia.Must(gorgonia.OuterProd(fm.x, fm.x))
	fmt.Println(fm.numFeatures, a.Shape())
	interactions = append(interactions, gorgonia.Must(gorgonia.Sum(a, 0)))
	//for i := 0; i < fm.numFeatures; i++ {
	//	for j := i + 1; j < fm.numFeatures; j++ {
	//		v1 := gorgonia.Must(gorgonia.Slice(fm.v, gorgonia.S(i)))
	//		v2 := gorgonia.Must(gorgonia.Slice(fm.v, gorgonia.S(j)))
	//		x1 := gorgonia.Must(gorgonia.Slice(fm.x, gorgonia.S(i)))
	//		x2 := gorgonia.Must(gorgonia.Slice(fm.x, gorgonia.S(j)))
	//		v1v2 := gorgonia.Must(gorgonia.Mul(v1, v2))
	//		x1x2 := gorgonia.Must(gorgonia.Mul(x1, x2))
	//		interactions = append(interactions, gorgonia.Must(gorgonia.Mul(v1v2, x1x2)))
	//	}
	//}
	fm.y = gorgonia.Must(gorgonia.ReduceAdd(interactions))
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
