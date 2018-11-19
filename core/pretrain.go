package core

import (
	"gonum.org/v1/gonum/floats"
	"math"
)

type MixtureSVD struct {
	Base
	// Parameters
	globalBias   float64
	prior        []float64
	itemBias     []float64
	userBias     []float64
	itemFactors  [][]float64
	userFactors  [][]float64
	userClusters [][]float64
	// Hyper-parameters
	nEpochs    int
	nFactors   int
	nClusters  int
	threshold  int
	initMean   float64
	initStdDev float64
	// Pre-allocated
	_globalBias float64
	_userBias   []float64
	_itemBias   []float64
	_userFactor [][]float64
	_itemFactor [][]float64
	tempV       []float64
	tempM       [][]float64
	a           []float64
	b           []float64
}

func NewClusterSVD(params Parameters) *MixtureSVD {
	svd := new(MixtureSVD)
	svd.SetParams(params)
	return svd
}

func (svd *MixtureSVD) SetParams(params Parameters) {
	svd.Base.SetParams(params)
	svd.nEpochs = svd.Params.GetInt("nEpochs", 20)
	svd.nFactors = params.GetInt("nFactors", 10)
	svd.nClusters = params.GetInt("nClusters", 20)
	svd.threshold = params.GetInt("threshold", 10)
	svd.initMean = svd.Params.GetFloat64("initMean", 0)
	svd.initStdDev = svd.Params.GetFloat64("initStdDev", 0.1)
}

func (svd *MixtureSVD) Fit(set TrainSet) {
	svd.Base.Fit(set)
	// Initialize
	svd.globalBias = set.GlobalMean
	svd.itemBias = svd.newNormalVector(set.ItemCount, svd.initMean, svd.initStdDev)
	svd.userBias = svd.newNormalVector(set.UserCount, svd.initMean, svd.initStdDev)
	svd.userFactors = svd.newNormalMatrix(svd.nClusters, svd.nFactors, svd.initMean, svd.initStdDev)
	svd.itemFactors = svd.newNormalMatrix(set.ItemCount, svd.nFactors, svd.initMean, svd.initStdDev)
	svd.userClusters = newZeroMatrix(set.UserCount, svd.nClusters)
	svd.prior = newVector(svd.nClusters, 1.0/float64(svd.nClusters))
	svd._userBias = make([]float64, set.UserCount)
	svd._itemBias = make([]float64, set.ItemCount)
	svd._userFactor = newZeroMatrix(svd.nClusters, svd.nFactors)
	svd._itemFactor = newZeroMatrix(set.ItemCount, svd.nFactors)
	svd.tempV = make([]float64, set.ItemCount)
	svd.tempM = newZeroMatrix(max([]int{svd.nClusters, set.ItemCount}), svd.nFactors)
	svd.a = make([]float64, svd.nFactors)
	svd.b = make([]float64, svd.nFactors)
	// EM algorithm
	for ep := 0; ep < svd.nEpochs; ep++ {
		// E step
		for i := range svd.userClusters {
			for k := range svd.userFactors {
				svd.userClusters[i][k] = 0.0
				for _, ir := range set.UserRatings()[i] {
					prediction := svd.predict(i, ir.Id, k)
					svd.userClusters[i][k] += -math.Pow(ir.Rating-prediction, 2.0) / 2
				}
			}
			softMax(svd.userClusters[i], svd.prior)
		}
		// M step
		// 1. Update \pi
		for k := range svd.prior {
			svd.prior[k] = 0
			for i := range svd.userClusters {
				svd.prior[k] += svd.userClusters[i][k]
			}
			svd.prior[k] /= float64(set.UserCount)
		}
		// 2. Update global mean
		svd._globalBias = 0.0
		root := 0.0
		for i := range svd.userClusters {
			for k := 0; k < svd.nClusters; k++ {
				for _, ir := range set.UserRatings()[i] {
					svd._globalBias += svd.userClusters[i][k] *
						(ir.Rating - floats.Dot(svd.userFactors[k], svd.itemFactors[ir.Id]) -
							svd.userBias[i] - svd.itemBias[ir.Id])
				}
				root += svd.userClusters[i][k] * float64(len(set.UserRatings()[i]))
			}
		}
		svd._globalBias /= root
		svd.globalBias = svd._globalBias
		// 3. Update user bias
		for i := range svd.userClusters {
			svd._userBias[i] = 0.0
			root = 0
			for k := 0; k < svd.nClusters; k++ {
				for _, ir := range set.UserRatings()[i] {
					svd._userBias[i] += svd.userClusters[i][k] *
						(ir.Rating - floats.Dot(svd.userFactors[k], svd.itemFactors[ir.Id]) -
							svd.globalBias - svd.itemBias[ir.Id])
				}
				root += svd.userClusters[i][k] * float64(len(set.UserRatings()[i]))
			}
			svd._userBias[i] /= root
		}
		copy(svd.userBias, svd._userBias)
		// 4. Update item bias
		resetZeroVector(svd._itemBias)
		resetZeroVector(svd.tempV)
		for i := range svd.userClusters {
			for k := 0; k < svd.nClusters; k++ {
				for _, ir := range set.UserRatings()[i] {
					svd._itemBias[ir.Id] += svd.userClusters[i][k] *
						(ir.Rating - floats.Dot(svd.userFactors[k], svd.itemFactors[ir.Id]) -
							svd.userBias[i] - svd.globalBias)
					svd.tempV[ir.Id] += svd.userClusters[i][k]
				}
			}
		}
		for i := range svd._itemBias {
			svd._itemBias[i] /= svd.tempV[i]
		}
		copy(svd.itemBias, svd._itemBias)
		// 5. Update user factor
		resetZeroMatrix(svd._userFactor)
		resetZeroMatrix(svd.tempM)
		for i := range svd.userClusters {
			for k := 0; k < svd.nClusters; k++ {
				for _, ir := range set.UserRatings()[i] {
					copy(svd.a, svd.itemFactors[ir.Id])
					mulConst(svd.userClusters[i][k]*(ir.Rating-svd.userBias[i]-svd.itemBias[ir.Id]-svd.globalBias), svd.a)
					floats.Add(svd._userFactor[k], svd.a)
					copy(svd.a, svd.itemFactors[ir.Id])
					floats.Mul(svd.a, svd.itemFactors[ir.Id])
					mulConst(svd.userClusters[i][k], svd.a)
					floats.Add(svd.tempM[k], svd.a)
				}
			}
		}
		//for k := range svd.userFactors {
		//	floats.Div(svd._userFactor[k], svd.tempM[k])
		//	copy(svd.userFactors[k], svd._userFactor[k])
		//}
		// 6. Update item factor
		resetZeroMatrix(svd._itemFactor)
		resetZeroMatrix(svd.tempM)
		for i := range svd.userClusters {
			for k := 0; k < svd.nClusters; k++ {
				for _, ir := range set.UserRatings()[i] {
					copy(svd.a, svd.userFactors[k])
					mulConst(svd.userClusters[i][k]*(ir.Rating-svd.userBias[i]-svd.itemBias[ir.Id]-svd.globalBias), svd.a)
					floats.Add(svd._itemFactor[ir.Id], svd.a)
					copy(svd.a, svd.userFactors[k])
					floats.Mul(svd.a, svd.userFactors[k])
					mulConst(svd.userClusters[i][k], svd.a)
					floats.Add(svd.tempM[ir.Id], svd.a)
				}
			}
		}
		//for i := range svd.itemFactors {
		//	floats.Div(svd._itemFactor[i], svd.tempM[i])
		//	copy(svd.itemFactors[i], svd._itemFactor[i])
		//}
	}
}

