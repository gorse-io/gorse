package model

import (
	"fmt"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/floats"
	"gonum.org/v1/gonum/mat"
	"log"
	"math"
	"sync"
)

/* SVD */

// SVD algorithm, as popularized by Simon Funk during the
// Netflix Prize. The prediction \hat{r}_{ui} is set as:
//
//   \hat{r}_{ui} = μ + b_u + b_i + q_i^Tp_u
//
// If user u is unknown, then the Bias b_u and the factors p_u are
// assumed to be zero. The same applies for item i with b_i and q_i.
// Hyper-parameters:
//   UseBias    - Add useBias in SVD model. Default is true.
//	 Reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.02.
//	 Lr 		- The learning rate of SGD. Default is 0.005.
//	 nFactors	- The number of latent factors. Default is 100.
//	 NEpochs	- The number of iteration of the SGD procedure. Default is 20.
//	 InitMean	- The mean of initial random latent factors. Default is 0.
//	 InitStdDev	- The standard deviation of initial random latent factors. Default is 0.1.
type SVD struct {
	Base
	// Model parameters
	UserFactor [][]float64 // p_u
	ItemFactor [][]float64 // q_i
	UserBias   []float64   // b_u
	ItemBias   []float64   // b_i
	GlobalMean float64     // mu
	// Hyper parameters
	useBias    bool
	nFactors   int
	nEpochs    int
	lr         float64
	reg        float64
	initMean   float64
	initStdDev float64
	target     base.ParamString
}

// NewSVD creates a SVD model.
func NewSVD(params base.Params) *SVD {
	svd := new(SVD)
	svd.SetParams(params)
	return svd
}

// SetParams sets hyper-parameters of the SVD model.
func (svd *SVD) SetParams(params base.Params) {
	svd.Base.SetParams(params)
	// Setup hyper-parameters
	svd.useBias = svd.Params.GetBool(base.UseBias, true)
	svd.nFactors = svd.Params.GetInt(base.NFactors, 100)
	svd.nEpochs = svd.Params.GetInt(base.NEpochs, 20)
	svd.lr = svd.Params.GetFloat64(base.Lr, 0.005)
	svd.reg = svd.Params.GetFloat64(base.Reg, 0.02)
	svd.initMean = svd.Params.GetFloat64(base.InitMean, 0)
	svd.initStdDev = svd.Params.GetFloat64(base.InitStdDev, 0.1)
	svd.target = svd.Params.GetString(base.Optimizer, base.SGD)
}

// Predict by the SVD model.
func (svd *SVD) Predict(userId int, itemId int) float64 {
	// Convert sparse IDs to dense IDs
	denseUserId := svd.UserIdSet.ToDenseId(userId)
	denseItemId := svd.ItemIdSet.ToDenseId(itemId)
	return svd.predict(denseUserId, denseItemId)
}

func (svd *SVD) predict(denseUserId int, denseItemId int) float64 {
	ret := svd.GlobalMean
	// + b_u
	if denseUserId != base.NotId {
		ret += svd.UserBias[denseUserId]
	}
	// + b_i
	if denseItemId != base.NotId {
		ret += svd.ItemBias[denseItemId]
	}
	// + q_i^Tp_u
	if denseItemId != base.NotId && denseUserId != base.NotId {
		userFactor := svd.UserFactor[denseUserId]
		itemFactor := svd.ItemFactor[denseItemId]
		ret += floats.Dot(userFactor, itemFactor)
	}
	return ret
}

