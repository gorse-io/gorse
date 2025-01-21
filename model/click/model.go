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
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/chewxy/math32"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/thoas/go-funk"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/copier"
	"github.com/zhenghaoz/gorse/base/encoding"
	"github.com/zhenghaoz/gorse/base/floats"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/parallel"
	"github.com/zhenghaoz/gorse/base/progress"
	"github.com/zhenghaoz/gorse/base/task"
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
	*task.JobsAllocator
	Verbose int
}

func NewFitConfig() *FitConfig {
	return &FitConfig{
		Verbose: 10,
	}
}

func (config *FitConfig) SetVerbose(verbose int) *FitConfig {
	config.Verbose = verbose
	return config
}

func (config *FitConfig) SetJobsAllocator(allocator *task.JobsAllocator) *FitConfig {
	config.JobsAllocator = allocator
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
	Predict(userId, itemId string, userFeatures, itemFeatures []Feature) float32
	InternalPredict(x []int32, values []float32) float32
	Fit(ctx context.Context, trainSet *Dataset, testSet *Dataset, config *FitConfig) Score
	Marshal(w io.Writer) error
}

type BatchInference interface {
	BatchPredict(inputs []lo.Tuple4[string, string, []Feature, []Feature]) []float32
	BatchInternalPredict(x []lo.Tuple2[[]int32, []float32]) []float32
}

type FactorizationMachineCloner interface {
	Clone() FactorizationMachine
}

type FactorizationMachineSpawner interface {
	Spawn() FactorizationMachine
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
	optimizer  string
}

