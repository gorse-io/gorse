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

package click

import (
	"encoding/binary"
	"fmt"
	"github.com/juju/errors"
	"github.com/zhenghaoz/gorse/base/copier"
	"io"
	"time"

	"github.com/chewxy/math32"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/floats"
	"github.com/zhenghaoz/gorse/model"
	"go.uber.org/zap"
)

type Score struct {
	Task      FMTask
	RMSE      float32
	Precision float32
	Recall    float32
	Accuracy  float32
	AUC       float32
}

func (score Score) ZapFields() []zap.Field {
	switch score.Task {
	case FMRegression:
		return []zap.Field{zap.Float32("RMSE", score.RMSE)}
	case FMClassification:
		return []zap.Field{
			zap.Float32("Accuracy", score.Accuracy),
			zap.Float32("Precision", score.Precision),
			zap.Float32("Recall", score.Recall),
			zap.Float32("AUC", score.AUC),
		}
	default:
		return nil
	}
}

func (score Score) GetValue() float32 {
	switch score.Task {
	case FMRegression:
		return score.RMSE
	case FMClassification:
		return score.Precision
	default:
		return math32.NaN()
	}
}

func (score Score) BetterThan(s Score) bool {
	if s.Task == 0 && score.Task != 0 {
		return true
	} else if s.Task != 0 && score.Task == 0 {
		return false
	}
	if score.Task != s.Task {
		panic("task type doesn't match")
	}
	switch score.Task {
	case FMRegression:
		return score.RMSE < s.RMSE
	case FMClassification:
		return score.AUC > s.AUC
	default:
		return true
	}
}

