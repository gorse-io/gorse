package core

import (
	"gonum.org/v1/gonum/floats"
	"math"
	"runtime"
	"sync"
)

/* SVD */

// SVD algorithm, as popularized by Simon Funk during the
// Netflix Prize. The prediction \hat{r}_{ui} is set as:
//
//               \hat{r}_{ui} = μ + b_u + b_i + q_i^Tp_u
//
// If user u is unknown, then the Bias b_u and the factors p_u are
// assumed to be zero. The same applies for item i with b_i and q_i.
type SVD struct {
	Base
	// Model parameters
	UserFactor [][]float64 // p_u
	ItemFactor [][]float64 // q_i
	UserBias   []float64   // b_u
	ItemBias   []float64   // b_i
	GlobalBias float64     // mu
	// Hyper parameters
	bias       bool
	nFactors   int
	nEpochs    int
	lr         float64
	reg        float64
	initMean   float64
	initStdDev float64
	batchSize  int
	// Optimization
	a []float64 // Pre-allocated buffer 'a'
	b []float64 // Pre-allocated buffer 'b'
}

// NewSVD creates a SVD model. Parameters:
//   bias       - Add bias in SVD model. Default is true.
//	 reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.02.
//	 lr 		- The learning rate of SGD. Default is 0.005.
//	 nFactors	- The number of latent factors. Default is 100.
//	 nEpochs	- The number of iteration of the SGD procedure. Default is 20.
//	 initMean	- The mean of initial random latent factors. Default is 0.
//	 initStdDev	- The standard deviation of initial random latent factors. Default is 0.1.
//   optimizer  - The optimizer to optimize model parameters. Default is SGDOptimizer.
func NewSVD(params Parameters) *SVD {
	svd := new(SVD)
	svd.SetParams(params)
	return svd
}

// SetParams sets hyper parameters.
func (svd *SVD) SetParams(params Parameters) {
	svd.Base.SetParams(params)
	svd.bias = svd.Params.GetBool("bias", true)
	svd.nFactors = svd.Params.GetInt("nFactors", 100)
	svd.nEpochs = svd.Params.GetInt("nEpochs", 20)
	svd.lr = svd.Params.GetFloat64("lr", 0.005)
	svd.reg = svd.Params.GetFloat64("reg", 0.02)
	svd.initMean = svd.Params.GetFloat64("initMean", 0)
	svd.initStdDev = svd.Params.GetFloat64("initStdDev", 0.1)
	svd.batchSize = svd.Params.GetInt("batchSize", 100)
}

// Predict by a SVD model.
func (svd *SVD) Predict(userId int, itemId int) float64 {
	innerUserId := svd.Data.ConvertUserId(userId)
	innerItemId := svd.Data.ConvertItemId(itemId)
	ret := svd.GlobalBias
	// + b_u
	if innerUserId != NewId {
		ret += svd.UserBias[innerUserId]
	}
	// + b_i
	if innerItemId != NewId {
		ret += svd.ItemBias[innerItemId]
	}
	// + q_i^Tp_u
	if innerItemId != NewId && innerUserId != NewId {
		userFactor := svd.UserFactor[innerUserId]
		itemFactor := svd.ItemFactor[innerItemId]
		ret += floats.Dot(userFactor, itemFactor)
	}
	return ret
}

