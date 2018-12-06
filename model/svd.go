package model

import (
	. "github.com/zhenghaoz/gorse/base"
	. "github.com/zhenghaoz/gorse/core"
	"gonum.org/v1/gonum/floats"
	"log"
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
}

// NewSVD creates a SVD model. Params:
//   UseBias    - Add useBias in SVD model. Default is true.
//	 Reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.02.
//	 Lr 		- The learning rate of SGD. Default is 0.005.
//	 nFactors	- The number of latent factors. Default is 100.
//	 NEpochs	- The number of iteration of the SGD procedure. Default is 20.
//	 InitMean	- The mean of initial random latent factors. Default is 0.
//	 InitStdDev	- The standard deviation of initial random latent factors. Default is 0.1.
func NewSVD(params Params) *SVD {
	svd := new(SVD)
	svd.SetParams(params)
	return svd
}

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

func (svd *SVD) Predict(userId int, itemId int) float64 {
	denseUserId := svd.UserIdSet.ToDenseId(userId)
	denseItemId := svd.ItemIdSet.ToDenseId(itemId)
	return svd.predict(denseUserId, denseItemId)
}

func (svd *SVD) predict(denseUserId int, denseItemId int) float64 {
	ret := svd.GlobalBias
	// + b_u
	if denseUserId != NotId {
		ret += svd.UserBias[denseUserId]
	}
	// + b_i
	if denseItemId != NotId {
		ret += svd.ItemBias[denseItemId]
	}
	// + q_i^Tp_u
	if denseItemId != NotId && denseUserId != NotId {
		userFactor := svd.UserFactor[denseUserId]
		itemFactor := svd.ItemFactor[denseItemId]
		ret += floats.Dot(userFactor, itemFactor)
	}
	return ret
}

func (svd *SVD) Fit(trainSet TrainSet, options ...RuntimeOption) {
	svd.Init(trainSet, options)
	// Initialize parameters
	svd.UserBias = make([]float64, trainSet.UserCount())
	svd.ItemBias = make([]float64, trainSet.ItemCount())
	svd.UserFactor = svd.rng.MakeNormalMatrix(trainSet.UserCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	svd.ItemFactor = svd.rng.MakeNormalMatrix(trainSet.ItemCount(), svd.nFactors, svd.initMean, svd.initStdDev)
	// Create buffers
	a := make([]float64, svd.nFactors)
	b := make([]float64, svd.nFactors)
	// Optimize
	for epoch := 0; epoch < svd.nEpochs; epoch++ {
		perm := svd.rng.Perm(trainSet.Length())
		for _, i := range perm {
			denseUserId, denseItemId, rating := trainSet.GetDense(i)
			// Compute error
			upGrad := rating - svd.predict(denseUserId, denseItemId)
			if svd.useBias {
				userBias := svd.UserBias[denseUserId]
				itemBias := svd.ItemBias[denseItemId]
				// Update global Bias
				gradGlobalBias := upGrad
				svd.GlobalBias += svd.lr * gradGlobalBias
				// Update user Bias
				gradUserBias := upGrad + svd.reg*userBias
				svd.UserBias[denseUserId] += svd.lr * gradUserBias
				// Update item Bias
				gradItemBias := upGrad + svd.reg*itemBias
				svd.ItemBias[denseItemId] += svd.lr * gradItemBias
			}
			userFactor := svd.UserFactor[denseUserId]
			itemFactor := svd.ItemFactor[denseItemId]
			// Update user latent factor
			copy(a, itemFactor)
			MulConst(upGrad, a)
			copy(b, userFactor)
			MulConst(svd.reg, b)
			floats.Sub(a, b)
			MulConst(svd.lr, a)
			floats.Add(svd.UserFactor[denseUserId], a)
			// Update item latent factor
			copy(a, userFactor)
			MulConst(upGrad, a)
			copy(b, itemFactor)
			MulConst(svd.reg, b)
			floats.Sub(a, b)
			MulConst(svd.lr, a)
			floats.Add(svd.ItemFactor[denseItemId], a)
		}
	}
}

// PairUpdate updates model parameters by pair.
//func (svd *SVD) PairUpdate(upGrad float64, innerUserId, positiveItemId, negativeItemId int) {
//	userFactor := svd.UserFactor[innerUserId]
//	positiveItemFactor := svd.ItemFactor[positiveItemId]
//	negativeItemFactor := svd.ItemFactor[negativeItemId]
//	// Update positive item latent factor: +w_u
//	copy(svd.a, userFactor)
//	MulConst(upGrad, svd.a)
//	MulConst(svd.lr, svd.a)
//	floats.Add(svd.ItemFactor[positiveItemId], svd.a)
//	// Update negative item latent factor: -w_u
//	copy(svd.a, userFactor)
//	Neg(svd.a)
//	MulConst(upGrad, svd.a)
//	MulConst(svd.lr, svd.a)
//	floats.Add(svd.ItemFactor[negativeItemId], svd.a)
//	// Update user latent factor: h_i-h_j
//	copy(svd.a, positiveItemFactor)
//	floats.Sub(svd.a, negativeItemFactor)
//	MulConst(upGrad, svd.a)
//	MulConst(svd.lr, svd.a)
//	floats.Add(svd.UserFactor[innerUserId], svd.a)
//}

/* NMF */

// NMF: Non-negative Matrix Factorization[3].
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
	denseUserId := nmf.UserIdSet.ToDenseId(userId)
	denseItemId := nmf.ItemIdSet.ToDenseId(itemId)
	return nmf.predict(denseUserId, denseItemId)
}

