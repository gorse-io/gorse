// The famous SVD algorithm, as popularized by Simon Funk during the
// Netflix Prize. When baselines are not used, this is equivalent to
// Probabilistic Matrix Factorization [SM08] (see note below). The
// prediction r^ui is set as:
//               \hat{r}_{ui} = μ + b_u + b_i + q_i^Tp_u
// If user u is unknown, then the bias bu and the factors pu are
// assumed to be zero. The same applies for item i with bi and qi.

package algo

import (
	"../data"
	"github.com/gonum/floats"
)

type SVD struct {
	userFactor map[int][]float64
	itemFactor map[int][]float64
	userBias   map[int]float64
	itemBias   map[int]float64
	globalBias float64
}

func NewSVDRecommend() *SVD {
	return &SVD{}
}

func (svd *SVD) Predict(userId int, itemId int) float64 {
	userFactor, _ := svd.userFactor[userId]
	itemFactor, _ := svd.itemFactor[itemId]
	dot := .0
	if len(userFactor) == len(itemFactor) {
		dot = floats.Dot(userFactor, itemFactor)
	}
	userBias, _ := svd.userBias[userId]
	itemBias, _ := svd.itemBias[itemId]
	return svd.globalBias + userBias + itemBias + dot
}

func (svd *SVD) Fit(trainSet data.Set, options ...OptionEditor) {
	// Setup options
	option := Option{
		nFactors:       100,
		nEpoch:         20,
		learningRate:   0.005,
		regularization: 0.02,
	}
	for _, editor := range options {
		editor(&option)
	}
	// Initialize parameters
	svd.userBias = make(map[int]float64)
	svd.itemBias = make(map[int]float64)
	svd.userFactor = make(map[int][]float64)
	svd.itemFactor = make(map[int][]float64)
	for _, userId := range trainSet.AllUsers() {
		svd.userBias[userId] = 0
		svd.userFactor[userId] = make([]float64, option.nFactors)
	}
	for _, itemId := range trainSet.AllItems() {
		svd.itemBias[itemId] = 0
		svd.itemFactor[itemId] = make([]float64, option.nFactors)
	}
	// Stochastic Gradient Descent
	users, items, ratings := trainSet.AllInteraction()
	buffer := make([]float64, option.nFactors)
	for epoch := 0; epoch < option.nEpoch; epoch++ {
		for i := 0; i < trainSet.NRow(); i++ {
			userId := users[i]
			itemId := items[i]
			rating := ratings[i]
			userBias, _ := svd.userBias[userId]
			itemBias, _ := svd.itemBias[itemId]
			userFactor, _ := svd.userFactor[userId]
			itemFactor, _ := svd.itemFactor[itemId]
			// Compute error
			diff := rating - svd.globalBias - userBias - itemBias - floats.Dot(userFactor, itemFactor)
			// Update global bias
			gradGlobalBias := -2 * diff
			svd.globalBias -= option.learningRate * gradGlobalBias
			// Update user bias
			gradUserBias := -2*diff + 2*option.regularization*userBias
			svd.userBias[userId] -= option.learningRate * gradUserBias
			// Update item bias
			gradItemBias := -2*diff + 2*option.regularization*itemBias
			svd.itemBias[itemId] -= option.learningRate * gradItemBias
			// Update user latent factor
			gradUserFactor := Copy(buffer, userFactor)
			floats.Sub(MulConst(-2*diff, itemFactor), MulConst(2*option.regularization, userFactor))
			floats.Sub(svd.userFactor[userId], MulConst(option.learningRate, gradUserFactor))
			// Update item latent factor
			gradItemFactor := Copy(buffer, itemFactor)
			floats.Sub(MulConst(-2*diff, userFactor), MulConst(2*option.regularization, itemFactor))
			floats.Sub(svd.itemFactor[itemId], MulConst(option.learningRate, gradItemFactor))
		}
	}
}
