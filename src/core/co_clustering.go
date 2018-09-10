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
	globalMean       float64
	userMeans        []float64
	itemMeans        []float64
	userClusters     []int
	itemClusters     []int
	userClusterMeans []float64
	itemClusterMeans []float64
	coClusterMeans   [][]float64
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
	} else if innerItemId == newId {
		// old user - new item
		prediction = coc.userMeans[innerUserId]
	} else if innerUserId == newId {
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
	reader := newParameterReader(params)
	nUserClusters := reader.getInt("nUserClusters", 3)
	nItemClusters := reader.getInt("nItemClusters", 3)
	nEpochs := reader.getInt("nEpochs", 20)
	// Initialize parameters
	coc.trainSet = trainSet
	coc.globalMean = trainSet.GlobalMean()
	coc.userMeans = means(trainSet.UserRatings())
	coc.itemMeans = means(trainSet.ItemRatings())
	coc.userClusters = newUniformVectorInt(trainSet.UserCount(), 0, nUserClusters)
	coc.itemClusters = newUniformVectorInt(trainSet.ItemCount(), 0, nItemClusters)
	coc.userClusterMeans = make([]float64, nUserClusters)
	coc.itemClusterMeans = make([]float64, nItemClusters)
	coc.coClusterMeans = newZeroMatrix(nUserClusters, nItemClusters)
	// Compute average
	clusterMean(coc.userClusterMeans, coc.userClusters, trainSet.UserRatings())
	clusterMean(coc.itemClusterMeans, coc.itemClusters, trainSet.ItemRatings())
	// A^{tmp1}_{ij} = A_{ij} - A^R_i - A^C_j
	userRatings := trainSet.UserRatings()
	tmp1 := newNanMatrix(trainSet.UserCount(), trainSet.ItemCount())
	for i := range tmp1 {
		for j := range tmp1[i] {
			if !math.IsNaN(userRatings[i][j]) {
				tmp1[i][j] = userRatings[i][j] - coc.userMeans[i] - coc.itemMeans[j]
			}
		}
	}
	// Clustering
	for ep := 0; ep < nEpochs; ep++ {
		// Compute averages A^{COC}, A^{RC}, A^{CC}, A^R, A^C
		clusterMean(coc.userClusterMeans, coc.userClusters, trainSet.UserRatings())
		clusterMean(coc.itemClusterMeans, coc.itemClusters, trainSet.ItemRatings())
		coClusterMean(coc.coClusterMeans, coc.userClusters, coc.itemClusters, userRatings)
		// Update row (user) cluster assignments
		for i := range coc.userClusters {
			bestCluster, leastCost := -1, math.Inf(1)
			for k := 0; k < nUserClusters; k++ {
				// \sum^n_{j=1}W_{ij}(A^{tmp1}_{ij}-A^{COC)_{gy(j)}+A^{RC}_g+A^{RC}_{y(j)})^2
				cost := 0.0
				for j := range coc.itemClusters {
					if !math.IsNaN(userRatings[i][j]) {
						cost += tmp1[i][j] -
							coc.coClusterMeans[k][coc.itemClusters[j]] +
							coc.userClusterMeans[k] +
							coc.itemClusterMeans[coc.itemClusters[j]]
					}
				}
				if cost < leastCost {
					bestCluster = k
				}
			}
			coc.userClusters[i] = bestCluster
		}
		// Update column (item) cluster assignments
		for j := range coc.itemClusters {
			bestCluster, leastCost := -1, math.Inf(1)
			for k := 0; k < nItemClusters; k++ {
				// \sum^m_{i=1}W_{ij}(A^{tmp1}_{ij}-A^{COC)_{p(i)h}+A^{RC}_{p(i)}+A^{RC}_h)^2
				cost := 0.0
				for i := range coc.userClusters {
					if !math.IsNaN(userRatings[i][j]) {
						cost += tmp1[i][j] -
							coc.coClusterMeans[coc.itemClusters[i]][k] +
							coc.userClusterMeans[coc.itemClusters[i]] +
							coc.itemClusterMeans[k]
					}
				}
				if cost < leastCost {
					bestCluster = k
				}
			}
			coc.itemClusters[j] = bestCluster
		}
	}
}

func clusterMean(dst []float64, clusters []int, ratings [][]float64) {
	resetZeroVector(dst)
	count := make([]float64, len(dst))
	for id, cluster := range clusters {
		for _, rating := range ratings[id] {
			if !math.IsNaN(rating) {
				dst[cluster] += rating
				count[cluster]++
			}
		}
	}
	floats.Div(dst, count)
}

func coClusterMean(dst [][]float64, userClusters, itemClusters []int, ratings [][]float64) {
	resetZeroMatrix(dst)
	count := newZeroMatrix(len(dst), len(dst[0]))
	for userId, userCluster := range userClusters {
		for itemId, rating := range ratings[userId] {
			if !math.IsNaN(rating) {
				itemCluster := itemClusters[itemId]
				count[userCluster][itemCluster]++
				dst[userCluster][itemCluster] += rating
			}
		}
	}
	for i := range dst {
		for j := range dst[i] {
			dst[i][j] /= count[i][j]
		}
	}
}
