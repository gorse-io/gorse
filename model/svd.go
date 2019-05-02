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
	optimizer  string
	// Fallback model
	UserRatings []*base.MarginalSubSet
	ItemPop     *ItemPop
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
	svd.optimizer = svd.Params.GetString(base.Optimizer, base.SGD)
}

// Predict by the SVD model.
func (svd *SVD) Predict(userId int, itemId int) float64 {
	// Convert sparse IDs to dense IDs
	userIndex := svd.UserIndexer.ToIndex(userId)
	itemIndex := svd.ItemIndexer.ToIndex(itemId)
	switch svd.optimizer {
	case base.SGD:
		return svd.predict(userIndex, itemIndex)
	case base.BPR:
		if userIndex == base.NotId || svd.UserRatings[userIndex].Len() == 0 {
			// If users not exist in dataset, use ItemPop model.
			return svd.ItemPop.Predict(userId, itemId)
		}
		return svd.predict(userIndex, itemIndex)
	}
	panic(fmt.Sprintf("Unknown optimizer: %v", svd.optimizer))
}

func (svd *SVD) predict(userIndex int, itemIndex int) float64 {
	ret := svd.GlobalMean
	// + b_u
	if userIndex != base.NotId {
		ret += svd.UserBias[userIndex]
	}
	// + b_i
	if itemIndex != base.NotId {
		ret += svd.ItemBias[itemIndex]
	}
	// + q_i^Tp_u
	if itemIndex != base.NotId && userIndex != base.NotId {
		userFactor := svd.UserFactor[userIndex]
		itemFactor := svd.ItemFactor[itemIndex]
		ret += floats.Dot(userFactor, itemFactor)
	}
	return ret
}

