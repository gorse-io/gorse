package core

import "gonum.org/v1/gonum/floats"

type FM struct {
	Base
	UserFactor [][]float64
	ItemFactor [][]float64
}

func NewFM(params Parameters) *FM {
	fm := new(FM)
	fm.SetParams(params)
	return fm
}

func (fm *FM) Predict(userId, itemId int) float64 {
	innerUserId := fm.Data.ConvertUserId(userId)
	innerItemId := fm.Data.ConvertItemId(itemId)
	if innerUserId == NewId || innerItemId == NewId {
		return 0
	}
	return floats.Dot(fm.UserFactor[innerUserId], fm.ItemFactor[innerItemId])
}

func (fm *FM) Fit(set TrainSet) {
	fm.Base.Fit(set)
	// Setup parameters
	nFactors := fm.Params.GetInt("nFactors", 100)
	nEpochs := fm.Params.GetInt("nEpochs", 20)
	lr := fm.Params.GetFloat64("lr", 0.005)
	reg := fm.Params.GetFloat64("reg", 0.02)
	initMean := fm.Params.GetFloat64("initMean", 0)
	initStdDev := fm.Params.GetFloat64("initStdDev", 0.1)
	// Initialize parameters
	fm.UserFactor = fm.newNormalMatrix(set.UserCount, nFactors, initMean, initStdDev)
	fm.ItemFactor = fm.newNormalMatrix(set.ItemCount, nFactors, initMean, initStdDev)
	a := make([]float64, nFactors)
	b := make([]float64, nFactors)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < nEpochs; epoch++ {
		for i := 0; i < set.Length(); i++ {
			userId, itemId, rating := set.Index(i)
			innerUserId := set.ConvertUserId(userId)
			innerItemId := set.ConvertItemId(itemId)
			//userBias := svd.UserBias[innerUserId]
			//itemBias := svd.ItemBias[innerItemId]
			userFactor := fm.UserFactor[innerUserId]
			itemFactor := fm.ItemFactor[innerItemId]
			//// Compute error
			diff := fm.Predict(userId, itemId) - rating
			//// Update global Bias
			//gradGlobalBias := diff
			//fm.GlobalBias -= lr * gradGlobalBias
			//// Update user Bias
			//gradUserBias := diff + reg*userBias
			//fm.UserBias[innerUserId] -= lr * gradUserBias
			//// Update item Bias
			//gradItemBias := diff + reg*itemBias
			//fm.ItemBias[innerItemId] -= lr * gradItemBias
			// Update user latent factor
			copy(a, itemFactor)
			mulConst(diff, a)
			copy(b, userFactor)
			mulConst(reg, b)
			floats.Add(a, b)
			mulConst(lr, a)
			floats.Sub(fm.UserFactor[innerUserId], a)
			// Update item latent factor
			copy(a, userFactor)
			mulConst(diff, a)
			copy(b, itemFactor)
			mulConst(reg, b)
			floats.Add(a, b)
			mulConst(lr, a)
			floats.Sub(fm.ItemFactor[innerItemId], a)
		}
	}
}
