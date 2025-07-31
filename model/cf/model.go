// Copyright 2021 gorse Project Authors
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

package cf

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/bits-and-blooms/bitset"
	"github.com/chewxy/math32"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/base"
	"github.com/gorse-io/gorse/base/copier"
	"github.com/gorse-io/gorse/base/encoding"
	"github.com/gorse-io/gorse/base/log"
	"github.com/gorse-io/gorse/base/progress"
	"github.com/gorse-io/gorse/common/floats"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model"
	"github.com/gorse-io/gorse/protocol"
	"github.com/juju/errors"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type Score struct {
	NDCG      float32
	Precision float32
	Recall    float32
}

type FitConfig struct {
	Jobs       int
	Verbose    int
	Candidates int
	TopK       int
}

func NewFitConfig() *FitConfig {
	return &FitConfig{
		Jobs:       1,
		Verbose:    10,
		Candidates: 100,
		TopK:       10,
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

type Model interface {
	model.Model
	// Fit a model with a train set and parameters.
	Fit(ctx context.Context, trainSet, validateSet dataset.CFSplit, config *FitConfig) Score
	// GetItemIndex returns item index.
	GetItemIndex() *dataset.FreqDict
	// Marshal model into byte stream.
	Marshal(w io.Writer) error
	// Unmarshal model from byte stream.
	Unmarshal(r io.Reader) error
	// GetUserFactor returns latent factor of a user.
	GetUserFactor(userIndex int32) []float32
	// GetItemFactor returns latent factor of an item.
	GetItemFactor(itemIndex int32) []float32
}

type MatrixFactorization interface {
	Model
	// Predict the rating given by a user (userId) to a item (itemId).
	Predict(userId, itemId string) float32
	// InternalPredict predicts rating given by a user index and a item index
	internalPredict(userIndex, itemIndex int32) float32
	// GetUserIndex returns user index.
	GetUserIndex() *dataset.FreqDict
	// GetItemIndex returns item index.
	GetItemIndex() *dataset.FreqDict
	// IsUserPredictable returns false if user has no feedback and its embedding vector never be trained.
	IsUserPredictable(userIndex int32) bool
	// IsItemPredictable returns false if item has no feedback and its embedding vector never be trained.
	IsItemPredictable(itemIndex int32) bool
	// Marshal model into byte stream.
	Marshal(w io.Writer) error
	// Unmarshal model from byte stream.
	Unmarshal(r io.Reader) error
}

type BaseMatrixFactorization struct {
	model.BaseModel
	UserIndex       *dataset.FreqDict
	ItemIndex       *dataset.FreqDict
	UserPredictable *bitset.BitSet
	ItemPredictable *bitset.BitSet
	// Model parameters
	UserFactor [][]float32 // p_u
	ItemFactor [][]float32 // q_i
}

func (baseModel *BaseMatrixFactorization) Init(trainSet dataset.CFSplit) {
	baseModel.UserIndex = trainSet.GetUserDict()
	baseModel.ItemIndex = trainSet.GetItemDict()
	// set user trained flags
	baseModel.UserPredictable = bitset.New(uint(baseModel.UserIndex.Count()))
	for userIndex := int32(0); userIndex < baseModel.UserIndex.Count(); userIndex++ {
		if len(trainSet.GetUserFeedback()[userIndex]) > 0 {
			baseModel.UserPredictable.Set(uint(userIndex))
		}
	}
	// set item trained flags
	baseModel.ItemPredictable = bitset.New(uint(baseModel.ItemIndex.Count()))
	for itemIndex := int32(0); itemIndex < baseModel.ItemIndex.Count(); itemIndex++ {
		if len(trainSet.GetItemFeedback()[itemIndex]) > 0 {
			baseModel.ItemPredictable.Set(uint(itemIndex))
		}
	}
}

func (baseModel *BaseMatrixFactorization) GetUserIndex() *dataset.FreqDict {
	return baseModel.UserIndex
}

func (baseModel *BaseMatrixFactorization) GetItemIndex() *dataset.FreqDict {
	return baseModel.ItemIndex
}

// IsUserPredictable returns false if user has no feedback and its embedding vector never be trained.
func (baseModel *BaseMatrixFactorization) IsUserPredictable(userIndex int32) bool {
	if userIndex >= baseModel.UserIndex.Count() || userIndex < 0 {
		return false
	}
	return baseModel.UserPredictable.Test(uint(userIndex))
}

// IsItemPredictable returns false if item has no feedback and its embedding vector never be trained.
func (baseModel *BaseMatrixFactorization) IsItemPredictable(itemIndex int32) bool {
	if itemIndex >= baseModel.ItemIndex.Count() || itemIndex < 0 {
		return false
	}
	return baseModel.ItemPredictable.Test(uint(itemIndex))
}

// GetUserFactor returns the latent factor of a user.
func (baseModel *BaseMatrixFactorization) GetUserFactor(userIndex int32) []float32 {
	return baseModel.UserFactor[userIndex]
}

// GetItemFactor returns the latent factor of an item.
func (baseModel *BaseMatrixFactorization) GetItemFactor(itemIndex int32) []float32 {
	return baseModel.ItemFactor[itemIndex]
}

func (baseModel *BaseMatrixFactorization) Predict(userId, itemId string) float32 {
	// Convert sparse Names to dense Names
	userIndex := baseModel.UserIndex.Id(userId)
	itemIndex := baseModel.ItemIndex.Id(itemId)
	if userIndex < 0 {
		log.Logger().Warn("unknown user", zap.String("user_id", userId))
	}
	if itemIndex < 0 {
		log.Logger().Warn("unknown item", zap.String("item_id", itemId))
	}
	return baseModel.internalPredict(userIndex, itemIndex)
}

func (baseModel *BaseMatrixFactorization) internalPredict(userIndex, itemIndex int32) float32 {
	ret := float32(0.0)
	if itemIndex >= 0 && userIndex >= 0 {
		ret = floats.Dot(baseModel.UserFactor[userIndex], baseModel.ItemFactor[itemIndex])
	} else {
		log.Logger().Warn("unknown user or item")
	}
	return ret
}

// Marshal model into byte stream.
func (baseModel *BaseMatrixFactorization) Marshal(w io.Writer) error {
	// write params
	err := encoding.WriteGob(w, baseModel.Params)
	if err != nil {
		return errors.Trace(err)
	}
	// write predictable user count
	if err := binary.Write(w, binary.LittleEndian, int64(baseModel.UserPredictable.Count())); err != nil {
		return errors.Trace(err)
	}
	// write user latent factors
	for userIndex := int32(0); userIndex < baseModel.UserIndex.Count(); userIndex++ {
		if baseModel.UserPredictable.Test(uint(userIndex)) {
			userId, _ := baseModel.UserIndex.String(userIndex)
			latentFactor := &protocol.LatentFactor{
				Id:   userId,
				Data: baseModel.UserFactor[userIndex],
			}
			if _, err := pbutil.WriteDelimited(w, latentFactor); err != nil {
				return errors.Trace(err)
			}
		}
	}
	// write predictable item count
	if err := binary.Write(w, binary.LittleEndian, int64(baseModel.ItemPredictable.Count())); err != nil {
		return errors.Trace(err)
	}
	// write item latent factors
	for itemIndex := int32(0); itemIndex < baseModel.ItemIndex.Count(); itemIndex++ {
		if baseModel.ItemPredictable.Test(uint(itemIndex)) {
			itemId, _ := baseModel.ItemIndex.String(itemIndex)
			latentFactor := &protocol.LatentFactor{
				Id:   itemId,
				Data: baseModel.ItemFactor[itemIndex],
			}
			if _, err := pbutil.WriteDelimited(w, latentFactor); err != nil {
				return errors.Trace(err)
			}
		}
	}
	return nil
}

// Unmarshal model from byte stream.
func (baseModel *BaseMatrixFactorization) Unmarshal(r io.Reader) error {
	// read params
	if err := encoding.ReadGob(r, &baseModel.Params); err != nil {
		return errors.Trace(err)
	}
	// read predictable user count
	var userPredictableCount int64
	if err := binary.Read(r, binary.LittleEndian, &userPredictableCount); err != nil {
		return errors.Trace(err)
	}
	// read user latent factors
	baseModel.UserIndex = dataset.NewFreqDict()
	baseModel.UserPredictable = bitset.New(uint(userPredictableCount))
	baseModel.UserFactor = make([][]float32, userPredictableCount)
	for i := 0; i < int(userPredictableCount); i++ {
		latentFactor := new(protocol.LatentFactor)
		if _, err := pbutil.ReadDelimited(r, latentFactor); err != nil {
			return errors.Trace(err)
		}
		userIndex := baseModel.UserIndex.Add(latentFactor.Id)
		baseModel.UserPredictable.Set(uint(userIndex))
		baseModel.UserFactor[userIndex] = latentFactor.Data
	}
	// read predictable item count
	var itemPredictableCount int64
	if err := binary.Read(r, binary.LittleEndian, &itemPredictableCount); err != nil {
		return errors.Trace(err)
	}
	// read item latent factors
	baseModel.ItemIndex = dataset.NewFreqDict()
	baseModel.ItemPredictable = bitset.New(uint(itemPredictableCount))
	baseModel.ItemFactor = make([][]float32, itemPredictableCount)
	for i := 0; i < int(itemPredictableCount); i++ {
		latentFactor := new(protocol.LatentFactor)
		if _, err := pbutil.ReadDelimited(r, latentFactor); err != nil {
			return errors.Trace(err)
		}
		itemIndex := baseModel.ItemIndex.Add(latentFactor.Id)
		baseModel.ItemPredictable.Set(uint(itemIndex))
		baseModel.ItemFactor[itemIndex] = latentFactor.Data
	}
	return nil
}

func (baseModel *BaseMatrixFactorization) Clear() {
	baseModel.UserIndex = nil
	baseModel.ItemIndex = nil
	baseModel.ItemFactor = nil
	baseModel.UserFactor = nil
}

func (baseModel *BaseMatrixFactorization) Invalid() bool {
	return baseModel == nil ||
		baseModel.UserIndex == nil ||
		baseModel.ItemIndex == nil ||
		baseModel.ItemFactor == nil ||
		baseModel.UserFactor == nil
}

// Clone a model with deep copy.
func Clone(m MatrixFactorization) MatrixFactorization {
	var copied MatrixFactorization
	if err := copier.Copy(&copied, m); err != nil {
		panic(err)
	} else {
		copied.SetParams(copied.GetParams())
		return copied
	}
}

func GetModelName(m Model) string {
	switch m.(type) {
	case *BPR:
		return "bpr"
	case *ALS:
		return "als"
	default:
		return reflect.TypeOf(m).String()
	}
}

func MarshalModel(w io.Writer, m Model) error {
	if err := encoding.WriteString(w, GetModelName(m)); err != nil {
		return errors.Trace(err)
	}
	if err := m.Marshal(w); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func UnmarshalModel(r io.Reader) (MatrixFactorization, error) {
	name, err := encoding.ReadString(r)
	if err != nil {
		return nil, errors.Trace(err)
	}
	switch name {
	case "bpr":
		var bpr BPR
		if err := bpr.Unmarshal(r); err != nil {
			return nil, errors.Trace(err)
		}
		return &bpr, nil
	case "als":
		var als ALS
		if err := als.Unmarshal(r); err != nil {
			return nil, errors.Trace(err)
		}
		return &als, nil
	}
	return nil, fmt.Errorf("unknown model %v", name)
}

// BPR means Bayesian Personal Ranking, is a pairwise learning algorithm for matrix factorization
// model with implicit feedback. The pairwise ranking between item i and j for user u is estimated
// by:
//
//	p(i >_u j) = \sigma( p_u^T (q_i - q_j) )
//
// Hyper-parameters:
//
//	 Reg 		- The regularization parameter of the cost function that is
//				  optimized. Default is 0.01.
//	 Lr 		- The learning rate of SGD. Default is 0.05.
//	 nFactors	- The number of latent factors. Default is 10.
//	 NEpochs	- The number of iteration of the SGD procedure. Default is 100.
//	 InitMean	- The mean of initial random latent factors. Default is 0.
//	 InitStdDev	- The standard deviation of initial random latent factors. Default is 0.001.
type BPR struct {
	BaseMatrixFactorization
	// Hyper parameters
	nFactors   int
	nEpochs    int
	lr         float32
	reg        float32
	initMean   float32
	initStdDev float32
}

// NewBPR creates a BPR model.
func NewBPR(params model.Params) *BPR {
	bpr := new(BPR)
	bpr.SetParams(params)
	return bpr
}

// SetParams sets hyper-parameters of the BPR model.
func (bpr *BPR) SetParams(params model.Params) {
	bpr.BaseMatrixFactorization.SetParams(params)
	// Setup hyper-parameters
	bpr.nFactors = bpr.Params.GetInt(model.NFactors, 16)
	bpr.nEpochs = bpr.Params.GetInt(model.NEpochs, 100)
	bpr.lr = bpr.Params.GetFloat32(model.Lr, 0.05)
	bpr.reg = bpr.Params.GetFloat32(model.Reg, 0.01)
	bpr.initMean = bpr.Params.GetFloat32(model.InitMean, 0)
	bpr.initStdDev = bpr.Params.GetFloat32(model.InitStdDev, 0.001)
}

func (bpr *BPR) GetParamsGrid(withSize bool) model.ParamsGrid {
	return model.ParamsGrid{
		model.NFactors:   lo.If(withSize, []interface{}{8, 16, 32, 64}).Else([]interface{}{16}),
		model.Lr:         []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
		model.Reg:        []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
		model.InitMean:   []interface{}{0},
		model.InitStdDev: []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
	}
}

// Fit the BPR model. Its task complexity is O(bpr.nEpochs).
func (bpr *BPR) Fit(ctx context.Context, trainSet, valSet dataset.CFSplit, config *FitConfig) Score {
	log.Logger().Info("fit bpr",
		zap.Int("train_set_size", trainSet.CountFeedback()),
		zap.Int("test_set_size", valSet.CountFeedback()),
		zap.Any("params", bpr.GetParams()),
		zap.Any("config", config))
	bpr.Init(trainSet)
	// Create buffers
	temp := base.NewMatrix32(config.Jobs, bpr.nFactors)
	userFactor := base.NewMatrix32(config.Jobs, bpr.nFactors)
	positiveItemFactor := base.NewMatrix32(config.Jobs, bpr.nFactors)
	negativeItemFactor := base.NewMatrix32(config.Jobs, bpr.nFactors)
	rng := make([]base.RandomGenerator, config.Jobs)
	for i := 0; i < config.Jobs; i++ {
		rng[i] = base.NewRandomGenerator(bpr.GetRandomGenerator().Int63())
	}
	// Convert array to hashmap
	userFeedback := make([]mapset.Set[int32], trainSet.CountUsers())
	for u := range userFeedback {
		userFeedback[u] = mapset.NewSet[int32]()
		for _, i := range trainSet.GetUserFeedback()[u] {
			userFeedback[u].Add(i)
		}
	}
	evalStart := time.Now()
	scores := Evaluate(bpr, valSet, trainSet, config.TopK, config.Candidates, config.Jobs, NDCG, Precision, Recall)
	evalTime := time.Since(evalStart)
	log.Logger().Debug(fmt.Sprintf("fit bpr %v/%v", 0, bpr.nEpochs),
		zap.String("eval_time", evalTime.String()),
		zap.Float32(fmt.Sprintf("NDCG@%v", config.TopK), scores[0]),
		zap.Float32(fmt.Sprintf("Precision@%v", config.TopK), scores[1]),
		zap.Float32(fmt.Sprintf("Recall@%v", config.TopK), scores[2]))
	// Training
	_, span := progress.Start(ctx, "BPR.Fit", bpr.nEpochs)
	for epoch := 1; epoch <= bpr.nEpochs; epoch++ {
		fitStart := time.Now()
		// Training epoch
		cost := make([]float32, config.Jobs)
		_ = parallel.Parallel(trainSet.CountFeedback(), config.Jobs, func(workerId, _ int) error {
			// Select a user
			var userIndex int32
			var ratingCount int
			for {
				userIndex = rng[workerId].Int31n(int32(trainSet.CountUsers()))
				ratingCount = len(trainSet.GetUserFeedback()[userIndex])
				if ratingCount > 0 {
					break
				}
			}
			posIndex := trainSet.GetUserFeedback()[userIndex][rng[workerId].Intn(ratingCount)]
			// Select a negative sample
			negIndex := int32(-1)
			for {
				temp := rng[workerId].Int31n(int32(trainSet.CountItems()))
				if !userFeedback[userIndex].Contains(temp) {
					negIndex = temp
					break
				}
			}
			diff := bpr.internalPredict(userIndex, posIndex) - bpr.internalPredict(userIndex, negIndex)
			cost[workerId] += math32.Log1p(math32.Exp(-diff))
			grad := math32.Exp(-diff) / (1.0 + math32.Exp(-diff))
			// Pairwise update
			copy(userFactor[workerId], bpr.UserFactor[userIndex])
			copy(positiveItemFactor[workerId], bpr.ItemFactor[posIndex])
			copy(negativeItemFactor[workerId], bpr.ItemFactor[negIndex])
			// Update positive item latent factor: +w_u
			floats.MulConstTo(userFactor[workerId], grad, temp[workerId])
			floats.MulConstAdd(positiveItemFactor[workerId], -bpr.reg, temp[workerId])
			floats.MulConstAdd(temp[workerId], bpr.lr, bpr.ItemFactor[posIndex])
			// Update negative item latent factor: -w_u
			floats.MulConstTo(userFactor[workerId], -grad, temp[workerId])
			floats.MulConstAdd(negativeItemFactor[workerId], -bpr.reg, temp[workerId])
			floats.MulConstAdd(temp[workerId], bpr.lr, bpr.ItemFactor[negIndex])
			// Update user latent factor: h_i-h_j
			floats.SubTo(positiveItemFactor[workerId], negativeItemFactor[workerId], temp[workerId])
			floats.MulConst(temp[workerId], grad)
			floats.MulConstAdd(userFactor[workerId], -bpr.reg, temp[workerId])
			floats.MulConstAdd(temp[workerId], bpr.lr, bpr.UserFactor[userIndex])
			return nil
		})
		fitTime := time.Since(fitStart)
		// Cross validation
		if epoch%config.Verbose == 0 || epoch == bpr.nEpochs {
			evalStart = time.Now()
			scores = Evaluate(bpr, valSet, trainSet, config.TopK, config.Candidates, config.Jobs, NDCG, Precision, Recall)
			evalTime = time.Since(evalStart)
			log.Logger().Info(fmt.Sprintf("fit bpr %v/%v", epoch, bpr.nEpochs),
				zap.String("fit_time", fitTime.String()),
				zap.String("eval_time", evalTime.String()),
				zap.Float32(fmt.Sprintf("NDCG@%v", config.TopK), scores[0]),
				zap.Float32(fmt.Sprintf("Precision@%v", config.TopK), scores[1]),
				zap.Float32(fmt.Sprintf("Recall@%v", config.TopK), scores[2]))
		}
		span.Add(1)
	}
	span.End()
	log.Logger().Info("fit bpr complete",
		zap.Float32(fmt.Sprintf("NDCG@%v", config.TopK), scores[0]),
		zap.Float32(fmt.Sprintf("Precision@%v", config.TopK), scores[1]),
		zap.Float32(fmt.Sprintf("Recall@%v", config.TopK), scores[2]))
	return Score{
		NDCG:      scores[0],
		Precision: scores[1],
		Recall:    scores[2],
	}
}

func (bpr *BPR) Init(trainSet dataset.CFSplit) {
	// Initialize parameters
	newUserFactor := bpr.GetRandomGenerator().NormalMatrix(trainSet.CountUsers(), bpr.nFactors, bpr.initMean, bpr.initStdDev)
	newItemFactor := bpr.GetRandomGenerator().NormalMatrix(trainSet.CountItems(), bpr.nFactors, bpr.initMean, bpr.initStdDev)
	// Initialize base
	bpr.UserFactor = newUserFactor
	bpr.ItemFactor = newItemFactor
	bpr.BaseMatrixFactorization.Init(trainSet)
}

// Marshal model into byte stream.
func (bpr *BPR) Marshal(w io.Writer) error {
	if err := bpr.BaseMatrixFactorization.Marshal(w); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// Unmarshal model from byte stream.
func (bpr *BPR) Unmarshal(r io.Reader) error {
	if err := bpr.BaseMatrixFactorization.Unmarshal(r); err != nil {
		return errors.Trace(err)
	}
	bpr.SetParams(bpr.Params)
	return nil
}

type ALS struct {
	BaseMatrixFactorization
	// Hyper parameters
	nFactors   int
	nEpochs    int
	reg        float32
	initMean   float32
	initStdDev float32
	weight     float32
}

// NewALS creates a eALS model.
func NewALS(params model.Params) *ALS {
	fast := new(ALS)
	fast.SetParams(params)
	return fast
}

// SetParams sets hyper-parameters for the ALS model.
func (als *ALS) SetParams(params model.Params) {
	als.BaseMatrixFactorization.SetParams(params)
	als.nFactors = als.Params.GetInt(model.NFactors, 16)
	als.nEpochs = als.Params.GetInt(model.NEpochs, 50)
	als.initMean = als.Params.GetFloat32(model.InitMean, 0)
	als.initStdDev = als.Params.GetFloat32(model.InitStdDev, 0.1)
	als.reg = als.Params.GetFloat32(model.Reg, 0.06)
	als.weight = als.Params.GetFloat32(model.Alpha, 0.001)
}

func (als *ALS) GetParamsGrid(withSize bool) model.ParamsGrid {
	return model.ParamsGrid{
		model.NFactors:   lo.If(withSize, []interface{}{8, 16, 32, 64}).Else([]interface{}{16}),
		model.InitMean:   []interface{}{0},
		model.InitStdDev: []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
		model.Reg:        []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
		model.Alpha:      []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
	}
}

func (als *ALS) Init(trainSet dataset.CFSplit) {
	// Initialize
	newUserFactor := als.GetRandomGenerator().NormalMatrix(trainSet.CountUsers(), als.nFactors, als.initMean, als.initStdDev)
	newItemFactor := als.GetRandomGenerator().NormalMatrix(trainSet.CountItems(), als.nFactors, als.initMean, als.initStdDev)
	// Initialize base
	als.UserFactor = newUserFactor
	als.ItemFactor = newItemFactor
	als.BaseMatrixFactorization.Init(trainSet)
}

// Fit the ALS model. Its task complexity is O(ccd.nEpochs).
func (als *ALS) Fit(ctx context.Context, trainSet, valSet dataset.CFSplit, config *FitConfig) Score {
	log.Logger().Info("fit als",
		zap.Int("train_set_size", trainSet.CountFeedback()),
		zap.Int("test_set_size", valSet.CountFeedback()),
		zap.Any("params", als.GetParams()),
		zap.Any("config", config))
	als.Init(trainSet)
	// Create temporary matrix
	s := base.NewMatrix32(als.nFactors, als.nFactors)
	userPredictions := make([][]float32, config.Jobs)
	itemPredictions := make([][]float32, config.Jobs)
	userRes := make([][]float32, config.Jobs)
	itemRes := make([][]float32, config.Jobs)
	for i := 0; i < config.Jobs; i++ {
		userPredictions[i] = make([]float32, trainSet.CountItems())
		itemPredictions[i] = make([]float32, trainSet.CountUsers())
		userRes[i] = make([]float32, trainSet.CountItems())
		itemRes[i] = make([]float32, trainSet.CountUsers())
	}
	// evaluate initial model
	evalStart := time.Now()
	scores := Evaluate(als, valSet, trainSet, config.TopK, config.Candidates, config.Jobs, NDCG, Precision, Recall)
	evalTime := time.Since(evalStart)
	log.Logger().Debug(fmt.Sprintf("fit als %v/%v", 0, als.nEpochs),
		zap.String("eval_time", evalTime.String()),
		zap.Float32(fmt.Sprintf("NDCG@%v", config.TopK), scores[0]),
		zap.Float32(fmt.Sprintf("Precision@%v", config.TopK), scores[1]),
		zap.Float32(fmt.Sprintf("Recall@%v", config.TopK), scores[2]))

	_, span := progress.Start(ctx, "ALS.Fit", als.nEpochs)
	for ep := 1; ep <= als.nEpochs; ep++ {
		fitStart := time.Now()
		// Update user factors
		// S^q <- \sum^N_{itemIndex=1} c_i q_i q_i^T
		floats.MatZero(s)
		for itemIndex := 0; itemIndex < trainSet.CountItems(); itemIndex++ {
			if len(trainSet.GetItemFeedback()[itemIndex]) > 0 {
				for i := 0; i < als.nFactors; i++ {
					for j := 0; j < als.nFactors; j++ {
						s[i][j] += als.ItemFactor[itemIndex][i] * als.ItemFactor[itemIndex][j]
					}
				}
			}
		}
		_ = parallel.Parallel(trainSet.CountUsers(), config.Jobs, func(workerId, userIndex int) error {
			userFeedback := trainSet.GetUserFeedback()[userIndex]
			for _, i := range userFeedback {
				userPredictions[workerId][i] = als.internalPredict(int32(userIndex), i)
			}
			for f := 0; f < als.nFactors; f++ {
				// for itemIndex \in R_u do   \hat_{r}^f_{ui} <- \hat_{r}_{ui} - p_{uf]q_{if}
				for _, i := range userFeedback {
					userRes[workerId][i] = userPredictions[workerId][i] - als.UserFactor[userIndex][f]*als.ItemFactor[i][f]
				}
				// p_{uf} <-
				a, b, c := float32(0), float32(0), float32(0)
				for _, i := range userFeedback {
					a += (1 - (1-als.weight)*userRes[workerId][i]) * als.ItemFactor[i][f]
					c += (1 - als.weight) * als.ItemFactor[i][f] * als.ItemFactor[i][f]
				}
				for k := 0; k < als.nFactors; k++ {
					if k != f {
						b += als.weight * als.UserFactor[userIndex][k] * s[k][f]
					}
				}
				als.UserFactor[userIndex][f] = (a - b) / (c + als.weight*s[f][f] + als.reg)
				// for itemIndex \in R_u do   \hat_{r}_{ui} <- \hat_{r}^f_{ui} - p_{uf]q_{if}
				for _, i := range userFeedback {
					userPredictions[workerId][i] = userRes[workerId][i] + als.UserFactor[userIndex][f]*als.ItemFactor[i][f]
				}
			}
			return nil
		})
		// Update item factors
		// S^p <- P^T P
		floats.MatZero(s)
		for userIndex := 0; userIndex < trainSet.CountUsers(); userIndex++ {
			if len(trainSet.GetUserFeedback()[userIndex]) > 0 {
				for i := 0; i < als.nFactors; i++ {
					for j := 0; j < als.nFactors; j++ {
						s[i][j] += als.UserFactor[userIndex][i] * als.UserFactor[userIndex][j]
					}
				}
			}
		}
		_ = parallel.Parallel(trainSet.CountItems(), config.Jobs, func(workerId, itemIndex int) error {
			itemFeedback := trainSet.GetItemFeedback()[itemIndex]
			for _, u := range itemFeedback {
				itemPredictions[workerId][u] = als.internalPredict(u, int32(itemIndex))
			}
			for f := 0; f < als.nFactors; f++ {
				// for itemIndex \in R_u do   \hat_{r}^f_{ui} <- \hat_{r}_{ui} - p_{uf]q_{if}
				for _, u := range itemFeedback {
					itemRes[workerId][u] = itemPredictions[workerId][u] - als.UserFactor[u][f]*als.ItemFactor[itemIndex][f]
				}
				// q_{if} <-
				a, b, c := float32(0), float32(0), float32(0)
				for _, u := range itemFeedback {
					a += (1 - (1-als.weight)*itemRes[workerId][u]) * als.UserFactor[u][f]
					c += (1 - als.weight) * als.UserFactor[u][f] * als.UserFactor[u][f]
				}
				for k := 0; k < als.nFactors; k++ {
					if k != f {
						b += als.weight * als.ItemFactor[itemIndex][k] * s[k][f]
					}
				}
				als.ItemFactor[itemIndex][f] = (a - b) / (c + als.weight*s[f][f] + als.reg)
				// for itemIndex \in R_u do   \hat_{r}_{ui} <- \hat_{r}^f_{ui} - p_{uf]q_{if}
				for _, u := range itemFeedback {
					itemPredictions[workerId][u] = itemRes[workerId][u] + als.UserFactor[u][f]*als.ItemFactor[itemIndex][f]
				}
			}
			return nil
		})
		fitTime := time.Since(fitStart)
		// Cross validation
		if ep%config.Verbose == 0 || ep == als.nEpochs {
			evalStart = time.Now()
			scores = Evaluate(als, valSet, trainSet, config.TopK, config.Candidates, config.Jobs, NDCG, Precision, Recall)
			evalTime = time.Since(evalStart)
			log.Logger().Debug(fmt.Sprintf("fit als %v/%v", ep, als.nEpochs),
				zap.String("fit_time", fitTime.String()),
				zap.String("eval_time", evalTime.String()),
				zap.Float32(fmt.Sprintf("NDCG@%v", config.TopK), scores[0]),
				zap.Float32(fmt.Sprintf("Precision@%v", config.TopK), scores[1]),
				zap.Float32(fmt.Sprintf("Recall@%v", config.TopK), scores[2]))
		}
		span.Add(1)
	}
	span.End()

	log.Logger().Info("fit als complete",
		zap.Float32(fmt.Sprintf("NDCG@%v", config.TopK), scores[0]),
		zap.Float32(fmt.Sprintf("Precision@%v", config.TopK), scores[1]),
		zap.Float32(fmt.Sprintf("Recall@%v", config.TopK), scores[2]))
	return Score{
		NDCG:      scores[0],
		Precision: scores[1],
		Recall:    scores[2],
	}
}

// Marshal model into byte stream.
func (als *ALS) Marshal(w io.Writer) error {
	if err := als.BaseMatrixFactorization.Marshal(w); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// Unmarshal model from byte stream.
func (als *ALS) Unmarshal(r io.Reader) error {
	if err := als.BaseMatrixFactorization.Unmarshal(r); err != nil {
		return errors.Trace(err)
	}
	als.SetParams(als.Params)
	return nil
}