func (nmf *NMF) predict(denseUserId int, denseItemId int) float64 {
	if denseItemId != NotId && denseUserId != NotId {
		return floats.Dot(nmf.UserFactor[denseUserId], nmf.ItemFactor[denseItemId])
	}
	return 0
}

func (nmf *NMF) Fit(trainSet TrainSet, options ...RuntimeOption) {
	nmf.Init(trainSet, options)
	// Initialize parameters
	nmf.UserFactor = nmf.rng.MakeUniformMatrix(trainSet.UserCount(), nmf.nFactors, nmf.initLow, nmf.initHigh)
	nmf.ItemFactor = nmf.rng.MakeUniformMatrix(trainSet.ItemCount(), nmf.nFactors, nmf.initLow, nmf.initHigh)
	// Create intermediate matrix buffer
	buffer := make([]float64, nmf.nFactors)
	userUp := MakeMatrix(trainSet.UserCount(), nmf.nFactors)
	userDown := MakeMatrix(trainSet.UserCount(), nmf.nFactors)
	itemUp := MakeMatrix(trainSet.ItemCount(), nmf.nFactors)
	itemDown := MakeMatrix(trainSet.ItemCount(), nmf.nFactors)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < nmf.nEpochs; epoch++ {
		// Reset intermediate matrices
		ResetMatrix(userUp)
		ResetMatrix(userDown)
		ResetMatrix(itemUp)
		ResetMatrix(itemDown)
		// Calculate intermediate matrices
		for i := 0; i < trainSet.Length(); i++ {
			denseUserId, denseItemId, rating := trainSet.GetDense(i)
			prediction := nmf.predict(denseUserId, denseItemId)
			// Update userUp
			copy(buffer, nmf.ItemFactor[denseItemId])
			MulConst(rating, buffer)
			floats.Add(userUp[denseUserId], buffer)
			// Update userDown
			copy(buffer, nmf.ItemFactor[denseItemId])
			MulConst(prediction, buffer)
			floats.Add(userDown[denseUserId], buffer)
			copy(buffer, nmf.UserFactor[denseUserId])
			MulConst(nmf.reg, buffer)
			floats.Add(userDown[denseUserId], buffer)
			// Update itemUp
			copy(buffer, nmf.UserFactor[denseUserId])
			MulConst(rating, buffer)
			floats.Add(itemUp[denseItemId], buffer)
			// Update itemDown
			copy(buffer, nmf.UserFactor[denseUserId])
			MulConst(prediction, buffer)
			floats.Add(itemDown[denseItemId], buffer)
			copy(buffer, nmf.ItemFactor[denseItemId])
			MulConst(nmf.reg, buffer)
			floats.Add(itemDown[denseItemId], buffer)
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
	svd.Base.SetParams(params)
	// Setup parameters
	svd.nFactors = svd.Params.GetInt(NFactors, 20)
	svd.nEpochs = svd.Params.GetInt(NEpochs, 20)
	log.Print(svd.nEpochs)
	svd.lr = svd.Params.GetFloat64(Lr, 0.007)
	svd.reg = svd.Params.GetFloat64(Reg, 0.02)
	svd.initMean = svd.Params.GetFloat64(InitMean, 0)
	svd.initStdDev = svd.Params.GetFloat64(InitStdDev, 0.1)
}

func (svd *SVDpp) Predict(userId int, itemId int) float64 {
	denseUserId := svd.UserIdSet.ToDenseId(userId)
	denseItemId := svd.ItemIdSet.ToDenseId(itemId)
	ret, _ := svd.predict(denseUserId, denseItemId)
	return ret
}

func (svd *SVDpp) predict(denseUserId int, denseItemId int) (float64, []float64) {
	ret := svd.GlobalBias
	// + b_u
	if denseUserId != NotId {
		ret += svd.UserBias[denseUserId]
	}
	// + b_i
	if denseItemId != NotId {
		ret += svd.ItemBias[denseItemId]
	}
	// + q_i^T\left(p_u + |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right)
	if denseItemId != NotId && denseUserId != NotId {
		userFactor := svd.UserFactor[denseUserId]
		itemFactor := svd.ItemFactor[denseItemId]
		sumImpFactor := svd.summarizeImplFactors(denseUserId)
		temp := make([]float64, len(itemFactor))
		floats.Add(temp, userFactor)
		floats.Add(temp, sumImpFactor)
		ret += floats.Dot(temp, itemFactor)
		return ret, sumImpFactor
	}
	return ret, []float64{}
}

func (svd *SVDpp) summarizeImplFactors(denseUserId int) []float64 {
	sumImpFactor := make([]float64, svd.nFactors)
	// User history exists
	count := 0
	svd.UserRatings[denseUserId].ForEach(func(i, index int, value float64) {
		floats.Add(sumImpFactor, svd.ImplFactor[index])
		count++
	})
	DivConst(math.Sqrt(float64(count)), sumImpFactor)
	return sumImpFactor
}

func (svd *SVDpp) Fit(trainSet TrainSet, setters ...RuntimeOption) {
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
			denseUserId, denseItemId, rating := trainSet.GetDense(i)
			userBias := svd.UserBias[denseUserId]
			itemBias := svd.ItemBias[denseItemId]
			userFactor := svd.UserFactor[denseUserId]
			itemFactor := svd.ItemFactor[denseItemId]
			// Compute error
			pred, emImpFactor := svd.predict(denseUserId, denseItemId)
			diff := pred - rating
			// Update global Bias
			gradGlobalBias := diff
			svd.GlobalBias -= svd.lr * gradGlobalBias
			// Update user Bias
			gradUserBias := diff + svd.reg*userBias
			svd.UserBias[denseUserId] -= svd.lr * gradUserBias
			// Update item Bias
			gradItemBias := diff + svd.reg*itemBias
			svd.ItemBias[denseItemId] -= svd.lr * gradItemBias
			// Update user latent factor
			copy(a, itemFactor)
			MulConst(diff, a)
			copy(b, userFactor)
			MulConst(svd.reg, b)
			floats.Add(a, b)
			MulConst(svd.lr, a)
			floats.Sub(svd.UserFactor[denseUserId], a)
			// Update item latent factor
			copy(a, userFactor)
			if len(emImpFactor) > 0 {
				floats.Add(a, emImpFactor)
			}
			MulConst(diff, a)
			copy(b, itemFactor)
			MulConst(svd.reg, b)
			floats.Add(a, b)
			MulConst(svd.lr, a)
			floats.Sub(svd.ItemFactor[denseItemId], a)
			// Update implicit latent factor
			nRating := svd.UserRatings[denseUserId].Len()
			var wg sync.WaitGroup
			wg.Add(svd.rtOptions.NJobs)
			for j := 0; j < svd.rtOptions.NJobs; j++ {
				go func(jobId int) {
					low := nRating * jobId / svd.rtOptions.NJobs
					high := nRating * (jobId + 1) / svd.rtOptions.NJobs
					a := make([]float64, svd.nFactors)
					b := make([]float64, svd.nFactors)
					for i := low; i < high; i++ {
						implFactor := svd.ImplFactor[svd.UserRatings[denseUserId].Indices[i]]
						copy(a, itemFactor)
						MulConst(diff, a)
						DivConst(math.Sqrt(float64(svd.UserRatings[denseUserId].Len())), a)
						copy(b, implFactor)
						MulConst(svd.reg, b)
						floats.Add(a, b)
						MulConst(svd.lr, a)
						floats.Sub(svd.ImplFactor[svd.UserRatings[denseUserId].Indices[i]], a)
					}
					wg.Done()
				}(j)
			}
			wg.Wait()
		}
	}
}
