// The famous SVD algorithm, as popularized by Simon Funk during the
// Netflix Prize. When baselines are not used, this is equivalent to
// Probabilistic Matrix Factorization [SM08] (see note below). The
// prediction r^ui is set as:
//               \hat{r}_{ui} = μ + b_u + b_i + q_i^Tp_u
// If user u is unknown, then the bias bu and the factors pu are
// assumed to be zero. The same applies for item i with bi and qi.

package core

import (
	"github.com/gonum/floats"
)

type SVD struct {
	userFactor [][]float64 // p_u
	itemFactor [][]float64 // q_i
	userBias   []float64   // b_u
	itemBias   []float64   // b_i
	globalBias float64     // mu
	trainSet   TrainSet
}

func NewSVD() *SVD {
	return new(SVD)
}

func (svd *SVD) Predict(userId int, itemId int) float64 {
	innerUserId := svd.trainSet.ConvertUserId(userId)
	innerItemId := svd.trainSet.ConvertItemId(itemId)
	ret := svd.globalBias
	// + b_u
	if innerUserId != noBody {
		ret += svd.userBias[innerUserId]
	}
	// + b_i
	if innerItemId != noBody {
		ret += svd.itemBias[innerItemId]
	}
	// + q_i^Tp_u
	if innerItemId != noBody && innerUserId != noBody {
		userFactor := svd.userFactor[innerUserId]
		itemFactor := svd.itemFactor[innerItemId]
		ret += floats.Dot(userFactor, itemFactor)
	}
	return ret
}

func (svd *SVD) Fit(trainSet TrainSet, options ...OptionSetter) {
	// Setup options
	option := Option{
		nFactors:   100,
		nEpochs:    20,
		lr:         0.005,
		reg:        0.02,
		biased:     true,
		initMean:   0,
		initStdDev: 0.1,
	}
	for _, editor := range options {
		editor(&option)
	}
	// Initialize parameters
	svd.trainSet = trainSet
	svd.userBias = make([]float64, trainSet.UserCount())
	svd.itemBias = make([]float64, trainSet.ItemCount())
	svd.userFactor = make([][]float64, trainSet.UserCount())
	svd.itemFactor = make([][]float64, trainSet.ItemCount())
	for innerUserId := range svd.userFactor {
		svd.userFactor[innerUserId] = newNormalVector(option.nFactors, option.initMean, option.initStdDev)
	}
	for innerItemId := range svd.itemFactor {
		svd.itemFactor[innerItemId] = newNormalVector(option.nFactors, option.initMean, option.initStdDev)
	}
	// Create buffers
	a := make([]float64, option.nFactors)
	b := make([]float64, option.nFactors)
	// Stochastic Gradient Descent
	users, items, ratings := trainSet.Interactions()
	for epoch := 0; epoch < option.nEpochs; epoch++ {
		for i := 0; i < trainSet.Length(); i++ {
			userId, itemId, rating := users[i], items[i], ratings[i]
			innerUserId := trainSet.ConvertUserId(userId)
			innerItemId := trainSet.ConvertItemId(itemId)
			userBias := svd.userBias[innerUserId]
			itemBias := svd.itemBias[innerItemId]
			userFactor := svd.userFactor[innerUserId]
			itemFactor := svd.itemFactor[innerItemId]
			// Compute error
			diff := svd.Predict(userId, itemId) - rating
			// Update global bias
			gradGlobalBias := diff
			svd.globalBias -= option.lr * gradGlobalBias
			// Update user bias
			gradUserBias := diff + option.reg*userBias
			svd.userBias[innerUserId] -= option.lr * gradUserBias
			// Update item bias
			gradItemBias := diff + option.reg*itemBias
			svd.itemBias[innerItemId] -= option.lr * gradItemBias
			// Update user latent factor
			copy(a, itemFactor)
			mulConst(diff, a)
			copy(b, userFactor)
			mulConst(option.reg, b)
			floats.Add(a, b)
			mulConst(option.lr, a)
			floats.Sub(svd.userFactor[innerUserId], a)
			// Update item latent factor
			copy(a, userFactor)
			mulConst(diff, a)
			copy(b, itemFactor)
			mulConst(option.reg, b)
			floats.Add(a, b)
			mulConst(option.lr, a)
			floats.Sub(svd.itemFactor[innerItemId], a)
		}
	}
}