// Fit the SVD model.
func (svd *SVD) Fit(trainSet core.DataSetInterface, options *base.RuntimeOptions) {
	svd.Init(trainSet)
	// Initialize parameters
	svd.GlobalMean = 0
	svd.UserBias = make([]float64, trainSet.UserCount())
	svd.ItemBias = make([]float64, trainSet.ItemCount())
	svd.UserFactor = svd.rng.NewNormalMatrix(trainSet.UserCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	svd.ItemFactor = svd.rng.NewNormalMatrix(trainSet.ItemCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	// Select fit function
	switch svd.optimizer {
	case base.SGD:
		svd.fitSGD(trainSet, options)
	case base.BPR:
		svd.fitBPR(trainSet, options)
	default:
		panic(fmt.Sprintf("Unknown optimizer: %v", svd.optimizer))
	}
}

func (svd *SVD) fitSGD(trainSet core.DataSetInterface, options *base.RuntimeOptions) {
	svd.GlobalMean = trainSet.GlobalMean()
	// Create buffers
	temp := make([]float64, svd.nFactors)
	userFactor := make([]float64, svd.nFactors)
	itemFactor := make([]float64, svd.nFactors)
	// Optimize
	for epoch := 0; epoch < svd.nEpochs; epoch++ {
		options.Logf("epoch = %v/%v", epoch+1, svd.nEpochs)
		for i := 0; i < trainSet.Count(); i++ {
			userIndex, itemIndex, rating := trainSet.GetWithIndex(i)
			// Compute error: e_{ui} = r - \hat r
			upGrad := rating - svd.predict(userIndex, itemIndex)
			if svd.useBias {
				userBias := svd.UserBias[userIndex]
				itemBias := svd.ItemBias[itemIndex]
				// Update user Bias: b_u <- b_u + \gamma (e_{ui} - \lambda b_u)
				gradUserBias := upGrad - svd.reg*userBias
				svd.UserBias[userIndex] += svd.lr * gradUserBias
				// Update item Bias: p_i <- p_i + \gamma (e_{ui} - \lambda b_i)
				gradItemBias := upGrad - svd.reg*itemBias
				svd.ItemBias[itemIndex] += svd.lr * gradItemBias
			}
			copy(userFactor, svd.UserFactor[userIndex])
			copy(itemFactor, svd.ItemFactor[itemIndex])
			// Update user latent factor
			floats.MulConstTo(itemFactor, upGrad, temp)
			floats.MulConstAddTo(userFactor, -svd.reg, temp)
			floats.MulConstAddTo(temp, svd.lr, svd.UserFactor[userIndex])
			// Update item latent factor
			floats.MulConstTo(userFactor, upGrad, temp)
			floats.MulConstAddTo(itemFactor, -svd.reg, temp)
			floats.MulConstAddTo(temp, svd.lr, svd.ItemFactor[itemIndex])
		}
	}
}

func (svd *SVD) fitBPR(trainSet core.DataSetInterface, options *base.RuntimeOptions) {
	svd.UserRatings = trainSet.Users()
	// Create item pop model
	svd.ItemPop = NewItemPop(nil)
	svd.ItemPop.Fit(trainSet, options)
	// Create buffers
	temp := make([]float64, svd.nFactors)
	userFactor := make([]float64, svd.nFactors)
	positiveItemFactor := make([]float64, svd.nFactors)
	negativeItemFactor := make([]float64, svd.nFactors)
	// Training
	for epoch := 0; epoch < svd.nEpochs; epoch++ {
		// Training epoch
		for i := 0; i < trainSet.Count(); i++ {
			// Select a user
			var userIndex, ratingCount int
			for {
				userIndex = svd.rng.Intn(trainSet.UserCount())
				ratingCount = trainSet.UserByIndex(userIndex).Len()
				if ratingCount > 0 {
					break
				}
			}
			posIndex := trainSet.UserByIndex(userIndex).GetIndex(svd.rng.Intn(ratingCount))
			// Select a negative sample
			negIndex := -1
			for {
				temp := svd.rng.Intn(trainSet.ItemCount())
				tempId := svd.ItemIndexer.ToID(temp)
				if !trainSet.UserByIndex(userIndex).Contain(tempId) {
					negIndex = temp
					break
				}
			}
			diff := svd.predict(userIndex, posIndex) - svd.predict(userIndex, negIndex)
			grad := math.Exp(-diff) / (1.0 + math.Exp(-diff))
			// Pairwise update
			copy(userFactor, svd.UserFactor[userIndex])
			copy(positiveItemFactor, svd.ItemFactor[posIndex])
			copy(negativeItemFactor, svd.ItemFactor[negIndex])
			// Update positive item latent factor: +w_u
			floats.MulConstTo(userFactor, grad, temp)
			floats.MulConstAddTo(positiveItemFactor, -svd.reg, temp)
			floats.MulConstAddTo(temp, svd.lr, svd.ItemFactor[posIndex])
			// Update negative item latent factor: -w_u
			floats.MulConstTo(userFactor, -grad, temp)
			floats.MulConstAddTo(negativeItemFactor, -svd.reg, temp)
			floats.MulConstAddTo(temp, svd.lr, svd.ItemFactor[negIndex])
			// Update user latent factor: h_i-h_j
			floats.SubTo(positiveItemFactor, negativeItemFactor, temp)
			floats.MulConst(temp, grad)
			floats.MulConstAddTo(userFactor, -svd.reg, temp)
			floats.MulConstAddTo(temp, svd.lr, svd.UserFactor[userIndex])
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
	userIndex := nmf.UserIndexer.ToIndex(userId)
	itemIndex := nmf.ItemIndexer.ToIndex(itemId)
	return nmf.predict(userIndex, itemIndex)
}

func (nmf *NMF) predict(userIndex int, itemIndex int) float64 {
	if itemIndex == base.NotId || userIndex == base.NotId ||
		math.IsNaN(nmf.UserFactor[userIndex][0]) ||
		math.IsNaN(nmf.ItemFactor[itemIndex][0]) {
		return nmf.GlobalMean
	}
	return floats.Dot(nmf.UserFactor[userIndex], nmf.ItemFactor[itemIndex])
}

// Fit the NMF model.
func (nmf *NMF) Fit(trainSet core.DataSetInterface, options *base.RuntimeOptions) {
	nmf.Init(trainSet)
	// Initialize parameters
	nmf.GlobalMean = trainSet.GlobalMean()
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
		for i := 0; i < trainSet.Count(); i++ {
			userIndex, itemIndex, rating := trainSet.GetWithIndex(i)
			prediction := nmf.predict(userIndex, itemIndex)
			// Update \sum_{i\in{I_u}} q_{if}⋅r_{ui}
			floats.MulConstAddTo(nmf.ItemFactor[itemIndex], rating, userNum[userIndex])
			// Update \sum_{i\in{I_u}} q_{if}⋅\hat{r}_{ui} + \lambda|I_u|p_{uf}
			floats.MulConstAddTo(nmf.ItemFactor[itemIndex], prediction, userDen[userIndex])
			floats.MulConstAddTo(nmf.UserFactor[userIndex], nmf.reg, userDen[userIndex])
			// Update \sum_{u\in{U_i}}p_{uf}⋅r_{ui}
			floats.MulConstAddTo(nmf.UserFactor[userIndex], rating, itemNum[itemIndex])
			// Update \sum_{u\in{U_i}}p_{uf}⋅\hat{r}_{ui} + \lambda|U_i|q_{if}
			floats.MulConstAddTo(nmf.UserFactor[userIndex], prediction, itemDen[itemIndex])
			floats.MulConstAddTo(nmf.ItemFactor[itemIndex], nmf.reg, itemDen[itemIndex])
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
	TrainSet   core.DataSetInterface
	UserFactor [][]float64 // p_u
	ItemFactor [][]float64 // q_i
	ImplFactor [][]float64 // y_i
	UserBias   []float64   // b_u
	ItemBias   []float64   // b_i
	GlobalMean float64     // mu
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
	userIndex := svd.UserIndexer.ToIndex(userId)
	itemIndex := svd.ItemIndexer.ToIndex(itemId)
	ret := svd.predict(userIndex, itemIndex, nil)
	return ret
}

func (svd *SVDpp) predict(userIndex int, itemIndex int, sumFactor []float64) float64 {
	ret := svd.GlobalMean
	// + b_u
	if userIndex != base.NotId {
		ret += svd.UserBias[userIndex]
	}
	// + b_i
	if itemIndex != base.NotId {
		ret += svd.ItemBias[itemIndex]
	}
	// + q_i^T\left(p_u + |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right)
	if itemIndex != base.NotId && userIndex != base.NotId {
		userFactor := svd.UserFactor[userIndex]
		itemFactor := svd.ItemFactor[itemIndex]
		if len(sumFactor) == 0 {
			sumFactor = svd.sumOverImplicitFactors(userIndex)
		}
		temp := make([]float64, len(itemFactor))
		floats.AddTo(userFactor, sumFactor, temp)
		ret += floats.Dot(temp, itemFactor)
	}
	return ret
}

func (svd *SVDpp) sumOverImplicitFactors(userIndex int) []float64 {
	sumFactor := make([]float64, svd.nFactors)
	// User history exists
	svd.TrainSet.UserByIndex(userIndex).ForEachIndex(func(i, index int, value float64) {
		floats.Add(sumFactor, svd.ImplFactor[index])
	})
	scale := math.Pow(float64(svd.TrainSet.UserByIndex(userIndex).Len()), -0.5)
	floats.MulConst(sumFactor, scale)
	return sumFactor
}

// Fit the SVD++ model.
func (svd *SVDpp) Fit(trainSet core.DataSetInterface, options *base.RuntimeOptions) {
	svd.Init(trainSet)
	// Initialize parameters
	svd.GlobalMean = trainSet.GlobalMean()
	svd.UserBias = make([]float64, trainSet.UserCount())
	svd.ItemBias = make([]float64, trainSet.ItemCount())
	svd.UserFactor = svd.rng.NewNormalMatrix(trainSet.UserCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	svd.ItemFactor = svd.rng.NewNormalMatrix(trainSet.ItemCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	svd.ImplFactor = svd.rng.NewNormalMatrix(trainSet.ItemCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	// Build user rating set
	svd.TrainSet = trainSet
	// Create buffers
	a := make([]float64, svd.nFactors)
	step := make([]float64, svd.nFactors)
	userFactor := make([]float64, svd.nFactors)
	itemFactor := make([]float64, svd.nFactors)
	c := base.NewMatrix(options.GetJobs(), svd.nFactors)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < svd.nEpochs; epoch++ {
		for userIndex := 0; userIndex < trainSet.UserCount(); userIndex++ {
			base.FillZeroVector(step)
			size := svd.TrainSet.UserByIndex(userIndex).Len()
			scale := math.Pow(float64(size), -0.5)
			sumFactor := svd.sumOverImplicitFactors(userIndex)
			trainSet.UserByIndex(userIndex).ForEachIndex(func(i, itemIndex int, rating float64) {
				userBias := svd.UserBias[userIndex]
				itemBias := svd.ItemBias[itemIndex]
				copy(userFactor, svd.UserFactor[userIndex])
				copy(itemFactor, svd.ItemFactor[itemIndex])
				// Compute error: e_{ui} = r - \hat r
				pred := svd.predict(userIndex, itemIndex, sumFactor)
				diff := rating - pred
				// Update user Bias: b_u <- b_u + \gamma (e_{ui} - \lambda b_u)
				gradUserBias := diff - svd.reg*userBias
				svd.UserBias[userIndex] += svd.lr * gradUserBias
				// Update item Bias: p_i <- p_i + \gamma (e_{ui} - \lambda b_i)
				gradItemBias := diff - svd.reg*itemBias
				svd.ItemBias[itemIndex] += svd.lr * gradItemBias
				// Update user latent factor
				floats.MulConstTo(itemFactor, diff, a)
				floats.MulConstAddTo(userFactor, -svd.reg, a)
				floats.MulConstAddTo(a, svd.lr, svd.UserFactor[userIndex])
				// Update item latent factor
				floats.AddTo(userFactor, sumFactor, a)
				floats.MulConst(a, diff)
				floats.MulConstAddTo(itemFactor, -svd.reg, a)
				floats.MulConstAddTo(a, svd.lr, svd.ItemFactor[itemIndex])
				// Update implicit latent factor: e_{ui}q_j|I_u|^{-1/2}
				floats.MulConstAddTo(itemFactor, scale*diff, step)
			})
			// Update implicit latent factor
			var wg sync.WaitGroup
			wg.Add(options.GetJobs())
			for j := 0; j < options.GetJobs(); j++ {
				go func(jobId int) {
					low := size * jobId / options.GetJobs()
					high := size * (jobId + 1) / options.GetJobs()
					a := c[jobId]
					for i := low; i < high; i++ {
						itemIndex := svd.TrainSet.UserByIndex(userIndex).Indices[i]
						implFactor := svd.ImplFactor[itemIndex]
						// a <- e_{ui}q_j|I_u|^{-1/2}
						floats.MulConstTo(step, 1/float64(size), a)
						// + \lambda y_k
						floats.MulConstAddTo(implFactor, -svd.reg, a)
						// \mu (e_{ui}q_j|I_u|^{-1/2} + \lambda y_k)
						floats.MulConstAddTo(a, svd.lr, svd.ImplFactor[itemIndex])
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
	// Fallback model
	UserRatings []*base.MarginalSubSet
	ItemPop     *ItemPop
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
	userIndex := mf.UserIndexer.ToIndex(userId)
	itemIndex := mf.ItemIndexer.ToIndex(itemId)
	if userIndex == base.NotId || mf.UserRatings[userIndex].Len() == 0 {
		// If users not exist in dataset, use ItemPop model.
		return mf.ItemPop.Predict(userId, itemId)
	}
	if itemIndex == base.NotId {
		return 0
	}
	return mat.Dot(mf.UserFactor.RowView(userIndex),
		mf.ItemFactor.RowView(itemIndex))
}

// Fit the WRMF model.
func (mf *WRMF) Fit(set core.DataSetInterface, options *base.RuntimeOptions) {
	mf.Init(set)
	// Create ItemPop model
	mf.UserRatings = set.Users()
	mf.ItemPop = NewItemPop(nil)
	mf.ItemPop.Fit(set, options)
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
			set.UserByIndex(u).ForEachIndex(func(_, index int, value float64) {
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
				log.Fatal(err)
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
			set.ItemByIndex(i).ForEachIndex(func(_, index int, value float64) {
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
				log.Fatal(err)
			}
			temp2.MulVec(temp1, b)
			mf.ItemFactor.SetRow(i, temp2.RawVector().Data)
		}
	}
}

func (mf *WRMF) weight(value float64) float64 {
	return mf.alpha * value
}
