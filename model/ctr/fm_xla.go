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
	"context"
	"fmt"
	"io"
	"iter"
	"sync"
	"time"

	"github.com/c-bata/goptuna"
	"github.com/gomlx/compute"
	"github.com/gomlx/compute/dtypes"
	"github.com/gomlx/compute/shapes"
	_ "github.com/gomlx/go-xla/compute/xla/autoinstall"
	"github.com/gomlx/gomlx/core/graph"
	"github.com/gomlx/gomlx/core/tensors"
	"github.com/gomlx/gomlx/ml/layers"
	"github.com/gomlx/gomlx/ml/layers/activation"
	mlx_model "github.com/gomlx/gomlx/ml/model"
	"github.com/gomlx/gomlx/ml/model/initializer"
	"github.com/gomlx/gomlx/ml/train"
	"github.com/gomlx/gomlx/ml/train/loss"
	"github.com/gomlx/gomlx/ml/train/optimizer"
	"github.com/gorse-io/gorse/common/bfloats"
	"github.com/gorse-io/gorse/common/encoding"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

const (
	headerAFM  = "AFM"
	headerAFM2 = "AFM2"
)

type AFM struct {
	BaseFactorizationMachines
	mu      sync.RWMutex
	store   *mlx_model.Store
	backend compute.Backend
	// hyper parameters
	batchSize  int
	nFactors   int
	nEpochs    int
	lr         float32
	reg        float32
	initMean   float32
	initStdDev float32
	optimizer  string
	autoScale  bool
	// dataset stats
	numFeatures    int
	numDimension   int
	embeddingDim   []int
	embeddingIndex *dataset.Index
	// numerical feature scalers: feature_index -> AutoScaler
	Scalers map[int32]*AutoScaler

	// compiled executors
	predictExecutor *mlx_model.ExecOneOutput
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
	fm.autoScale = fm.Params.GetBool(model.AutoScale, true)
}

