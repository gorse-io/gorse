// Copyright 2020 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package model

import (
	"github.com/chewxy/math32"
	log "github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/floats"
	"gonum.org/v1/gonum/mat"
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
	UserFactor [][]float32 // p_u
	ItemFactor [][]float32 // q_i
	// Hyper parameters
	nFactors   int
	nEpochs    int
	lr         float32
	reg        float32
	initMean   float32
	initStdDev float32
}

// NewBPR creates a BPR model.
func NewBPR(params Params) *BPR {
	bpr := new(BPR)
	bpr.SetParams(params)
	return bpr
}

// SetParams sets hyper-parameters of the BPR model.
func (bpr *BPR) SetParams(params Params) {
	bpr.Base.SetParams(params)
	// Setup hyper-parameters
	bpr.nFactors = bpr.Params.GetInt(NFactors, 10)
	bpr.nEpochs = bpr.Params.GetInt(NEpochs, 100)
	bpr.lr = bpr.Params.GetFloat32(Lr, 0.05)
	bpr.reg = bpr.Params.GetFloat32(Reg, 0.01)
	bpr.initMean = bpr.Params.GetFloat32(InitMean, 0)
	bpr.initStdDev = bpr.Params.GetFloat32(InitStdDev, 0.001)
}

// Predict by the BPR model.
func (bpr *BPR) Predict(userId, itemId string) float32 {
	// Convert sparse Names to dense Names
	userIndex := bpr.UserIndex.ToNumber(userId)
	itemIndex := bpr.ItemIndex.ToNumber(itemId)
	if userIndex == NotId {
		log.Warn("unknown user:", userId)
	}
	if itemIndex == NotId {
		log.Warn("unknown item:", itemId)
	}
	return bpr.predict(userIndex, itemIndex)
}

func (bpr *BPR) predict(userIndex int, itemIndex int) float32 {
	ret := float32(0.0)
	// + q_i^Tp_u
	if itemIndex != NotId && userIndex != NotId {
		userFactor := bpr.UserFactor[userIndex]
		itemFactor := bpr.ItemFactor[itemIndex]
		ret += floats.Dot(userFactor, itemFactor)
	}
	return ret
}

