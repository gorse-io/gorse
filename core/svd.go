package core

import (
	"gonum.org/v1/gonum/floats"
	"math"
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
	useBias    bool
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

// NewSVD creates a SVD model. Params:
//   useBias       - Add useBias in SVD model. Default is true.
//	 reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.02.
//	 lr 		- The learning rate of SGD. Default is 0.005.
//	 nFactors	- The number of latent factors. Default is 100.
//	 nEpochs	- The number of iteration of the SGD procedure. Default is 20.
//	 initMean	- The mean of initial random latent factors. Default is 0.
//	 initStdDev	- The standard deviation of initial random latent factors. Default is 0.1.
//   optimizer  - The optimizer to optimize model parameters. Default is SGDOptimizer.
func NewSVD(params Params) *SVD {
	svd := new(SVD)
	svd.SetParams(params)
	return svd
}

// SetParams sets hyper parameters.
func (svd *SVD) SetParams(params Params) {
	svd.Base.SetParams(params)
	svd.useBias = svd.Params.GetBool(UseBias, true)
	svd.nFactors = svd.Params.GetInt(NFactors, 100)
	svd.nEpochs = svd.Params.GetInt(NEpochs, 20)
	svd.lr = svd.Params.GetFloat64(Lr, 0.005)
	svd.reg = svd.Params.GetFloat64(Reg, 0.02)
	svd.initMean = svd.Params.GetFloat64(InitMean, 0)
	svd.initStdDev = svd.Params.GetFloat64(InitStdDev, 0.1)
}

// Predict by a SVD model.
func (svd *SVD) Predict(userId int, itemId int) float64 {
	innerUserId := svd.UserIdSet.ToDenseId(userId)
	innerItemId := svd.ItemIdSet.ToDenseId(itemId)
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
func (svd *SVD) Fit(trainSet TrainSet, setters ...RuntimeOptionSetter) {
	svd.Init(trainSet, setters)
	// Initialize parameters
	svd.UserBias = make([]float64, trainSet.UserCount())
	svd.ItemBias = make([]float64, trainSet.ItemCount())
	svd.UserFactor = svd.rng.MakeNormalMatrix(trainSet.UserCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	svd.ItemFactor = svd.rng.MakeNormalMatrix(trainSet.ItemCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	// Create buffers
	svd.a = make([]float64, svd.nFactors)
	svd.b = make([]float64, svd.nFactors)
	// Optimize
	for epoch := 0; epoch < svd.nEpochs; epoch++ {
		perm := svd.rng.Perm(trainSet.Length())
		for _, i := range perm {
			userId, itemId, rating := trainSet.Index(i)
			innerUserId := trainSet.UserIdSet.ToDenseId(userId)
			innerItemId := trainSet.ItemIdSet.ToDenseId(itemId)
			// Compute error
			upGrad := rating - svd.Predict(userId, itemId)
			// Point-wise update
			if svd.useBias {
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
	nFactors   int
	nEpochs    int
	initLow    float64
	initHigh   float64
	reg        float64
}

// NewNMF creates a NMF model. Params:
//	 Reg      - The regularization parameter of the cost function that is
//              optimized. Default is 0.06.
//	 NFactors - The number of latent factors. Default is 15.
//	 NEpochs  - The number of iteration of the SGD procedure. Default is 50.
//	 InitLow  - The lower bound of initial random latent factor. Default is 0.
//	 InitHigh - The upper bound of initial random latent factor. Default is 1.
func NewNMF(params Params) *NMF {
	nmf := new(NMF)
	nmf.SetParams(params)
	return nmf
}

func (nmf *NMF) SetParams(params Params) {
	nmf.Base.SetParams(params)
	nmf.nFactors = nmf.Params.GetInt(NFactors, 15)
	nmf.nEpochs = nmf.Params.GetInt(NEpochs, 50)
	nmf.initLow = nmf.Params.GetFloat64(InitLow, 0)
	nmf.initHigh = nmf.Params.GetFloat64(InitHigh, 1)
	nmf.reg = nmf.Params.GetFloat64(Reg, 0.06)
}

func (nmf *NMF) Predict(userId int, itemId int) float64 {
	innerUserId := nmf.UserIdSet.ToDenseId(userId)
	innerItemId := nmf.ItemIdSet.ToDenseId(itemId)
	if innerItemId != NewId && innerUserId != NewId {
		return floats.Dot(nmf.UserFactor[innerUserId], nmf.ItemFactor[innerItemId])
	}
	return 0
}

func (nmf *NMF) Fit(trainSet TrainSet, setters ...RuntimeOptionSetter) {
	nmf.Init(trainSet, setters)
	// Initialize parameters
	nmf.UserFactor = nmf.rng.MakeUniformMatrix(trainSet.UserCount(), nmf.nFactors, nmf.initLow, nmf.initHigh)
	nmf.ItemFactor = nmf.rng.MakeUniformMatrix(trainSet.ItemCount(), nmf.nFactors, nmf.initLow, nmf.initHigh)
	// Create intermediate matrix buffer
	buffer := make([]float64, nmf.nFactors)
	userUp := zeros(trainSet.UserCount(), nmf.nFactors)
	userDown := zeros(trainSet.UserCount(), nmf.nFactors)
	itemUp := zeros(trainSet.ItemCount(), nmf.nFactors)
	itemDown := zeros(trainSet.ItemCount(), nmf.nFactors)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < nmf.nEpochs; epoch++ {
		// Reset intermediate matrices
		resetZeroMatrix(userUp)
		resetZeroMatrix(userDown)
		resetZeroMatrix(itemUp)
		resetZeroMatrix(itemDown)
		// Calculate intermediate matrices
		for i := 0; i < trainSet.Length(); i++ {
			userId, itemId, rating := trainSet.Index(i)
			innerUserId := trainSet.UserIdSet.ToDenseId(userId)
			innerItemId := trainSet.ItemIdSet.ToDenseId(itemId)
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
			mulConst(nmf.reg, buffer)
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
			mulConst(nmf.reg, buffer)
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
	UserRatings []SparseVector // I_u
	UserFactor  [][]float64    // p_u
	ItemFactor  [][]float64    // q_i
	ImplFactor  [][]float64    // y_i
	UserBias    []float64      // b_u
	ItemBias    []float64      // b_i
	GlobalBias  float64        // mu
	nFactors    int
	nEpochs     int
	reg         float64
	lr          float64
	initMean    float64
	initStdDev  float64
}

// NewSVDpp creates a SVD++ model. Params:
//	 Reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.02.
//	 Lr 		- The learning rate of SGD. Default is 0.007.
//	 NFactors	- The number of latent factors. Default is 20.
//	 NEpochs	- The number of iteration of the SGD procedure. Default is 20.
//	 InitMean	- The mean of initial random latent factors. Default is 0.
//	 InitStdDev	- The standard deviation of initial random latent factors. Default is 0.1.
func NewSVDpp(params Params) *SVDpp {
	svd := new(SVDpp)
	svd.SetParams(params)
	return svd
}

func (svd *SVDpp) SetParams(params Params) {
	// Setup parameters
	svd.nFactors = svd.Params.GetInt(NFactors, 20)
	svd.nEpochs = svd.Params.GetInt(NEpochs, 20)
	svd.lr = svd.Params.GetFloat64(Lr, 0.007)
	svd.reg = svd.Params.GetFloat64(Reg, 0.02)
	svd.initMean = svd.Params.GetFloat64(InitMean, 0)
	svd.initStdDev = svd.Params.GetFloat64(InitStdDev, 0.1)
}

func (svd *SVDpp) ensembleImplFactors(innerUserId int) []float64 {
	nFactors := svd.Params.GetInt("nFactors", 20)
	emImpFactor := make([]float64, nFactors)
	// User history exists
	count := 0
	svd.UserRatings[innerUserId].ForEach(func(i, index int, value float64) {
		floats.Add(emImpFactor, svd.ImplFactor[index])
		count++
	})
	divConst(math.Sqrt(float64(count)), emImpFactor)
	return emImpFactor
}

func (svd *SVDpp) internalPredict(userId int, itemId int) (float64, []float64) {
	// Convert to inner ID
	innerUserId := svd.UserIdSet.ToDenseId(userId)
	innerItemId := svd.ItemIdSet.ToDenseId(itemId)
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
func (svd *SVDpp) Fit(trainSet TrainSet, setters ...RuntimeOptionSetter) {
	svd.Init(trainSet, setters)
	// Initialize parameters
	svd.UserBias = make([]float64, trainSet.UserCount())
	svd.ItemBias = make([]float64, trainSet.ItemCount())
	svd.UserFactor = svd.rng.MakeNormalMatrix(trainSet.UserCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	svd.ItemFactor = svd.rng.MakeNormalMatrix(trainSet.ItemCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	svd.ImplFactor = svd.rng.MakeNormalMatrix(trainSet.ItemCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	// Build user rating set
	svd.UserRatings = trainSet.UserRatings
	// Create buffers
	a := make([]float64, svd.nFactors)
	b := make([]float64, svd.nFactors)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < svd.nEpochs; epoch++ {
		perm := svd.rng.Perm(trainSet.Length())
		for _, i := range perm {
			userId, itemId, rating := trainSet.Index(i)
			innerUserId := trainSet.UserIdSet.ToDenseId(userId)
			innerItemId := trainSet.ItemIdSet.ToDenseId(itemId)
			userBias := svd.UserBias[innerUserId]
			itemBias := svd.ItemBias[innerItemId]
			userFactor := svd.UserFactor[innerUserId]
			itemFactor := svd.ItemFactor[innerItemId]
			// Compute error
			pred, emImpFactor := svd.internalPredict(userId, itemId)
			diff := pred - rating
			// Update global Bias
			gradGlobalBias := diff
			svd.GlobalBias -= svd.lr * gradGlobalBias
			// Update user Bias
			gradUserBias := diff + svd.reg*userBias
			svd.UserBias[innerUserId] -= svd.lr * gradUserBias
			// Update item Bias
			gradItemBias := diff + svd.reg*itemBias
			svd.ItemBias[innerItemId] -= svd.lr * gradItemBias
			// Update user latent factor
			copy(a, itemFactor)
			mulConst(diff, a)
			copy(b, userFactor)
			mulConst(svd.reg, b)
			floats.Add(a, b)
			mulConst(svd.lr, a)
			floats.Sub(svd.UserFactor[innerUserId], a)
			// Update item latent factor
			copy(a, userFactor)
			if len(emImpFactor) > 0 {
				floats.Add(a, emImpFactor)
			}
			mulConst(diff, a)
			copy(b, itemFactor)
			mulConst(svd.reg, b)
			floats.Add(a, b)
			mulConst(svd.lr, a)
			floats.Sub(svd.ItemFactor[innerItemId], a)
			// Update implicit latent factor
			nRating := svd.UserRatings[innerUserId].Length()
			var wg sync.WaitGroup
			wg.Add(svd.rtOptions.NJobs)
			for j := 0; j < svd.rtOptions.NJobs; j++ {
				go func(jobId int) {
					low := nRating * jobId / svd.rtOptions.NJobs
					high := nRating * (jobId + 1) / svd.rtOptions.NJobs
					a := make([]float64, svd.nFactors)
					b := make([]float64, svd.nFactors)
					for i := low; i < high; i++ {
						implFactor := svd.ImplFactor[svd.UserRatings[innerUserId].Indices[i]]
						copy(a, itemFactor)
						mulConst(diff, a)
						divConst(math.Sqrt(float64(svd.UserRatings[innerUserId].Length())), a)
						copy(b, implFactor)
						mulConst(svd.reg, b)
						floats.Add(a, b)
						mulConst(svd.lr, a)
						floats.Sub(svd.ImplFactor[svd.UserRatings[innerUserId].Indices[i]], a)
					}
					wg.Done()
				}(j)
			}
			wg.Wait()
		}
	}
}
