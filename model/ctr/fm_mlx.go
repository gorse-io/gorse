//go:build cgo && xla

// Copyright 2025 gorse Project Authors
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
	std_context "context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/c-bata/goptuna"
	"github.com/gomlx/gomlx/backends"
	_ "github.com/gomlx/gomlx/backends/xla"
	"github.com/gomlx/gomlx/pkg/core/dtypes"
	"github.com/gomlx/gomlx/pkg/core/graph"
	"github.com/gomlx/gomlx/pkg/core/shapes"
	"github.com/gomlx/gomlx/pkg/core/tensors"
	mlx_context "github.com/gomlx/gomlx/pkg/ml/context"
	"github.com/gomlx/gomlx/pkg/ml/layers"
	"github.com/gomlx/gomlx/pkg/ml/layers/activations"
	"github.com/gomlx/gomlx/pkg/ml/train"
	"github.com/gomlx/gomlx/pkg/ml/train/losses"
	"github.com/gomlx/gomlx/pkg/ml/train/optimizers"
	"github.com/gorse-io/gorse/common/encoding"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"modernc.org/mathutil"
)

const headerAFM = "AFM"

type AFM struct {
	BaseFactorizationMachines
	mu  sync.RWMutex
	ctx *mlx_context.Context
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
	numFeatures    int
	numDimension   int
	embeddingDim   []int
	embeddingIndex *dataset.Index

	// compiled executors
	predictExecutor *mlx_context.Exec
}

func NewAFM(params model.Params) *AFM {
	fm := new(AFM)
	fm.SetParams(params)
	return fm
}

func (fm *AFM) SuggestParams(trial goptuna.Trial) model.Params {
	return model.Params{
		model.NFactors:   16,
		model.Lr:         lo.Must(trial.SuggestLogFloat(string(model.Lr), 0.001, 0.1)),
		model.Reg:        lo.Must(trial.SuggestLogFloat(string(model.Reg), 0.001, 0.1)),
		model.InitMean:   0,
		model.InitStdDev: lo.Must(trial.SuggestLogFloat(string(model.InitStdDev), 0.001, 0.1)),
	}
}

func (fm *AFM) SetParams(params model.Params) {
	fm.BaseFactorizationMachines.SetParams(params)
	fm.batchSize = fm.Params.GetInt(model.BatchSize, 1024)
	fm.nFactors = fm.Params.GetInt(model.NFactors, 16)
	fm.nEpochs = fm.Params.GetInt(model.NEpochs, 50)
	fm.lr = fm.Params.GetFloat32(model.Lr, 0.001)
	fm.reg = fm.Params.GetFloat32(model.Reg, 0.0002)
	fm.initMean = fm.Params.GetFloat32(model.InitMean, 0)
	fm.initStdDev = fm.Params.GetFloat32(model.InitStdDev, 0.01)
	fm.optimizer = fm.Params.GetString(model.Optimizer, model.Adam)
}

func (fm *AFM) Clear() {
	fm.Index = nil
}

func (fm *AFM) Invalid() bool {
	return fm == nil || fm.Index == nil
}

func (fm *AFM) Predict(_, _ string, _, _ []Label) float32 {
	panic("Predict is unsupported for deep learning models")
}

func (fm *AFM) InternalPredict(_ []int32, _ []float32) float32 {
	panic("InternalPredict is unsupported for deep learning models")
}

func (fm *AFM) attentionForward(ctx *mlx_context.Context, x *graph.Node, dimensions, k int) *graph.Node {
	g := x.Graph()
	// W: Linear(dimensions -> k)
	wCtx := ctx.In("attention_w")
	w := layers.Dense(wCtx, x, true, k)
	w = activations.Relu(w)

	// H: [k, dimensions]
	hCtx := ctx.In("attention_h")
	hVar := hCtx.VariableWithShape("H", shapes.Make(dtypes.F32, k, dimensions))
	h := hVar.ValueGraph(g)

	// Softmax(W * H, 1)
	// w: [batchSize, k]
	// h: [k, dimensions]
	// score: [batchSize, dimensions]
	score := graph.Dot(w, h)
	score = graph.Softmax(score, 1)

	// score * x
	return graph.Mul(score, x)
}

