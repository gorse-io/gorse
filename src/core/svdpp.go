// The SVD++ algorithm, an extension of SVD taking into account implicit
// interactionRatings. The prediction \hat{r}_{ui} is set as:
//
// \hat{r}_{ui} = \mu + b_u + b_i + q_i^T\left(p_u + |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right)
//
// Where the y_j terms are a new set of item factors that capture implicit
// interactionRatings. Here, an implicit rating describes the fact that a user u
// userHistory an item j, regardless of the rating value. If user u is unknown,
// then the bias b_u and the factors p_u are assumed to be zero. The same
// applies for item i with b_i, q_i and y_i.

package core

import (
	"github.com/gonum/floats"
	"math"
)

type SVDPP struct {
	userHistory [][]float64 // I_u
	userFactor  [][]float64 // p_u
	itemFactor  [][]float64 // q_i
	implFactor  [][]float64 // y_i
	userBias    []float64   // b_u
	itemBias    []float64   // b_i
	globalBias  float64     // mu
	trainSet    TrainSet
	// Optimization with cache
	//cacheFailed bool
	//cacheFactor map[int][]float64 // |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right
}

func NewSVDPP() *SVDPP {
	return new(SVDPP)
}

func (svdpp *SVDPP) EnsembleImplFactors(innerUserId int) []float64 {
	history := svdpp.userHistory[innerUserId]
	emImpFactor := make([]float64, 0)
	// User history exists
	for itemId := range history {
		if len(emImpFactor) == 0 {
			// Create ensemble implicit factor
			emImpFactor = make([]float64, len(svdpp.implFactor[itemId]))
		}
		floats.Add(emImpFactor, svdpp.implFactor[itemId])
	}
	divConst(math.Sqrt(float64(len(history))), emImpFactor)
	return emImpFactor
}

func (svdpp *SVDPP) InternalPredict(userId int, itemId int) (float64, []float64) {
	// Convert to inner Id
	innerUserId := svdpp.trainSet.ConvertUserId(userId)
	innerItemId := svdpp.trainSet.ConvertItemId(itemId)
	ret := svdpp.globalBias
	// + b_u
	if innerUserId != newId {
		ret += svdpp.userBias[innerUserId]
	}
	// + b_i
	if innerItemId != newId {
		ret += svdpp.itemBias[innerItemId]
	}
	// + q_i^T\left(p_u + |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right)
	if innerItemId != newId && innerUserId != newId {
		userFactor := svdpp.userFactor[innerUserId]
		itemFactor := svdpp.itemFactor[innerItemId]
		emImpFactor := svdpp.EnsembleImplFactors(innerUserId)
		temp := make([]float64, len(itemFactor))
		floats.Add(temp, userFactor)
		floats.Add(temp, emImpFactor)
		ret = floats.Dot(temp, itemFactor)
		return ret, emImpFactor
	}
	return ret, []float64{}
}

func (svdpp *SVDPP) Predict(userId int, itemId int) float64 {
	ret, _ := svdpp.InternalPredict(userId, itemId)
	return ret
}

func (svdpp *SVDPP) Fit(trainSet TrainSet, options Options) {
	// Setup options
	nFactors := options.GetInt("nFactors", 20)
	nEpochs := options.GetInt("nEpochs", 20)
	lr := options.GetFloat64("lr", 0.007)
	reg := options.GetFloat64("reg", 0.02)
	initMean := options.GetFloat64("initMean", 0)
	initStdDev := options.GetFloat64("initStdDev", 0.1)
	// Initialize parameters
	svdpp.trainSet = trainSet
	svdpp.userBias = make([]float64, trainSet.UserCount())
	svdpp.itemBias = make([]float64, trainSet.ItemCount())
	svdpp.userFactor = make([][]float64, trainSet.UserCount())
	svdpp.itemFactor = make([][]float64, trainSet.ItemCount())
	svdpp.implFactor = make([][]float64, trainSet.ItemCount())
	//svdpp.cacheFactor = make(map[int][]float64)
	for innerUserId := range svdpp.userBias {
		svdpp.userFactor[innerUserId] = newNormalVector(nFactors, initMean, initStdDev)
	}
	for innerItemId := range svdpp.itemBias {
		svdpp.itemFactor[innerItemId] = newNormalVector(nFactors, initMean, initStdDev)
		svdpp.implFactor[innerItemId] = newNormalVector(nFactors, initMean, initStdDev)
	}
	// Build user rating set
	svdpp.userHistory = trainSet.UserRatings()
	// Create buffers
	a := make([]float64, nFactors)
	b := make([]float64, nFactors)
	// Stochastic Gradient Descent
	users, items, ratings := trainSet.Interactions()
	for epoch := 0; epoch < nEpochs; epoch++ {
		for i := 0; i < trainSet.Length(); i++ {
			userId, itemId, rating := users[i], items[i], ratings[i]
			innerUserId := trainSet.ConvertUserId(userId)
			innerItemId := trainSet.ConvertItemId(itemId)
			userBias := svdpp.userBias[innerUserId]
			itemBias := svdpp.itemBias[innerItemId]
			userFactor := svdpp.userFactor[innerUserId]
			itemFactor := svdpp.itemFactor[innerItemId]
			// Compute error
			pred, emImpFactor := svdpp.InternalPredict(userId, itemId)
			diff := pred - rating
			// Update global bias
			gradGlobalBias := diff
			svdpp.globalBias -= lr * gradGlobalBias
			// Update user bias
			gradUserBias := diff + reg*userBias
			svdpp.userBias[innerUserId] -= lr * gradUserBias
			// Update item bias
			gradItemBias := diff + reg*itemBias
			svdpp.itemBias[innerItemId] -= lr * gradItemBias
			// Update user latent factor
			copy(a, itemFactor)
			mulConst(diff, a)
			copy(b, userFactor)
			mulConst(reg, b)
			floats.Add(a, b)
			mulConst(lr, a)
			floats.Sub(svdpp.userFactor[innerUserId], a)
			// Update item latent factor
			copy(a, userFactor)
			if len(emImpFactor) > 0 {
				floats.Add(a, emImpFactor)
			}
			mulConst(diff, a)
			copy(b, itemFactor)
			mulConst(reg, b)
			floats.Add(a, b)
			mulConst(lr, a)
			floats.Sub(svdpp.itemFactor[innerItemId], a)
			// Update implicit latent factor
			set := svdpp.userHistory[innerUserId]
			for itemId := range set {
				if !math.IsNaN(svdpp.userHistory[innerUserId][itemId]) {
					implFactor := svdpp.implFactor[itemId]
					copy(a, itemFactor)
					mulConst(diff, a)
					divConst(math.Sqrt(float64(len(set))), a)
					copy(b, implFactor)
					mulConst(reg, b)
					floats.Add(a, b)
					mulConst(lr, a)
					floats.Sub(svdpp.implFactor[itemId], a)
				}
			}
		}
	}
}
