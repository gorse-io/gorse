package model

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/floats"
	"gonum.org/v1/gonum/mat"
	"log"
	"math"
)

// BPR means Bayesian Personal Ranking, is a pairwise learning algorithm for matrix factorization
// model with implicit feedback. The pairwise ranking between item i and j for user u is estimated
// by:
//
//   p(i >_u j) = \sigma( p_u^T (q_i - q_j) )
//
// Hyper-parameters:
//	 Reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.01.
//	 Lr 		- The learning rate of SGD. Default is 0.05.
//	 nFactors	- The number of latent factors. Default is 10.
//	 NEpochs	- The number of iteration of the SGD procedure. Default is 100.
//	 InitMean	- The mean of initial random latent factors. Default is 0.
//	 InitStdDev	- The standard deviation of initial random latent factors. Default is 0.001.
type BPR struct {
	Base
	// Model parameters
	UserFactor [][]float64 // p_u
	ItemFactor [][]float64 // q_i
	// Hyper parameters
	useBias    bool
	nFactors   int
	nEpochs    int
	lr         float64
	reg        float64
	initMean   float64
	initStdDev float64
	// Fallback model
	UserRatings []*base.MarginalSubSet
	ItemPop     *ItemPop
}

// NewBPR creates a BPR model.
func NewBPR(params base.Params) *BPR {
	bpr := new(BPR)
	bpr.SetParams(params)
	return bpr
}

// SetParams sets hyper-parameters of the BPR model.
func (bpr *BPR) SetParams(params base.Params) {
	bpr.Base.SetParams(params)
	// Setup hyper-parameters
	bpr.nFactors = bpr.Params.GetInt(base.NFactors, 10)
	bpr.nEpochs = bpr.Params.GetInt(base.NEpochs, 100)
	bpr.lr = bpr.Params.GetFloat64(base.Lr, 0.05)
	bpr.reg = bpr.Params.GetFloat64(base.Reg, 0.01)
	bpr.initMean = bpr.Params.GetFloat64(base.InitMean, 0)
	bpr.initStdDev = bpr.Params.GetFloat64(base.InitStdDev, 0.001)
}

// Predict by the BPR model.
func (bpr *BPR) Predict(userId, itemId string) float64 {
	// Convert sparse IDs to dense IDs
	userIndex := bpr.UserIndexer.ToIndex(userId)
	itemIndex := bpr.ItemIndexer.ToIndex(itemId)
	if userIndex == base.NotId || bpr.UserRatings[userIndex].Len() == 0 {
		// If users not exist in dataset, use ItemPop model.
		return bpr.ItemPop.Predict(userId, itemId)
	}
	return bpr.predict(userIndex, itemIndex)
}

func (bpr *BPR) predict(userIndex int, itemIndex int) float64 {
	ret := 0.0
	// + q_i^Tp_u
	if itemIndex != base.NotId && userIndex != base.NotId {
		userFactor := bpr.UserFactor[userIndex]
		itemFactor := bpr.ItemFactor[itemIndex]
		ret += floats.Dot(userFactor, itemFactor)
	}
	return ret
}

