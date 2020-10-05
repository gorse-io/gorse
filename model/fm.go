package model

import (
	"fmt"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/floats"
	"math"
)

type _BiasUpdateCache struct {
	cache map[int]float64
}

func _NewBiasUpdateCache() *_BiasUpdateCache {
	cache := new(_BiasUpdateCache)
	cache.cache = make(map[int]float64)
	return cache
}

func (cache *_BiasUpdateCache) Add(index int, update float64) {
	// Create if not exist
	if _, exist := cache.cache[index]; !exist {
		cache.cache[index] = 0
	}
	cache.cache[index] += update
}

func (cache *_BiasUpdateCache) Commit(bias []float64) {
	for index, update := range cache.cache {
		bias[index] += update
	}
}

type _FactorUpdateCache struct {
	nFactor int
	cache   map[int][]float64
}

func _NewFactorUpdateCache(nFactor int) *_FactorUpdateCache {
	cache := new(_FactorUpdateCache)
	cache.nFactor = nFactor
	cache.cache = make(map[int][]float64)
	return cache
}

func (cache *_FactorUpdateCache) Add(index int, update []float64) {
	// Create if not exist
	if _, exist := cache.cache[index]; !exist {
		cache.cache[index] = make([]float64, cache.nFactor)
	}
	floats.Add(cache.cache[index], update)
}

func (cache *_FactorUpdateCache) Commit(factors [][]float64) {
	for index, update := range cache.cache {
		floats.Add(factors[index], update)
	}
}

// FM is the implementation of factorization machine [12]. The prediction is given by
//
//   \hat y(x) = w_0 + \sum^n_{i=1} w_i x_i + \sum^n_{i=1} \sum^n_{j=i+1} <v_i, v_j>x_i x_j
//
// Hyper-parameters:
//	 Reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.02.
//	 Lr 		- The learning rate of SGD. Default is 0.005.
//	 nFactors	- The number of latent factors. Default is 100.
//	 NEpochs	- The number of iteration of the SGD procedure. Default is 20.
//	 InitMean	- The mean of initial random latent factors. Default is 0.
//	 InitStdDev	- The standard deviation of initial random latent factors. Default is 0.1.
type FM struct {
	Base
	UserFeatures []*base.SparseVector
	ItemFeatures []*base.SparseVector
	// ModelInterface parameters
	GlobalBias float64     // w_0
	Bias       []float64   // w_i
	Factors    [][]float64 // v_i
	// Hyper parameters
	useBias    bool
	nFactors   int
	nEpochs    int
	lr         float64
	reg        float64
	initMean   float64
	initStdDev float64
	optimizer  string
	// Fallback model
	UserRatings []*base.MarginalSubSet
	ItemPop     *ItemPop
}

// NewFM creates a factorization machine.
func NewFM(params base.Params) *FM {
	fm := new(FM)
	fm.SetParams(params)
	return fm
}

// SetParams sets hyper-parameters of the factorization machine.
func (fm *FM) SetParams(params base.Params) {
	fm.Base.SetParams(params)
	// Setup hyper-parameters
	fm.useBias = fm.Params.GetBool(base.UseBias, true)
	fm.nFactors = fm.Params.GetInt(base.NFactors, 100)
	fm.nEpochs = fm.Params.GetInt(base.NEpochs, 20)
	fm.lr = fm.Params.GetFloat64(base.Lr, 0.005)
	fm.reg = fm.Params.GetFloat64(base.Reg, 0.02)
	fm.initMean = fm.Params.GetFloat64(base.InitMean, 0)
	fm.initStdDev = fm.Params.GetFloat64(base.InitStdDev, 0.1)
	fm.optimizer = fm.Params.GetString(base.Optimizer, base.SGDOptimizer)
}

// Predict by the factorization machine.
func (fm *FM) Predict(userId string, itemId string) float64 {
	// Convert sparse IDs to dense IDs
	userIndex := fm.UserIndexer.ToIndex(userId)
	itemIndex := fm.ItemIndexer.ToIndex(itemId)
	vector := fm.encode(userIndex, itemIndex)
	return fm.predict(vector)
}