func (fm *FM) GetParamsGrid(withSize bool) model.ParamsGrid {
	return model.ParamsGrid{
		model.NFactors:   lo.If(withSize, []interface{}{8, 16, 32, 64}).Else([]interface{}{16}),
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
	fm.nFactors = fm.Params.GetInt(model.NFactors, 16)
	fm.nEpochs = fm.Params.GetInt(model.NEpochs, 200)
	fm.lr = fm.Params.GetFloat32(model.Lr, 0.01)
	fm.reg = fm.Params.GetFloat32(model.Reg, 0.0)
	fm.initMean = fm.Params.GetFloat32(model.InitMean, 0)
	fm.initStdDev = fm.Params.GetFloat32(model.InitStdDev, 0.01)
	fm.optimizer = fm.Params.GetString(model.Optimizer, model.Adam)
}

func (fm *FM) Predict(userId, itemId string, userFeatures, itemFeatures []Feature) float32 {
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
	// encode user labels
	for _, userFeature := range userFeatures {
		if userFeatureIndex := fm.Index.EncodeUserLabel(userFeature.Name); userFeatureIndex != base.NotId {
			features = append(features, userFeatureIndex)
			values = append(values, userFeature.Value)
		}
	}
	// encode item labels
	for _, itemFeature := range itemFeatures {
		if itemFeatureIndex := fm.Index.EncodeItemLabel(itemFeature.Name); itemFeatureIndex != base.NotId {
			features = append(features, itemFeatureIndex)
			values = append(values, itemFeature.Value)
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
	temp := make([]float32, fm.nFactors)
	a := make([]float32, fm.nFactors)
	b := make([]float32, fm.nFactors)
	for it, i := range features {
		// 1) \sum^n_{i=1} v^2_{i,f} x_i
		floats.MulConstAddTo(fm.V[i], values[it], a)
		// 2) \sum^n_{i=1} v^2_{i,f} x^2_i
		floats.MulTo(fm.V[i], fm.V[i], temp)
		floats.MulConstAddTo(temp, values[it]*values[it], b)
	}
	// 3) (\sum^n_{i=1} v^2_{i,f} x^2_i)^2 - \sum^n_{i=1} v^2_{i,f} x^2_i
	floats.MulTo(a, a, temp)
	floats.MulConstAddTo(b, -1, temp)
	pred += funk.SumFloat32(temp) / 2
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

// Fit trains the model. Its task complexity is O(fm.nEpochs).
func (fm *FM) Fit(ctx context.Context, trainSet, testSet *Dataset, config *FitConfig) Score {
	config = config.LoadDefaultIfNil()
	log.Logger().Info("fit FM",
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
	maxJobs := config.MaxJobs()
	temp := base.NewMatrix32(maxJobs, fm.nFactors)
	vGrad := base.NewMatrix32(maxJobs, fm.nFactors)
	vGrad2 := base.NewMatrix32(maxJobs, fm.nFactors)
	mV := base.NewTensor32(maxJobs, int(trainSet.Index.Len()), fm.nFactors)
	mW := base.NewMatrix32(maxJobs, int(trainSet.Index.Len()))
	mB := make([]float32, maxJobs)
	vV := base.NewTensor32(maxJobs, int(trainSet.Index.Len()), fm.nFactors)
	vW := base.NewMatrix32(maxJobs, int(trainSet.Index.Len()))
	vB := make([]float32, maxJobs)
	mVHat := base.NewMatrix32(maxJobs, fm.nFactors)
	vVHat := base.NewMatrix32(maxJobs, fm.nFactors)

	evalStart := time.Now()
	var score Score
	switch fm.Task {
	case FMRegression:
		score = EvaluateRegression(fm, testSet)
	case FMClassification:
		score = EvaluateClassification(fm, testSet)
	default:
		log.Logger().Fatal("unknown task", zap.String("task", string(fm.Task)))
	}
	evalTime := time.Since(evalStart)
	fields := append([]zap.Field{zap.String("eval_time", evalTime.String())}, score.ZapFields()...)
	log.Logger().Debug(fmt.Sprintf("fit fm %v/%v", 0, fm.nEpochs), fields...)

	_, span := progress.Start(ctx, "FM.Fit", fm.nEpochs)
	for epoch := 1; epoch <= fm.nEpochs; epoch++ {
		for i := 0; i < trainSet.Target.Len(); i++ {
			fm.MinTarget = math32.Min(fm.MinTarget, trainSet.Target.Get(i))
			fm.MaxTarget = math32.Max(fm.MaxTarget, trainSet.Target.Get(i))
		}
		fitStart := time.Now()
		cost := float32(0)
		_ = parallel.BatchParallel(trainSet.Count(), config.AvailableJobs(), 128, func(workerId, beginJobId, endJobId int) error {
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
					cost += (1 + target) * math32.Log1p(exp(-prediction)) / 2
					cost += (1 - target) * math32.Log1p(exp(prediction)) / 2
				default:
					log.Logger().Fatal("unknown task", zap.String("task", string(fm.Task)))
				}
				// \sum^n_{j=1}v_j,fx_j
				floats.Zero(temp[workerId])
				for it, j := range features {
					floats.MulConstAddTo(fm.V[j], values[it], temp[workerId])
				}

				correct1 := 1 / (1 - math32.Pow(beta1, float32(epoch)))
				correct2 := 1 / (1 - math32.Pow(beta2, float32(epoch)))

				// Update w_0
				switch fm.optimizer {
				case model.SGD:
					fm.B -= fm.lr * grad
				case model.Adam:
					// m_t = \beta_1 m_{t-1} + (1 - \beta_1) g_t
					mB[workerId] = beta1*mB[workerId] + (1-beta1)*grad
					// v_t = \beta_2 v_{t-1} + (1 - \beta_2) g^2_t
					vB[workerId] = beta2*vB[workerId] + (1-beta2)*grad*grad
					// \hat{m}_t = m_t / (1 - \beta^t_1)
					mBHat := mB[workerId] * correct1
					// \hat{v}_t = v_t / (1 - \beta^t_2)
					vBHat := vB[workerId] * correct2
					// w_0 = w_0 - \eta \hat{m}_t / (\sqrt{\hat{v}_t} + \epsilon)
					fm.B -= fm.lr * mBHat / (math32.Sqrt(vBHat) + eps)
				default:
					log.Logger().Fatal("unknown optimizer", zap.String("optimizer", fm.optimizer))
				}

				for it, i := range features {
					// Update w_i
					switch fm.optimizer {
					case model.SGD:
						fm.W[i] -= fm.lr * grad
					case model.Adam:
						// m_t = \beta_1 m_{t-1} + (1 - \beta_1) g_t
						mW[workerId][i] = beta1*mW[workerId][i] + (1-beta1)*grad
						// v_t = \beta_2 v_{t-1} + (1 - \beta_2) g^2_t
						vW[workerId][i] = beta2*vW[workerId][i] + (1-beta2)*grad*grad
						// \hat{m}_t = m_t / (1 - \beta^t_1)
						mWHat := mW[workerId][i] * correct1
						// \hat{v}_t = v_t / (1 - \beta^t_2)
						vWHat := vW[workerId][i] * correct2
						// w_i = w_i - \eta \hat{m}_t / (\sqrt{\hat{v}_t} + \epsilon)
						fm.W[i] -= fm.lr * mWHat / (math32.Sqrt(vWHat) + eps)
					default:
						log.Logger().Fatal("unknown optimizer", zap.String("optimizer", fm.optimizer))
					}

					// Update v_{i,f}
					floats.MulConstTo(temp[workerId], values[it], vGrad[workerId])
					floats.MulConstAddTo(fm.V[i], -values[it]*values[it], vGrad[workerId])
					floats.MulConst(vGrad[workerId], grad)
					floats.MulConstAddTo(fm.V[i], fm.reg, vGrad[workerId])
					switch fm.optimizer {
					case model.SGD:
						floats.MulConstAddTo(vGrad[workerId], -fm.lr, fm.V[i])
					case model.Adam:
						// m_t = \beta_1 m_{t-1} + (1 - \beta_1) g_t
						floats.MulConst(mV[workerId][i], beta1)
						floats.MulConstAddTo(vGrad[workerId], 1-beta1, mV[workerId][i])
						// v_t = \beta_2 v_{t-1} + (1 - \beta_2) g^2_t
						floats.MulConst(vV[workerId][i], beta2)
						floats.MulTo(vGrad[workerId], vGrad[workerId], vGrad2[workerId])
						floats.MulConstAddTo(vGrad2[workerId], 1-beta2, vV[workerId][i])
						// \hat{m}_t = m_t / (1 - \beta^t_1)
						floats.MulConstTo(mV[workerId][i], correct1, mVHat[workerId])
						// \hat{v}_t = v_t / (1 - \beta^t_2)
						floats.MulConstTo(vV[workerId][i], correct2, vVHat[workerId])
						// v_{i,f} = v_{i,f} - \eta \hat{m}_t / (\sqrt{\hat{v}_t} + \epsilon)
						floats.Sqrt(vVHat[workerId])
						floats.AddConst(vVHat[workerId], eps)
						floats.Div(mVHat[workerId], vVHat[workerId])
						floats.MulConstAddTo(mVHat[workerId], -fm.lr, fm.V[i])
					default:
						log.Logger().Fatal("unknown optimizer", zap.String("optimizer", fm.optimizer))
					}
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
				log.Logger().Fatal("unknown task", zap.String("task", string(fm.Task)))
			}
			evalTime = time.Since(evalStart)
			fields = append([]zap.Field{
				zap.String("fit_time", fitTime.String()),
				zap.String("eval_time", evalTime.String()),
				zap.Float32("loss", cost),
			}, score.ZapFields()...)
			log.Logger().Debug(fmt.Sprintf("fit fm %v/%v", epoch, fm.nEpochs), fields...)
			// check NaN
			if math32.IsNaN(cost) || math32.IsNaN(score.GetValue()) {
				log.Logger().Warn("model diverged", zap.Float32("lr", fm.lr))
				span.Fail(errors.New("model diverged"))
				break
			}
		}
		span.Add(1)
	}
	span.End()
	// restore best snapshot
	log.Logger().Info("fit fm complete", score.ZapFields()...)
	return score
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
	// write header
	var err error
	switch m.(type) {
	case *FM:
		err = encoding.WriteString(w, headerFM)
	case *DeepFM:
		err = encoding.WriteString(w, headerDeepFM)
	default:
		return fmt.Errorf("unknown model: %v", reflect.TypeOf(m))
	}
	if err != nil {
		return err
	}
	return m.Marshal(w)
}

const (
	headerFM     = "FM"
	headerDeepFM = "DeepFM"
)

func UnmarshalModel(r io.Reader) (FactorizationMachine, error) {
	// read header
	header, err := encoding.ReadString(r)
	if err != nil {
		return nil, err
	}
	switch header {
	case headerFM:
		var fm FM
		if err := fm.Unmarshal(r); err != nil {
			return nil, errors.Trace(err)
		}
		return &fm, nil
	case headerDeepFM:
		fm := NewDeepFM(nil)
		if err := fm.Unmarshal(r); err != nil {
			return nil, errors.Trace(err)
		}
		return fm, nil
	}
	return nil, fmt.Errorf("unknown model: %v", header)
}

// Clone a model with deep copy.
func Clone(m FactorizationMachine) FactorizationMachine {
	if cloner, ok := m.(FactorizationMachineCloner); ok {
		return cloner.Clone()
	}
	var copied FactorizationMachine
	if err := copier.Copy(&copied, m); err != nil {
		panic(err)
	} else {
		copied.SetParams(copied.GetParams())
		return copied
	}
}

func Spawn(m FactorizationMachine) FactorizationMachine {
	if cloner, ok := m.(FactorizationMachineSpawner); ok {
		return cloner.Spawn()
	}
	return m
}

// Marshal model into byte stream.
func (fm *FM) Marshal(w io.Writer) error {
	// write params
	err := encoding.WriteGob(w, fm.Params)
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
	err = encoding.WriteMatrix(w, fm.V)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// Unmarshal model from byte stream.
func (fm *FM) Unmarshal(r io.Reader) error {
	// read params
	err := encoding.ReadGob(r, &fm.Params)
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
	err = encoding.ReadMatrix(r, fm.V)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func exp(x float32) float32 {
	e := math32.Exp(x)
	if math32.IsInf(e, 1) {
		return math32.MaxFloat32
	}
	return e
}