// Fit the BPR model.
func (bpr *BPR) Fit(trainSet core.DataSetInterface, options *base.RuntimeOptions) {
	options.Logf("Fit BPR with hyper-parameters: "+
		"n_factors = %v, n_epochs = %v, lr = %v, reg = %v, init_mean = %v, init_stddev = %v",
		bpr.nFactors, bpr.nEpochs, bpr.lr, bpr.reg, bpr.initMean, bpr.initStdDev)
	bpr.Init(trainSet)
	// Initialize parameters
	bpr.UserFactor = bpr.rng.NewNormalMatrix(trainSet.UserCount(), bpr.nFactors, bpr.initMean, bpr.initStdDev)
	bpr.ItemFactor = bpr.rng.NewNormalMatrix(trainSet.ItemCount(), bpr.nFactors, bpr.initMean, bpr.initStdDev)
	// Select fit function
	bpr.UserRatings = trainSet.Users()
	// Create item pop model
	bpr.ItemPop = NewItemPop(nil)
	bpr.ItemPop.Fit(trainSet, options)
	// Create buffers
	temp := make([]float64, bpr.nFactors)
	userFactor := make([]float64, bpr.nFactors)
	positiveItemFactor := make([]float64, bpr.nFactors)
	negativeItemFactor := make([]float64, bpr.nFactors)
	// Training
	for epoch := 0; epoch < bpr.nEpochs; epoch++ {
		// Training epoch
		cost := 0.0
		for i := 0; i < trainSet.Count(); i++ {
			// Select a user
			var userIndex, ratingCount int
			for {
				userIndex = bpr.rng.Intn(trainSet.UserCount())
				ratingCount = trainSet.UserByIndex(userIndex).Len()
				if ratingCount > 0 {
					break
				}
			}
			posIndex := trainSet.UserByIndex(userIndex).GetIndex(bpr.rng.Intn(ratingCount))
			// Select a negative sample
			negIndex := -1
			for {
				temp := bpr.rng.Intn(trainSet.ItemCount())
				tempId := bpr.ItemIndexer.ToID(temp)
				if !trainSet.UserByIndex(userIndex).Contain(tempId) {
					negIndex = temp
					break
				}
			}
			diff := bpr.predict(userIndex, posIndex) - bpr.predict(userIndex, negIndex)
			cost += math.Log(1 + math.Exp(-diff))
			grad := math.Exp(-diff) / (1.0 + math.Exp(-diff))
			// Pairwise update
			copy(userFactor, bpr.UserFactor[userIndex])
			copy(positiveItemFactor, bpr.ItemFactor[posIndex])
			copy(negativeItemFactor, bpr.ItemFactor[negIndex])
			// Update positive item latent factor: +w_u
			floats.MulConstTo(userFactor, grad, temp)
			floats.MulConstAddTo(positiveItemFactor, -bpr.reg, temp)
			floats.MulConstAddTo(temp, bpr.lr, bpr.ItemFactor[posIndex])
			// Update negative item latent factor: -w_u
			floats.MulConstTo(userFactor, -grad, temp)
			floats.MulConstAddTo(negativeItemFactor, -bpr.reg, temp)
			floats.MulConstAddTo(temp, bpr.lr, bpr.ItemFactor[negIndex])
			// Update user latent factor: h_i-h_j
			floats.SubTo(positiveItemFactor, negativeItemFactor, temp)
			floats.MulConst(temp, grad)
			floats.MulConstAddTo(userFactor, -bpr.reg, temp)
			floats.MulConstAddTo(temp, bpr.lr, bpr.UserFactor[userIndex])
		}
		options.Logf("epoch = %v/%v, loss = %v", epoch+1, bpr.nEpochs, cost)
	}
}

// ALS [7] is the Weighted Regularized Matrix Factorization, which exploits
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
type ALS struct {
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

// NewALS creates a ALS model.
func NewALS(params base.Params) *ALS {
	mf := new(ALS)
	mf.SetParams(params)
	return mf
}

// SetParams sets hyper-parameters for the ALS model.
func (mf *ALS) SetParams(params base.Params) {
	mf.Base.SetParams(params)
	mf.nFactors = mf.Params.GetInt(base.NFactors, 15)
	mf.nEpochs = mf.Params.GetInt(base.NEpochs, 50)
	mf.initMean = mf.Params.GetFloat64(base.InitMean, 0)
	mf.initStdDev = mf.Params.GetFloat64(base.InitStdDev, 0.1)
	mf.reg = mf.Params.GetFloat64(base.Reg, 0.06)
	mf.alpha = mf.Params.GetFloat64(base.Alpha, 1)
}

// Predict by the ALS model.
func (mf *ALS) Predict(userId, itemId string) float64 {
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

// Fit the ALS model.
func (mf *ALS) Fit(set core.DataSetInterface, options *base.RuntimeOptions) {
	options.Logf("Fit ALS with hyper-parameters: "+
		"n_factors = %v, n_epochs = %v, reg = %v, alpha = %v, init_mean = %v, init_stddev = %v",
		mf.nFactors, mf.nEpochs, mf.reg, mf.alpha, mf.initMean, mf.initStdDev)
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
				w := mf.weight(value)
				temp1.Outer(w-1, mf.ItemFactor.RowView(index), mf.ItemFactor.RowView(index))
				a.Add(a, temp1)
				// Y^T C^u p(u)
				temp2.ScaleVec(w, mf.ItemFactor.RowView(index))
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
				w := mf.weight(value)
				temp1.Outer(w-1, mf.UserFactor.RowView(index), mf.UserFactor.RowView(index))
				a.Add(a, temp1)
				// X^T C^i p(i)
				temp2.ScaleVec(w, mf.UserFactor.RowView(index))
				b.AddVec(b, temp2)
			})
			a.Add(a, regI)
			if err := temp1.Inverse(a); err != nil {
				log.Fatal(err)
			}
			temp2.MulVec(temp1, b)
			mf.ItemFactor.SetRow(i, temp2.RawVector().Data)
		}
		options.Logf("epoch = %v/%v", ep+1, mf.nEpochs)
	}
}

func (mf *ALS) weight(value float64) float64 {
	return mf.alpha*value + 1
}