type FitConfig struct {
	Jobs    int
	Verbose int
	Tracker model.Tracker
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

func (config *FitConfig) SetJobs(nJobs int) *FitConfig {
	config.Jobs = nJobs
	return config
}

func (config *FitConfig) SetTracker(tracker model.Tracker) *FitConfig {
	config.Tracker = tracker
	return config
}

func (config *FitConfig) LoadDefaultIfNil() *FitConfig {
	if config == nil {
		return NewFitConfig()
	}
	return config
}

type FactorizationMachine interface {
	model.Model
	Predict(userId, itemId string, userLabels, itemLabels []string) float32
	InternalPredict(x []int32, values []float32) float32
	Fit(trainSet *Dataset, testSet *Dataset, config *FitConfig) Score
	Marshal(w io.Writer) error
}

type BaseFactorizationMachine struct {
	model.BaseModel
	Index UnifiedIndex
}

func (b *BaseFactorizationMachine) Init(trainSet *Dataset) {
	b.Index = trainSet.Index
}

type FMTask uint8

const (
	FMClassification FMTask = 'c'
	FMRegression     FMTask = 'r'
)

type FM struct {
	BaseFactorizationMachine
	// Model parameters
	V         [][]float32
	W         []float32
	B         float32
	MinTarget float32
	MaxTarget float32
	Task      FMTask
	// Hyper parameters
	nFactors   int
	nEpochs    int
	lr         float32
	reg        float32
	initMean   float32
	initStdDev float32
}

func (fm *FM) GetParamsGrid() model.ParamsGrid {
	return model.ParamsGrid{
		model.NFactors:   []interface{}{8, 16, 32, 64, 128},
		model.Lr:         []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
		model.Reg:        []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
		model.InitMean:   []interface{}{0},
		model.InitStdDev: []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
	}
}

func NewFM(task FMTask, params model.Params) *FM {
	fm := new(FM)
	fm.Task = task
	fm.SetParams(params)
	return fm
}

func (fm *FM) SetParams(params model.Params) {
	fm.BaseFactorizationMachine.SetParams(params)
	// Setup hyper-parameters
	fm.nFactors = fm.Params.GetInt(model.NFactors, 128)
	fm.nEpochs = fm.Params.GetInt(model.NEpochs, 200)
	fm.lr = fm.Params.GetFloat32(model.Lr, 0.01)
	fm.reg = fm.Params.GetFloat32(model.Reg, 0.0)
	fm.initMean = fm.Params.GetFloat32(model.InitMean, 0)
	fm.initStdDev = fm.Params.GetFloat32(model.InitStdDev, 0.01)
}

func (fm *FM) Predict(userId, itemId string, userLabels, itemLabels []string) float32 {
	var features []int32
	var values []float32
	// encode user
	if userIndex := fm.Index.EncodeUser(userId); userIndex != base.NotId {
		features = append(features, userIndex)
		values = append(values, 1)
	}
	// encode item
	if itemIndex := fm.Index.EncodeItem(itemId); itemIndex != base.NotId {
		features = append(features, itemIndex)
		values = append(values, 1)
	}
	// normalization
	norm := math32.Sqrt(float32(len(userLabels) + len(itemLabels)))
	// encode user labels
	for _, userLabel := range userLabels {
		if userLabelIndex := fm.Index.EncodeUserLabel(userLabel); userLabelIndex != base.NotId {
			features = append(features, userLabelIndex)
			values = append(values, 1/norm)
		}
	}
	// encode item labels
	for _, itemLabel := range itemLabels {
		if itemLabelIndex := fm.Index.EncodeItemLabel(itemLabel); itemLabelIndex != base.NotId {
			features = append(features, itemLabelIndex)
			values = append(values, 1/norm)
		}
	}
	return fm.InternalPredict(features, values)
}

func (fm *FM) internalPredictImpl(features []int32, values []float32) float32 {
	// w_0
	pred := fm.B
	// \sum^n_{i=1} w_i x_i
	for it, i := range features {
		pred += fm.W[i] * values[it]
	}
	// \sum^n_{i=1}\sum^n_{j=i+1} <v_i,v_j> x_i x_j
	sum := float32(0)
	for f := 0; f < fm.nFactors; f++ {
		a, b := float32(0), float32(0)
		for it, i := range features {
			// 1) \sum^n_{i=1} v^2_{i,f} x_i
			a += fm.V[i][f] * values[it]
			// 2) \sum^n_{i=1} v^2_{i,f} x^2_i
			b += fm.V[i][f] * fm.V[i][f] * values[it] * values[it]
		}
		// 3) (\sum^n_{i=1} v^2_{i,f} x^2_i)^2 - \sum^n_{i=1} v^2_{i,f} x^2_i
		sum += a*a - b
	}
	pred += sum / 2
	return pred
}

func (fm *FM) InternalPredict(features []int32, values []float32) float32 {
	pred := fm.internalPredictImpl(features, values)
	if fm.Task == FMRegression {
		if pred < fm.MinTarget {
			pred = fm.MinTarget
		} else if pred > fm.MaxTarget {
			pred = fm.MaxTarget
		}
	}
	return pred
}

func (fm *FM) Fit(trainSet, testSet *Dataset, config *FitConfig) Score {
	config = config.LoadDefaultIfNil()
	if config.Tracker != nil {
		config.Tracker.Start(fm.nEpochs)
	}
	base.Logger().Info("fit FM",
		zap.Int("train_size", trainSet.Count()),
		zap.Int("train_positive_count", trainSet.PositiveCount),
		zap.Int("train_negative_count", trainSet.NegativeCount),
		zap.Int("test_size", testSet.Count()),
		zap.Int("test_positive_count", testSet.PositiveCount),
		zap.Int("test_negative_count", testSet.NegativeCount),
		zap.String("task", string(fm.Task)),
		zap.Any("params", fm.GetParams()),
		zap.Any("config", config))
	fm.Init(trainSet)
	temp := base.NewMatrix32(config.Jobs, fm.nFactors)
	vGrad := base.NewMatrix32(config.Jobs, fm.nFactors)

	snapshots := SnapshotManger{}
	evalStart := time.Now()
	var score Score
	switch fm.Task {
	case FMRegression:
		score = EvaluateRegression(fm, testSet)
	case FMClassification:
		score = EvaluateClassification(fm, testSet)
	default:
		base.Logger().Fatal("unknown task", zap.String("task", string(fm.Task)))
	}
	evalTime := time.Since(evalStart)
	fields := append([]zap.Field{zap.String("eval_time", evalTime.String())}, score.ZapFields()...)
	base.Logger().Debug(fmt.Sprintf("fit fm %v/%v", 0, fm.nEpochs), fields...)
	snapshots.AddSnapshot(score, fm.V, fm.W, fm.B)

	for epoch := 1; epoch <= fm.nEpochs; epoch++ {
		for i := 0; i < trainSet.Target.Len(); i++ {
			fm.MinTarget = math32.Min(fm.MinTarget, trainSet.Target.Get(i))
			fm.MaxTarget = math32.Max(fm.MaxTarget, trainSet.Target.Get(i))
		}
		fitStart := time.Now()
		cost := float32(0)
		_ = base.BatchParallel(trainSet.Count(), config.Jobs, 128, func(workerId, beginJobId, endJobId int) error {
			for i := beginJobId; i < endJobId; i++ {
				features, values, target := trainSet.Get(i)
				prediction := fm.internalPredictImpl(features, values)
				var grad float32
				switch fm.Task {
				case FMRegression:
					grad = prediction - target
					cost += grad * grad / 2
				case FMClassification:
					grad = -target * (1 - 1/(1+math32.Exp(-target*prediction)))
					cost += (1 + target) * math32.Log(1+math32.Exp(-prediction)) / 2
					cost += (1 - target) * math32.Log(1+math32.Exp(prediction)) / 2
				default:
					base.Logger().Fatal("unknown task", zap.String("task", string(fm.Task)))
				}
				// \sum^n_{j=1}v_j,fx_j
				floats.Zero(temp[workerId])
				for it, j := range features {
					floats.MulConstAddTo(fm.V[j], values[it], temp[workerId])
				}
				// Update w_0
				fm.B -= fm.lr * grad
				for it, i := range features {
					// Update w_i
					fm.W[i] -= fm.lr * grad * values[it]
					// Update v_{i,f}
					floats.MulConstTo(temp[workerId], values[it], vGrad[workerId])
					floats.MulConstAddTo(fm.V[i], -values[it]*values[it], vGrad[workerId])
					floats.MulConst(vGrad[workerId], grad)
					floats.MulConstAddTo(fm.V[i], fm.reg, vGrad[workerId])
					floats.MulConstAddTo(vGrad[workerId], -fm.lr, fm.V[i])
				}
			}
			return nil
		})
		fitTime := time.Since(fitStart)
		// Cross validation
		if epoch%config.Verbose == 0 || epoch == fm.nEpochs {
			evalStart = time.Now()
			switch fm.Task {
			case FMRegression:
				score = EvaluateRegression(fm, testSet)
			case FMClassification:
				score = EvaluateClassification(fm, testSet)
			default:
				base.Logger().Fatal("unknown task", zap.String("task", string(fm.Task)))
			}
			evalTime = time.Since(evalStart)
			fields = append([]zap.Field{
				zap.String("fit_time", fitTime.String()),
				zap.String("eval_time", evalTime.String()),
				zap.Float32("loss", cost),
			}, score.ZapFields()...)
			base.Logger().Debug(fmt.Sprintf("fit fm %v/%v", epoch, fm.nEpochs), fields...)
			// check NaN
			if math32.IsNaN(cost) || math32.IsNaN(score.GetValue()) {
				base.Logger().Warn("model diverged", zap.Float32("lr", fm.lr))
				break
			}
			snapshots.AddSnapshot(score, fm.V, fm.W, fm.B)
		}
		if config.Tracker != nil {
			config.Tracker.Update(epoch)
		}
	}
	// restore best snapshot
	fm.V = snapshots.BestWeights[0].([][]float32)
	fm.W = snapshots.BestWeights[1].([]float32)
	fm.B = snapshots.BestWeights[2].(float32)
	base.Logger().Info("fit fm complete", snapshots.BestScore.ZapFields()...)
	if config.Tracker != nil {
		config.Tracker.Finish()
	}
	return snapshots.BestScore
}

func (fm *FM) Clear() {
	fm.B = 0.0
	fm.V = nil
	fm.W = nil
	fm.Index = nil
}

func (fm *FM) Invalid() bool {
	return fm == nil ||
		fm.V == nil ||
		fm.W == nil ||
		fm.Index == nil
}

func (fm *FM) Init(trainSet *Dataset) {
	newV := fm.GetRandomGenerator().NormalMatrix(int(trainSet.Index.Len()), fm.nFactors, fm.initMean, fm.initStdDev)
	newW := make([]float32, trainSet.Index.Len())
	// Relocate parameters
	if fm.Index != nil {
		// users
		for _, userId := range trainSet.Index.GetUsers() {
			oldIndex := fm.Index.EncodeUser(userId)
			newIndex := trainSet.Index.EncodeUser(userId)
			if oldIndex != base.NotId {
				newW[newIndex] = fm.W[oldIndex]
				newV[newIndex] = fm.V[oldIndex]
			}
		}
		// items
		for _, itemId := range trainSet.Index.GetItems() {
			oldIndex := fm.Index.EncodeItem(itemId)
			newIndex := trainSet.Index.EncodeItem(itemId)
			if oldIndex != base.NotId {
				newW[newIndex] = fm.W[oldIndex]
				newV[newIndex] = fm.V[oldIndex]
			}
		}
		// user labels
		for _, label := range trainSet.Index.GetUserLabels() {
			oldIndex := fm.Index.EncodeUserLabel(label)
			newIndex := trainSet.Index.EncodeUserLabel(label)
			if oldIndex != base.NotId {
				newW[newIndex] = fm.W[oldIndex]
				newV[newIndex] = fm.V[oldIndex]
			}
		}
		// item labels
		for _, label := range trainSet.Index.GetItemLabels() {
			oldIndex := fm.Index.EncodeItemLabel(label)
			newIndex := trainSet.Index.EncodeItemLabel(label)
			if oldIndex != base.NotId {
				newW[newIndex] = fm.W[oldIndex]
				newV[newIndex] = fm.V[oldIndex]
			}
		}
		// context labels
		for _, label := range trainSet.Index.GetContextLabels() {
			oldIndex := fm.Index.EncodeContextLabel(label)
			newIndex := trainSet.Index.EncodeContextLabel(label)
			if oldIndex != base.NotId {
				newW[newIndex] = fm.W[oldIndex]
				newV[newIndex] = fm.V[oldIndex]
			}
		}
	}
	fm.MinTarget = math32.Inf(1)
	fm.MaxTarget = math32.Inf(-1)
	fm.V = newV
	fm.W = newW
	fm.BaseFactorizationMachine.Init(trainSet)
}

func MarshalModel(w io.Writer, m FactorizationMachine) error {
	return m.Marshal(w)
}

func UnmarshalModel(r io.Reader) (FactorizationMachine, error) {
	var fm FM
	if err := fm.Unmarshal(r); err != nil {
		return nil, err
	}
	return &fm, nil
}

// Clone a model with deep copy.
func Clone(m FactorizationMachine) FactorizationMachine {
	var copied FactorizationMachine
	if err := copier.Copy(&copied, m); err != nil {
		panic(err)
	} else {
		copied.SetParams(copied.GetParams())
		return copied
	}
}

// Marshal model into byte stream.
func (fm *FM) Marshal(w io.Writer) error {
	// write params
	err := base.WriteGob(w, fm.Params)
	if err != nil {
		return errors.Trace(err)
	}
	// write index
	err = MarshalIndex(w, fm.Index)
	if err != nil {
		return errors.Trace(err)
	}
	// write scalars
	err = binary.Write(w, binary.LittleEndian, fm.MaxTarget)
	if err != nil {
		return errors.Trace(err)
	}
	err = binary.Write(w, binary.LittleEndian, fm.MinTarget)
	if err != nil {
		return errors.Trace(err)
	}
	err = binary.Write(w, binary.LittleEndian, fm.Task)
	if err != nil {
		return errors.Trace(err)
	}
	err = binary.Write(w, binary.LittleEndian, fm.B)
	if err != nil {
		return errors.Trace(err)
	}
	// write vector
	err = binary.Write(w, binary.LittleEndian, fm.W)
	if err != nil {
		return errors.Trace(err)
	}
	// write matrix
	err = base.WriteMatrix(w, fm.V)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// Unmarshal model from byte stream.
func (fm *FM) Unmarshal(r io.Reader) error {
	// read params
	err := base.ReadGob(r, &fm.Params)
	if err != nil {
		return errors.Trace(err)
	}
	fm.SetParams(fm.Params)
	// read index
	fm.Index, err = UnmarshalIndex(r)
	if err != nil {
		return errors.Trace(err)
	}
	// read scalars
	err = binary.Read(r, binary.LittleEndian, &fm.MaxTarget)
	if err != nil {
		return errors.Trace(err)
	}
	err = binary.Read(r, binary.LittleEndian, &fm.MinTarget)
	if err != nil {
		return errors.Trace(err)
	}
	err = binary.Read(r, binary.LittleEndian, &fm.Task)
	if err != nil {
		return errors.Trace(err)
	}
	err = binary.Read(r, binary.LittleEndian, &fm.B)
	if err != nil {
		return errors.Trace(err)
	}
	// read vector
	fm.W = make([]float32, fm.Index.Len())
	err = binary.Read(r, binary.LittleEndian, fm.W)
	if err != nil {
		return errors.Trace(err)
	}
	// read matrix
	fm.V = base.NewMatrix32(int(fm.Index.Len()), fm.nFactors)
	err = base.ReadMatrix(r, fm.V)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
