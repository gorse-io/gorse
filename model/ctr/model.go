// Copyright 2020 gorse Project Authors
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
	"context"
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"

	"github.com/c-bata/goptuna"
	"github.com/chewxy/math32"
	"github.com/gorse-io/gorse/common/encoding"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/common/nn"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"modernc.org/mathutil"
)

type Score struct {
	RMSE      float32
	Precision float32
	Recall    float32
	Accuracy  float32
	AUC       float32
}

func (score Score) ZapFields() []zap.Field {
	return []zap.Field{
		zap.Float32("Accuracy", score.Accuracy),
		zap.Float32("Precision", score.Precision),
		zap.Float32("Recall", score.Recall),
		zap.Float32("AUC", score.AUC),
	}
}

func (score Score) GetValue() float32 {
	return score.Precision
}

func (score Score) BetterThan(s Score) bool {
	return score.AUC > s.AUC
}

type FitConfig struct {
	Jobs    int
	Verbose int
}

func NewFitConfig() *FitConfig {
	return &FitConfig{
		Jobs:    1,
		Verbose: 10,
	}
}

func (config *FitConfig) SetVerbose(verbose int) *FitConfig {
	config.Verbose = verbose
	return config
}

func (config *FitConfig) SetJobs(jobs int) *FitConfig {
	config.Jobs = jobs
	return config
}

func (config *FitConfig) LoadDefaultIfNil() *FitConfig {
	if config == nil {
		return NewFitConfig()
	}
	return config
}

type FactorizationMachines interface {
	model.Model
	Predict(userId, itemId string, userFeatures, itemFeatures []Label) float32
	InternalPredict(x []int32, values []float32) float32
	Fit(ctx context.Context, trainSet, testSet dataset.CTRSplit, config *FitConfig) Score
	Marshal(w io.Writer) error
}

type BatchInference interface {
	BatchPredict(inputs []lo.Tuple4[string, string, []Label, []Label], jobs int) []float32
	BatchInternalPredict(x []lo.Tuple2[[]int32, []float32], jobs int) []float32
}

type FactorizationMachineCloner interface {
	Clone() FactorizationMachines
}

type FactorizationMachineSpawner interface {
	Spawn() FactorizationMachines
}

type BaseFactorizationMachines struct {
	model.BaseModel
	Index dataset.UnifiedIndex
}

func (b *BaseFactorizationMachines) Init(trainSet dataset.CTRSplit) {
	b.Index = trainSet.GetIndex()
}

func MarshalModel(w io.Writer, m FactorizationMachines) error {
	// write header
	var err error
	switch m.(type) {
	case *FMV2:
		err = encoding.WriteString(w, headerFMV2)
	default:
		return fmt.Errorf("unknown model: %v", reflect.TypeOf(m))
	}
	if err != nil {
		return err
	}
	return m.Marshal(w)
}

const headerFMV2 = "FM2"

func UnmarshalModel(r io.Reader) (FactorizationMachines, error) {
	// read header
	header, err := encoding.ReadString(r)
	if err != nil {
		return nil, err
	}
	switch header {
	case headerFMV2:
		var fm FMV2
		if err := fm.Unmarshal(r); err != nil {
			return nil, errors.Trace(err)
		}
		return &fm, nil
	}
	return nil, fmt.Errorf("unknown model: %v", header)
}

func Spawn(m FactorizationMachines) FactorizationMachines {
	if cloner, ok := m.(FactorizationMachineSpawner); ok {
		return cloner.Spawn()
	}
	return m
}

type FMV2 struct {
	BaseFactorizationMachines
	mu sync.RWMutex
	// parameters
	B *nn.Tensor
	W nn.Layer
	V nn.Layer
	// hyper parameters
	batchSize  int
	nFactors   int
	nEpochs    int
	lr         float32
	reg        float32
	initMean   float32
	initStdDev float32
	optimizer  string
	// dataset stats
	numFeatures  int
	numDimension int
}

func NewFMV2(params model.Params) *FMV2 {
	fm := new(FMV2)
	fm.SetParams(params)
	return fm
}

func (fm *FMV2) SuggestParams(trial goptuna.Trial) model.Params {
	return model.Params{
		model.NFactors:   16,
		model.Lr:         lo.Must(trial.SuggestLogFloat(string(model.Lr), 0.001, 0.1)),
		model.Reg:        lo.Must(trial.SuggestLogFloat(string(model.Reg), 0.001, 0.1)),
		model.InitMean:   0,
		model.InitStdDev: lo.Must(trial.SuggestLogFloat(string(model.InitStdDev), 0.001, 0.1)),
	}
}

