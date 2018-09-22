package core

import (
	"gonum.org/v1/gonum/stat"
	"math/rand"
)

/* Base */

// An algorithm interface to predict ratings. Any estimator in this
// package should implement it.
type Estimator interface {
	// Set parameters.
	SetParams(params Parameters)
	// Predict the rating given by a user (userId) to a item (itemId).
	Predict(userId, itemId int) float64
	// Fit a model with a train set and parameters.
	Fit(trainSet TrainSet)
}

// Parameters for an algorithm. Given by:
//   map[string]interface{}{
//	   "<parameter name 1>": <parameter value 1>,
//	   "<parameter name 2>": <parameter value 2>,
//	   ...
//	   "<parameter name n>": <parameter value n>,
//	 }
type Parameters map[string]interface{}

// Copy parameters.
func (parameters Parameters) Copy() Parameters {
	newParams := make(Parameters)
	for k, v := range parameters {
		newParams[k] = v
	}
	return newParams
}

// Get a integer parameter.
func (parameters Parameters) GetInt(name string, _default int) int {
	if val, exist := parameters[name]; exist {
		return val.(int)
	}
	return _default
}

// Get a bool parameter.
func (parameters Parameters) GetBool(name string, _default bool) bool {
	if val, exist := parameters[name]; exist {
		return val.(bool)
	}
	return _default
}

// Get a float parameter.
func (parameters Parameters) GetFloat64(name string, _default float64) float64 {
	if val, exist := parameters[name]; exist {
		return val.(float64)
	}
	return _default
}

// Get a string parameter
func (parameters Parameters) GetString(name string, _default string) string {
	if val, exist := parameters[name]; exist {
		return val.(string)
	}
	return _default
}

// Get a similarity function from parameters.
func (parameters Parameters) GetSim(name string, _default Sim) Sim {
	if val, exist := parameters[name]; exist {
		return val.(Sim)
	}
	return _default
}

/* Base */

// Base structure of all estimators.
type Base struct {
	Params Parameters
	Data   TrainSet
}

func (base *Base) SetParams(params Parameters) {
	base.Params = params
}

func (base *Base) Predict(userId, itemId int) float64 {
	panic("Predict() not implemented")
}

func (base *Base) Fit(trainSet TrainSet) {
	panic("Fit() not implemented")
}

/* Random */

// Algorithm predicting a random rating based on the distribution of
// the training set, which is assumed to be normal. The prediction
// \hat{r}_{ui} is generated from a normal distribution N(\hat{μ},\hat{σ}^2)
// where \hat{μ} and \hat{σ}^2 are estimated from the training data
// using Maximum Likelihood Estimation
type Random struct {
	Base
	Mean   float64 // mu
	StdDev float64 // sigma
	Low    float64 // The lower bound of rating scores
	High   float64 // The upper bound of rating scores
}

// Create a random model.
func NewRandom(params Parameters) *Random {
	random := new(Random)
	random.Params = params
	return random
}

func (random *Random) Predict(userId int, itemId int) float64 {
	ret := rand.NormFloat64()*random.StdDev + random.Mean
	// Crop prediction
	if ret < random.Low {
		ret = random.Low
	} else if ret > random.High {
		ret = random.High
	}
	return ret
}

func (random *Random) Fit(trainSet TrainSet) {
	ratings := trainSet.Ratings
	random.Mean = trainSet.GlobalMean
	random.StdDev = stat.StdDev(ratings, nil)
	random.Low, random.High = trainSet.RatingRange()
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

// Create a baseline model. Parameters:
//	 reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.02.
//	 lr 		- The learning rate of SGD. Default is 0.005.
//	 nEpochs	- The number of iteration of the SGD procedure. Default is 20.
func NewBaseLine(params Parameters) *BaseLine {
	baseLine := new(BaseLine)
	baseLine.Params = params
	return baseLine
}

func (baseLine *BaseLine) Predict(userId, itemId int) float64 {
	// Convert to inner Id
	innerUserId := baseLine.Data.ConvertUserId(userId)
	innerItemId := baseLine.Data.ConvertItemId(itemId)
	ret := baseLine.GlobalBias
	if innerUserId != NewId {
		ret += baseLine.UserBias[innerUserId]
	}
	if innerItemId != NewId {
		ret += baseLine.ItemBias[innerItemId]
	}
	return ret
}

func (baseLine *BaseLine) Fit(trainSet TrainSet) {
	// Setup parameters
	reg := baseLine.Params.GetFloat64("reg", 0.02)
	lr := baseLine.Params.GetFloat64("lr", 0.005)
	nEpochs := baseLine.Params.GetInt("nEpochs", 20)
	// Initialize parameters
	baseLine.Data = trainSet
	baseLine.UserBias = make([]float64, trainSet.UserCount)
	baseLine.ItemBias = make([]float64, trainSet.ItemCount)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < nEpochs; epoch++ {
		for i := 0; i < trainSet.Length(); i++ {
			userId, itemId, rating := trainSet.Users[i], trainSet.Items[i], trainSet.Ratings[i]
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