func (fm *AFM) Clear() {
	fm.Index = nil
	fm.Scalers = nil
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

func (fm *AFM) attentionForward(scope *mlx_model.Scope, x *graph.Node, dimensions, k int) *graph.Node {
	g := x.Graph()
	// W: Linear(dimensions -> k)
	wCtx := scope.In("attention_w")
	w := layers.Dense(wCtx, x, true, k)
	w = activation.Relu(w)

	// H: [k, dimensions]
	hCtx := scope.In("attention_h")
	hVar := hCtx.VariableWithShape("H", shapes.Make(dtypes.F32, k, dimensions))
	h := hVar.NodeValue(g)

	// Softmax(W * H, 1)
	// w: [batchSize, k]
	// h: [k, dimensions]
	// score: [batchSize, dimensions]
	score := graph.DotGeneral(w, []int{1}, nil, h, []int{0}, nil)
	score = graph.Softmax(score, 1)

	// score * x
	return graph.Mul(score, x)
}

func (fm *AFM) forwardGraph(scope *mlx_model.Scope, indices, values *graph.Node, additionalEmbeddings []*graph.Node) *graph.Node {
	scope = scope.WithInitializer(initializer.RandomNormalFn(scope, float64(fm.initStdDev)))
	g := indices.Graph()
	batchSize := indices.Shape().Dimensions[0]
	numDimension := indices.Shape().Dimensions[1]

	// V: Embedding(numFeatures, nFactors)
	vCtx := scope.In("V")
	v := layers.Embedding(vCtx, indices, dtypes.F32, fm.numFeatures, fm.nFactors) // [batchSize, numDimension, nFactors]

	// x: values [batchSize, numDimension, 1]
	x := graph.Reshape(values, batchSize, numDimension, 1)

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
	wCtx := scope.In("W")
	w := layers.Embedding(wCtx, indices, dtypes.F32, fm.numFeatures, 1) // [batchSize, numDimension, 1]
	linear := graph.DotGeneral(w, []int{1}, []int{0}, x, []int{1}, []int{0})
	linear = graph.Reshape(linear, batchSize, 1)

	// Bias
	bCtx := scope.In("B")
	bVar := bCtx.VariableWithShape("bias", shapes.Make(dtypes.F32, 1))
	bias := bVar.NodeValue(g)
	bias = graph.Reshape(bias, 1, 1) // Reshape to [1, 1] for broadcasting with [batchSize, 1]

	fmOutput := graph.Add(graph.Add(linear, interaction), bias) // [batchSize, 1]

	// Additional embeddings with attention
	for i, embedding := range additionalEmbeddings {
		// A: Attention
		aCtx := scope.In("A_%d", i)
		attended := fm.attentionForward(aCtx, embedding, fm.embeddingDim[i], fm.nFactors)

		// E: Linear(dim -> nFactors)
		eCtx := scope.In("E_%d", i)
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

// applyScalers applies scalers to numerical features in the input.
func (fm *AFM) applyScalers(x []lo.Tuple2[[]int32, []float32]) []lo.Tuple2[[]int32, []float32] {
	result := make([]lo.Tuple2[[]int32, []float32], len(x))
	for i, sample := range x {
		result[i].A = sample.A
		result[i].B = make([]float32, len(sample.B))
		copy(result[i].B, sample.B)
		for j, idx := range sample.A {
			if scaler, ok := fm.Scalers[idx]; ok {
				result[i].B[j] = scaler.Transform(sample.B[j])
			}
		}
	}
	return result
}

func (fm *AFM) BatchInternalPredict(x []lo.Tuple2[[]int32, []float32], e [][][]uint16, jobs int) []float32 {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	// Apply scalers to numerical features if enabled
	var scaledX []lo.Tuple2[[]int32, []float32]
	if fm.autoScale {
		scaledX = fm.applyScalers(x)
	} else {
		scaledX = x
	}

	if fm.predictExecutor == nil {
		var err error
		fm.predictExecutor, err = mlx_model.NewExec1(fm.backend, fm.store, func(scope *mlx_model.Scope, nodes []*graph.Node) *graph.Node {
			return fm.forwardGraph(scope, nodes[0], nodes[1], nodes[2:])
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
		end := min(start+fm.batchSize, len(x))
		batchSize := end - start
		numDimension := fm.numDimension
		for i := start; i < end; i++ {
			numDimension = max(numDimension, len(scaledX[i].A))
		}

		indicesData := make([]int32, batchSize*numDimension)
		valuesData := make([]float32, batchSize*numDimension)
		additionalData := make([][]float32, len(fm.embeddingDim))
		for i := range additionalData {
			additionalData[i] = make([]float32, batchSize*fm.embeddingDim[i])
		}

		for i := 0; i < batchSize; i++ {
			row := scaledX[start+i]
			for j := 0; j < len(row.A); j++ {
				indicesData[i*numDimension+j] = row.A[j]
				valuesData[i*numDimension+j] = row.B[j]
			}
			for j := range fm.embeddingDim {
				if len(e[start+i]) > j && len(e[start+i][j]) == fm.embeddingDim[j] {
					copy(additionalData[j][i*fm.embeddingDim[j]:], bfloats.ToFloat32(e[start+i][j]))
				}
			}
		}

		inputs := []any{
			tensors.FromFlatDataAndDimensions(indicesData, batchSize, numDimension),
			tensors.FromFlatDataAndDimensions(valuesData, batchSize, numDimension),
		}
		for i := range additionalData {
			inputs = append(inputs, tensors.FromFlatDataAndDimensions(additionalData[i], batchSize, fm.embeddingDim[i]))
		}

		output := fm.predictExecutor.MustCall(inputs...)
		batchPreds := output.Value().([]float32)
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
	e := make([][][]uint16, len(inputs))
	for i := range inputs {
		e[i] = make([][]uint16, len(fm.embeddingDim))
		if fm.embeddingIndex == nil {
			continue
		}
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
		fm.numDimension = max(fm.numDimension, len(x))
	}
	fm.embeddingDim = trainSet.GetItemEmbeddingDim()
	fm.embeddingIndex = trainSet.GetItemEmbeddingIndex()

	if fm.store == nil {
		fm.store = mlx_model.NewStore()
		fm.store.SetParam(initializer.ParamInitialSeed, int64(42))
	}
	if fm.backend == nil {
		var err error
		fm.backend, err = compute.New()
		if err != nil {
			panic(err)
		}
	}
	// Collect numerical features and fit scalers if enabled
	if fm.autoScale {
		fm.fitScalers(trainSet)
	}
	fm.BaseFactorizationMachines.Init(trainSet)
}

// fitScalers collects numerical feature values and fits AutoScaler for each.
func (fm *AFM) fitScalers(trainSet dataset.CTRSplit) {
	fm.Scalers = make(map[int32]*AutoScaler)

	// Collect values for each feature index
	featureValues := make(map[int32][]float32)
	for i := 0; i < trainSet.Count(); i++ {
		indices, values, _, _ := trainSet.Get(i)
		for j, idx := range indices {
			featureValues[idx] = append(featureValues[idx], values[j])
		}
	}

	// Identify numerical features (values not all equal to 1) and fit scalers
	for idx, values := range featureValues {
		isNumerical := false
		for _, v := range values {
			if v != 1 {
				isNumerical = true
				break
			}
		}
		if isNumerical {
			scaler := NewAutoScaler()
			scaler.Fit(values)
			fm.Scalers[idx] = scaler
		}
	}

	if len(fm.Scalers) > 0 {
		log.Logger().Info("fitted scalers for numerical features",
			zap.Int("num_numerical_features", len(fm.Scalers)))
	}
}

type ctrDataset struct {
	trainSet     dataset.CTRSplit
	numFeatures  int
	numDimension int
	embeddingDim []int
	batchSize    int
	scalers      map[int32]*AutoScaler
}

func (d *ctrDataset) Name() string { return "CTRDataset" }

func (d *ctrDataset) batch(offset int) train.Batch {
	batchSize := min(d.batchSize, d.trainSet.Count()-offset)
	indicesData := make([]int32, batchSize*d.numDimension)
	valuesData := make([]float32, batchSize*d.numDimension)
	additionalData := make([][]float32, len(d.embeddingDim))
	for i := range additionalData {
		additionalData[i] = make([]float32, batchSize*d.embeddingDim[i])
	}
	labelsData := make([]float32, batchSize)

	for i := 0; i < batchSize; i++ {
		indices, values, embeddings, target := d.trainSet.Get(offset + i)
		// Apply scalers to numerical features
		scaledValues := make([]float32, len(values))
		copy(scaledValues, values)
		for j, idx := range indices {
			if scaler, ok := d.scalers[idx]; ok {
				scaledValues[j] = scaler.Transform(values[j])
			}
		}
		for j := 0; j < len(indices); j++ {
			indicesData[i*d.numDimension+j] = indices[j]
			valuesData[i*d.numDimension+j] = scaledValues[j]
		}
		for j := range d.embeddingDim {
			if len(embeddings) > j && len(embeddings[j]) == d.embeddingDim[j] {
				copy(additionalData[j][i*d.embeddingDim[j]:], bfloats.ToFloat32(embeddings[j]))
			}
		}
		// Convert target from {-1, 1} to {0, 1} for GoMLX BinaryCrossentropy
		labelsData[i] = (target + 1) / 2
	}

	inputs := []*tensors.Tensor{
		tensors.FromFlatDataAndDimensions(indicesData, batchSize, d.numDimension),
		tensors.FromFlatDataAndDimensions(valuesData, batchSize, d.numDimension),
	}
	for i := range additionalData {
		inputs = append(inputs, tensors.FromFlatDataAndDimensions(additionalData[i], batchSize, d.embeddingDim[i]))
	}
	labels := []*tensors.Tensor{tensors.FromFlatDataAndDimensions(labelsData, batchSize)}
	return train.Batch{Inputs: inputs, Labels: labels}
}

func (d *ctrDataset) Iter() iter.Seq2[train.Batch, error] {
	return func(yield func(train.Batch, error) bool) {
		for offset := 0; offset < d.trainSet.Count(); offset += d.batchSize {
			if !yield(d.batch(offset), nil) {
				return
			}
		}
	}
}

func (fm *AFM) Fit(ctx context.Context, trainSet, testSet dataset.CTRSplit, config *FitConfig) Score {
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

	modelFn := func(scope *mlx_model.Scope, inputs []*graph.Node) *graph.Node {
		return fm.forwardGraph(scope, inputs[0], inputs[1], inputs[2:])
	}
	lossFn := func(labels, predictions []*graph.Node) *graph.Node {
		return graph.ReduceAllMean(loss.BinaryCrossentropyLogits(labels, predictions))
	}

	theOptimizer := optimizer.Adam().LearningRate(float64(fm.lr)).Done()
	trainer := train.NewTrainer(fm.backend, fm.store, modelFn, lossFn, theOptimizer, nil, nil)
	loop := train.NewLoop(trainer)

	ds := &ctrDataset{
		trainSet:     trainSet,
		numFeatures:  fm.numFeatures,
		numDimension: fm.numDimension,
		embeddingDim: fm.embeddingDim,
		batchSize:    fm.batchSize,
		scalers:      fm.Scalers,
	}

	_, span := monitor.Start(ctx, "FM.Fit", fm.nEpochs)
	defer span.End()

	for epoch := 1; epoch <= fm.nEpochs; epoch++ {
		fitStart := time.Now()
		_, err := loop.RunSteps(ds, (trainSet.Count()+fm.batchSize-1)/fm.batchSize)
		if err != nil {
			panic(err)
		}
		fitTime := time.Since(fitStart)

		if epoch%config.Verbose == 0 || epoch == fm.nEpochs {
			evalStart = time.Now()
			score = EvaluateClassification(fm, testSet, config.Jobs)
			scores = append(scores, lo.Tuple2[int, float32]{A: epoch, B: score.AUC})
			evalTime = time.Since(evalStart)
			fields := append([]zap.Field{
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

type savedVariable struct {
	Dimensions []int
	Data       any
	Scope      string
	Name       string
}

func (fm *AFM) Marshal(w io.Writer) error {
	// write params
	if err := encoding.WriteGob(w, fm.Params); err != nil {
		return errors.WithStack(err)
	}
	// write index
	if err := dataset.MarshalUnifiedIndex(w, fm.Index); err != nil {
		return errors.WithStack(err)
	}
	// write dataset stats
	if err := encoding.WriteGob(w, fm.numFeatures); err != nil {
		return errors.WithStack(err)
	}
	if err := encoding.WriteGob(w, fm.numDimension); err != nil {
		return errors.WithStack(err)
	}
	if err := encoding.WriteGob(w, fm.embeddingDim); err != nil {
		return errors.WithStack(err)
	}
	if len(fm.embeddingDim) > 0 {
		if err := dataset.MarshalIndex(w, fm.embeddingIndex); err != nil {
			return errors.WithStack(err)
		}
	}
	// write scalers
	if fm.autoScale {
		if err := encoding.WriteGob(w, len(fm.Scalers)); err != nil {
			return errors.WithStack(err)
		}
		for idx, scaler := range fm.Scalers {
			if err := encoding.WriteGob(w, idx); err != nil {
				return errors.WithStack(err)
			}
			if err := scaler.Marshal(w); err != nil {
				return errors.WithStack(err)
			}
		}
	}
	// write parameters (GoMLX variables)
	variables := make(map[string]savedVariable)
	for v := range fm.store.IterVariables() {
		val, err := v.Value()
		if err != nil {
			panic(err)
		}
		var flatData any
		val.MustConstFlatData(func(flat any) {
			flatData = flat
		})
		variables[v.Path()] = savedVariable{
			Dimensions: val.Shape().Dimensions,
			Data:       flatData,
			Scope:      v.Scope(),
			Name:       v.Name(),
		}
	}
	if err := encoding.WriteGob(w, variables); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (fm *AFM) Unmarshal(r io.Reader) error {
	// read params
	err := encoding.ReadGob(r, &fm.Params)
	if err != nil {
		return errors.WithStack(err)
	}
	fm.SetParams(fm.Params)
	// read index
	fm.Index, err = dataset.UnmarshalUnifiedIndex(r)
	if err != nil {
		return errors.WithStack(err)
	}
	// read dataset stats
	if err = encoding.ReadGob(r, &fm.numFeatures); err != nil {
		return errors.WithStack(err)
	}
	if err = encoding.ReadGob(r, &fm.numDimension); err != nil {
		return errors.WithStack(err)
	}
	if err = encoding.ReadGob(r, &fm.embeddingDim); err != nil {
		return errors.WithStack(err)
	}
	if len(fm.embeddingDim) > 0 {
		fm.embeddingIndex, err = dataset.UnmarshalIndex(r)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	// read scalers
	if fm.autoScale {
		var numScalers int
		if err = encoding.ReadGob(r, &numScalers); err != nil {
			return errors.WithStack(err)
		}
		fm.Scalers = make(map[int32]*AutoScaler, numScalers)
		for i := 0; i < numScalers; i++ {
			var idx int32
			if err = encoding.ReadGob(r, &idx); err != nil {
				return errors.WithStack(err)
			}
			scaler := NewAutoScaler()
			if err = scaler.Unmarshal(r); err != nil {
				return errors.WithStack(err)
			}
			fm.Scalers[idx] = scaler
		}
	}
	// read parameters
	var variables map[string]savedVariable
	if err = encoding.ReadGob(r, &variables); err != nil {
		return errors.WithStack(err)
	}
	if fm.store == nil {
		fm.store = mlx_model.NewStore()
	}
	if fm.backend == nil {
		var err error
		fm.backend, err = compute.New()
		if err != nil {
			return errors.WithStack(err)
		}
	}
	for _, data := range variables {
		// Use a type switch to handle different data types in tensors
		var t *tensors.Tensor
		switch d := data.Data.(type) {
		case []float32:
			t = tensors.FromFlatDataAndDimensions(d, data.Dimensions...)
		case []float64:
			t = tensors.FromFlatDataAndDimensions(d, data.Dimensions...)
		case []int32:
			t = tensors.FromFlatDataAndDimensions(d, data.Dimensions...)
		case []int64:
			t = tensors.FromFlatDataAndDimensions(d, data.Dimensions...)
		case []uint64:
			t = tensors.FromFlatDataAndDimensions(d, data.Dimensions...)
		default:
			log.Logger().Warn("unknown variable type", zap.String("scope", data.Scope), zap.String("name", data.Name), zap.Any("type", fmt.Sprintf("%T", d)))
			continue
		}
		if _, err = fm.store.Scope(data.Scope).CreateVariable(data.Name, t); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}