func (fm *AFM) forwardGraph(ctx *mlx_context.Context, indices, values *graph.Node, additionalEmbeddings []*graph.Node) *graph.Node {
	g := indices.Graph()
	batchSize := indices.Shape().Dimensions[0]

	// V: Embedding(numFeatures, nFactors)
	vCtx := ctx.In("V")
	v := layers.Embedding(vCtx, indices, dtypes.F32, fm.numFeatures, fm.nFactors) // [batchSize, numDimension, nFactors]

	// x: values [batchSize, numDimension, 1]
	x := graph.Reshape(values, batchSize, fm.numDimension, 1)

	// vx: BMM(v, x, true, false) -> [batchSize, nFactors, 1]
	// contracting axes: [1] (numDimension), batch axes: [0]
	vx := graph.DotGeneral(v, []int{1}, []int{0}, x, []int{1}, []int{0})

	// Interaction part: 0.5 * sum(vx^2 - sum(v^2 * x^2))
	sumSquare := graph.Square(vx)
	e2 := graph.Square(v)
	x2 := graph.Square(x)
	squareSum := graph.DotGeneral(e2, []int{1}, []int{0}, x2, []int{1}, []int{0})
	interaction := graph.Sub(sumSquare, squareSum)
	interaction = graph.ReduceSum(interaction, 1) // [batchSize, 1]
	interaction = graph.Mul(interaction, graph.Scalar(g, dtypes.F32, 0.5))

	// Linear part: sum(W[indices] * values)
	wCtx := ctx.In("W")
	w := layers.Embedding(wCtx, indices, dtypes.F32, fm.numFeatures, 1) // [batchSize, numDimension, 1]
	linear := graph.DotGeneral(w, []int{1}, []int{0}, x, []int{1}, []int{0})
	linear = graph.Reshape(linear, batchSize, 1)

	// Bias
	bCtx := ctx.In("B")
	bVar := bCtx.VariableWithShape("bias", shapes.Make(dtypes.F32, 1))
	bias := bVar.ValueGraph(g)

	fmOutput := graph.Add(graph.Add(linear, interaction), bias) // [batchSize, 1]

	// Additional embeddings with attention
	for i, embedding := range additionalEmbeddings {
		// A: Attention
		aCtx := ctx.In(fmt.Sprintf("A_%d", i))
		attended := fm.attentionForward(aCtx, embedding, fm.embeddingDim[i], fm.nFactors)

		// E: Linear(dim -> nFactors)
		eCtx := ctx.In(fmt.Sprintf("E_%d", i))
		encoded := layers.Dense(eCtx, attended, true, fm.nFactors)
		encoded = graph.Reshape(encoded, batchSize, fm.nFactors, 1)

		// Output: vx^T * encoded -> [batchSize, 1, 1]
		// vx: [batch, nFactors, 1], encoded: [batch, nFactors, 1]
		// contracting axes: [1] (nFactors), batch axes: [0]
		term := graph.DotGeneral(vx, []int{1}, []int{0}, encoded, []int{1}, []int{0})
		fmOutput = graph.Add(fmOutput, graph.Reshape(term, batchSize, 1))
	}

	return graph.Reshape(fmOutput, batchSize)
}

func (fm *AFM) BatchInternalPredict(x []lo.Tuple2[[]int32, []float32], e [][][]float32, jobs int) []float32 {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if fm.predictExecutor == nil {
		backend, err := backends.New()
		if err != nil {
			panic(err)
		}
		fm.predictExecutor, err = mlx_context.NewExec(backend, fm.ctx, func(ctx *mlx_context.Context, nodes []*graph.Node) *graph.Node {
			indices := nodes[0]
			values := nodes[1]
			var additionalEmbeddings []*graph.Node
			for i := 2; i < len(nodes); i++ {
				additionalEmbeddings = append(additionalEmbeddings, nodes[i])
			}
			return fm.forwardGraph(ctx, indices, values, additionalEmbeddings)
		})
		if err != nil {
			panic(err)
		}
	}

	// Prepare data
	numBatches := (len(x) + fm.batchSize - 1) / fm.batchSize
	predictions := make([]float32, 0, len(x))

	for b := 0; b < numBatches; b++ {
		start := b * fm.batchSize
		end := mathutil.Min(start+fm.batchSize, len(x))
		batchSize := end - start

		indicesData := make([]int32, batchSize*fm.numDimension)
		valuesData := make([]float32, batchSize*fm.numDimension)
		additionalData := make([][]float32, len(fm.embeddingDim))
		for i := range additionalData {
			additionalData[i] = make([]float32, batchSize*fm.embeddingDim[i])
		}

		for i := 0; i < batchSize; i++ {
			row := x[start+i]
			for j := 0; j < len(row.A); j++ {
				indicesData[i*fm.numDimension+j] = row.A[j]
				valuesData[i*fm.numDimension+j] = row.B[j]
			}
			for j := range fm.embeddingDim {
				if len(e[start+i]) > j && len(e[start+i][j]) == fm.embeddingDim[j] {
					copy(additionalData[j][i*fm.embeddingDim[j]:], e[start+i][j])
				}
			}
		}

		inputs := []any{
			indicesData,
			valuesData,
		}
		for i := range additionalData {
			inputs = append(inputs, additionalData[i])
		}

		outputs := fm.predictExecutor.MustExec(inputs...)
		batchPreds := outputs[0].Value().([]float32)
		predictions = append(predictions, batchPreds...)
	}

	return predictions[:len(x)]
}