func (fm *FMV2) SetParams(params model.Params) {
	fm.BaseFactorizationMachines.SetParams(params)
	fm.batchSize = fm.Params.GetInt(model.BatchSize, 1024)
	fm.nFactors = fm.Params.GetInt(model.NFactors, 16)
	fm.nEpochs = fm.Params.GetInt(model.NEpochs, 50)
	fm.lr = fm.Params.GetFloat32(model.Lr, 0.01)
	fm.reg = fm.Params.GetFloat32(model.Reg, 0.0)
	fm.initMean = fm.Params.GetFloat32(model.InitMean, 0)
	fm.initStdDev = fm.Params.GetFloat32(model.InitStdDev, 0.01)
	fm.optimizer = fm.Params.GetString(model.Optimizer, model.Adam)
}

func (fm *FMV2) Clear() {
	fm.Index = nil
}

func (fm *FMV2) Invalid() bool {
	return fm == nil || fm.Index == nil
}

func (fm *FMV2) Forward(indices, values *nn.Tensor, jobs int) *nn.Tensor {
	batchSize := indices.Shape()[0]
	v := fm.V.Forward(indices)
	x := nn.Reshape(values, batchSize, fm.numDimension, 1)
	vx := nn.BMM(v, x, true, false, jobs)
	sumSquare := nn.Square(vx)
	e2 := nn.Square(v)
	x2 := nn.Square(x)
	squareSum := nn.BMM(e2, x2, true, false, jobs)
	sum := nn.Sub(sumSquare, squareSum)
	sum = nn.Sum(sum, 1)
	sum = nn.Mul(sum, nn.NewScalar(0.5))
	w := fm.W.Forward(indices)
	linear := nn.BMM(w, x, true, false, jobs)
	fmOutput := nn.Add(nn.Reshape(linear, batchSize), nn.Reshape(sum, batchSize), fm.B)
	return nn.Flatten(fmOutput)
}

func (fm *FMV2) Parameters() []*nn.Tensor {
	var params []*nn.Tensor
	params = append(params, fm.B)
	params = append(params, fm.V.Parameters()...)
	params = append(params, fm.W.Parameters()...)
	return params
}

func (fm *FMV2) Predict(_, _ string, _, _ []Label) float32 {
	panic("Predict is unsupported for deep learning models")
}

func (fm *FMV2) InternalPredict(_ []int32, _ []float32) float32 {
	panic("InternalPredict is unsupported for deep learning models")
}

func (fm *FMV2) BatchInternalPredict(x []lo.Tuple2[[]int32, []float32], jobs int) []float32 {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	indicesTensor, valuesTensor, _ := fm.convertToTensors(x, nil)
	predictions := make([]float32, 0, len(x))
	for i := 0; i < len(x); i += fm.batchSize {
		j := mathutil.Min(i+fm.batchSize, len(x))
		output := fm.Forward(indicesTensor.Slice(i, j), valuesTensor.Slice(i, j), jobs)
		predictions = append(predictions, output.Data()...)
	}
	return predictions[:len(x)]
}

func (fm *FMV2) BatchPredict(inputs []lo.Tuple4[string, string, []Label, []Label], jobs int) []float32 {
	x := make([]lo.Tuple2[[]int32, []float32], len(inputs))
	for i, input := range inputs {
		// encode user
		if userIndex := fm.Index.EncodeUser(input.A); userIndex != dataset.NotId {
			x[i].A = append(x[i].A, userIndex)
			x[i].B = append(x[i].B, 1)
		}
		// encode item
		if itemIndex := fm.Index.EncodeItem(input.B); itemIndex != dataset.NotId {
			x[i].A = append(x[i].A, itemIndex)
			x[i].B = append(x[i].B, 1)
		}
		// encode user labels
		for _, userFeature := range input.C {
			if userFeatureIndex := fm.Index.EncodeUserLabel(userFeature.Name); userFeatureIndex != dataset.NotId {
				x[i].A = append(x[i].A, userFeatureIndex)
				x[i].B = append(x[i].B, userFeature.Value)
			}
		}
		// encode item labels
		for _, itemFeature := range input.D {
			if itemFeatureIndex := fm.Index.EncodeItemLabel(itemFeature.Name); itemFeatureIndex != dataset.NotId {
				x[i].A = append(x[i].A, itemFeatureIndex)
				x[i].B = append(x[i].B, itemFeature.Value)
			}
		}
	}
	return fm.BatchInternalPredict(x, jobs)
}

func (fm *FMV2) Init(trainSet dataset.CTRSplit) {
	fm.numFeatures = int(trainSet.GetIndex().Len())
	fm.numDimension = 0
	for i := 0; i < trainSet.Count(); i++ {
		_, x, _ := trainSet.Get(i)
		fm.numDimension = mathutil.MaxVal(fm.numDimension, len(x))
	}
	fm.B = nn.Zeros()
	fm.W = nn.NewEmbedding(int(trainSet.GetIndex().Len()), 1)
	fm.V = nn.NewEmbedding(int(trainSet.GetIndex().Len()), fm.nFactors)
	fm.BaseFactorizationMachines.Init(trainSet)
}

