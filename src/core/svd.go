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
	userFactor map[int][]float64 // p_u
	itemFactor map[int][]float64 // q_i
	userBias   map[int]float64   // b_u
	itemBias   map[int]float64   // b_i
	globalBias float64           // mu
}

func NewSVD() *SVD {
	return new(SVD)
}

func (svd *SVD) Predict(userId int, itemId int) float64 {
	userFactor, _ := svd.userFactor[userId]
	itemFactor, _ := svd.itemFactor[itemId]
	product := .0
	if len(userFactor) == len(itemFactor) {
		product = floats.Dot(userFactor, itemFactor)
	}
	userBias, _ := svd.userBias[userId]
	itemBias, _ := svd.itemBias[itemId]
	ret := svd.globalBias + userBias + itemBias + product
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
	svd.userBias = make(map[int]float64)
	svd.itemBias = make(map[int]float64)
	svd.userFactor = make(map[int][]float64)
	svd.itemFactor = make(map[int][]float64)
	for userId := range trainSet.Users() {
		svd.userBias[userId] = 0
		svd.userFactor[userId] = newNormalVector(option.nFactors, option.initMean, option.initStdDev)
	}
	for itemId := range trainSet.Items() {
		svd.itemBias[itemId] = 0
		svd.itemFactor[itemId] = newNormalVector(option.nFactors, option.initMean, option.initStdDev)
	}
	// Create buffers
	a := make([]float64, option.nFactors)
	b := make([]float64, option.nFactors)
	// Stochastic Gradient Descent
	users, items, ratings := trainSet.Interactions()
	for epoch := 0; epoch < option.nEpochs; epoch++ {
		for i := 0; i < trainSet.Length(); i++ {
			userId := users[i]
			itemId := items[i]
			rating := ratings[i]
			userBias, _ := svd.userBias[userId]
			itemBias, _ := svd.itemBias[itemId]
			userFactor, _ := svd.userFactor[userId]
			itemFactor, _ := svd.itemFactor[itemId]
			// Compute error
			diff := svd.Predict(userId, itemId) - rating
			// Update global bias
			gradGlobalBias := diff
			svd.globalBias -= option.lr * gradGlobalBias
			// Update user bias
			gradUserBias := diff + option.reg*userBias
			svd.userBias[userId] -= option.lr * gradUserBias
			// Update item bias
			gradItemBias := diff + option.reg*itemBias
			svd.itemBias[itemId] -= option.lr * gradItemBias
			// Update user latent factor
			copy(a, itemFactor)
			mulConst(diff, a)
			copy(b, userFactor)
			mulConst(option.reg, b)
			floats.Add(a, b)
			mulConst(option.lr, a)
			floats.Sub(svd.userFactor[userId], a)
			// Update item latent factor
			copy(a, userFactor)
			mulConst(diff, a)
			copy(b, itemFactor)
			mulConst(option.reg, b)
			floats.Add(a, b)
			mulConst(option.lr, a)
			floats.Sub(svd.itemFactor[itemId], a)
		}
	}
}
