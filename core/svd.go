package core

import (
	"gonum.org/v1/gonum/floats"
	"math"
)

/* SVD */

// The famous SVD algorithm, as popularized by Simon Funk during the
// Netflix Prize. The prediction \hat{r}_{ui} is set as:
//
//               \hat{r}_{ui} = μ + b_u + b_i + q_i^Tp_u
//
// If user u is unknown, then the bias b_u and the factors p_u are
// assumed to be zero. The same applies for item i with b_i and q_i.
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
	if innerUserId != newId {
		ret += svd.userBias[innerUserId]
	}
	// + b_i
	if innerItemId != newId {
		ret += svd.itemBias[innerItemId]
	}
	// + q_i^Tp_u
	if innerItemId != newId && innerUserId != newId {
		userFactor := svd.userFactor[innerUserId]
		itemFactor := svd.itemFactor[innerItemId]
		ret += floats.Dot(userFactor, itemFactor)
	}
	return ret
}

// Fit a SVD model.
// Parameters:
//	 reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.02.
//	 lr 		- The learning rate of SGD. Default is 0.005.
//	 nFactors	- The number of latent factors. Default is 100.
//	 nEpochs	- The number of iteration of the SGD procedure. Default is 20.
//	 initMean	- The mean of initial random latent factors. Default is 0.
//	 initStdDev	- The standard deviation of initial random latent factors. Default is 0.1.
func (svd *SVD) Fit(trainSet TrainSet, params Parameters) {
	// Setup parameters
	nFactors := params.GetInt("nFactors", 100)
	nEpochs := params.GetInt("nEpochs", 20)
	lr := params.GetFloat64("lr", 0.005)
	reg := params.GetFloat64("reg", 0.02)
	initMean := params.GetFloat64("initMean", 0)
	initStdDev := params.GetFloat64("initStdDev", 0.1)
	// Initialize parameters
	svd.trainSet = trainSet
	svd.userBias = make([]float64, trainSet.UserCount)
	svd.itemBias = make([]float64, trainSet.ItemCount)
	svd.userFactor = make([][]float64, trainSet.UserCount)
	svd.itemFactor = make([][]float64, trainSet.ItemCount)
	for innerUserId := range svd.userFactor {
		svd.userFactor[innerUserId] = newNormalVector(nFactors, initMean, initStdDev)
	}
	for innerItemId := range svd.itemFactor {
		svd.itemFactor[innerItemId] = newNormalVector(nFactors, initMean, initStdDev)
	}
	// Create buffers
	a := make([]float64, nFactors)
	b := make([]float64, nFactors)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < nEpochs; epoch++ {
		for i := 0; i < trainSet.Length(); i++ {
			userId, itemId, rating := trainSet.Index(i)
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
			mulConst(diff, a)
			copy(b, itemFactor)
			mulConst(reg, b)
			floats.Add(a, b)
			mulConst(lr, a)
			floats.Sub(svd.itemFactor[innerItemId], a)
		}
	}
}

