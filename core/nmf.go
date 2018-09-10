package core

import "github.com/gonum/floats"

// A collaborative filtering algorithm based on Non-negative
// Matrix Factorization[1].
//
// [1] Luo, Xin, et al. "An efficient non-negative matrix-
// factorization-based approach to collaborative filtering
// for recommender systems." IEEE Transactions on Industrial
// Informatics 10.2 (2014): 1273-1284.
type NMF struct {
	userFactor [][]float64 // p_u
	itemFactor [][]float64 // q_i
	trainSet   TrainSet
}

func NewNMF() *NMF {
	return new(NMF)
}

func (nmf *NMF) Predict(userId int, itemId int) float64 {
	innerUserId := nmf.trainSet.ConvertUserId(userId)
	innerItemId := nmf.trainSet.ConvertItemId(itemId)
	if innerItemId != newId && innerUserId != newId {
		return floats.Dot(nmf.userFactor[innerUserId], nmf.itemFactor[innerItemId])
	}
	return 0
}

// Fit a NMF model.
// Parameters:
//	 reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.06.
//	 nFactors	- The number of latent factors. Default is 15.
//	 nEpochs	- The number of iteration of the SGD procedure. Default is 50.
//	 initLow	- The lower bound of initial random latent factor. Default is 0.
//	 initHigh	- The upper bound of initial random latent factor. Default is 1.
func (nmf *NMF) Fit(trainSet TrainSet, params Parameters) {
	reader := newParameterReader(params)
	nFactors := reader.getInt("nFactors", 15)
	nEpochs := reader.getInt("nEpochs", 50)
	initLow := reader.getFloat64("initLow", 0)
	initHigh := reader.getFloat64("initHigh", 1)
	reg := reader.getFloat64("reg", 0.06)
	// Initialize parameters
	nmf.trainSet = trainSet
	nmf.userFactor = newUniformMatrix(trainSet.UserCount(), nFactors, initLow, initHigh)
	nmf.itemFactor = newUniformMatrix(trainSet.ItemCount(), nFactors, initLow, initHigh)
	// Create intermediate matrix buffer
	buffer := make([]float64, nFactors)
	userUp := newZeroMatrix(trainSet.UserCount(), nFactors)
	userDown := newZeroMatrix(trainSet.UserCount(), nFactors)
	itemUp := newZeroMatrix(trainSet.ItemCount(), nFactors)
	itemDown := newZeroMatrix(trainSet.ItemCount(), nFactors)
	// Stochastic Gradient Descent
	users, items, ratings := trainSet.Interactions()
	for epoch := 0; epoch < nEpochs; epoch++ {
		// Reset intermediate matrices
		resetZeroMatrix(userUp)
		resetZeroMatrix(userDown)
		resetZeroMatrix(itemUp)
		resetZeroMatrix(itemDown)
		// Calculate intermediate matrices
		for i := 0; i < len(ratings); i++ {
			userId, itemId, rating := users[i], items[i], ratings[i]
			innerUserId := trainSet.ConvertUserId(userId)
			innerItemId := trainSet.ConvertItemId(itemId)
			prediction := nmf.Predict(userId, itemId)
			// Update userUp
			copy(buffer, nmf.itemFactor[innerItemId])
			mulConst(rating, buffer)
			floats.Add(userUp[innerUserId], buffer)
			// Update userDown
			copy(buffer, nmf.itemFactor[innerItemId])
			mulConst(prediction, buffer)
			floats.Add(userDown[innerUserId], buffer)
			copy(buffer, nmf.userFactor[innerUserId])
			mulConst(reg, buffer)
			floats.Add(userDown[innerUserId], buffer)
			// Update itemUp
			copy(buffer, nmf.userFactor[innerUserId])
			mulConst(rating, buffer)
			floats.Add(itemUp[innerItemId], buffer)
			// Update itemDown
			copy(buffer, nmf.userFactor[innerUserId])
			mulConst(prediction, buffer)
			floats.Add(itemDown[innerItemId], buffer)
			copy(buffer, nmf.itemFactor[innerItemId])
			mulConst(reg, buffer)
			floats.Add(itemDown[innerItemId], buffer)
		}
		// Update user factors
		for u := range nmf.userFactor {
			copy(buffer, userUp[u])
			floats.Div(buffer, userDown[u])
			floats.Mul(nmf.userFactor[u], buffer)
		}
		// Update item factors
		for i := range nmf.itemFactor {
			copy(buffer, itemUp[i])
			floats.Div(buffer, itemDown[i])
			floats.Mul(nmf.itemFactor[i], buffer)
		}
	}
}