// Fit a SVD model.
func (svd *SVD) Fit(trainSet TrainSet) {
	svd.Base.Fit(trainSet)
	// Initialize parameters
	svd.UserBias = make([]float64, trainSet.UserCount)
	svd.ItemBias = make([]float64, trainSet.ItemCount)
	svd.UserFactor = svd.newNormalMatrix(trainSet.UserCount, svd.nFactors, svd.initMean, svd.initStdDev)
	svd.ItemFactor = svd.newNormalMatrix(trainSet.ItemCount, svd.nFactors, svd.initMean, svd.initStdDev)
	// Create buffers
	svd.a = make([]float64, svd.nFactors)
	svd.b = make([]float64, svd.nFactors)
	// Optimize
	for epoch := 0; epoch < svd.nEpochs; epoch++ {
		perm := svd.rng.Perm(trainSet.Length())
		for _, i := range perm {
			userId, itemId, rating := trainSet.Index(i)
			innerUserId := trainSet.ConvertUserId(userId)
			innerItemId := trainSet.ConvertItemId(itemId)
			// Compute error
			upGrad := rating - svd.Predict(userId, itemId)
			// Point-wise update
			if svd.bias {
				userBias := svd.UserBias[innerUserId]
				itemBias := svd.ItemBias[innerItemId]
				// Update global Bias
				gradGlobalBias := upGrad
				svd.GlobalBias += svd.lr * gradGlobalBias
				// Update user Bias
				gradUserBias := upGrad + svd.reg*userBias
				svd.UserBias[innerUserId] += svd.lr * gradUserBias
				// Update item Bias
				gradItemBias := upGrad + svd.reg*itemBias
				svd.ItemBias[innerItemId] += svd.lr * gradItemBias
			}
			userFactor := svd.UserFactor[innerUserId]
			itemFactor := svd.ItemFactor[innerItemId]
			// Update user latent factor
			copy(svd.a, itemFactor)
			mulConst(upGrad, svd.a)
			copy(svd.b, userFactor)
			mulConst(svd.reg, svd.b)
			floats.Sub(svd.a, svd.b)
			mulConst(svd.lr, svd.a)
			floats.Add(svd.UserFactor[innerUserId], svd.a)
			// Update item latent factor
			copy(svd.a, userFactor)
			mulConst(upGrad, svd.a)
			copy(svd.b, itemFactor)
			mulConst(svd.reg, svd.b)
			floats.Sub(svd.a, svd.b)
			mulConst(svd.lr, svd.a)
			floats.Add(svd.ItemFactor[innerItemId], svd.a)
		}
	}
}

// PairUpdate updates model parameters by pair.
func (svd *SVD) PairUpdate(upGrad float64, innerUserId, positiveItemId, negativeItemId int) {
	userFactor := svd.UserFactor[innerUserId]
	positiveItemFactor := svd.ItemFactor[positiveItemId]
	negativeItemFactor := svd.ItemFactor[negativeItemId]
	// Update positive item latent factor: +w_u
	copy(svd.a, userFactor)
	mulConst(upGrad, svd.a)
	mulConst(svd.lr, svd.a)
	floats.Add(svd.ItemFactor[positiveItemId], svd.a)
	// Update negative item latent factor: -w_u
	copy(svd.a, userFactor)
	neg(svd.a)
	mulConst(upGrad, svd.a)
	mulConst(svd.lr, svd.a)
	floats.Add(svd.ItemFactor[negativeItemId], svd.a)
	// Update user latent factor: h_i-h_j
	copy(svd.a, positiveItemFactor)
	floats.Sub(svd.a, negativeItemFactor)
	mulConst(upGrad, svd.a)
	mulConst(svd.lr, svd.a)
	floats.Add(svd.UserFactor[innerUserId], svd.a)
}

/* NMF */

// NMF: Non-negative Matrix Factorization[1].
//
// [1] Luo, Xin, et al. "An efficient non-negative matrix-
// factorization-based approach to collaborative filtering
// for recommender systems." IEEE Transactions on Industrial
// Informatics 10.2 (2014): 1273-1284.
type NMF struct {
	Base
	UserFactor [][]float64 // p_u
	ItemFactor [][]float64 // q_i
}

// NewNMF creates a NMF model. Parameters:
//	 reg      - The regularization parameter of the cost function that is
//              optimized. Default is 0.06.
//	 nFactors - The number of latent factors. Default is 15.
//	 nEpochs  - The number of iteration of the SGD procedure. Default is 50.
//	 initLow  - The lower bound of initial random latent factor. Default is 0.
//	 initHigh - The upper bound of initial random latent factor. Default is 1.
func NewNMF(params Parameters) *NMF {
	nmf := new(NMF)
	nmf.Params = params
	return nmf
}

// Predict by a NMF model.
func (nmf *NMF) Predict(userId int, itemId int) float64 {
	innerUserId := nmf.Data.ConvertUserId(userId)
	innerItemId := nmf.Data.ConvertItemId(itemId)
	if innerItemId != NewId && innerUserId != NewId {
		return floats.Dot(nmf.UserFactor[innerUserId], nmf.ItemFactor[innerItemId])
	}
	return 0
}

