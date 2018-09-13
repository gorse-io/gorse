package core

import (
	"github.com/gonum/floats"
	"math"
)

// Collaborative filtering based on co-clustering[1].
//
// [1] George, Thomas, and Srujana Merugu. "A scalable collaborative filtering
// framework based on co-clustering." Data Mining, Fifth IEEE international
// conference on. IEEE, 2005.
type CoClustering struct {
	globalMean       float64     // A^{global}
	userMeans        []float64   // A^{R}
	itemMeans        []float64   // A^{R}
	userClusters     []int       // p(i)
	itemClusters     []int       // y(i)
	userClusterMeans []float64   // A^{RC}
	itemClusterMeans []float64   // A^{CC}
	coClusterMeans   [][]float64 // A^{COC}
	trainSet         TrainSet
}

func NewCoClustering() *CoClustering {
	return new(CoClustering)
}

func (coc *CoClustering) Predict(userId, itemId int) float64 {
	// Convert to inner Id
	innerUserId := coc.trainSet.ConvertUserId(userId)
	innerItemId := coc.trainSet.ConvertItemId(itemId)
	prediction := 0.0
	if innerUserId != newId && innerItemId != newId {
		// old user - old item
		userCluster := coc.userClusters[innerUserId]
		itemCluster := coc.itemClusters[innerItemId]
		prediction = coc.userMeans[innerUserId] + coc.itemMeans[innerItemId] -
			coc.userClusterMeans[userCluster] - coc.itemClusterMeans[itemCluster] +
			coc.coClusterMeans[userCluster][itemCluster]
	} else if innerUserId != newId {
		// old user - new item
		prediction = coc.userMeans[innerUserId]
	} else if innerItemId != newId {
		// new user - old item
		prediction = coc.itemMeans[innerItemId]
	} else {
		// new user - new item
		prediction = coc.globalMean
	}
	return prediction
}

// Fit a co-clustering model.
// Parameters:
//	 nEpochs		- The number of iteration of the SGD procedure. Default is 20.
//	 nUserClusters	- The number of user clusters.
//	 nItemClusters	- The number of item clusters.
func (coc *CoClustering) Fit(trainSet TrainSet, params Parameters) {
	// Setup parameters
	nUserClusters := params.GetInt("nUserClusters", 3)
	nItemClusters := params.GetInt("nItemClusters", 3)
	nEpochs := params.GetInt("nEpochs", 20)
	// Initialize parameters
	coc.trainSet = trainSet
	coc.globalMean = trainSet.GlobalMean
	userRatings := trainSet.UserRatings()
	itemRatings := trainSet.ItemRatings()
	coc.userMeans = means(userRatings)
	coc.itemMeans = means(itemRatings)
	coc.userClusters = newUniformVectorInt(trainSet.UserCount, 0, nUserClusters)
	coc.itemClusters = newUniformVectorInt(trainSet.ItemCount, 0, nItemClusters)
	coc.userClusterMeans = make([]float64, nUserClusters)
	coc.itemClusterMeans = make([]float64, nItemClusters)
	coc.coClusterMeans = newZeroMatrix(nUserClusters, nItemClusters)
	// A^{tmp1}_{ij} = A_{ij} - A^R_i - A^C_j
	tmp1 := newNanMatrix(trainSet.UserCount, trainSet.ItemCount)
	for i := range tmp1 {
		for _, idRating := range userRatings[i] {
			tmp1[i][idRating.Id] = idRating.Rating - coc.userMeans[i] - coc.itemMeans[idRating.Id]
		}
	}
	// Clustering
	for ep := 0; ep < nEpochs; ep++ {
		// Compute averages A^{COC}, A^{RC}, A^{CC}, A^R, A^C
		clusterMean(coc.userClusterMeans, coc.userClusters, userRatings)
		clusterMean(coc.itemClusterMeans, coc.itemClusters, itemRatings)
		coClusterMean(coc.coClusterMeans, coc.userClusters, coc.itemClusters, userRatings)
		// Update row (user) cluster assignments
		for i := range coc.userClusters {
			bestCluster, leastCost := coc.userClusters[i], math.Inf(1)
			for k := 0; k < nUserClusters; k++ {
				// \sum^n_{j=1}W_{ij}(A^{tmp1}_{ij}-A^{COC)_{gy(j)}+A^{RC}_g+A^{RC}_{y(j)})^2
				cost := 0.0
				for _, ir := range userRatings[i] {
					temp := tmp1[i][ir.Id] -
						coc.coClusterMeans[k][coc.itemClusters[ir.Id]] +
						coc.userClusterMeans[k] +
						coc.itemClusterMeans[coc.itemClusters[ir.Id]]
					cost += temp * temp
				}
				if cost < leastCost {
					bestCluster = k
					leastCost = cost
				}
			}
			coc.userClusters[i] = bestCluster
		}
		// Update column (item) cluster assignments
		for j := range coc.itemClusters {
			bestCluster, leastCost := coc.itemClusters[j], math.Inf(1)
			for k := 0; k < nItemClusters; k++ {
				// \sum^m_{i=1}W_{ij}(A^{tmp1}_{ij}-A^{COC)_{p(i)h}+A^{RC}_{p(i)}+A^{RC}_h)^2
				cost := 0.0
				for _, ur := range itemRatings[j] {
					temp := tmp1[ur.Id][j] -
						coc.coClusterMeans[coc.userClusters[ur.Id]][k] +
						coc.userClusterMeans[coc.userClusters[ur.Id]] +
						coc.itemClusterMeans[k]
					cost += temp * temp
				}
				if cost < leastCost {
					bestCluster = k
					leastCost = cost
				}
			}
			coc.itemClusters[j] = bestCluster
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
