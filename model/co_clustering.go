package model

import (
	. "github.com/zhenghaoz/gorse/base"
	. "github.com/zhenghaoz/gorse/core"
	"gonum.org/v1/gonum/floats"
	"math"
)

// CoClustering: Collaborative filtering based on co-clustering[5].
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
	nUserClusters    int
	nItemClusters    int
	nEpochs          int
}

// NewCoClustering creates a co-clustering model. Params:
//   NEpochs       - The number of iteration of the SGD procedure. Default is 20.
//   NUserClusters - The number of user clusters. Default is 3.
//   NItemClusters - The number of item clusters. Default is 3.
//   randState     - The random seed. Default is UNIX time step.
func NewCoClustering(params Params) *CoClustering {
	coc := new(CoClustering)
	coc.SetParams(params)
	return coc
}

func (coc *CoClustering) SetParams(params Params) {
	coc.Base.SetParams(params)
	// Setup parameters
	coc.nUserClusters = coc.Params.GetInt(NUserClusters, 3)
	coc.nItemClusters = coc.Params.GetInt(NItemClusters, 3)
	coc.nEpochs = coc.Params.GetInt(NEpochs, 20)
}

func (coc *CoClustering) Predict(userId, itemId int) float64 {
	// Convert to inner Id
	innerUserId := coc.UserIdSet.ToDenseId(userId)
	innerItemId := coc.ItemIdSet.ToDenseId(itemId)
	prediction := 0.0
	if innerUserId != NotId && innerItemId != NotId {
		// old user - old item
		userCluster := coc.UserClusters[innerUserId]
		itemCluster := coc.ItemClusters[innerItemId]
		prediction = coc.UserMeans[innerUserId] + coc.ItemMeans[innerItemId] -
			coc.UserClusterMeans[userCluster] - coc.ItemClusterMeans[itemCluster] +
			coc.CoClusterMeans[userCluster][itemCluster]
	} else if innerUserId != NotId {
		// old user - new item
		prediction = coc.UserMeans[innerUserId]
	} else if innerItemId != NotId {
		// new user - old item
		prediction = coc.ItemMeans[innerItemId]
	} else {
		// new user - new item
		prediction = coc.GlobalMean
	}
	return prediction
}