// Fit a NMF model.
func (nmf *NMF) Fit(trainSet TrainSet) {
	nmf.Base.Fit(trainSet)
	nFactors := nmf.Params.GetInt("nFactors", 15)
	nEpochs := nmf.Params.GetInt("nEpochs", 50)
	initLow := nmf.Params.GetFloat64("initLow", 0)
	initHigh := nmf.Params.GetFloat64("initHigh", 1)
	reg := nmf.Params.GetFloat64("reg", 0.06)
	// Initialize parameters
	nmf.UserFactor = nmf.newUniformMatrix(trainSet.UserCount, nFactors, initLow, initHigh)
	nmf.ItemFactor = nmf.newUniformMatrix(trainSet.ItemCount, nFactors, initLow, initHigh)
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
			copy(buffer, nmf.ItemFactor[innerItemId])
			mulConst(rating, buffer)
			floats.Add(userUp[innerUserId], buffer)
			// Update userDown
			copy(buffer, nmf.ItemFactor[innerItemId])
			mulConst(prediction, buffer)
			floats.Add(userDown[innerUserId], buffer)
			copy(buffer, nmf.UserFactor[innerUserId])
			mulConst(reg, buffer)
			floats.Add(userDown[innerUserId], buffer)
			// Update itemUp
			copy(buffer, nmf.UserFactor[innerUserId])
			mulConst(rating, buffer)
			floats.Add(itemUp[innerItemId], buffer)
			// Update itemDown
			copy(buffer, nmf.UserFactor[innerUserId])
			mulConst(prediction, buffer)
			floats.Add(itemDown[innerItemId], buffer)
			copy(buffer, nmf.ItemFactor[innerItemId])
			mulConst(reg, buffer)
			floats.Add(itemDown[innerItemId], buffer)
		}
		// Update user factors
		for u := range nmf.UserFactor {
			copy(buffer, userUp[u])
			floats.Div(buffer, userDown[u])
			floats.Mul(nmf.UserFactor[u], buffer)
		}
		// Update item factors
		for i := range nmf.ItemFactor {
			copy(buffer, itemUp[i])
			floats.Div(buffer, itemDown[i])
			floats.Mul(nmf.ItemFactor[i], buffer)
		}
	}
}

/* SVD++ */

// SVD++ algorithm, an extension of SVD taking into account implicit
// interactionRatings. The prediction \hat{r}_{ui} is set as:
//
// 	\hat{r}_{ui} = \mu + b_u + b_i + q_i^T\left(p_u + |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right)
//
// Where the y_j terms are a new set of item factors that capture implicit
// interactionRatings. Here, an implicit rating describes the fact that a user u
// UserRatings an item j, regardless of the rating value. If user u is unknown,
// then the Bias b_u and the factors p_u are assumed to be zero. The same
// applies for item i with b_i, q_i and y_i.
type SVDpp struct {
	Base
	UserRatings [][]IdRating // I_u
	UserFactor  [][]float64  // p_u
	ItemFactor  [][]float64  // q_i
	ImplFactor  [][]float64  // y_i
	UserBias    []float64    // b_u
	ItemBias    []float64    // b_i
	GlobalBias  float64      // mu
}

// NewSVDpp creates a SVD++ model. Parameters:
//	 reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.02.
//	 lr 		- The learning rate of SGD. Default is 0.007.
//	 nFactors	- The number of latent factors. Default is 20.
//	 nEpochs	- The number of iteration of the SGD procedure. Default is 20.
//	 initMean	- The mean of initial random latent factors. Default is 0.
//	 initStdDev	- The standard deviation of initial random latent factors. Default is 0.1.
//	 nJobs		- The number of goroutines to update implicit factors. Default is the number of CPUs.
func NewSVDpp(params Parameters) *SVDpp {
	svd := new(SVDpp)
	svd.Params = params
	return svd
}

func (svd *SVDpp) ensembleImplFactors(innerUserId int) []float64 {
	nFactors := svd.Params.GetInt("nFactors", 20)
	emImpFactor := make([]float64, nFactors)
	// User history exists
	count := 0
	for _, ir := range svd.UserRatings[innerUserId] {
		floats.Add(emImpFactor, svd.ImplFactor[ir.Id])
		count++
	}
	divConst(math.Sqrt(float64(count)), emImpFactor)
	return emImpFactor
}

func (svd *SVDpp) internalPredict(userId int, itemId int) (float64, []float64) {
	// Convert to inner ID
	innerUserId := svd.Data.ConvertUserId(userId)
	innerItemId := svd.Data.ConvertItemId(itemId)
	ret := svd.GlobalBias
	// + b_u
	if innerUserId != NewId {
		ret += svd.UserBias[innerUserId]
	}
	// + b_i
	if innerItemId != NewId {
		ret += svd.ItemBias[innerItemId]
	}
	// + q_i^T\left(p_u + |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right)
	if innerItemId != NewId && innerUserId != NewId {
		userFactor := svd.UserFactor[innerUserId]
		itemFactor := svd.ItemFactor[innerItemId]
		emImpFactor := svd.ensembleImplFactors(innerUserId)
		temp := make([]float64, len(itemFactor))
		floats.Add(temp, userFactor)
		floats.Add(temp, emImpFactor)
		ret += floats.Dot(temp, itemFactor)
		return ret, emImpFactor
	}
	return ret, []float64{}
}

