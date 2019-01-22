package model

import "github.com/zhenghaoz/gorse/core"
import "github.com/zhenghaoz/gorse/base"

/* Base */

// Base model must be included by every recommendation model. Hyper-parameters,
// ID sets, random generator and fitting options are managed the Base model.
type Base struct {
	Params     base.Params          // Hyper-parameters
	UserIdSet  *base.SparseIdSet    // Users' ID set
	ItemIdSet  *base.SparseIdSet    // Items' ID set
	rng        base.RandomGenerator // Random generator
	randState  int64                // Random seed
	fitOptions *core.FitOptions     // Fit options
	// Tracker
	isSetParamsCalled bool // Check whether SetParams called
}

// SetParams sets hyper-parameters for the Base model.
func (model *Base) SetParams(params base.Params) {
	model.isSetParamsCalled = true
	model.Params = params
	model.randState = model.Params.GetInt64(base.RandomState, 0)
}

// GetParams returns all hyper-parameters.
func (model *Base) GetParams() base.Params {
	return model.Params
}

// Predict has not been implemented.
func (model *Base) Predict(userId, itemId int) float64 {
	panic("Predict() not implemented")
}

// Fit has not been implemented,
func (model *Base) Fit(trainSet core.DataSet, options ...core.FitOption) {
	panic("Fit() not implemented")
}

// Init the Base model. The method must be called at the beginning of Fit.
func (model *Base) Init(trainSet *core.DataSet, options []core.FitOption) {
	// Check Base.GetParams() called
	if model.isSetParamsCalled == false {
		panic("Base.GetParams() not called")
	}
	// Setup ID set
	model.UserIdSet = trainSet.UserIdSet
	model.ItemIdSet = trainSet.ItemIdSet
	// Setup random state
	model.rng = base.NewRandomGenerator(model.randState)
	// Setup runtime options
	model.fitOptions = core.NewFitOptions(options)
}

// BaseLine predicts the rating for given user and item by
//  \hat{r}_{ui} = b_{ui} = μ + b_u + b_i
// If user u is unknown, then the Bias b_u is assumed to be zero. The same
// applies for item i with b_i. Hyper-parameters:
//  Reg         - The regularization parameter of the cost function that is
//              optimized. Default is 0.02.
//  Lr          - The learning rate of SGD. Default is 0.005.
//  NEpochs     - The number of iteration of the SGD procedure. Default is 20.
//  RandomState - The random seed. Default is 0.
type BaseLine struct {
	Base
	UserBias   []float64 // b_u
	ItemBias   []float64 // b_i
	GlobalBias float64   // mu
	// Hyper-parameters
	reg     float64
	lr      float64
	nEpochs int
}

// NewBaseLine creates a baseline model.
func NewBaseLine(params base.Params) *BaseLine {
	baseLine := new(BaseLine)
	baseLine.SetParams(params)
	return baseLine
}

// SetParams sets hyper-parameters for the BaseLine model.
func (baseLine *BaseLine) SetParams(params base.Params) {
	baseLine.Base.SetParams(params)
	// Setup parameters
	baseLine.reg = baseLine.Params.GetFloat64(base.Reg, 0.02)
	baseLine.lr = baseLine.Params.GetFloat64(base.Lr, 0.005)
	baseLine.nEpochs = baseLine.Params.GetInt(base.NEpochs, 20)
}

// Predict by the BaseLine model.
func (baseLine *BaseLine) Predict(userId, itemId int) float64 {
	denseUserId := baseLine.UserIdSet.ToDenseId(userId)
	denseItemId := baseLine.ItemIdSet.ToDenseId(itemId)
	return baseLine.predict(denseUserId, denseItemId)
}

func (baseLine *BaseLine) predict(denseUserId, denseItemId int) float64 {
	ret := baseLine.GlobalBias
	if denseUserId != base.NotId {
		ret += baseLine.UserBias[denseUserId]
	}
	if denseItemId != base.NotId {
		ret += baseLine.ItemBias[denseItemId]
	}
	return ret
}

// Fit the BaseLine model.
func (baseLine *BaseLine) Fit(trainSet *core.DataSet, options ...core.FitOption) {
	baseLine.Init(trainSet, options)
	// Initialize parameters
	baseLine.GlobalBias = trainSet.GlobalMean
	baseLine.UserBias = make([]float64, trainSet.UserCount())
	baseLine.ItemBias = make([]float64, trainSet.ItemCount())
	// Stochastic Gradient Descent
	for epoch := 0; epoch < baseLine.nEpochs; epoch++ {
		for i := 0; i < trainSet.Len(); i++ {
			denseUserId, denseItemId, rating := trainSet.GetDense(i)
			userBias := baseLine.UserBias[denseUserId]
			itemBias := baseLine.ItemBias[denseItemId]
			// Compute gradient
			diff := baseLine.predict(denseUserId, denseItemId) - rating
			gradUserBias := diff + baseLine.reg*userBias
			gradItemBias := diff + baseLine.reg*itemBias
			// Update parameters
			baseLine.UserBias[denseUserId] -= baseLine.lr * gradUserBias
			baseLine.ItemBias[denseItemId] -= baseLine.lr * gradItemBias
		}
	}
}

// ItemPop recommends items by their popularity. The popularity of a item is
// defined as the occurrence frequency of the item in the training data set.
type ItemPop struct {
	Base
	Pop []float64
}

// NewItemPop creates an ItemPop model.
func NewItemPop(params base.Params) *ItemPop {
	pop := new(ItemPop)
	pop.SetParams(params)
	return pop
}

// Fit the ItemPop model.
func (pop *ItemPop) Fit(set *core.DataSet, options ...core.FitOption) {
	pop.Init(set, options)
	// Get items' popularity
	pop.Pop = make([]float64, set.ItemCount())
	for i := range set.DenseItemRatings {
		pop.Pop[i] = float64(set.DenseItemRatings[i].Len())
	}
}

// Predict by the ItemPop model.
func (pop *ItemPop) Predict(userId, itemId int) float64 {
	// Return items' popularity
	denseItemId := pop.ItemIdSet.ToDenseId(itemId)
	if denseItemId == base.NotId {
		return 0
	}
	return pop.Pop[denseItemId]
}