func (fm *AFM) BatchPredict(inputs []lo.Tuple4[string, string, []Label, []Label], embeddings [][]Embedding, jobs int) []float32 {
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
	e := make([][][]float32, len(inputs))
	for i := range inputs {
		e[i] = make([][]float32, len(fm.embeddingDim))
		for _, embedding := range embeddings[i] {
			itemIndex := fm.embeddingIndex.ToNumber(embedding.Name)
			if itemIndex == dataset.NotId {
				// unknown embedding
				continue
			}
			index := int(itemIndex)
			if len(embedding.Value) != fm.embeddingDim[index] {
				// dimension mismatch
				continue
			}
			e[i][index] = embedding.Value
		}
	}
	return fm.BatchInternalPredict(x, e, jobs)
}

func (fm *AFM) Init(trainSet dataset.CTRSplit) {
	fm.numFeatures = int(trainSet.GetIndex().Len())
	fm.numDimension = 0
	for i := 0; i < trainSet.Count(); i++ {
		_, x, _, _ := trainSet.Get(i)
		fm.numDimension = mathutil.MaxVal(fm.numDimension, len(x))
	}
	fm.embeddingDim = trainSet.GetItemEmbeddingDim()
	fm.embeddingIndex = trainSet.GetItemEmbeddingIndex()

	if fm.ctx == nil {
		fm.ctx = mlx_context.New()
	}
	fm.BaseFactorizationMachines.Init(trainSet)
}

type ctrDataset struct {
	trainSet      dataset.CTRSplit
	numFeatures   int
	numDimension  int
	embeddingDim  []int
	batchSize     int
	currentOffset int
}

func (d *ctrDataset) Name() string { return "CTRDataset" }
func (d *ctrDataset) Reset()      { d.currentOffset = 0 }
func (d *ctrDataset) Yield() (spec any, inputs []*tensors.Tensor, labels []*tensors.Tensor, err error) {
	if d.currentOffset >= d.trainSet.Count() {
		return nil, nil, nil, io.EOF
	}

	batchSize := mathutil.Min(d.batchSize, d.trainSet.Count()-d.currentOffset)
	indicesData := make([]int32, batchSize*d.numDimension)
	valuesData := make([]float32, batchSize*d.numDimension)
	additionalData := make([][]float32, len(d.embeddingDim))
	for i := range additionalData {
		additionalData[i] = make([]float32, batchSize*d.embeddingDim[i])
	}
	labelsData := make([]float32, batchSize)

	for i := 0; i < batchSize; i++ {
		indices, values, embeddings, target := d.trainSet.Get(d.currentOffset + i)
		for j := 0; j < len(indices); j++ {
			indicesData[i*d.numDimension+j] = indices[j]
			valuesData[i*d.numDimension+j] = values[j]
		}
		for j := range d.embeddingDim {
			if len(embeddings) > j && len(embeddings[j]) == d.embeddingDim[j] {
				copy(additionalData[j][i*d.embeddingDim[j]:], embeddings[j])
			}
		}
		labelsData[i] = target
	}

	d.currentOffset += batchSize

	inputs = []*tensors.Tensor{
		tensors.FromFlatDataAndDimensions(indicesData, batchSize, d.numDimension),
		tensors.FromFlatDataAndDimensions(valuesData, batchSize, d.numDimension),
	}
	for i := range additionalData {
		inputs = append(inputs, tensors.FromFlatDataAndDimensions(additionalData[i], batchSize, d.embeddingDim[i]))
	}
	labels = []*tensors.Tensor{tensors.FromFlatDataAndDimensions(labelsData, batchSize)}
	return nil, inputs, labels, nil
}