// Predict by a SVD++ model.
func (svd *SVDpp) Predict(userId int, itemId int) float64 {
	ret, _ := svd.internalPredict(userId, itemId)
	return ret
}

// Fit a SVD++ model.
func (svd *SVDpp) Fit(trainSet TrainSet) {
	svd.Base.Fit(trainSet)
	// Setup parameters
	nFactors := svd.Params.GetInt("nFactors", 20)
	nEpochs := svd.Params.GetInt("nEpochs", 20)
	lr := svd.Params.GetFloat64("lr", 0.007)
	reg := svd.Params.GetFloat64("reg", 0.02)
	initMean := svd.Params.GetFloat64("initMean", 0)
	initStdDev := svd.Params.GetFloat64("initStdDev", 0.1)
	nJobs := svd.Params.GetInt("nJobs", runtime.NumCPU())
	// Initialize parameters
	svd.UserBias = make([]float64, trainSet.UserCount)
	svd.ItemBias = make([]float64, trainSet.ItemCount)
	svd.UserFactor = svd.newNormalMatrix(trainSet.UserCount, nFactors, initMean, initStdDev)
	svd.ItemFactor = svd.newNormalMatrix(trainSet.ItemCount, nFactors, initMean, initStdDev)
	svd.ImplFactor = svd.newNormalMatrix(trainSet.ItemCount, nFactors, initMean, initStdDev)
	//svd.cacheFactor = make(map[int][]float64)
	// Build user rating set
	svd.UserRatings = trainSet.UserRatings()
	// Create buffers
	a := make([]float64, nFactors)
	b := make([]float64, nFactors)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < nEpochs; epoch++ {
		perm := svd.rng.Perm(trainSet.Length())
		for _, i := range perm {
			userId, itemId, rating := trainSet.Index(i)
			innerUserId := trainSet.ConvertUserId(userId)
			innerItemId := trainSet.ConvertItemId(itemId)
			userBias := svd.UserBias[innerUserId]
			itemBias := svd.ItemBias[innerItemId]
			userFactor := svd.UserFactor[innerUserId]
			itemFactor := svd.ItemFactor[innerItemId]
			// Compute error
			pred, emImpFactor := svd.internalPredict(userId, itemId)
			diff := pred - rating
			// Update global Bias
			gradGlobalBias := diff
			svd.GlobalBias -= lr * gradGlobalBias
			// Update user Bias
			gradUserBias := diff + reg*userBias
			svd.UserBias[innerUserId] -= lr * gradUserBias
			// Update item Bias
			gradItemBias := diff + reg*itemBias
			svd.ItemBias[innerItemId] -= lr * gradItemBias
			// Update user latent factor
			copy(a, itemFactor)
			mulConst(diff, a)
			copy(b, userFactor)
			mulConst(reg, b)
			floats.Add(a, b)
			mulConst(lr, a)
			floats.Sub(svd.UserFactor[innerUserId], a)
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
			floats.Sub(svd.ItemFactor[innerItemId], a)
			// Update implicit latent factor
			nRating := len(svd.UserRatings[innerUserId])
			var wg sync.WaitGroup
			wg.Add(nJobs)
			for j := 0; j < nJobs; j++ {
				go func(jobId int) {
					low := nRating * jobId / nJobs
					high := nRating * (jobId + 1) / nJobs
					a := make([]float64, nFactors)
					b := make([]float64, nFactors)
					for i := low; i < high; i++ {
						implFactor := svd.ImplFactor[svd.UserRatings[innerUserId][i].Id]
						copy(a, itemFactor)
						mulConst(diff, a)
						divConst(math.Sqrt(float64(len(svd.UserRatings[innerUserId]))), a)
						copy(b, implFactor)
						mulConst(reg, b)
						floats.Add(a, b)
						mulConst(lr, a)
						floats.Sub(svd.ImplFactor[svd.UserRatings[innerUserId][i].Id], a)
					}
					wg.Done()
				}(j)
			}
			wg.Wait()
		}
	}
}
