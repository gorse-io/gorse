package model

import (
	"fmt"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"math"
)

// KNN for collaborate filtering.
//   Type        - The type of KNN ('Basic', 'Centered', 'ZScore', 'Baseline').
//                    Default is 'basic'.
//   Similarity  - The similarity function. Default is MSD.
//   UserBased      - User based or item based? Default is true.
//   K              - The maximum k neighborhoods to predict the rating. Default is 40.
//   MinK           - The minimum k neighborhoods to predict the rating. Default is 1.
type KNN struct {
	Base
	GlobalMean   float64
	SimMatrix    [][]float64
	LeftRatings  []*base.SparseVector
	RightRatings []*base.SparseVector
	UserRatings  []*base.SparseVector
	LeftMean     []float64 // Centered KNN: user (item) Mean
	StdDev       []float64 // KNN with Z Score: user (item) standard deviation
	Bias         []float64 // KNN Baseline: Bias
	// Hyper-parameters
	_type      string
	userBased  bool
	similarity base.FuncSimilarity
	k          int
	minK       int
	shrinkage  int
}

// NewKNN creates a KNN model.
func NewKNN(params base.Params) *KNN {
	knn := new(KNN)
	knn.SetParams(params)
	return knn
}

// SetParams sets hyper-parameters for the KNN model.
func (knn *KNN) SetParams(params base.Params) {
	knn.Base.SetParams(params)
	// Setup parameters
	knn._type = knn.Params.GetString(base.Type, base.Basic)
	knn.userBased = knn.Params.GetBool(base.UserBased, true)
	knn.k = knn.Params.GetInt(base.K, 40)
	knn.minK = knn.Params.GetInt(base.MinK, 1)
	knn.shrinkage = knn.Params.GetInt(base.Shrinkage, 100)
	// Setup similarity function
	switch name := knn.Params.GetString(base.Similarity, base.MSD); name {
	case base.MSD:
		knn.similarity = base.MSDSimilarity
	case base.Cosine:
		knn.similarity = base.CosineSimilarity
	case base.Pearson:
		knn.similarity = base.PearsonSimilarity
	default:
		panic(fmt.Sprintf("Unknown similarity function: %v", name))
	}
}

// Predict by the KNN model.
func (knn *KNN) Predict(userId, itemId int) float64 {
	// Convert sparse IDs to dense IDs
	denseUserId := knn.UserIdSet.ToDenseId(userId)
	denseItemId := knn.ItemIdSet.ToDenseId(itemId)
	// Set user based or item based
	var leftId, rightId int
	if knn.userBased {
		leftId, rightId = denseUserId, denseItemId
	} else {
		leftId, rightId = denseItemId, denseUserId
	}
	// Return global mean for new users and new items
	if leftId == base.NotId || rightId == base.NotId {
		return knn.GlobalMean
	}
	// Find user (item) interacted with item (user)
	neighbors := base.NewKNNHeap(knn.k)
	knn.RightRatings[rightId].ForEach(func(i, index int, value float64) {
		neighbors.Add(index, value, knn.SimMatrix[leftId][index])
	})
	// Return global mean for a user (item) with the number of neighborhoods less than min k
	if neighbors.Len() < knn.minK {
		return knn.GlobalMean
	}
	// Predict
	weightSum := 0.0
	weightRating := 0.0
	neighbors.SparseVector.ForEach(func(i, index int, value float64) {
		weightSum += knn.SimMatrix[leftId][index]
		rating := value
		if knn._type == base.Centered {
			rating -= knn.LeftMean[index]
		} else if knn._type == base.ZScore {
			rating = (rating - knn.LeftMean[index]) / knn.StdDev[index]
		} else if knn._type == base.Baseline {
			rating -= knn.Bias[index]
		}
		weightRating += knn.SimMatrix[leftId][index] * rating
	})
	prediction := weightRating / weightSum
	if knn._type == base.Centered {
		prediction += knn.LeftMean[leftId]
	} else if knn._type == base.ZScore {
		prediction *= knn.StdDev[leftId]
		prediction += knn.LeftMean[leftId]
	} else if knn._type == base.Baseline {
		prediction += knn.Bias[leftId]
	}
	return prediction
}

// Fit the KNN model.
func (knn *KNN) Fit(trainSet *core.DataSet, options ...core.RuntimeOption) {
	knn.Init(trainSet, options)
	// Set global GlobalMean for new users (items)
	knn.GlobalMean = trainSet.GlobalMean
	// Retrieve user (item) iRatings
	if knn.userBased {
		knn.LeftRatings = trainSet.DenseUserRatings
		knn.RightRatings = trainSet.DenseItemRatings
	} else {
		knn.LeftRatings = trainSet.DenseItemRatings
		knn.RightRatings = trainSet.DenseUserRatings
	}
	// Retrieve user (item) Mean
	if knn._type == base.Centered || knn._type == base.ZScore {
		knn.LeftMean = base.SparseVectorsMean(knn.LeftRatings)
	}
	// Retrieve user (item) standard deviation
	if knn._type == base.ZScore {
		knn.StdDev = make([]float64, len(knn.LeftRatings))
		for i := range knn.LeftMean {
			sum, count := 0.0, 0.0
			knn.LeftRatings[i].ForEach(func(_, index int, value float64) {
				sum += (value - knn.LeftMean[i]) * (value - knn.LeftMean[i])
				count++
			})
			knn.StdDev[i] = math.Sqrt(sum / count)
		}
	}
	if knn._type == base.Baseline {
		baseLine := NewBaseLine(knn.Params)
		baseLine.Fit(trainSet)
		if knn.userBased {
			knn.Bias = baseLine.UserBias
		} else {
			knn.Bias = baseLine.ItemBias
		}
	}
	// Pairwise similarity
	for i := range knn.LeftRatings {
		// Call SortIndex() to make sure similarity() reentrant
		knn.LeftRatings[i].SortIndex()
	}
	knn.SimMatrix = base.NewMatrix(len(knn.LeftRatings), len(knn.LeftRatings))
	base.Parallel(len(knn.LeftRatings), knn.fitOptions.NJobs, func(begin, end int) {
		for iId := begin; iId < end; iId++ {
			iRatings := knn.LeftRatings[iId]
			for jId, jRatings := range knn.LeftRatings {
				if iId != jId {
					ret := knn.similarity(iRatings, jRatings)
					// Get the number of common
					common := 0.0
					iRatings.ForIntersection(jRatings, func(index int, a, b float64) {
						common += 1
					})
					if !math.IsNaN(ret) {
						knn.SimMatrix[iId][jId] = (common - 1) / (common - 1 + float64(knn.shrinkage)) * ret
					}
				}
			}
		}
	})
}