// Fit the BPR model.
func (bpr *BPR) Fit(trainSet *DataSet, valSet *DataSet, config *FitConfig) {
	log.Info("fit BPR with hyper-parameters: "+
		"n_factors = %v, n_epochs = %v, lr = %v, reg = %v, init_mean = %v, init_stddev = %v",
		bpr.nFactors, bpr.nEpochs, bpr.lr, bpr.reg, bpr.initMean, bpr.initStdDev)
	bpr.Init(trainSet)
	// Create buffers
	temp := make([]float32, bpr.nFactors)
	userFactor := make([]float32, bpr.nFactors)
	positiveItemFactor := make([]float32, bpr.nFactors)
	negativeItemFactor := make([]float32, bpr.nFactors)
	// Convert array to hashmap
	userFeedback := make([]map[int]interface{}, trainSet.UserCount())
	for u := range userFeedback {
		userFeedback[u] = make(map[int]interface{})
		for _, i := range trainSet.UserFeedback[u] {
			userFeedback[u][i] = nil
		}
	}
	// Training
	for epoch := 1; epoch <= bpr.nEpochs; epoch++ {
		// Training epoch
		cost := float32(0.0)
		for i := 0; i < trainSet.Count(); i++ {
			// Select a user
			var userIndex, ratingCount int
			for {
				userIndex = bpr.rng.Intn(trainSet.UserCount())
				ratingCount = len(trainSet.UserFeedback[userIndex])
				if ratingCount > 0 {
					break
				}
			}
			posIndex := trainSet.UserFeedback[userIndex][bpr.rng.Intn(ratingCount)]
			// Select a negative sample
			negIndex := -1
			for {
				temp := bpr.rng.Intn(trainSet.ItemCount())
				if _, exist := userFeedback[userIndex][temp]; !exist {
					negIndex = temp
					break
				}
			}
			diff := bpr.predict(userIndex, posIndex) - bpr.predict(userIndex, negIndex)
			cost += math32.Log(1 + math32.Exp(-diff))
			grad := math32.Exp(-diff) / (1.0 + math32.Exp(-diff))
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
		log.Info("epoch = %v/%v, loss = %v", epoch+1, bpr.nEpochs, cost)
		// Cross validation
		if epoch%config.Verbose == 0 || epoch == bpr.nEpochs {
			scores := Evaluate(bpr, valSet, trainSet, config.TopK, config.Candidates, NDCG, Precision, Recall)
			log.Infof("NDCG@{}={}, Precision@{}={}, Recall@{}={}",
				config.TopK, scores[0], config.TopK, scores[1], config.TopK, scores[2])
		}
	}
}

func (bpr *BPR) Init(trainSet *DataSet) {
	// Initialize parameters
	newUserFactor := bpr.rng.NormalMatrix(trainSet.UserCount(), bpr.nFactors, bpr.initMean, float32(bpr.initStdDev))
	newItemFactor := bpr.rng.NormalMatrix(trainSet.ItemCount(), bpr.nFactors, bpr.initMean, float32(bpr.initStdDev))
	// Relocate parameters
	for _, userId := range trainSet.UserIndex.Names {
		oldIndex := bpr.UserIndex.ToNumber(userId)
		newIndex := trainSet.UserIndex.ToNumber(userId)
		if oldIndex != NotId {
			newUserFactor[newIndex] = bpr.UserFactor[oldIndex]
		}
	}
	for _, itemId := range trainSet.ItemIndex.Names {
		oldIndex := bpr.ItemIndex.ToNumber(itemId)
		newIndex := trainSet.ItemIndex.ToNumber(itemId)
		if oldIndex != NotId {
			newItemFactor[newIndex] = bpr.ItemFactor[oldIndex]
		}
	}
	// Initialize base
	bpr.UserFactor = newUserFactor
	bpr.ItemFactor = newItemFactor
	bpr.Base.Init(trainSet)
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
}

// NewALS creates a ALS model.
func NewALS(params Params) *ALS {
	als := new(ALS)
	als.SetParams(params)
	return als
}

// SetParams sets hyper-parameters for the ALS model.
func (als *ALS) SetParams(params Params) {
	als.Base.SetParams(params)
	als.nFactors = als.Params.GetInt(NFactors, 15)
	als.nEpochs = als.Params.GetInt(NEpochs, 50)
	als.initMean = als.Params.GetFloat64(InitMean, 0)
	als.initStdDev = als.Params.GetFloat64(InitStdDev, 0.1)
	als.reg = als.Params.GetFloat64(Reg, 0.06)
	als.alpha = als.Params.GetFloat64(Alpha, 1)
}

// Predict by the ALS model.
func (als *ALS) Predict(userId, itemId string) float32 {
	userIndex := als.UserIndex.ToNumber(userId)
	itemIndex := als.ItemIndex.ToNumber(itemId)
	if userIndex == NotId {
		log.Info("unknown user:", userId)
		return 0
	}
	if itemIndex == NotId {
		log.Info("unknown item:", itemId)
		return 0
	}
	return float32(mat.Dot(als.UserFactor.RowView(userIndex),
		als.ItemFactor.RowView(itemIndex)))
}

// Fit the ALS model.
func (als *ALS) Fit(trainSet *DataSet, valSet *DataSet, config *FitConfig) {
	log.Info("fit ALS with hyper-parameters: "+
		"n_factors = %v, n_epochs = %v, reg = %v, alpha = %v, init_mean = %v, init_stddev = %v",
		als.nFactors, als.nEpochs, als.reg, als.alpha, als.initMean, als.initStdDev)
	als.Init(trainSet)
	// Create temporary matrix
	temp1 := mat.NewDense(als.nFactors, als.nFactors, nil)
	temp2 := mat.NewVecDense(als.nFactors, nil)
	a := mat.NewDense(als.nFactors, als.nFactors, nil)
	c := mat.NewDense(als.nFactors, als.nFactors, nil)
	p := mat.NewDense(trainSet.UserCount(), trainSet.ItemCount(), nil)
	// Create regularization matrix
	regs := make([]float64, als.nFactors)
	for i := range regs {
		regs[i] = als.reg
	}
	regI := mat.NewDiagDense(als.nFactors, regs)
	for ep := 1; ep <= als.nEpochs; ep++ {
		// Recompute all user factors: x_u = (Y^T C^u Y + \lambda reg)^{-1} Y^T C^u p(u)
		// Y^T Y
		c.Mul(als.ItemFactor.T(), als.ItemFactor)
		// X Y^T
		p.Mul(als.UserFactor, als.ItemFactor.T())
		for u := 0; u < trainSet.UserCount(); u++ {
			a.Copy(c)
			b := mat.NewVecDense(als.nFactors, nil)
			for _, index := range trainSet.UserFeedback[u] {
				// Y^T (C^u-I) Y
				w := als.weight(1)
				temp1.Outer(w-1, als.ItemFactor.RowView(index), als.ItemFactor.RowView(index))
				a.Add(a, temp1)
				// Y^T C^u p(u)
				temp2.ScaleVec(w, als.ItemFactor.RowView(index))
				b.AddVec(b, temp2)
			}
			a.Add(a, regI)
			if err := temp1.Inverse(a); err != nil {
				log.Fatal(err)
			}
			temp2.MulVec(temp1, b)
			als.UserFactor.SetRow(u, temp2.RawVector().Data)
		}
		// Recompute all item factors: y_i = (X^T C^i X + \lambda reg)^{-1} X^T C^i p(i)
		// X^T X
		c.Mul(als.UserFactor.T(), als.UserFactor)
		// X Y^T
		p.Mul(als.UserFactor, als.ItemFactor.T())
		for i := 0; i < trainSet.ItemCount(); i++ {
			a.Copy(c)
			b := mat.NewVecDense(als.nFactors, nil)
			for _, index := range trainSet.ItemFeedback[i] {
				// X^T (C^i-I) X
				w := als.weight(1)
				temp1.Outer(w-1, als.UserFactor.RowView(index), als.UserFactor.RowView(index))
				a.Add(a, temp1)
				// X^T C^i p(i)
				temp2.ScaleVec(w, als.UserFactor.RowView(index))
				b.AddVec(b, temp2)
			}
			a.Add(a, regI)
			if err := temp1.Inverse(a); err != nil {
				log.Fatal(err)
			}
			temp2.MulVec(temp1, b)
			als.ItemFactor.SetRow(i, temp2.RawVector().Data)
		}
		log.Info("epoch = %v/%v", ep+1, als.nEpochs)
		// Cross validation
		if ep%config.Verbose == 0 || ep == als.nEpochs {
			scores := Evaluate(als, valSet, trainSet, config.TopK, config.Candidates, NDCG, Precision, Recall)
			log.Infof("NDCG@{}={}, Precision@{}={}, Recall@{}={}",
				config.TopK, scores[0], config.TopK, scores[1], config.TopK, scores[2])
		}
	}
}

func (als *ALS) weight(value float64) float64 {
	return als.alpha*value + 1
}

func (als *ALS) Init(trainSet *DataSet) {
	// Initialize
	newUserFactor := mat.NewDense(trainSet.UserCount(), als.nFactors,
		als.rng.NormalVector64(trainSet.UserCount()*als.nFactors, als.initMean, als.initStdDev))
	newItemFactor := mat.NewDense(trainSet.ItemCount(), als.nFactors,
		als.rng.NormalVector64(trainSet.ItemCount()*als.nFactors, als.initMean, als.initStdDev))
	// Relocate parameters
	for _, userId := range trainSet.UserIndex.Names {
		oldIndex := als.UserIndex.ToNumber(userId)
		newIndex := trainSet.UserIndex.ToNumber(userId)
		if oldIndex != NotId {
			newUserFactor.SetRow(newIndex, als.UserFactor.RawRowView(oldIndex))
		}
	}
	for _, itemId := range trainSet.ItemIndex.Names {
		oldIndex := als.ItemIndex.ToNumber(itemId)
		newIndex := trainSet.ItemIndex.ToNumber(itemId)
		if oldIndex != NotId {
			newItemFactor.SetRow(newIndex, als.ItemFactor.RawRowView(oldIndex))
		}
	}
	// Initialize base
	als.UserFactor = newUserFactor
	als.ItemFactor = newItemFactor
	als.Base.Init(trainSet)
}