/* NMF */

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
	nFactors := params.GetInt("nFactors", 15)
	nEpochs := params.GetInt("nEpochs", 50)
	initLow := params.GetFloat64("initLow", 0)
	initHigh := params.GetFloat64("initHigh", 1)
	reg := params.GetFloat64("reg", 0.06)
	// Initialize parameters
	nmf.trainSet = trainSet
	nmf.userFactor = newUniformMatrix(trainSet.UserCount, nFactors, initLow, initHigh)
	nmf.itemFactor = newUniformMatrix(trainSet.ItemCount, nFactors, initLow, initHigh)
	// Create intermediate matrix buffer
	buffer := make([]float64, nFactors)
	userUp := newZeroMatrix(trainSet.UserCount, nFactors)
	userDown := newZeroMatrix(trainSet.UserCount, nFactors)
	itemUp := newZeroMatrix(trainSet.ItemCount, nFactors)
	itemDown := newZeroMatrix(trainSet.ItemCount, nFactors)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < nEpochs; epoch++ {
		// Reset intermediate matrices
		resetZeroMatrix(userUp)
		resetZeroMatrix(userDown)
		resetZeroMatrix(itemUp)
		resetZeroMatrix(itemDown)
		// Calculate intermediate matrices
		for i := 0; i < trainSet.Length(); i++ {
			userId, itemId, rating := trainSet.Index(i)
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

/* SVD++ */

// The SVD++ algorithm, an extension of SVD taking into account implicit
// interactionRatings. The prediction \hat{r}_{ui} is set as:
//
// 	\hat{r}_{ui} = \mu + b_u + b_i + q_i^T\left(p_u + |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right)
//
// Where the y_j terms are a new set of item factors that capture implicit
// interactionRatings. Here, an implicit rating describes the fact that a user u
// userRatings an item j, regardless of the rating value. If user u is unknown,
// then the bias b_u and the factors p_u are assumed to be zero. The same
// applies for item i with b_i, q_i and y_i.
type SVDpp struct {
	userRatings [][]IdRating // I_u
	userFactor  [][]float64  // p_u
	itemFactor  [][]float64  // q_i
	implFactor  [][]float64  // y_i
	userBias    []float64    // b_u
	itemBias    []float64    // b_i
	globalBias  float64      // mu
	trainSet    TrainSet
	// Optimization with cache
	//cacheFailed bool
	//cacheFactor map[int][]float64 // |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right
}

func NewSVDpp() *SVDpp {
	return new(SVDpp)
}

func (svd *SVDpp) ensembleImplFactors(innerUserId int) []float64 {
	emImpFactor := make([]float64, 0)
	// User history exists
	count := 0
	for _, ir := range svd.userRatings[innerUserId] {
		if len(emImpFactor) == 0 {
			// Create ensemble implicit factor
			emImpFactor = make([]float64, len(svd.implFactor[ir.Id]))
		}
		floats.Add(emImpFactor, svd.implFactor[ir.Id])
		count++
	}
	divConst(math.Sqrt(float64(count)), emImpFactor)
	return emImpFactor
}

func (svd *SVDpp) internalPredict(userId int, itemId int) (float64, []float64) {
	// Convert to inner ID
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
		ret += floats.Dot(temp, itemFactor)
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
	nFactors := params.GetInt("nFactors", 20)
	nEpochs := params.GetInt("nEpochs", 20)
	lr := params.GetFloat64("lr", 0.007)
	reg := params.GetFloat64("reg", 0.02)
	initMean := params.GetFloat64("initMean", 0)
	initStdDev := params.GetFloat64("initStdDev", 0.1)
	// Initialize parameters
	svd.trainSet = trainSet
	svd.userBias = make([]float64, trainSet.UserCount)
	svd.itemBias = make([]float64, trainSet.ItemCount)
	svd.userFactor = make([][]float64, trainSet.UserCount)
	svd.itemFactor = make([][]float64, trainSet.ItemCount)
	svd.implFactor = make([][]float64, trainSet.ItemCount)
	//svd.cacheFactor = make(map[int][]float64)
	for innerUserId := range svd.userBias {
		svd.userFactor[innerUserId] = newNormalVector(nFactors, initMean, initStdDev)
	}
	for innerItemId := range svd.itemBias {
		svd.itemFactor[innerItemId] = newNormalVector(nFactors, initMean, initStdDev)
		svd.implFactor[innerItemId] = newNormalVector(nFactors, initMean, initStdDev)
	}
	// Build user rating set
	svd.userRatings = trainSet.UserRatings()
	// Create buffers
	a := make([]float64, nFactors)
	b := make([]float64, nFactors)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < nEpochs; epoch++ {
		for i := 0; i < trainSet.Length(); i++ {
			userId, itemId, rating := trainSet.Index(i)
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
			for _, ir := range svd.userRatings[innerUserId] {
				implFactor := svd.implFactor[ir.Id]
				copy(a, itemFactor)
				mulConst(diff, a)
				divConst(math.Sqrt(float64(len(svd.userRatings[innerUserId]))), a)
				copy(b, implFactor)
				mulConst(reg, b)
				floats.Add(a, b)
				mulConst(lr, a)
				floats.Sub(svd.implFactor[ir.Id], a)
			}
		}
	}
}
