package model

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"math"
)

// CoClustering: Collaborative filtering based on co-clustering[5].
type CoClustering struct {
	BaseModel
	GlobalMean       float64     // A^{global}
	UserMeans        []float64   // A^{R}
	ItemMeans        []float64   // A^{R}
	UserClusters     []int       // p(i)
	ItemClusters     []int       // y(i)
	UserClusterMeans []float64   // A^{RC}
	ItemClusterMeans []float64   // A^{CC}
	CoClusterMeans   [][]float64 // A^{COC}
	nUserClusters    int
	nItemClusters    int
	nEpochs          int
}

// NewCoClustering creates a co-clustering model. Params:
//   NEpochs       - The number of iteration of the SGD procedure. Default is 20.
//   NUserClusters - The number of user clusters. Default is 3.
//   NItemClusters - The number of item clusters. Default is 3.
//   randState     - The random seed. Default is UNIX time step.
func NewCoClustering(params base.Params) *CoClustering {
	coc := new(CoClustering)
	coc.SetParams(params)
	return coc
}

func (coc *CoClustering) SetParams(params base.Params) {
	coc.BaseModel.SetParams(params)
	// Setup parameters
	coc.nUserClusters = coc.Params.GetInt(base.NUserClusters, 3)
	coc.nItemClusters = coc.Params.GetInt(base.NItemClusters, 3)
	coc.nEpochs = coc.Params.GetInt(base.NEpochs, 20)
}

func (coc *CoClustering) Predict(userId, itemId int) float64 {
	denseUserId := coc.UserIdSet.ToDenseId(userId)
	denseItemId := coc.ItemIdSet.ToDenseId(itemId)
	return coc.predict(denseUserId, denseItemId)
}

func (coc *CoClustering) predict(denseUserId, denseItemId int) float64 {
	prediction := 0.0
	if denseUserId != base.NotId && denseItemId != base.NotId {
		// old user - old item
		userCluster := coc.UserClusters[denseUserId]
		itemCluster := coc.ItemClusters[denseItemId]
		prediction = coc.UserMeans[denseUserId] + coc.ItemMeans[denseItemId] -
			coc.UserClusterMeans[userCluster] - coc.ItemClusterMeans[itemCluster] +
			coc.CoClusterMeans[userCluster][itemCluster]
	} else if denseUserId != base.NotId {
		// old user - new item
		prediction = coc.UserMeans[denseUserId]
	} else if denseItemId != base.NotId {
		// new user - old item
		prediction = coc.ItemMeans[denseItemId]
	} else {
		// new user - new item
		prediction = coc.GlobalMean
	}
	return prediction
}

func (coc *CoClustering) Fit(trainSet core.DataSet, options ...base.FitOption) {
	coc.Init(trainSet, options)
	// Initialize parameters
	coc.GlobalMean = trainSet.GlobalMean
	userRatings := trainSet.DenseUserRatings
	itemRatings := trainSet.DenseItemRatings
	coc.UserMeans = base.SparseVectorsMean(userRatings)
	coc.ItemMeans = base.SparseVectorsMean(itemRatings)
	coc.UserClusters = coc.rng.NewUniformVectorInt(trainSet.UserCount(), 0, coc.nUserClusters)
	coc.ItemClusters = coc.rng.NewUniformVectorInt(trainSet.ItemCount(), 0, coc.nItemClusters)
	coc.UserClusterMeans = make([]float64, coc.nUserClusters)
	coc.ItemClusterMeans = make([]float64, coc.nItemClusters)
	coc.CoClusterMeans = base.NewMatrix(coc.nUserClusters, coc.nItemClusters)
	// A^{tmp1}_{ij} = A_{ij} - A^R_i - A^C_j
	tmp1 := base.NewMatrix(trainSet.UserCount(), trainSet.ItemCount())
	for i := range tmp1 {
		userRatings[i].ForEach(func(_, index int, value float64) {
			tmp1[i][index] = value - coc.UserMeans[i] - coc.ItemMeans[index]
		})
	}
	// Clustering
	for ep := 0; ep < coc.nEpochs; ep++ {
		// Compute averages A^{COC}, A^{RC}, A^{CC}, A^R, A^C
		coc.clusterMean(coc.UserClusterMeans, coc.UserClusters, userRatings)
		coc.clusterMean(coc.ItemClusterMeans, coc.ItemClusters, itemRatings)
		coc.coClusterMean(coc.CoClusterMeans, coc.UserClusters, coc.ItemClusters, userRatings)
		// Update row (user) cluster assignments
		for denseUserId := 0; denseUserId < trainSet.UserCount(); denseUserId++ {
			bestCluster, leastCost := -1, math.Inf(1)
			for g := 0; g < coc.nUserClusters; g++ {
				cost := 0.0
				userRatings[denseUserId].ForEach(func(_, denseItemId int, value float64) {
					itemCluster := coc.ItemClusters[denseItemId]
					prediction := coc.UserMeans[denseUserId] + coc.ItemMeans[denseItemId] -
						coc.UserClusterMeans[g] -
						coc.ItemClusterMeans[itemCluster] +
						coc.CoClusterMeans[g][itemCluster]
					temp := prediction - value
					cost += temp * temp
				})
				if cost < leastCost {
					bestCluster = g
					leastCost = cost
				}
			}
			coc.UserClusters[denseUserId] = bestCluster
		}
		// Update column (item) cluster assignments
		for denseItemId := 0; denseItemId < trainSet.ItemCount(); denseItemId++ {
			bestCluster, leastCost := -1, math.Inf(1)
			for h := 0; h < coc.nItemClusters; h++ {
				cost := 0.0
				itemRatings[denseItemId].ForEach(func(_, denseUserId int, value float64) {
					userCluster := coc.UserClusters[denseUserId]
					prediction := coc.UserMeans[denseUserId] + coc.ItemMeans[denseItemId] -
						coc.UserClusterMeans[userCluster] - coc.ItemClusterMeans[h] +
						coc.CoClusterMeans[userCluster][h]
					temp := prediction - value
					cost += temp * temp
				})
				if cost < leastCost {
					bestCluster = h
					leastCost = cost
				}
			}
			coc.ItemClusters[denseItemId] = bestCluster
		}
	}
}

func (coc *CoClustering) clusterMean(dst []float64, clusters []int, ratings []*base.SparseVector) {
	base.FillZeroVector(dst)
	count := make([]float64, len(dst))
	for id, cluster := range clusters {
		ratings[id].ForEach(func(_, index int, value float64) {
			dst[cluster] += value
			count[cluster]++
		})
	}
	for i := range dst {
		if count[i] > 0 {
			dst[i] /= count[i]
		} else {
			dst[i] = coc.GlobalMean
		}
	}
}

func (coc *CoClustering) coClusterMean(dst [][]float64, userClusters, itemClusters []int, userRatings []*base.SparseVector) {
	base.FillZeroMatrix(dst)
	count := base.NewMatrix(coc.nUserClusters, coc.nItemClusters)
	for denseUserId, userCluster := range userClusters {
		userRatings[denseUserId].ForEach(func(_, denseItemId int, value float64) {
			itemCluster := itemClusters[denseItemId]
			count[userCluster][itemCluster]++
			dst[userCluster][itemCluster] += value
		})
	}
	for i := range dst {
		for j := range dst[i] {
			if count[i][j] > 0 {
				dst[i][j] /= count[i][j]
			} else {
				dst[i][j] = coc.GlobalMean
			}
		}
	}
}