func (svd *MixtureSVD) Predict(userId, itemId int) float64 {
	// Convert to inner ID
	innerUserId := svd.Data.ConvertUserId(userId)
	innerItemId := svd.Data.ConvertItemId(itemId)
	// Return the global mean for new users and new items
	if innerUserId == NewId || innerItemId == NewId {
		return svd.Data.GlobalMean
	}
	// Prediction: \sum^K_{k=1}\pi_{ik}(\mu + b_i + b_j + u_k^Tv_j)
	prediction := 0.0
	for k := range svd.userClusters[innerUserId] {
		prediction += svd.userClusters[innerUserId][k] *
			(svd.globalBias + svd.userBias[innerUserId] + svd.itemBias[innerItemId] +
				floats.Dot(svd.userFactors[k], svd.itemFactors[innerItemId]))
	}
	return prediction
}

func (svd *MixtureSVD) predict(u, i, k int) float64 {
	return svd.globalBias + svd.userBias[u] + svd.itemBias[i] +
		floats.Dot(svd.userFactors[k], svd.itemFactors[i])
}

func softMax(dst []float64, weight []float64) {
	// Find the minimum
	max := floats.Max(dst)
	// SoftMax with numeric stability
	sum := 0.0
	for i := range dst {
		dst[i] -= max
		dst[i] = math.Exp(dst[i]) + 1e-10
		if weight != nil {
			dst[i] *= weight[i]
		}
		sum += dst[i]
	}
	divConst(sum, dst)
}