// Fit the SVD model.
func (svd *SVD) Fit(trainSet *core.DataSet, options ...core.FitOption) {
	svd.Init(trainSet, options)
	// Initialize parameters
	svd.GlobalMean = 0
	svd.UserBias = make([]float64, trainSet.UserCount())
	svd.ItemBias = make([]float64, trainSet.ItemCount())
	svd.UserFactor = svd.rng.NewNormalMatrix(trainSet.UserCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	svd.ItemFactor = svd.rng.NewNormalMatrix(trainSet.ItemCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	// Select fit function
	switch svd.target {
	case base.SGD:
		svd.fitSGD(trainSet)
	case base.BPR:
		svd.fitBPR(trainSet)
	default:
		panic(fmt.Sprintf("Unknown target: %v", svd.target))
	}
}

func (svd *SVD) fitSGD(trainSet *core.DataSet) {
	svd.GlobalMean = trainSet.GlobalMean
	// Create buffers
	temp := make([]float64, svd.nFactors)
	userFactor := make([]float64, svd.nFactors)
	itemFactor := make([]float64, svd.nFactors)
	// Optimize
	for epoch := 0; epoch < svd.nEpochs; epoch++ {
		perm := svd.rng.Perm(trainSet.Len())
		for _, i := range perm {
			denseUserId, denseItemId, rating := trainSet.GetDense(i)
			// Compute error: e_{ui} = r - \hat r
			upGrad := rating - svd.predict(denseUserId, denseItemId)
			if svd.useBias {
				userBias := svd.UserBias[denseUserId]
				itemBias := svd.ItemBias[denseItemId]
				// Update user Bias: b_u <- b_u + \gamma (e_{ui} - \lambda b_u)
				gradUserBias := upGrad - svd.reg*userBias
				svd.UserBias[denseUserId] += svd.lr * gradUserBias
				// Update item Bias: p_i <- p_i + \gamma (e_{ui} - \lambda b_i)
				gradItemBias := upGrad - svd.reg*itemBias
				svd.ItemBias[denseItemId] += svd.lr * gradItemBias
			}
			copy(userFactor, svd.UserFactor[denseUserId])
			copy(itemFactor, svd.ItemFactor[denseItemId])
			// Update user latent factor
			floats.MulConstTo(itemFactor, upGrad, temp)
			floats.MulConstAddTo(userFactor, -svd.reg, temp)
			floats.MulConstAddTo(temp, svd.lr, svd.UserFactor[denseUserId])
			// Update item latent factor
			floats.MulConstTo(userFactor, upGrad, temp)
			floats.MulConstAddTo(itemFactor, -svd.reg, temp)
			floats.MulConstAddTo(temp, svd.lr, svd.ItemFactor[denseItemId])
		}
	}
}

func (svd *SVD) fitBPR(trainSet *core.DataSet) {
	// Create the set of positive feedback
	positiveSet := make([]map[int]float64, trainSet.UserCount())
	for denseUserId, userRating := range trainSet.DenseUserRatings {
		positiveSet[denseUserId] = make(map[int]float64)
		userRating.ForEach(func(i, index int, value float64) {
			positiveSet[denseUserId][index] = value
		})
	}
	// Create buffers
	temp := make([]float64, svd.nFactors)
	userFactor := make([]float64, svd.nFactors)
	positiveItemFactor := make([]float64, svd.nFactors)
	negativeItemFactor := make([]float64, svd.nFactors)
	// Training
	for epoch := 0; epoch < svd.nEpochs; epoch++ {
		// Training epoch
		for i := 0; i < trainSet.Len(); i++ {
			// Select a user
			denseUserId := svd.rng.Intn(trainSet.UserCount())
			densePosId := trainSet.DenseUserRatings[denseUserId].Indices[svd.rng.Intn(trainSet.DenseUserRatings[denseUserId].Len())]
			// Select a negative sample
			denseNegId := -1
			for {
				temp := svd.rng.Intn(trainSet.ItemCount())
				if _, exist := positiveSet[denseUserId][temp]; !exist {
					denseNegId = temp
					break
				}
			}
			diff := svd.predict(denseUserId, densePosId) - svd.predict(denseUserId, denseNegId)
			grad := math.Exp(-diff) / (1.0 + math.Exp(-diff))
			// Pairwise update
			copy(userFactor, svd.UserFactor[denseUserId])
			copy(positiveItemFactor, svd.ItemFactor[densePosId])
			copy(negativeItemFactor, svd.ItemFactor[denseNegId])
			// Update positive item latent factor: +w_u
			floats.MulConstTo(userFactor, grad, temp)
			floats.MulConstAddTo(positiveItemFactor, -svd.reg, temp)
			floats.MulConstAddTo(temp, svd.lr, svd.ItemFactor[densePosId])
			// Update negative item latent factor: -w_u
			floats.MulConstTo(userFactor, -grad, temp)
			floats.MulConstAddTo(negativeItemFactor, -svd.reg, temp)
			floats.MulConstAddTo(temp, svd.lr, svd.ItemFactor[denseNegId])
			// Update user latent factor: h_i-h_j
			floats.SubTo(positiveItemFactor, negativeItemFactor, temp)
			floats.MulConst(temp, grad)
			floats.MulConstAddTo(userFactor, -svd.reg, temp)
			floats.MulConstAddTo(temp, svd.lr, svd.UserFactor[denseUserId])
		}
	}
}

/* NMF */

// NMF [3] is the Matrix Factorization process with non-negative latent factors.
// During the MF process, the non-negativity, which ensures good representativeness
// of the learnt model, is critically important.
// Hyper-parameters:
//	 Reg      - The regularization parameter of the cost function that is
//              optimized. Default is 0.06.
//	 NFactors - The number of latent factors. Default is 15.
//	 NEpochs  - The number of iteration of the SGD procedure. Default is 50.
//	 InitLow  - The lower bound of initial random latent factor. Default is 0.
//	 InitHigh - The upper bound of initial random latent factor. Default is 1.
type NMF struct {
	Base
	GlobalMean float64     // the global mean of ratings
	UserFactor [][]float64 // p_u
	ItemFactor [][]float64 // q_i
	// Hyper-parameters
	nFactors int
	nEpochs  int
	initLow  float64
	initHigh float64
	reg      float64
}

// NewNMF creates a NMF model.
func NewNMF(params base.Params) *NMF {
	nmf := new(NMF)
	nmf.SetParams(params)
	return nmf
}

// SetParams sets hyper-parameters of the NMF model.
func (nmf *NMF) SetParams(params base.Params) {
	nmf.Base.SetParams(params)
	// Setup hyper-parameters
	nmf.nFactors = nmf.Params.GetInt(base.NFactors, 15)
	nmf.nEpochs = nmf.Params.GetInt(base.NEpochs, 50)
	nmf.initLow = nmf.Params.GetFloat64(base.InitLow, 0)
	nmf.initHigh = nmf.Params.GetFloat64(base.InitHigh, 1)
	nmf.reg = nmf.Params.GetFloat64(base.Reg, 0.06)
}

// Predict by the NMF model.
func (nmf *NMF) Predict(userId int, itemId int) float64 {
	denseUserId := nmf.UserIdSet.ToDenseId(userId)
	denseItemId := nmf.ItemIdSet.ToDenseId(itemId)
	return nmf.predict(denseUserId, denseItemId)
}

func (nmf *NMF) predict(denseUserId int, denseItemId int) float64 {
	if denseItemId != base.NotId && denseUserId != base.NotId {
		return floats.Dot(nmf.UserFactor[denseUserId], nmf.ItemFactor[denseItemId])
	}
	return nmf.GlobalMean
}

// Fit the NMF model.
func (nmf *NMF) Fit(trainSet *core.DataSet, options ...core.FitOption) {
	nmf.Init(trainSet, options)
	// Initialize parameters
	nmf.GlobalMean = trainSet.GlobalMean
	nmf.UserFactor = nmf.rng.NewUniformMatrix(trainSet.UserCount(), nmf.nFactors, nmf.initLow, nmf.initHigh)
	nmf.ItemFactor = nmf.rng.NewUniformMatrix(trainSet.ItemCount(), nmf.nFactors, nmf.initLow, nmf.initHigh)
	// Create intermediate matrix buffer
	buffer := make([]float64, nmf.nFactors)
	userNum := base.NewMatrix(trainSet.UserCount(), nmf.nFactors)
	userDen := base.NewMatrix(trainSet.UserCount(), nmf.nFactors)
	itemNum := base.NewMatrix(trainSet.ItemCount(), nmf.nFactors)
	itemDen := base.NewMatrix(trainSet.ItemCount(), nmf.nFactors)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < nmf.nEpochs; epoch++ {
		// Reset intermediate matrices
		base.FillZeroMatrix(userNum)
		base.FillZeroMatrix(userDen)
		base.FillZeroMatrix(itemNum)
		base.FillZeroMatrix(itemDen)
		// Calculate intermediate matrices
		for i := 0; i < trainSet.Len(); i++ {
			denseUserId, denseItemId, rating := trainSet.GetDense(i)
			prediction := nmf.predict(denseUserId, denseItemId)
			// Update \sum_{i\in{I_u}} q_{if}⋅r_{ui}
			floats.MulConstAddTo(nmf.ItemFactor[denseItemId], rating, userNum[denseUserId])
			// Update \sum_{i\in{I_u}} q_{if}⋅\hat{r}_{ui} + \lambda|I_u|p_{uf}
			floats.MulConstAddTo(nmf.ItemFactor[denseItemId], prediction, userDen[denseUserId])
			floats.MulConstAddTo(nmf.UserFactor[denseUserId], nmf.reg, userDen[denseUserId])
			// Update \sum_{u\in{U_i}}p_{uf}⋅r_{ui}
			floats.MulConstAddTo(nmf.UserFactor[denseUserId], rating, itemNum[denseItemId])
			// Update \sum_{u\in{U_i}}p_{uf}⋅\hat{r}_{ui} + \lambda|U_i|q_{if}
			floats.MulConstAddTo(nmf.UserFactor[denseUserId], prediction, itemDen[denseItemId])
			floats.MulConstAddTo(nmf.ItemFactor[denseItemId], nmf.reg, itemDen[denseItemId])
		}
		// Update user factors
		for u := range nmf.UserFactor {
			copy(buffer, userNum[u])
			floats.Div(buffer, userDen[u])
			floats.Mul(nmf.UserFactor[u], buffer)
		}
		// Update item factors
		for i := range nmf.ItemFactor {
			copy(buffer, itemNum[i])
			floats.Div(buffer, itemDen[i])
			floats.Mul(nmf.ItemFactor[i], buffer)
		}
	}
}

// SVDpp (SVD++) [10] is an extension of SVD taking into account implicit interactions.
// The predicted \hat{r}_{ui} is:
//
// 	\hat{r}_{ui} = \mu + b_u + b_i + q_i^T\left(p_u + |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right)
//
// Where the y_j terms are a new set of item factors that capture implicit interactions.
// Here, an implicit rating describes the fact that a user u rated an item j, regardless
// of the rating value. If user u is unknown, then the bias b_u and the factors p_u are
// assumed to be zero. The same applies for item i with b_i, q_i and y_i.
// Hyper-parameters:
//	 Reg        - The regularization parameter of the cost function that is
//                optimized. Default is 0.02.
//	 Lr         - The learning rate of SGD. Default is 0.007.
//	 NFactors   - The number of latent factors. Default is 20.
//	 NEpochs    - The number of iteration of the SGD procedure. Default is 20.
//	 InitMean   - The mean of initial random latent factors. Default is 0.
//	 InitStdDev - The standard deviation of initial random latent factors. Default is 0.1.
type SVDpp struct {
	Base
	UserRatings []*base.SparseVector // I_u
	UserFactor  [][]float64          // p_u
	ItemFactor  [][]float64          // q_i
	ImplFactor  [][]float64          // y_i
	UserBias    []float64            // b_u
	ItemBias    []float64            // b_i
	GlobalMean  float64              // mu
	// Hyper-parameters
	nFactors   int
	nEpochs    int
	reg        float64
	lr         float64
	initMean   float64
	initStdDev float64
}

// NewSVDpp creates a SVD++ model.
func NewSVDpp(params base.Params) *SVDpp {
	svd := new(SVDpp)
	svd.SetParams(params)
	return svd
}

// SetParams sets hyper-parameters of the SVD++ model.
func (svd *SVDpp) SetParams(params base.Params) {
	svd.Base.SetParams(params)
	// Setup parameters
	svd.nFactors = svd.Params.GetInt(base.NFactors, 20)
	svd.nEpochs = svd.Params.GetInt(base.NEpochs, 20)
	svd.lr = svd.Params.GetFloat64(base.Lr, 0.007)
	svd.reg = svd.Params.GetFloat64(base.Reg, 0.02)
	svd.initMean = svd.Params.GetFloat64(base.InitMean, 0)
	svd.initStdDev = svd.Params.GetFloat64(base.InitStdDev, 0.1)
}

// Predict by the SVD++ model.
func (svd *SVDpp) Predict(userId int, itemId int) float64 {
	denseUserId := svd.UserIdSet.ToDenseId(userId)
	denseItemId := svd.ItemIdSet.ToDenseId(itemId)
	ret := svd.predict(denseUserId, denseItemId, nil)
	return ret
}

func (svd *SVDpp) predict(denseUserId int, denseItemId int, sumFactor []float64) float64 {
	ret := svd.GlobalMean
	// + b_u
	if denseUserId != base.NotId {
		ret += svd.UserBias[denseUserId]
	}
	// + b_i
	if denseItemId != base.NotId {
		ret += svd.ItemBias[denseItemId]
	}
	// + q_i^T\left(p_u + |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right)
	if denseItemId != base.NotId && denseUserId != base.NotId {
		userFactor := svd.UserFactor[denseUserId]
		itemFactor := svd.ItemFactor[denseItemId]
		if len(sumFactor) == 0 {
			sumFactor = svd.sumOverImplicitFactors(denseUserId)
		}
		temp := make([]float64, len(itemFactor))
		floats.AddTo(userFactor, sumFactor, temp)
		ret += floats.Dot(temp, itemFactor)
	}
	return ret
}

func (svd *SVDpp) sumOverImplicitFactors(denseUserId int) []float64 {
	sumFactor := make([]float64, svd.nFactors)
	// User history exists
	svd.UserRatings[denseUserId].ForEach(func(i, index int, value float64) {
		floats.Add(sumFactor, svd.ImplFactor[index])
	})
	scale := math.Pow(float64(svd.UserRatings[denseUserId].Len()), -0.5)
	floats.MulConst(sumFactor, scale)
	return sumFactor
}

// Fit the SVD++ model.
func (svd *SVDpp) Fit(trainSet *core.DataSet, setters ...core.FitOption) {
	svd.Init(trainSet, setters)
	// Initialize parameters
	svd.GlobalMean = trainSet.GlobalMean
	svd.UserBias = make([]float64, trainSet.UserCount())
	svd.ItemBias = make([]float64, trainSet.ItemCount())
	svd.UserFactor = svd.rng.NewNormalMatrix(trainSet.UserCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	svd.ItemFactor = svd.rng.NewNormalMatrix(trainSet.ItemCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	svd.ImplFactor = svd.rng.NewNormalMatrix(trainSet.ItemCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	// Build user rating set
	svd.UserRatings = trainSet.DenseUserRatings
	// Create buffers
	a := make([]float64, svd.nFactors)
	step := make([]float64, svd.nFactors)
	userFactor := make([]float64, svd.nFactors)
	itemFactor := make([]float64, svd.nFactors)
	c := base.NewMatrix(svd.fitOptions.NJobs, svd.nFactors)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < svd.nEpochs; epoch++ {
		for denseUserId := 0; denseUserId < trainSet.UserCount(); denseUserId++ {
			base.FillZeroVector(step)
			size := svd.UserRatings[denseUserId].Len()
			scale := math.Pow(float64(size), -0.5)
			sumFactor := svd.sumOverImplicitFactors(denseUserId)
			trainSet.DenseUserRatings[denseUserId].ForEach(func(i, denseItemId int, rating float64) {
				userBias := svd.UserBias[denseUserId]
				itemBias := svd.ItemBias[denseItemId]
				copy(userFactor, svd.UserFactor[denseUserId])
				copy(itemFactor, svd.ItemFactor[denseItemId])
				// Compute error: e_{ui} = r - \hat r
				pred := svd.predict(denseUserId, denseItemId, sumFactor)
				diff := rating - pred
				// Update user Bias: b_u <- b_u + \gamma (e_{ui} - \lambda b_u)
				gradUserBias := diff - svd.reg*userBias
				svd.UserBias[denseUserId] += svd.lr * gradUserBias
				// Update item Bias: p_i <- p_i + \gamma (e_{ui} - \lambda b_i)
				gradItemBias := diff - svd.reg*itemBias
				svd.ItemBias[denseItemId] += svd.lr * gradItemBias
				// Update user latent factor
				floats.MulConstTo(itemFactor, diff, a)
				floats.MulConstAddTo(userFactor, -svd.reg, a)
				floats.MulConstAddTo(a, svd.lr, svd.UserFactor[denseUserId])
				// Update item latent factor
				floats.AddTo(userFactor, sumFactor, a)
				floats.MulConst(a, diff)
				floats.MulConstAddTo(itemFactor, -svd.reg, a)
				floats.MulConstAddTo(a, svd.lr, svd.ItemFactor[denseItemId])
				// Update implicit latent factor: e_{ui}q_j|I_u|^{-1/2}
				floats.MulConstAddTo(itemFactor, scale*diff, step)
			})
			// Update implicit latent factor
			var wg sync.WaitGroup
			wg.Add(svd.fitOptions.NJobs)
			for j := 0; j < svd.fitOptions.NJobs; j++ {
				go func(jobId int) {
					low := size * jobId / svd.fitOptions.NJobs
					high := size * (jobId + 1) / svd.fitOptions.NJobs
					a := c[jobId]
					for i := low; i < high; i++ {
						denseItemId := svd.UserRatings[denseUserId].Indices[i]
						implFactor := svd.ImplFactor[denseItemId]
						// a <- e_{ui}q_j|I_u|^{-1/2}
						floats.MulConstTo(step, 1/float64(size), a)
						// + \lambda y_k
						floats.MulConstAddTo(implFactor, -svd.reg, a)
						// \mu (e_{ui}q_j|I_u|^{-1/2} + \lambda y_k)
						floats.MulConstAddTo(a, svd.lr, svd.ImplFactor[denseItemId])
					}
					wg.Done()
				}(j)
			}
			//Wait all updates completed
			wg.Wait()
		}
	}
}

// WRMF [7] is the Weighted Regularized Matrix Factorization, which exploits
// unique properties of implicit feedback datasets. It treats the data as
// indication of positive and negative preference associated with vastly
// varying confidence levels. This leads to a factor model which is especially
// tailored for implicit feedback recommenders. Authors also proposed a
// scalable optimization procedure, which scales linearly with the data size.
// Hyper-parameters:
//   NFactors   - The number of latent factors. Default is 10.
//   NEpochs    - The number of training epochs. Default is 50.
//   InitMean   - The mean of initial latent factors. Default is 0.
//   InitStdDev - The standard deviation of initial latent factors. Default is 0.1.
//   Reg        - The strength of regularization.
type WRMF struct {
	Base
	// Model parameters
	UserFactor *mat.Dense // p_u
	ItemFactor *mat.Dense // q_i
	// Hyper parameters
	nFactors   int
	nEpochs    int
	reg        float64
	initMean   float64
	initStdDev float64
	alpha      float64
}

// NewWRMF creates a WRMF model.
func NewWRMF(params base.Params) *WRMF {
	mf := new(WRMF)
	mf.SetParams(params)
	return mf
}

// SetParams sets hyper-parameters for the WRMF model.
func (mf *WRMF) SetParams(params base.Params) {
	mf.Base.SetParams(params)
	mf.nFactors = mf.Params.GetInt(base.NFactors, 15)
	mf.nEpochs = mf.Params.GetInt(base.NEpochs, 50)
	mf.initMean = mf.Params.GetFloat64(base.InitMean, 0)
	mf.initStdDev = mf.Params.GetFloat64(base.InitStdDev, 0.1)
	mf.reg = mf.Params.GetFloat64(base.Reg, 0.06)
}

// Predict by the WRMF model.
func (mf *WRMF) Predict(userId, itemId int) float64 {
	denseUserId := mf.UserIdSet.ToDenseId(userId)
	denseItemId := mf.ItemIdSet.ToDenseId(itemId)
	if denseUserId == base.NotId || denseItemId == base.NotId {
		return 0
	}
	return mat.Dot(mf.UserFactor.RowView(denseUserId),
		mf.ItemFactor.RowView(denseItemId))
}

// Fit the WRMF model.
func (mf *WRMF) Fit(set *core.DataSet, options ...core.FitOption) {
	mf.Init(set, options)
	// Initialize
	mf.UserFactor = mat.NewDense(set.UserCount(), mf.nFactors,
		mf.rng.NewNormalVector(set.UserCount()*mf.nFactors, mf.initMean, mf.initStdDev))
	mf.ItemFactor = mat.NewDense(set.ItemCount(), mf.nFactors,
		mf.rng.NewNormalVector(set.ItemCount()*mf.nFactors, mf.initMean, mf.initStdDev))
	// Create temporary matrix
	temp1 := mat.NewDense(mf.nFactors, mf.nFactors, nil)
	temp2 := mat.NewVecDense(mf.nFactors, nil)
	a := mat.NewDense(mf.nFactors, mf.nFactors, nil)
	c := mat.NewDense(mf.nFactors, mf.nFactors, nil)
	p := mat.NewDense(set.UserCount(), set.ItemCount(), nil)
	// Create regularization matrix
	regs := make([]float64, mf.nFactors)
	for i := range regs {
		regs[i] = mf.reg
	}
	regI := mat.NewDiagDense(mf.nFactors, regs)
	for ep := 0; ep < mf.nEpochs; ep++ {
		// Recompute all user factors: x_u = (Y^T C^u Y + \lambda reg)^{-1} Y^T C^u p(u)
		// Y^T Y
		c.Mul(mf.ItemFactor.T(), mf.ItemFactor)
		// X Y^T
		p.Mul(mf.UserFactor, mf.ItemFactor.T())
		for u := 0; u < set.UserCount(); u++ {
			a.Copy(c)
			b := mat.NewVecDense(mf.nFactors, nil)
			set.DenseUserRatings[u].ForEach(func(_, index int, value float64) {
				// Y^T (C^u-I) Y
				weight := value
				temp1.Outer(weight, mf.ItemFactor.RowView(index), mf.ItemFactor.RowView(index))
				a.Add(a, temp1)
				// Y^T C^u p(u)
				temp2.ScaleVec(weight+1, mf.ItemFactor.RowView(index))
				b.AddVec(b, temp2)
			})
			a.Add(a, regI)
			if err := temp1.Inverse(a); err != nil {
				log.Println(err)
				panic("A")
			}
			temp2.MulVec(temp1, b)
			mf.UserFactor.SetRow(u, temp2.RawVector().Data)
		}
		// Recompute all item factors: y_i = (X^T C^i X + \lambda reg)^{-1} X^T C^i p(i)
		// X^T X
		c.Mul(mf.UserFactor.T(), mf.UserFactor)
		// X Y^T
		p.Mul(mf.UserFactor, mf.ItemFactor.T())
		for i := 0; i < set.ItemCount(); i++ {
			a.Copy(c)
			b := mat.NewVecDense(mf.nFactors, nil)
			set.DenseItemRatings[i].ForEach(func(_, index int, value float64) {
				// X^T (C^i-I) X
				weight := value
				temp1.Outer(weight, mf.UserFactor.RowView(index), mf.UserFactor.RowView(index))
				a.Add(a, temp1)
				// X^T C^i p(i)
				temp2.ScaleVec(weight+1, mf.UserFactor.RowView(index))
				b.AddVec(b, temp2)
			})
			a.Add(a, regI)
			if err := temp1.Inverse(a); err != nil {
				log.Println(err)
				panic("B")
			}
			temp2.MulVec(temp1, b)
			mf.ItemFactor.SetRow(i, temp2.RawVector().Data)
		}
	}
}

func (mf *WRMF) weight(value float64) float64 {
	return mf.alpha * value
}