func (fm *FM) encode(userIndex int, itemIndex int) *base.SparseVector {
	// Get user features and item features
	var userFeature, itemFeature *base.SparseVector
	if fm.UserFeatures != nil {
		userFeature = fm.UserFeatures[userIndex]
	}
	if fm.ItemFeatures != nil {
		itemFeature = fm.ItemFeatures[itemIndex]
	}
	// Encode feature vector
	vectorSize := 2 + userFeature.Len() + itemFeature.Len()
	vector := &base.SparseVector{
		Indices: make([]int, 0, vectorSize),
		Values:  make([]float64, 0, vectorSize),
	}
	vector.Add(userIndex, 1)
	vector.Add(itemIndex+fm.UserIndexer.Len(), 1)
	userFeature.ForEach(func(i, index int, value float64) {
		vector.Add(index+fm.UserIndexer.Len()+fm.ItemIndexer.Len(), value)
	})
	itemFeature.ForEach(func(i, index int, value float64) {
		vector.Add(index+fm.UserIndexer.Len()+fm.ItemIndexer.Len(), value)
	})
	//fmt.Println(vector)
	return vector
}

func (fm *FM) predict(vector *base.SparseVector) float64 {
	predict := fm.GlobalBias
	vector.ForEach(func(i, index int, value float64) {
		predict += fm.Bias[index]
	})
	for i := 0; i < vector.Len(); i++ {
		for j := i + 1; j < vector.Len(); j++ {
			factor1 := fm.Factors[vector.Indices[i]]
			factor2 := fm.Factors[vector.Indices[j]]
			value1 := vector.Values[i]
			value2 := vector.Values[j]
			predict += floats.Dot(factor1, factor2) * value1 * value2
		}
	}
	return predict
}

// Fit the factorization machine.
func (fm *FM) Fit(trainSet DataSetInterface, options *base.RuntimeOptions) {
	fm.Init(trainSet)
	fm.UserFeatures = trainSet.UserFeatures()
	fm.ItemFeatures = trainSet.ItemFeatures()
	// Initialization
	paramCount := trainSet.UserCount() + trainSet.ItemCount() + trainSet.FeatureCount()
	fm.GlobalBias = trainSet.GlobalMean()
	fm.Bias = make([]float64, paramCount)
	fm.Factors = fm.rng.NewNormalMatrix(paramCount, fm.nFactors, fm.initMean, fm.initStdDev)
	// Optimize
	switch fm.optimizer {
	case base.SGDOptimizer:
		fm.fitSGD(trainSet, options)
	case base.BPROptimizer:
		fm.fitBPR(trainSet, options)
	default:
		panic(fmt.Sprintf("Unknown optimizer: %v", fm.optimizer))
	}
}

func (fm *FM) fitSGD(trainSet DataSetInterface, options *base.RuntimeOptions) {
	// Create buffers
	temp := make([]float64, fm.nFactors)
	gradFactor := make([]float64, fm.nFactors)
	// Optimize
	for epoch := 0; epoch < fm.nEpochs; epoch++ {
		cost := 0.0
		for i := 0; i < trainSet.Count(); i++ {
			userIndex, itemIndex, rating := trainSet.GetWithIndex(i)
			vector := fm.encode(userIndex, itemIndex)
			// Compute error: e_{ui} = r - \hat r
			upGrad := rating - fm.predict(vector)
			cost += upGrad * upGrad
			// Update global bias
			//   \frac {\partial\hat{y}(x)} {\partial w_0} = 1
			gradGlobalBias := upGrad - fm.reg*fm.GlobalBias
			fm.GlobalBias += fm.lr * gradGlobalBias
			// Update bias
			//   \frac {\partial\hat{y}(x)} {\partial w_i} = x_i
			vector.ForEach(func(_, index int, value float64) {
				gradBias := upGrad*value - fm.reg*fm.Bias[index]
				fm.Bias[index] += fm.lr * gradBias
			})
			// Update factors
			//   \frac {\partial\hat{y}(x)} {\partial v_{i,f}}
			//   = x_i \sum^n_{j=1} v_{i,f}x_j - v_{i,g}x^2_i
			// 1. Pre-compute \sum^n_{j=1} v_{i,f}x_j
			base.FillZeroVector(temp)
			vector.ForEach(func(_, index int, value float64) {
				floats.MulConstAddTo(fm.Factors[index], value, temp)
			})
			// 2. Update by x_i \sum^n_{j=1} v_{i,f}x_j - v_{i,g}x^2_i
			vector.ForEach(func(_, index int, value float64) {
				floats.MulConstTo(temp, upGrad*value, gradFactor)
				floats.MulConstAddTo(fm.Factors[index], -upGrad*value*value, gradFactor)
				floats.MulConstAddTo(fm.Factors[index], -fm.reg, gradFactor)
				floats.MulConstAddTo(gradFactor, fm.lr, fm.Factors[index])
			})
		}
		options.Logf("epoch = %v/%v, cost = %v", epoch+1, fm.nEpochs, cost)
	}
}

