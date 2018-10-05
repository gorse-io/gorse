package core

import (
	"gonum.org/v1/gonum/floats"
	"math"
)

// CoClustering: Collaborative filtering based on co-clustering[1].
//
// [1] George, Thomas, and Srujana Merugu. "A scalable collaborative filtering
// framework based on co-clustering." Data Mining, Fifth IEEE international
// conference on. IEEE, 2005.
type CoClustering struct {
	Base
	GlobalMean       float64     // A^{global}
	UserMeans        []float64   // A^{R}
	ItemMeans        []float64   // A^{R}
	UserClusters     []int       // p(i)
	ItemClusters     []int       // y(i)
	UserClusterMeans []float64   // A^{RC}
	ItemClusterMeans []float64   // A^{CC}
	CoClusterMeans   [][]float64 // A^{COC}
}

// NewCoClustering creates a co-clustering model. Parameters:
//   nEpochs       - The number of iteration of the SGD procedure. Default is 20.
//   nUserClusters - The number of user clusters. Default is 3.
//   nItemClusters - The number of item clusters. Default is 3.
//   randState     - The random seed. Default is UNIX time step.
func NewCoClustering(params Parameters) *CoClustering {
	cc := new(CoClustering)
	cc.Params = params
	return cc
}

// Predict by a co-clustering model.
func (coc *CoClustering) Predict(userId, itemId int) float64 {
	// Convert to inner Id
	innerUserId := coc.Data.ConvertUserId(userId)
	innerItemId := coc.Data.ConvertItemId(itemId)
	prediction := 0.0
	if innerUserId != NewId && innerItemId != NewId {
		// old user - old item
		userCluster := coc.UserClusters[innerUserId]
		itemCluster := coc.ItemClusters[innerItemId]
		prediction = coc.UserMeans[innerUserId] + coc.ItemMeans[innerItemId] -
			coc.UserClusterMeans[userCluster] - coc.ItemClusterMeans[itemCluster] +
			coc.CoClusterMeans[userCluster][itemCluster]
	} else if innerUserId != NewId {
		// old user - new item
		prediction = coc.UserMeans[innerUserId]
	} else if innerItemId != NewId {
		// new user - old item
		prediction = coc.ItemMeans[innerItemId]
	} else {
		// new user - new item
		prediction = coc.GlobalMean
	}
	return prediction
}

// Fit a co-clustering model.
func (coc *CoClustering) Fit(trainSet TrainSet) {
	coc.Base.Fit(trainSet)
	// Setup parameters
	nUserClusters := coc.Params.GetInt("nUserClusters", 3)
	nItemClusters := coc.Params.GetInt("nItemClusters", 3)
	nEpochs := coc.Params.GetInt("nEpochs", 20)
	// Initialize parameters
	coc.GlobalMean = trainSet.GlobalMean
	userRatings := trainSet.UserRatings()
	itemRatings := trainSet.ItemRatings()
	coc.UserMeans = means(userRatings)
	coc.ItemMeans = means(itemRatings)
	coc.UserClusters = coc.newUniformVectorInt(trainSet.UserCount, 0, nUserClusters)
	coc.ItemClusters = coc.newUniformVectorInt(trainSet.ItemCount, 0, nItemClusters)
	coc.UserClusterMeans = make([]float64, nUserClusters)
	coc.ItemClusterMeans = make([]float64, nItemClusters)
	coc.CoClusterMeans = newZeroMatrix(nUserClusters, nItemClusters)
	// A^{tmp1}_{ij} = A_{ij} - A^R_i - A^C_j
	tmp1 := newNanMatrix(trainSet.UserCount, trainSet.ItemCount)
	for i := range tmp1 {
		for _, idRating := range userRatings[i] {
			tmp1[i][idRating.Id] = idRating.Rating - coc.UserMeans[i] - coc.ItemMeans[idRating.Id]
		}
	}
	// Clustering
	for ep := 0; ep < nEpochs; ep++ {
		// Compute averages A^{COC}, A^{RC}, A^{CC}, A^R, A^C
		clusterMean(coc.UserClusterMeans, coc.UserClusters, userRatings)
		clusterMean(coc.ItemClusterMeans, coc.ItemClusters, itemRatings)
		coClusterMean(coc.CoClusterMeans, coc.UserClusters, coc.ItemClusters, userRatings)
		// Update row (user) cluster assignments
		for i := range coc.UserClusters {
			bestCluster, leastCost := coc.UserClusters[i], math.Inf(1)
			for k := 0; k < nUserClusters; k++ {
				// \sum^n_{j=1}W_{ij}(A^{tmp1}_{ij}-A^{COC)_{gy(j)}+A^{RC}_g+A^{RC}_{y(j)})^2
				cost := 0.0
				for _, ir := range userRatings[i] {
					temp := tmp1[i][ir.Id] -
						coc.CoClusterMeans[k][coc.ItemClusters[ir.Id]] +
						coc.UserClusterMeans[k] +
						coc.ItemClusterMeans[coc.ItemClusters[ir.Id]]
					cost += temp * temp
				}
				if cost < leastCost {
					bestCluster = k
					leastCost = cost
				}
			}
			coc.UserClusters[i] = bestCluster
		}
		// Update column (item) cluster assignments
		for j := range coc.ItemClusters {
			bestCluster, leastCost := coc.ItemClusters[j], math.Inf(1)
			for k := 0; k < nItemClusters; k++ {
				// \sum^m_{i=1}W_{ij}(A^{tmp1}_{ij}-A^{COC)_{p(i)h}+A^{RC}_{p(i)}+A^{RC}_h)^2
				cost := 0.0
				for _, ur := range itemRatings[j] {
					temp := tmp1[ur.Id][j] -
						coc.CoClusterMeans[coc.UserClusters[ur.Id]][k] +
						coc.UserClusterMeans[coc.UserClusters[ur.Id]] +
						coc.ItemClusterMeans[k]
					cost += temp * temp
				}
				if cost < leastCost {
					bestCluster = k
					leastCost = cost
				}
			}
			coc.ItemClusters[j] = bestCluster
		}
	}
}

func clusterMean(dst []float64, clusters []int, idRatings [][]IdRating) {
	resetZeroVector(dst)
	count := make([]float64, len(dst))
	for id, cluster := range clusters {
		for _, ir := range idRatings[id] {
			dst[cluster] += ir.Rating
			count[cluster]++
		}
	}
	floats.Div(dst, count)
}

func coClusterMean(dst [][]float64, userClusters, itemClusters []int, userRatings [][]IdRating) {
	resetZeroMatrix(dst)
	count := newZeroMatrix(len(dst), len(dst[0]))
	for userId, userCluster := range userClusters {
		for _, ir := range userRatings[userId] {
			itemCluster := itemClusters[ir.Id]
			count[userCluster][itemCluster]++
			dst[userCluster][itemCluster] += ir.Rating
		}
	}
	for i := range dst {
		for j := range dst[i] {
			dst[i][j] /= count[i][j]
		}
	}
}