func (fm *FMV2) Fit(ctx context.Context, trainSet, testSet dataset.CTRSplit, config *FitConfig) Score {
	fm.Init(trainSet)
	evalStart := time.Now()
	score := EvaluateClassification(fm, testSet, config.Jobs)
	evalTime := time.Since(evalStart)
	fields := append([]zap.Field{zap.String("eval_time", evalTime.String())}, score.ZapFields()...)
	log.Logger().Info(fmt.Sprintf("fit DeepFM %v/%v", 0, fm.nEpochs), fields...)

	var x []lo.Tuple2[[]int32, []float32]
	var y []float32
	for i := 0; i < trainSet.Count(); i++ {
		indices, values, target := trainSet.Get(i)
		x = append(x, lo.Tuple2[[]int32, []float32]{A: indices, B: values})
		y = append(y, target)
	}
	indices, values, target := fm.convertToTensors(x, y)

	var optimizer nn.Optimizer
	switch fm.optimizer {
	case model.SGD:
		optimizer = nn.NewSGD(fm.Parameters(), fm.lr)
	case model.Adam:
		optimizer = nn.NewAdam(fm.Parameters(), fm.lr)
	default:
		panic("unknown optimizer")
	}
	optimizer.SetWeightDecay(fm.reg)
	optimizer.SetJobs(config.Jobs)
	_, span := monitor.Start(ctx, "FM.Fit", fm.nEpochs)
	for epoch := 1; epoch <= fm.nEpochs; epoch++ {
		fitStart := time.Now()
		cost := float32(0)
		for i := 0; i < trainSet.Count(); i += fm.batchSize {
			j := mathutil.Min(i+fm.batchSize, trainSet.Count())
			batchIndices := indices.Slice(i, j)
			batchValues := values.Slice(i, j)
			batchTarget := target.Slice(i, j)
			batchOutput := fm.Forward(batchIndices, batchValues, config.Jobs)
			batchLoss := nn.BCEWithLogits(batchTarget, batchOutput, nil)
			cost += batchLoss.Data()[0]
			optimizer.ZeroGrad()
			batchLoss.Backward()
			optimizer.Step()
		}

		fitTime := time.Since(fitStart)
		// Cross validation
		if epoch%config.Verbose == 0 || epoch == fm.nEpochs {
			evalStart = time.Now()
			score = EvaluateClassification(fm, testSet, config.Jobs)
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
		span.Add(1)
	}
	span.End()
	return score
}

func (fm *FMV2) Marshal(w io.Writer) error {
	// write params
	if err := encoding.WriteGob(w, fm.Params); err != nil {
		return errors.Trace(err)
	}
	// write index
	if err := dataset.MarshalUnifiedIndex(w, fm.Index); err != nil {
		return errors.Trace(err)
	}
	// write dataset stats
	if err := encoding.WriteGob(w, fm.numFeatures); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.WriteGob(w, fm.numDimension); err != nil {
		return errors.Trace(err)
	}
	// write parameters
	if err := nn.Save(fm.Parameters(), w); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (fm *FMV2) Unmarshal(r io.Reader) error {
	// read params
	err := encoding.ReadGob(r, &fm.Params)
	if err != nil {
		return errors.Trace(err)
	}
	fm.SetParams(fm.Params)
	// read index
	fm.Index, err = dataset.UnmarshalUnifiedIndex(r)
	if err != nil {
		return errors.Trace(err)
	}
	// read dataset stats
	if err = encoding.ReadGob(r, &fm.numFeatures); err != nil {
		return errors.Trace(err)
	}
	if err = encoding.ReadGob(r, &fm.numDimension); err != nil {
		return errors.Trace(err)
	}
	// read parameters
	fm.B = nn.Zeros()
	fm.W = nn.NewEmbedding(fm.numFeatures, 1)
	fm.V = nn.NewEmbedding(fm.numFeatures, fm.nFactors)
	if err = nn.Load(fm.Parameters(), r); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (fm *FMV2) convertToTensors(x []lo.Tuple2[[]int32, []float32], y []float32) (indicesTensor, valuesTensor, targetTensor *nn.Tensor) {
	if y != nil && len(x) != len(y) {
		panic("length of x and y must be equal")
	}

	alignedIndices := make([]float32, len(x)*fm.numDimension)
	alignedValues := make([]float32, len(x)*fm.numDimension)
	alignedTarget := make([]float32, len(x))
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

	indicesTensor = nn.NewTensor(alignedIndices, len(x), fm.numDimension)
	valuesTensor = nn.NewTensor(alignedValues, len(x), fm.numDimension)
	if y != nil {
		targetTensor = nn.NewTensor(alignedTarget, len(x))
	}
	return
}
