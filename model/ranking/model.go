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

package ranking

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/bits-and-blooms/bitset"
	"github.com/chewxy/math32"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/copier"
	"github.com/zhenghaoz/gorse/base/floats"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/parallel"
	"github.com/zhenghaoz/gorse/base/progress"
	"github.com/zhenghaoz/gorse/base/task"
	"github.com/zhenghaoz/gorse/dataset"
	"github.com/zhenghaoz/gorse/model"
	"go.uber.org/zap"
)

type Score struct {
	NDCG      float32
	Precision float32
	Recall    float32
}

type FitConfig struct {
	*task.JobsAllocator
	Verbose    int
	Candidates int
	TopK       int
}

func NewFitConfig() *FitConfig {
	return &FitConfig{
		Verbose:    10,
		Candidates: 100,
		TopK:       10,
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

type Model interface {
	model.Model
	// Fit a model with a train set and parameters.
	Fit(ctx context.Context, trainSet *dataset.Dataset, validateSet *dataset.Dataset, config *FitConfig) Score
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
	InternalPredict(userIndex, itemIndex int32) float32
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

func (baseModel *BaseMatrixFactorization) Init(trainSet *dataset.Dataset) {
	baseModel.UserIndex = trainSet.GetUserDict()
	baseModel.ItemIndex = trainSet.GetItemDict()
	// set user trained flags
	baseModel.UserPredictable = bitset.New(uint(baseModel.UserIndex.Count()))
	for userIndex := 0; userIndex < baseModel.UserIndex.Count(); userIndex++ {
		if len(trainSet.GetUserFeedback()[userIndex]) > 0 {
			baseModel.UserPredictable.Set(uint(userIndex))
		}
	}
	// set item trained flags
	baseModel.ItemPredictable = bitset.New(uint(baseModel.ItemIndex.Count()))
	for itemIndex := 0; itemIndex < baseModel.ItemIndex.Count(); itemIndex++ {
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
	if int(userIndex) >= baseModel.UserIndex.Count() {
		return false
	}
	return baseModel.UserPredictable.Test(uint(userIndex))
}

// IsItemPredictable returns false if item has no feedback and its embedding vector never be trained.
func (baseModel *BaseMatrixFactorization) IsItemPredictable(itemIndex int32) bool {
	if int(itemIndex) >= baseModel.ItemIndex.Count() {
		return false
	}
	return baseModel.ItemPredictable.Test(uint(itemIndex))
}

// Marshal model into byte stream.
func (baseModel *BaseMatrixFactorization) Marshal(w io.Writer) error {
	// TODO: implement Marshal
	return nil
}

// Unmarshal model from byte stream.
func (baseModel *BaseMatrixFactorization) Unmarshal(r io.Reader) error {
	// TODO: implement Unmarshal
	return nil
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

// GetUserFactor returns the latent factor of a user.
func (bpr *BPR) GetUserFactor(userIndex int32) []float32 {
	return bpr.UserFactor[userIndex]
}

// GetItemFactor returns the latent factor of an item.
func (bpr *BPR) GetItemFactor(itemIndex int32) []float32 {
	return bpr.ItemFactor[itemIndex]
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

// Predict by the BPR model.
func (bpr *BPR) Predict(userId, itemId string) float32 {
	// Convert sparse Names to dense Names
	userIndex := bpr.UserIndex.Id(userId)
	itemIndex := bpr.ItemIndex.Id(itemId)
	if userIndex < 0 {
		log.Logger().Warn("unknown user", zap.String("user_id", userId))
	}
	if itemIndex < 0 {
		log.Logger().Warn("unknown item", zap.String("item_id", itemId))
	}
	return bpr.InternalPredict(int32(userIndex), int32(itemIndex))
}

func (bpr *BPR) InternalPredict(userIndex, itemIndex int32) float32 {
	ret := float32(0.0)
	// + q_i^Tp_u
	if itemIndex >= 0 && userIndex >= 0 {
		ret += floats.Dot(bpr.UserFactor[userIndex], bpr.ItemFactor[itemIndex])
	} else {
		log.Logger().Warn("unknown user or item")
	}
	return ret
}

// Fit the BPR model. Its task complexity is O(bpr.nEpochs).
func (bpr *BPR) Fit(ctx context.Context, trainSet, valSet *dataset.Dataset, config *FitConfig) Score {
	config = config.LoadDefaultIfNil()
	log.Logger().Info("fit bpr",
		zap.Int("train_set_size", trainSet.Count()),
		zap.Int("test_set_size", valSet.Count()),
		zap.Any("params", bpr.GetParams()),
		zap.Any("config", config))
	bpr.Init(trainSet)
	// Create buffers
	maxJobs := config.MaxJobs()
	temp := base.NewMatrix32(maxJobs, bpr.nFactors)
	userFactor := base.NewMatrix32(maxJobs, bpr.nFactors)
	positiveItemFactor := base.NewMatrix32(maxJobs, bpr.nFactors)
	negativeItemFactor := base.NewMatrix32(maxJobs, bpr.nFactors)
	rng := make([]base.RandomGenerator, maxJobs)
	for i := 0; i < maxJobs; i++ {
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
	scores := Evaluate(bpr, valSet, trainSet, config.TopK, config.Candidates, config.AvailableJobs(), NDCG, Precision, Recall)
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
		numJobs := config.AvailableJobs()
		cost := make([]float32, numJobs)
		_ = parallel.Parallel(trainSet.Count(), numJobs, func(workerId, _ int) error {
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
			diff := bpr.InternalPredict(userIndex, posIndex) - bpr.InternalPredict(userIndex, negIndex)
			cost[workerId] += math32.Log1p(math32.Exp(-diff))
			grad := math32.Exp(-diff) / (1.0 + math32.Exp(-diff))
			// Pairwise update
			copy(userFactor[workerId], bpr.UserFactor[userIndex])
			copy(positiveItemFactor[workerId], bpr.ItemFactor[posIndex])
			copy(negativeItemFactor[workerId], bpr.ItemFactor[negIndex])
			// Update positive item latent factor: +w_u
			floats.MulConstTo(userFactor[workerId], grad, temp[workerId])
			floats.MulConstAddTo(positiveItemFactor[workerId], -bpr.reg, temp[workerId])
			floats.MulConstAddTo(temp[workerId], bpr.lr, bpr.ItemFactor[posIndex])
			// Update negative item latent factor: -w_u
			floats.MulConstTo(userFactor[workerId], -grad, temp[workerId])
			floats.MulConstAddTo(negativeItemFactor[workerId], -bpr.reg, temp[workerId])
			floats.MulConstAddTo(temp[workerId], bpr.lr, bpr.ItemFactor[negIndex])
			// Update user latent factor: h_i-h_j
			floats.SubTo(positiveItemFactor[workerId], negativeItemFactor[workerId], temp[workerId])
			floats.MulConst(temp[workerId], grad)
			floats.MulConstAddTo(userFactor[workerId], -bpr.reg, temp[workerId])
			floats.MulConstAddTo(temp[workerId], bpr.lr, bpr.UserFactor[userIndex])
			return nil
		})
		fitTime := time.Since(fitStart)
		// Cross validation
		if epoch%config.Verbose == 0 || epoch == bpr.nEpochs {
			evalStart = time.Now()
			scores = Evaluate(bpr, valSet, trainSet, config.TopK, config.Candidates, config.AvailableJobs(), NDCG, Precision, Recall)
			evalTime = time.Since(evalStart)
			log.Logger().Debug(fmt.Sprintf("fit bpr %v/%v", epoch, bpr.nEpochs),
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

func (bpr *BPR) Clear() {
	bpr.UserIndex = nil
	bpr.ItemIndex = nil
	bpr.UserFactor = nil
	bpr.ItemFactor = nil
}

func (bpr *BPR) Invalid() bool {
	return bpr == nil ||
		bpr.UserIndex == nil ||
		bpr.ItemIndex == nil ||
		bpr.UserFactor == nil ||
		bpr.ItemFactor == nil
}

func (bpr *BPR) Init(trainSet *dataset.Dataset) {
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
	// TODO: implement Marshal
	return nil
}

// Unmarshal model from byte stream.
func (bpr *BPR) Unmarshal(r io.Reader) error {
	// TODO: implement Unmarshal
	return nil
}

type CCD struct {
	BaseMatrixFactorization
	// Hyper parameters
	nFactors   int
	nEpochs    int
	reg        float32
	initMean   float32
	initStdDev float32
	weight     float32
}

// NewCCD creates a eALS model.
func NewCCD(params model.Params) *CCD {
	fast := new(CCD)
	fast.SetParams(params)
	return fast
}

// GetUserFactor returns latent factor of a user.
func (ccd *CCD) GetUserFactor(userIndex int32) []float32 {
	return ccd.UserFactor[userIndex]
}

// GetItemFactor returns latent factor of an item.
func (ccd *CCD) GetItemFactor(itemIndex int32) []float32 {
	return ccd.ItemFactor[itemIndex]
}

// SetParams sets hyper-parameters for the ALS model.
func (ccd *CCD) SetParams(params model.Params) {
	ccd.BaseMatrixFactorization.SetParams(params)
	ccd.nFactors = ccd.Params.GetInt(model.NFactors, 16)
	ccd.nEpochs = ccd.Params.GetInt(model.NEpochs, 50)
	ccd.initMean = ccd.Params.GetFloat32(model.InitMean, 0)
	ccd.initStdDev = ccd.Params.GetFloat32(model.InitStdDev, 0.1)
	ccd.reg = ccd.Params.GetFloat32(model.Reg, 0.06)
	ccd.weight = ccd.Params.GetFloat32(model.Alpha, 0.001)
}

func (ccd *CCD) GetParamsGrid(withSize bool) model.ParamsGrid {
	return model.ParamsGrid{
		model.NFactors:   lo.If(withSize, []interface{}{8, 16, 32, 64}).Else([]interface{}{16}),
		model.InitMean:   []interface{}{0},
		model.InitStdDev: []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
		model.Reg:        []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
		model.Alpha:      []interface{}{0.001, 0.005, 0.01, 0.05, 0.1},
	}
}

// Predict by the ALS model.
func (ccd *CCD) Predict(userId, itemId string) float32 {
	userIndex := ccd.UserIndex.Id(userId)
	itemIndex := ccd.ItemIndex.Id(itemId)
	if userIndex < 0 {
		log.Logger().Info("unknown user:", zap.String("user_id", userId))
		return 0
	}
	if itemIndex < 0 {
		log.Logger().Info("unknown item:", zap.String("item_id", itemId))
		return 0
	}
	return ccd.InternalPredict(int32(userIndex), int32(itemIndex))
}

func (ccd *CCD) InternalPredict(userIndex, itemIndex int32) float32 {
	ret := float32(0.0)
	if itemIndex >= 0 && userIndex >= 0 {
		ret = floats.Dot(ccd.UserFactor[userIndex], ccd.ItemFactor[itemIndex])
	} else {
		log.Logger().Warn("unknown user or item")
	}
	return ret
}

func (ccd *CCD) Clear() {
	ccd.UserIndex = nil
	ccd.ItemIndex = nil
	ccd.ItemFactor = nil
	ccd.UserFactor = nil
}

func (ccd *CCD) Invalid() bool {
	return ccd == nil ||
		ccd.UserIndex == nil ||
		ccd.ItemIndex == nil ||
		ccd.ItemFactor == nil ||
		ccd.UserFactor == nil
}

func (ccd *CCD) Init(trainSet *dataset.Dataset) {
	// Initialize
	newUserFactor := ccd.GetRandomGenerator().NormalMatrix(trainSet.CountUsers(), ccd.nFactors, ccd.initMean, ccd.initStdDev)
	newItemFactor := ccd.GetRandomGenerator().NormalMatrix(trainSet.CountItems(), ccd.nFactors, ccd.initMean, ccd.initStdDev)
	// Initialize base
	ccd.UserFactor = newUserFactor
	ccd.ItemFactor = newItemFactor
	ccd.BaseMatrixFactorization.Init(trainSet)
}

// Fit the CCD model. Its task complexity is O(ccd.nEpochs).
func (ccd *CCD) Fit(ctx context.Context, trainSet, valSet *dataset.Dataset, config *FitConfig) Score {
	config = config.LoadDefaultIfNil()
	log.Logger().Info("fit ccd",
		zap.Int("train_set_size", trainSet.Count()),
		zap.Int("test_set_size", valSet.Count()),
		zap.Any("params", ccd.GetParams()),
		zap.Any("config", config))
	ccd.Init(trainSet)
	// Create temporary matrix
	maxJobs := config.MaxJobs()
	s := base.NewMatrix32(ccd.nFactors, ccd.nFactors)
	userPredictions := make([][]float32, maxJobs)
	itemPredictions := make([][]float32, maxJobs)
	userRes := make([][]float32, maxJobs)
	itemRes := make([][]float32, maxJobs)
	for i := 0; i < maxJobs; i++ {
		userPredictions[i] = make([]float32, trainSet.CountItems())
		itemPredictions[i] = make([]float32, trainSet.CountUsers())
		userRes[i] = make([]float32, trainSet.CountItems())
		itemRes[i] = make([]float32, trainSet.CountUsers())
	}
	// evaluate initial model
	evalStart := time.Now()
	scores := Evaluate(ccd, valSet, trainSet, config.TopK, config.Candidates, config.AvailableJobs(), NDCG, Precision, Recall)
	evalTime := time.Since(evalStart)
	log.Logger().Debug(fmt.Sprintf("fit ccd %v/%v", 0, ccd.nEpochs),
		zap.String("eval_time", evalTime.String()),
		zap.Float32(fmt.Sprintf("NDCG@%v", config.TopK), scores[0]),
		zap.Float32(fmt.Sprintf("Precision@%v", config.TopK), scores[1]),
		zap.Float32(fmt.Sprintf("Recall@%v", config.TopK), scores[2]))

	_, span := progress.Start(ctx, "CCD.Fit", ccd.nEpochs)
	for ep := 1; ep <= ccd.nEpochs; ep++ {
		fitStart := time.Now()
		// Update user factors
		// S^q <- \sum^N_{itemIndex=1} c_i q_i q_i^T
		floats.MatZero(s)
		for itemIndex := 0; itemIndex < trainSet.CountItems(); itemIndex++ {
			if len(trainSet.GetItemFeedback()[itemIndex]) > 0 {
				for i := 0; i < ccd.nFactors; i++ {
					for j := 0; j < ccd.nFactors; j++ {
						s[i][j] += ccd.ItemFactor[itemIndex][i] * ccd.ItemFactor[itemIndex][j]
					}
				}
			}
		}
		_ = parallel.Parallel(trainSet.CountUsers(), config.AvailableJobs(), func(workerId, userIndex int) error {
			userFeedback := trainSet.GetUserFeedback()[userIndex]
			for _, i := range userFeedback {
				userPredictions[workerId][i] = ccd.InternalPredict(int32(userIndex), i)
			}
			for f := 0; f < ccd.nFactors; f++ {
				// for itemIndex \in R_u do   \hat_{r}^f_{ui} <- \hat_{r}_{ui} - p_{uf]q_{if}
				for _, i := range userFeedback {
					userRes[workerId][i] = userPredictions[workerId][i] - ccd.UserFactor[userIndex][f]*ccd.ItemFactor[i][f]
				}
				// p_{uf} <-
				a, b, c := float32(0), float32(0), float32(0)
				for _, i := range userFeedback {
					a += (1 - (1-ccd.weight)*userRes[workerId][i]) * ccd.ItemFactor[i][f]
					c += (1 - ccd.weight) * ccd.ItemFactor[i][f] * ccd.ItemFactor[i][f]
				}
				for k := 0; k < ccd.nFactors; k++ {
					if k != f {
						b += ccd.weight * ccd.UserFactor[userIndex][k] * s[k][f]
					}
				}
				ccd.UserFactor[userIndex][f] = (a - b) / (c + ccd.weight*s[f][f] + ccd.reg)
				// for itemIndex \in R_u do   \hat_{r}_{ui} <- \hat_{r}^f_{ui} - p_{uf]q_{if}
				for _, i := range userFeedback {
					userPredictions[workerId][i] = userRes[workerId][i] + ccd.UserFactor[userIndex][f]*ccd.ItemFactor[i][f]
				}
			}
			return nil
		})
		// Update item factors
		// S^p <- P^T P
		floats.MatZero(s)
		for userIndex := 0; userIndex < trainSet.CountUsers(); userIndex++ {
			if len(trainSet.GetUserFeedback()[userIndex]) > 0 {
				for i := 0; i < ccd.nFactors; i++ {
					for j := 0; j < ccd.nFactors; j++ {
						s[i][j] += ccd.UserFactor[userIndex][i] * ccd.UserFactor[userIndex][j]
					}
				}
			}
		}
		_ = parallel.Parallel(trainSet.CountItems(), config.AvailableJobs(), func(workerId, itemIndex int) error {
			itemFeedback := trainSet.GetItemFeedback()[itemIndex]
			for _, u := range itemFeedback {
				itemPredictions[workerId][u] = ccd.InternalPredict(u, int32(itemIndex))
			}
			for f := 0; f < ccd.nFactors; f++ {
				// for itemIndex \in R_u do   \hat_{r}^f_{ui} <- \hat_{r}_{ui} - p_{uf]q_{if}
				for _, u := range itemFeedback {
					itemRes[workerId][u] = itemPredictions[workerId][u] - ccd.UserFactor[u][f]*ccd.ItemFactor[itemIndex][f]
				}
				// q_{if} <-
				a, b, c := float32(0), float32(0), float32(0)
				for _, u := range itemFeedback {
					a += (1 - (1-ccd.weight)*itemRes[workerId][u]) * ccd.UserFactor[u][f]
					c += (1 - ccd.weight) * ccd.UserFactor[u][f] * ccd.UserFactor[u][f]
				}
				for k := 0; k < ccd.nFactors; k++ {
					if k != f {
						b += ccd.weight * ccd.ItemFactor[itemIndex][k] * s[k][f]
					}
				}
				ccd.ItemFactor[itemIndex][f] = (a - b) / (c + ccd.weight*s[f][f] + ccd.reg)
				// for itemIndex \in R_u do   \hat_{r}_{ui} <- \hat_{r}^f_{ui} - p_{uf]q_{if}
				for _, u := range itemFeedback {
					itemPredictions[workerId][u] = itemRes[workerId][u] + ccd.UserFactor[u][f]*ccd.ItemFactor[itemIndex][f]
				}
			}
			return nil
		})
		fitTime := time.Since(fitStart)
		// Cross validation
		if ep%config.Verbose == 0 || ep == ccd.nEpochs {
			evalStart = time.Now()
			scores = Evaluate(ccd, valSet, trainSet, config.TopK, config.Candidates, config.AvailableJobs(), NDCG, Precision, Recall)
			evalTime = time.Since(evalStart)
			log.Logger().Debug(fmt.Sprintf("fit ccd %v/%v", ep, ccd.nEpochs),
				zap.String("fit_time", fitTime.String()),
				zap.String("eval_time", evalTime.String()),
				zap.Float32(fmt.Sprintf("NDCG@%v", config.TopK), scores[0]),
				zap.Float32(fmt.Sprintf("Precision@%v", config.TopK), scores[1]),
				zap.Float32(fmt.Sprintf("Recall@%v", config.TopK), scores[2]))
		}
		span.Add(1)
	}
	span.End()

	log.Logger().Info("fit ccd complete",
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
func (ccd *CCD) Marshal(w io.Writer) error {
	// TODO: implement Marshal
	return nil
}

// Unmarshal model from byte stream.
func (ccd *CCD) Unmarshal(r io.Reader) error {
	// TODO: implement Unmarshal
	return nil
}
