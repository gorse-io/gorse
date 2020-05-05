package model

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"math"
)

// KNN is the KNN model for implicit feedback.
type KNN struct {
	Base
	Matrix [][]float64
	Users  []*base.MarginalSubSet
}

// NewKNN creates a KNN model for implicit feedback.
func NewKNN(params base.Params) *KNN {
	knn := new(KNN)
	knn.SetParams(params)
	return knn
}

// Predict by the KNN model.
func (knn *KNN) Predict(userId, itemId string) float64 {
	userIndex := knn.UserIndexer.ToIndex(userId)
	itemIndex := knn.ItemIndexer.ToIndex(itemId)
	score := 0.0
	if userIndex != base.NotId && itemIndex != base.NotId {
		knn.Users[userIndex].ForEachIndex(func(i, index int, value float64) {
			score += knn.Matrix[index][itemIndex]
		})
	}
	return score
}

// Fit the KNN model.
func (knn *KNN) Fit(trainSet core.DataSetInterface, options *base.RuntimeOptions) {
	knn.Init(trainSet)
	knn.Users = trainSet.Users()
	// Pairwise similarity
	options.Logf("compute similarity matrix")
	knn.Matrix = base.NewMatrix(trainSet.ItemCount(), trainSet.ItemCount())
	base.Parallel(trainSet.ItemCount(), options.GetFitJobs(), func(begin, end int) {
		for iIndex := begin; iIndex < end; iIndex++ {
			iRatings := trainSet.Items()[iIndex]
			for jIndex := 0; jIndex < trainSet.ItemCount(); jIndex++ {
				jRatings := trainSet.Items()[jIndex]
				if iIndex != jIndex {
					ret := base.ImplicitSimilarity(iRatings, jRatings)
					// Get the number of common
					common := 0.0
					iRatings.ForIntersection(jRatings, func(index string, a, b float64) {
						common += 1
					})
					if !math.IsNaN(ret) {
						knn.Matrix[iIndex][jIndex] = ret
					}
				}
			}
		}
	})
}
