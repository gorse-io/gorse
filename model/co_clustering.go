package model

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"math"
)

// CoClustering [5] is a novel collaborative filtering approach based on weighted
// co-clustering algorithm that involves simultaneous clustering of users and
// items.
//
// Let U={u_i}^m_{i=1} be the set of users such that |U|=m and P={p_j}^n_{j=1} be
// the set of items such that |P|=n. Let A be the m x n ratings matrix such that
// A_{ij} is the rating of the user u_i for the item p_j. The approximate matrix
// \hat{A}_{ij} is given by
//  \hat{A}_{ij} = A^{COC}_{gh} + (A^R_i - A^{RC}_g) + (A^C_j - A^{CC}_h)
// where g=ρ(i), h=γ(j) and A^R_i, A^C_j are the average ratings of user u_i and
// item p_j, and A^{COC}_{gh}, A^{RC}_g and A^{CC}_h are the average ratings of
// the corresponding co-cluster, user-cluster and item-cluster respectively.
//
// Hyper-parameters:
//  NEpochs       - The number of iterations of the optimization procedure. Default is 20.
//  NUserClusters - The number of user clusters. Default is 3.
//  NItemClusters - The number of item clusters. Default is 3.
//  RandomState   - The random seed. Default is 0.
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
	// Hyper-parameters
	nUserClusters int // The number of user clusters
	nItemClusters int // The number of user clusters
	nEpochs       int // The number of iterations of the optimization procedure
}

// NewCoClustering creates a CoClustering model.
func NewCoClustering(params base.Params) *CoClustering {
	coc := new(CoClustering)
	coc.SetParams(params)
	return coc
}

// SetParams sets hyper-parameters for the CoClustering model.
func (coc *CoClustering) SetParams(params base.Params) {
	coc.Base.SetParams(params)
	// Setup hyper-parameters
	coc.nUserClusters = coc.Params.GetInt(base.NUserClusters, 3)
	coc.nItemClusters = coc.Params.GetInt(base.NItemClusters, 3)
	coc.nEpochs = coc.Params.GetInt(base.NEpochs, 20)
}

// Predict by the CoClustering model.
func (coc *CoClustering) Predict(userId, itemId int) float64 {
	// Convert sparse IDs to dense IDs
	denseUserId := coc.UserIdSet.ToDenseId(userId)
	denseItemId := coc.ItemIdSet.ToDenseId(itemId)
	return coc.predict(denseUserId, denseItemId)
}

func (coc *CoClustering) predict(denseUserId, denseItemId int) float64 {
	prediction := 0.0
	if denseUserId != base.NotId && denseItemId != base.NotId {
		// old user & old item
		userCluster := coc.UserClusters[denseUserId]
		itemCluster := coc.ItemClusters[denseItemId]
		prediction = coc.UserMeans[denseUserId] + coc.ItemMeans[denseItemId] -
			coc.UserClusterMeans[userCluster] - coc.ItemClusterMeans[itemCluster] +
			coc.CoClusterMeans[userCluster][itemCluster]
	} else if denseUserId != base.NotId {
		// old user & new item
		prediction = coc.UserMeans[denseUserId]
	} else if denseItemId != base.NotId {
		// new user & old item
		prediction = coc.ItemMeans[denseItemId]
	} else {
		// new user & new item
		prediction = coc.GlobalMean
	}
	return prediction
}

// Fit the CoClustering model.
func (coc *CoClustering) Fit(trainSet *core.DataSet, options ...base.FitOption) {
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

// clusterMean computes the mean ratings of clusters.
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

// coClusterMean computes the mean ratings of co-clusters.
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