func (fm *AFM) Fit(ctx std_context.Context, trainSet, testSet dataset.CTRSplit, config *FitConfig) Score {
	log.Logger().Info("fit AFM (mlx)",
		zap.Int("train_set_size", trainSet.Count()),
		zap.Int("test_set_size", testSet.Count()),
		zap.Any("params", fm.GetParams()),
		zap.Any("config", config))
	fm.Init(trainSet)

	evalStart := time.Now()
	score := EvaluateClassification(fm, testSet, config.Jobs)
	scores := []lo.Tuple2[int, float32]{{A: 0, B: score.AUC}}
	evalTime := time.Since(evalStart)
	fields := append([]zap.Field{zap.String("eval_time", evalTime.String())}, score.ZapFields()...)
	log.Logger().Info(fmt.Sprintf("fit AFM %v/%v", 0, fm.nEpochs), fields...)

	backend, err := backends.New()
	if err != nil {
		panic(err)
	}
	modelFn := func(ctx *mlx_context.Context, spec any, inputs []*graph.Node) []*graph.Node {
		return []*graph.Node{fm.forwardGraph(ctx, inputs[0], inputs[1], inputs[2:])}
	}
	lossFn := func(labels, predictions []*graph.Node) *graph.Node {
		return graph.ReduceMean(losses.BinaryCrossentropyLogits(labels, predictions), 0)
	}

	optimizer := optimizers.Adam().LearningRate(float64(fm.lr)).Done()
	trainer := train.NewTrainer(backend, fm.ctx, modelFn, lossFn, optimizer, nil, nil)
	loop := train.NewLoop(trainer)

	ds := &ctrDataset{
		trainSet:     trainSet,
		numFeatures:  fm.numFeatures,
		numDimension: fm.numDimension,
		embeddingDim: fm.embeddingDim,
		batchSize:    fm.batchSize,
	}

	_, span := monitor.Start(ctx, "FM.Fit", fm.nEpochs)
	defer span.End()

	for epoch := 1; epoch <= fm.nEpochs; epoch++ {
		fitStart := time.Now()
		ds.Reset()
		_, err := loop.RunSteps(ds, (trainSet.Count()+fm.batchSize-1)/fm.batchSize)
		if err != nil {
			log.Logger().Error("fit AFM failed", zap.Error(err))
			break
		}
		fitTime := time.Since(fitStart)

		if epoch%config.Verbose == 0 || epoch == fm.nEpochs {
			evalStart = time.Now()
			score = EvaluateClassification(fm, testSet, config.Jobs)
			scores = append(scores, lo.Tuple2[int, float32]{A: epoch, B: score.AUC})
			evalTime = time.Since(evalStart)
			fields = append([]zap.Field{
				zap.String("fit_time", fitTime.String()),
				zap.String("eval_time", evalTime.String()),
			}, score.ZapFields()...)
			log.Logger().Info(fmt.Sprintf("fit AFM %v/%v", epoch, fm.nEpochs), fields...)

			if config.Patience > 0 && epoch > config.Patience {
				epochScore := lo.MaxBy(scores, func(a, b lo.Tuple2[int, float32]) bool { return a.B > b.B })
				if epochScore.A <= epoch-config.Patience {
					log.Logger().Info("early stopping",
						zap.Int("best_epoch", epochScore.A),
						zap.Float32("best_auc", epochScore.B),
						zap.Int("patience", config.Patience))
					break
				}
			}
		}
		span.Add(1)
	}

	return score
}

func (fm *AFM) Marshal(w io.Writer) error {
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
	if err := encoding.WriteGob(w, fm.embeddingDim); err != nil {
		return errors.Trace(err)
	}
	if len(fm.embeddingDim) > 0 {
		if err := dataset.MarshalIndex(w, fm.embeddingIndex); err != nil {
			return errors.Trace(err)
		}
	}
	// write parameters (GoMLX variables)
	variables := make(map[string]lo.Tuple2[[]int, []float32])
	fm.ctx.EnumerateVariables(func(v *mlx_context.Variable) {
		val, err := v.Value()
		if err != nil {
			log.Logger().Error("failed to get variable value", zap.Error(err))
			return
		}
		var flatData []float32
		val.MustConstFlatData(func(flat any) {
			flatData = flat.([]float32)
		})
		variables[v.Name()] = lo.Tuple2[[]int, []float32]{
			A: val.Shape().Dimensions,
			B: flatData,
		}
	})
	if err := encoding.WriteGob(w, variables); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (fm *AFM) Unmarshal(r io.Reader) error {
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
	if err = encoding.ReadGob(r, &fm.embeddingDim); err != nil {
		return errors.Trace(err)
	}
	if len(fm.embeddingDim) > 0 {
		fm.embeddingIndex, err = dataset.UnmarshalIndex(r)
		if err != nil {
			return errors.Trace(err)
		}
	}
	// read parameters
	var variables map[string]lo.Tuple2[[]int, []float32]
	if err = encoding.ReadGob(r, &variables); err != nil {
		return errors.Trace(err)
	}
	if fm.ctx == nil {
		fm.ctx = mlx_context.New()
	}
	for name, data := range variables {
		t := tensors.FromFlatDataAndDimensions(data.B, data.A...)
		fm.ctx.VariableWithValue(name, t)
	}
	return nil
}