func (coc *CoClustering) Fit(trainSet TrainSet, options ...RuntimeOption) {
	coc.Init(trainSet, options)
	// Initialize parameters
	coc.GlobalMean = trainSet.GlobalMean
	userRatings := trainSet.UserRatings
	itemRatings := trainSet.ItemRatings
	coc.UserMeans = SparseVectorsMean(userRatings)
	coc.ItemMeans = SparseVectorsMean(itemRatings)
	coc.UserClusters = coc.rng.MakeUniformVectorInt(trainSet.UserCount(), 0, coc.nUserClusters)
	coc.ItemClusters = coc.rng.MakeUniformVectorInt(trainSet.ItemCount(), 0, coc.nItemClusters)
	coc.UserClusterMeans = make([]float64, coc.nUserClusters)
	coc.ItemClusterMeans = make([]float64, coc.nItemClusters)
	coc.CoClusterMeans = MakeMatrix(coc.nUserClusters, coc.nItemClusters)
	// A^{tmp1}_{ij} = A_{ij} - A^R_i - A^C_j
	tmp1 := NewNanMatrix(trainSet.UserCount(), trainSet.ItemCount())
	for i := range tmp1 {
		userRatings[i].ForEach(func(_, index int, value float64) {
			tmp1[i][index] = value - coc.UserMeans[i] - coc.ItemMeans[index]
		})
	}
	// Clustering
	for ep := 0; ep < coc.nEpochs; ep++ {
		// Compute averages A^{COC}, A^{RC}, A^{CC}, A^R, A^C
		clusterMean(coc.UserClusterMeans, coc.UserClusters, userRatings)
		clusterMean(coc.ItemClusterMeans, coc.ItemClusters, itemRatings)
		coClusterMean(coc.CoClusterMeans, coc.UserClusters, coc.ItemClusters, userRatings)
		// A^{tmp2}_{ih} = \frac {\sum_{j'|y(j')=h}A^{tmp1}_{ij'}} {\sum_{j'|y(j')=h}W_{ij'}} + A^{CC}_h
		tmp2 := MakeMatrix(trainSet.UserCount(), coc.nItemClusters)
		count2 := MakeMatrix(trainSet.UserCount(), coc.nItemClusters)
		for i := range tmp2 {
			userRatings[i].ForEach(func(_, index int, value float64) {
				itemClass := coc.ItemClusters[index]
				tmp2[i][itemClass] += tmp1[i][index]
				count2[i][itemClass]++
			})
			for h := range tmp2[i] {
				tmp2[i][h] /= count2[i][h]
				tmp2[i][h] += coc.ItemClusterMeans[h]
			}
		}
		// Update row (user) cluster assignments
		for i := range coc.UserClusters {
			bestCluster, leastCost := coc.UserClusters[i], math.Inf(1)
			for g := 0; g < coc.nUserClusters; g++ {
				// \sum^l_{h=1} A^{tmp2}_{ig} - A^{COC}_{gh} + A^{RC}_g
				cost := 0.0
				for h := 0; h < coc.nItemClusters; h++ {
					if !math.IsNaN(tmp2[i][h]) {
						temp := tmp2[i][h] - coc.CoClusterMeans[g][h] + coc.UserClusterMeans[g]
						cost += temp * temp
					}
				}
				if cost < leastCost {
					bestCluster = g
					leastCost = cost
				}
			}
			coc.UserClusters[i] = bestCluster
		}
		// A^{tmp3}_{gj} = \frac {\sum_{i'|p(i')=g}A^{tmp1}_{i'j}} {\sum_{i'|p(i')=g}W_{i'j}} + A^{RC}_g
		tmp3 := MakeMatrix(coc.nUserClusters, trainSet.ItemCount())
		count3 := MakeMatrix(coc.nUserClusters, trainSet.ItemCount())
		for j := range coc.ItemClusters {
			itemRatings[j].ForEach(func(_, index int, value float64) {
				userClass := coc.UserClusters[index]
				tmp3[userClass][j] += tmp1[index][j]
				count3[userClass][j]++
			})
			for g := range tmp3 {
				tmp3[g][j] /= count3[g][j]
				tmp3[g][j] += coc.UserClusterMeans[g]
			}
		}
		// Update column (item) cluster assignments
		for j := range coc.ItemClusters {
			bestCluster, leastCost := coc.ItemClusters[j], math.Inf(1)
			for h := 0; h < coc.nItemClusters; h++ {
				// \sum^k_{h=1} A^{tmp3}_{gj} - A^{COC}_{gh} + A^{CC}_h
				cost := 0.0
				for g := 0; g < coc.nUserClusters; g++ {
					if !math.IsNaN(tmp3[g][j]) {
						temp := tmp3[g][j] - coc.CoClusterMeans[g][h] + coc.ItemClusterMeans[h]
						cost += temp * temp
					}
				}
				if cost < leastCost {
					bestCluster = h
					leastCost = cost
				}
			}
			coc.ItemClusters[j] = bestCluster
		}
	}
}

func clusterMean(dst []float64, clusters []int, ratings []SparseVector) {
	ResetVector(dst)
	count := make([]float64, len(dst))
	for id, cluster := range clusters {
		ratings[id].ForEach(func(_, index int, value float64) {
			dst[cluster] += value
			count[cluster]++
		})
	}
	floats.Div(dst, count)
}

func coClusterMean(dst [][]float64, userClusters, itemClusters []int, userRatings []SparseVector) {
	ResetMatrix(dst)
	count := MakeMatrix(len(dst), len(dst[0]))
	for userId, userCluster := range userClusters {
		userRatings[userId].ForEach(func(_, index int, value float64) {
			itemCluster := itemClusters[index]
			count[userCluster][itemCluster]++
			dst[userCluster][itemCluster] += value
		})
	}
	for i := range dst {
		floats.Div(dst[i], count[i])
	}
}