func (fm *FM) fitBPR(trainSet DataSetInterface, options *base.RuntimeOptions) {
	fm.UserRatings = trainSet.Users()
	// Create item pop model
	fm.ItemPop = NewItemPop(nil)
	fm.ItemPop.Fit(trainSet, options)
	// Create buffers
	temp := make([]float64, fm.nFactors)
	gradFactor := make([]float64, fm.nFactors)
	// Training
	for epoch := 0; epoch < fm.nEpochs; epoch++ {
		// Training epoch
		cost := 0.0
		for i := 0; i < trainSet.Count(); i++ {
			// Select a user
			var userIndex, ratingCount int
			for {
				userIndex = fm.rng.Intn(trainSet.UserCount())
				ratingCount = trainSet.UserByIndex(userIndex).Len()
				if ratingCount > 0 {
					break
				}
			}
			posIndex := trainSet.UserByIndex(userIndex).GetIndex(fm.rng.Intn(ratingCount))
			// Select a negative sample
			negIndex := -1
			for {
				temp := fm.rng.Intn(trainSet.ItemCount())
				tempId := fm.ItemIndexer.ToID(temp)
				if !trainSet.UserByIndex(userIndex).Contain(tempId) {
					negIndex = temp
					break
				}
			}
			posVec := fm.encode(userIndex, posIndex)
			negVec := fm.encode(userIndex, negIndex)
			diff := fm.predict(posVec) - fm.predict(negVec)
			cost += math.Log(1.0 + math.Exp(-diff))
			upGrad := math.Exp(-diff) / (1.0 + math.Exp(-diff))
			// Update bias:
			//   \frac {\partial\hat{y}(x)} {\partial w_i} = x_i
			biasCache := _NewBiasUpdateCache()
			// 1. Positive sample
			posVec.ForEach(func(_, index int, value float64) {
				gradBias := upGrad*value - fm.reg*fm.Bias[index]
				biasCache.Add(index, fm.lr*gradBias)
			})
			// 2. Negative sample
			negVec.ForEach(func(_, index int, value float64) {
				gradBias := -upGrad*value - fm.reg*fm.Bias[index]
				biasCache.Add(index, fm.lr*gradBias)
			})
			// 3. Commit update
			biasCache.Commit(fm.Bias)
			// Update factors:
			//   \frac {\partial\hat{y}(x)} {\partial v_{i,f}}
			//   = x_i \sum^n_{j=1} v_{i,f}x_j - v_{i,g}x^2_i
			factorCache := _NewFactorUpdateCache(fm.nFactors)
			// 1. Positive sample
			base.FillZeroVector(temp)
			posVec.ForEach(func(_, index int, value float64) {
				floats.MulConstAddTo(fm.Factors[index], value, temp)
			})
			posVec.ForEach(func(_, index int, value float64) {
				floats.MulConstTo(temp, upGrad*value, gradFactor)
				floats.MulConstAddTo(fm.Factors[index], -upGrad*value*value, gradFactor)
				floats.MulConstAddTo(fm.Factors[index], -fm.reg, gradFactor)
				floats.MulConst(gradFactor, fm.lr)
				factorCache.Add(index, gradFactor)
			})
			// 2. Negative sample
			base.FillZeroVector(temp)
			negVec.ForEach(func(_, index int, value float64) {
				floats.MulConstAddTo(fm.Factors[index], value, temp)
			})
			negVec.ForEach(func(_, index int, value float64) {
				floats.MulConstTo(temp, -upGrad*value, gradFactor)
				floats.MulConstAddTo(fm.Factors[index], upGrad*value*value, gradFactor)
				floats.MulConstAddTo(fm.Factors[index], -fm.reg, gradFactor)
				floats.MulConst(gradFactor, fm.lr)
				factorCache.Add(index, gradFactor)
			})
			// 3. Commit update
			factorCache.Commit(fm.Factors)
		}
		options.Logf("epoch = %v/%v, cost = %v", epoch+1, fm.nEpochs, cost)
	}
}
