package core

import (
	"github.com/gonum/floats"
	"math"
)

// The SVD++ algorithm, an extension of SVD taking into account implicit
// interactionRatings. The prediction \hat{r}_{ui} is set as:
//
// 	\hat{r}_{ui} = \mu + b_u + b_i + q_i^T\left(p_u + |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right)
//
// Where the y_j terms are a new set of item factors that capture implicit
// interactionRatings. Here, an implicit rating describes the fact that a user u
// userHistory an item j, regardless of the rating value. If user u is unknown,
// then the bias b_u and the factors p_u are assumed to be zero. The same
// applies for item i with b_i, q_i and y_i.
type SVDpp struct {
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

func NewSVDpp() *SVDpp {
	return new(SVDpp)
}

func (svd *SVDpp) ensembleImplFactors(innerUserId int) []float64 {
	history := svd.userHistory[innerUserId]
	emImpFactor := make([]float64, 0)
	// User history exists
	for itemId := range history {
		if len(emImpFactor) == 0 {
			// Create ensemble implicit factor
			emImpFactor = make([]float64, len(svd.implFactor[itemId]))
		}
		floats.Add(emImpFactor, svd.implFactor[itemId])
	}
	divConst(math.Sqrt(float64(len(history))), emImpFactor)
	return emImpFactor
}

func (svd *SVDpp) internalPredict(userId int, itemId int) (float64, []float64) {
	// Convert to inner Id
	innerUserId := svd.trainSet.ConvertUserId(userId)
	innerItemId := svd.trainSet.ConvertItemId(itemId)
	ret := svd.globalBias
	// + b_u
	if innerUserId != newId {
		ret += svd.userBias[innerUserId]
	}
	// + b_i
	if innerItemId != newId {
		ret += svd.itemBias[innerItemId]
	}
	// + q_i^T\left(p_u + |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right)
	if innerItemId != newId && innerUserId != newId {
		userFactor := svd.userFactor[innerUserId]
		itemFactor := svd.itemFactor[innerItemId]
		emImpFactor := svd.ensembleImplFactors(innerUserId)
		temp := make([]float64, len(itemFactor))
		floats.Add(temp, userFactor)
		floats.Add(temp, emImpFactor)
		ret = floats.Dot(temp, itemFactor)
		return ret, emImpFactor
	}
	return ret, []float64{}
}

func (svd *SVDpp) Predict(userId int, itemId int) float64 {
	ret, _ := svd.internalPredict(userId, itemId)
	return ret
}

// Fit a SVD++ model.
// Parameters:
//	 reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.02.
//	 lr 		- The learning rate of SGD. Default is 0.007.
//	 nFactors	- The number of latent factors. Default is 20.
//	 nEpochs	- The number of iteration of the SGD procedure. Default is 20.
//	 initMean	- The mean of initial random latent factors. Default is 0.
//	 initStdDev	- The standard deviation of initial random latent factors. Default is 0.1.
func (svd *SVDpp) Fit(trainSet TrainSet, params Parameters) {
	// Setup parameters
	reader := newParameterReader(params)
	nFactors := reader.getInt("nFactors", 20)
	nEpochs := reader.getInt("nEpochs", 20)
	lr := reader.getFloat64("lr", 0.007)
	reg := reader.getFloat64("reg", 0.02)
	initMean := reader.getFloat64("initMean", 0)
	initStdDev := reader.getFloat64("initStdDev", 0.1)
	// Initialize parameters
	svd.trainSet = trainSet
	svd.userBias = make([]float64, trainSet.UserCount())
	svd.itemBias = make([]float64, trainSet.ItemCount())
	svd.userFactor = make([][]float64, trainSet.UserCount())
	svd.itemFactor = make([][]float64, trainSet.ItemCount())
	svd.implFactor = make([][]float64, trainSet.ItemCount())
	//svd.cacheFactor = make(map[int][]float64)
	for innerUserId := range svd.userBias {
		svd.userFactor[innerUserId] = newNormalVector(nFactors, initMean, initStdDev)
	}
	for innerItemId := range svd.itemBias {
		svd.itemFactor[innerItemId] = newNormalVector(nFactors, initMean, initStdDev)
		svd.implFactor[innerItemId] = newNormalVector(nFactors, initMean, initStdDev)
	}
	// Build user rating set
	svd.userHistory = trainSet.UserRatings()
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
			userBias := svd.userBias[innerUserId]
			itemBias := svd.itemBias[innerItemId]
			userFactor := svd.userFactor[innerUserId]
			itemFactor := svd.itemFactor[innerItemId]
			// Compute error
			pred, emImpFactor := svd.internalPredict(userId, itemId)
			diff := pred - rating
			// Update global bias
			gradGlobalBias := diff
			svd.globalBias -= lr * gradGlobalBias
			// Update user bias
			gradUserBias := diff + reg*userBias
			svd.userBias[innerUserId] -= lr * gradUserBias
			// Update item bias
			gradItemBias := diff + reg*itemBias
			svd.itemBias[innerItemId] -= lr * gradItemBias
			// Update user latent factor
			copy(a, itemFactor)
			mulConst(diff, a)
			copy(b, userFactor)
			mulConst(reg, b)
			floats.Add(a, b)
			mulConst(lr, a)
			floats.Sub(svd.userFactor[innerUserId], a)
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
			floats.Sub(svd.itemFactor[innerItemId], a)
			// Update implicit latent factor
			set := svd.userHistory[innerUserId]
			for itemId := range set {
				if !math.IsNaN(svd.userHistory[innerUserId][itemId]) {
					implFactor := svd.implFactor[itemId]
					copy(a, itemFactor)
					mulConst(diff, a)
					divConst(math.Sqrt(float64(len(set))), a)
					copy(b, implFactor)
					mulConst(reg, b)
					floats.Add(a, b)
					mulConst(lr, a)
					floats.Sub(svd.implFactor[itemId], a)
				}
			}
		}
	}
}
