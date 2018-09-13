package core

import (
	"github.com/gonum/stat"
	"math/rand"
)

/* Base */

// An algorithm interface to predict ratings. Any estimator in this
// package should implement it.
type Estimator interface {
	// Predict the rating given by a user (userId) to a item (itemId).
	Predict(userId, itemId int) float64
	// Fit a model with a train set and parameters.
	Fit(trainSet TrainSet, params Parameters)
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

// Get a similarity function from parameters.
func (parameters Parameters) GetSim(name string, _default Sim) Sim {
	if val, exist := parameters[name]; exist {
		return val.(Sim)
	}
	return _default
}

/* Random */

// Algorithm predicting a random rating based on the distribution of
// the training set, which is assumed to be normal. The prediction
// \hat{r}_{ui} is generated from a normal distribution N(\hat{μ},\hat{σ}^2)
// where \hat{μ} and \hat{σ}^2 are estimated from the training data
// using Maximum Likelihood Estimation
type Random struct {
	mean   float64 // mu
	stdDev float64 // sigma
	low    float64 // The lower bound of rating scores
	high   float64 // The upper bound of rating scores
}

func NewRandom() *Random {
	return new(Random)
}

func (random *Random) Predict(userId int, itemId int) float64 {
	ret := rand.NormFloat64()*random.stdDev + random.mean
	// Crop prediction
	if ret < random.low {
		ret = random.low
	} else if ret > random.high {
		ret = random.high
	}
	return ret
}

func (random *Random) Fit(trainSet TrainSet, params Parameters) {
	ratings := trainSet.Ratings
	random.mean = stat.Mean(ratings, nil)
	random.stdDev = stat.StdDev(ratings, nil)
	random.low, random.high = trainSet.RatingRange()
}

/* Baseline */

// Algorithm predicting the baseline estimate for given user and item.
//
//                   \hat{r}_{ui} = b_{ui} = μ + b_u + b_i
//
// If user u is unknown, then the bias b_u is assumed to be zero. The same
// applies for item i with b_i.
type BaseLine struct {
	userBias   []float64 // b_u
	itemBias   []float64 // b_i
	globalBias float64   // mu
	trainSet   TrainSet
}

func NewBaseLine() *BaseLine {
	return new(BaseLine)
}

func (baseLine *BaseLine) Predict(userId, itemId int) float64 {
	// Convert to inner Id
	innerUserId := baseLine.trainSet.ConvertUserId(userId)
	innerItemId := baseLine.trainSet.ConvertItemId(itemId)
	ret := baseLine.globalBias
	if innerUserId != newId {
		ret += baseLine.userBias[innerUserId]
	}
	if innerItemId != newId {
		ret += baseLine.itemBias[innerItemId]
	}
	return ret
}

// Fit a baseline model.
// Parameters:
//	 reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.02.
//	 lr 		- The learning rate of SGD. Default is 0.005.
//	 nEpochs	- The number of iteration of the SGD procedure. Default is 20.
func (baseLine *BaseLine) Fit(trainSet TrainSet, params Parameters) {
	// Setup parameters
	reg := params.GetFloat64("reg", 0.02)
	lr := params.GetFloat64("lr", 0.005)
	nEpochs := params.GetInt("nEpochs", 20)
	// Initialize parameters
	baseLine.trainSet = trainSet
	baseLine.userBias = make([]float64, trainSet.UserCount)
	baseLine.itemBias = make([]float64, trainSet.ItemCount)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < nEpochs; epoch++ {
		for i := 0; i < trainSet.Length(); i++ {
			userId, itemId, rating := trainSet.Users[i], trainSet.Items[i], trainSet.Ratings[i]
			innerUserId := trainSet.ConvertUserId(userId)
			innerItemId := trainSet.ConvertItemId(itemId)
			userBias := baseLine.userBias[innerUserId]
			itemBias := baseLine.itemBias[innerItemId]
			// Compute gradient
			diff := baseLine.Predict(userId, itemId) - rating
			gradGlobalBias := diff
			gradUserBias := diff + reg*userBias
			gradItemBias := diff + reg*itemBias
			// Update parameters
			baseLine.globalBias -= lr * gradGlobalBias
			baseLine.userBias[innerUserId] -= lr * gradUserBias
			baseLine.itemBias[innerItemId] -= lr * gradItemBias
		}
	}
}
