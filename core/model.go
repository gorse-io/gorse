package core

import (
	"time"
)

/* Model */

// Model is the algorithm interface to predict ratings. Any estimator in this
// package should implement it.
type Model interface {
	// Set parameters.
	SetParams(params Params)
	// Get parameters.
	GetParams() Params
	// Predict the rating given by a user (userId) to a item (itemId).
	Predict(userId, itemId int) float64
	// Fit a model with a train set and parameters.
	Fit(trainSet TrainSet, setters ...RuntimeOptionSetter)
}

/* Base */

// Base structure of all estimators.
type Base struct {
	Params    Params          // Hyper-parameters
	UserIdSet SparseIdSet     // Users' ID set
	ItemIdSet SparseIdSet     // Items' ID set
	rng       RandomGenerator // Random generator
	randState int             // Random seed
}

func (base *Base) SetParams(params Params) {
	base.Params = params
	base.randState = base.Params.GetInt(RandomState, int(time.Now().UnixNano()))
}

func (base *Base) GetParams() Params {
	return base.Params
}

func (base *Base) Predict(userId, itemId int) float64 {
	panic("Predict() not implemented")
}

func (base *Base) Fit(trainSet TrainSet, setters ...RuntimeOptionSetter) {
	panic("Fit() not implemented")
}

// Init the base model.
func (base *Base) Init(trainSet TrainSet) {
	// Setup ID set
	base.UserIdSet = trainSet.UserIdSet
	base.ItemIdSet = trainSet.ItemIdSet
	// Setup random state
	base.rng = NewRandomGenerator(base.randState)
}

/* Random */

// Random: The algorithm predicts a random rating based on the distribution of
// the training set, which is assumed to be normal. The prediction
// \hat{r}_{ui} is generated from a normal distribution N(\hat{μ},\hat{σ}^2)
// where \hat{μ} and \hat{σ}^2 are estimated from the training data
// using Maximum Likelihood Estimation
type Random struct {
	Base
	// Parameters
	Mean   float64 // mu
	StdDev float64 // sigma
	Low    float64 // The lower bound of rating scores
	High   float64 // The upper bound of rating scores
}

// Create a random model.
func NewRandom(params Params) *Random {
	random := new(Random)
	random.SetParams(params)
	return random
}

func (random *Random) Predict(userId int, itemId int) float64 {
	ret := random.rng.NormFloat64()*random.StdDev + random.Mean
	// Crop prediction
	if ret < random.Low {
		ret = random.Low
	} else if ret > random.High {
		ret = random.High
	}
	return ret
}

func (random *Random) Fit(trainSet TrainSet, setters ...RuntimeOptionSetter) {
	random.Init(trainSet)
	random.Mean = trainSet.Mean()
	random.StdDev = trainSet.StdDev()
	random.Low, random.High = trainSet.Range()
}

/* Baseline */

// Algorithm predicting the baseline estimate for given user and item.
//
//                   \hat{r}_{ui} = b_{ui} = μ + b_u + b_i
//
// If user u is unknown, then the Bias b_u is assumed to be zero. The same
// applies for item i with b_i.
type BaseLine struct {
	Base
	UserBias   []float64 // b_u
	ItemBias   []float64 // b_i
	GlobalBias float64   // mu
}

// Create a baseline model. Params:
//	 Reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.02.
//	 Lr 		- The learning rate of SGD. Default is 0.005.
//	 NEpochs	- The number of iteration of the SGD procedure. Default is 20.
func NewBaseLine(params Params) *BaseLine {
	baseLine := new(BaseLine)
	baseLine.Params = params
	return baseLine
}

func (baseLine *BaseLine) Predict(userId, itemId int) float64 {
	// Convert to inner Id
	innerUserId := baseLine.UserIdSet.ToDenseId(userId)
	innerItemId := baseLine.ItemIdSet.ToDenseId(itemId)
	ret := baseLine.GlobalBias
	if innerUserId != NewId {
		ret += baseLine.UserBias[innerUserId]
	}
	if innerItemId != NewId {
		ret += baseLine.ItemBias[innerItemId]
	}
	return ret
}

func (baseLine *BaseLine) Fit(trainSet TrainSet, setters ...RuntimeOptionSetter) {
	// Setup parameters
	reg := baseLine.Params.GetFloat64(Reg, 0.02)
	lr := baseLine.Params.GetFloat64(Lr, 0.005)
	nEpochs := baseLine.Params.GetInt(NEpochs, 20)
	// Initialize parameters
	baseLine.UserBias = make([]float64, trainSet.UserCount)
	baseLine.ItemBias = make([]float64, trainSet.ItemCount)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < nEpochs; epoch++ {
		for i := 0; i < trainSet.Length(); i++ {
			userId, itemId, rating := trainSet.Index(i)
			innerUserId := trainSet.ConvertUserId(userId)
			innerItemId := trainSet.ConvertItemId(itemId)
			userBias := baseLine.UserBias[innerUserId]
			itemBias := baseLine.ItemBias[innerItemId]
			// Compute gradient
			diff := baseLine.Predict(userId, itemId) - rating
			gradGlobalBias := diff
			gradUserBias := diff + reg*userBias
			gradItemBias := diff + reg*itemBias
			// Update parameters
			baseLine.GlobalBias -= lr * gradGlobalBias
			baseLine.UserBias[innerUserId] -= lr * gradUserBias
			baseLine.ItemBias[innerItemId] -= lr * gradItemBias
		}
	}
}
