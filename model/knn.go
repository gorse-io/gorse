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
	LeftRatings  []*base.MarginalSubSet
	RightRatings []*base.MarginalSubSet
	UserRatings  []*base.MarginalSubSet
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
	// Convert IDs to indices
	userIndex := knn.UserIndexer.ToIndex(userId)
	itemIndex := knn.ItemIndexer.ToIndex(itemId)
	// Set user based or item based
	var leftIndex, rightIndex int
	if knn.userBased {
		leftIndex, rightIndex = userIndex, itemIndex
	} else {
		leftIndex, rightIndex = itemIndex, userIndex
	}
	// Return global mean for new users and new items
	if leftIndex == base.NotId || rightIndex == base.NotId {
		return knn.GlobalMean
	}

	// Find user (item) interacted with item (user)
	neighbors := base.NewMaxHeap(knn.k)
	type Neighbor struct {
		Id     int
		Rating float64
	}
	knn.RightRatings[rightIndex].ForEachIndex(func(i, index int, value float64) {
		sim := knn.SimMatrix[leftIndex][index]
		if sim > 0 {
			neighbors.Add(Neighbor{index, value}, sim)
		}
	})
	// Return global mean for a user (item) with the number of neighborhoods less than min k
	if neighbors.Len() < knn.minK {
		return knn.GlobalMean
	}
	// Predict
	weightSum := 0.0
	weightRating := 0.0
	for i := 0; i < neighbors.Len(); i++ {
		v := neighbors.Elem[i].(Neighbor)
		index := v.Id
		value := v.Rating
		weightSum += knn.SimMatrix[leftIndex][index]
		rating := value
		if knn._type == base.Centered {
			rating -= knn.LeftMean[index]
		} else if knn._type == base.ZScore {
			rating = (rating - knn.LeftMean[index]) / knn.StdDev[index]
		} else if knn._type == base.Baseline {
			rating -= knn.Bias[index]
		}
		weightRating += knn.SimMatrix[leftIndex][index] * rating
	}
	prediction := weightRating / weightSum
	if knn._type == base.Centered {
		prediction += knn.LeftMean[leftIndex]
	} else if knn._type == base.ZScore {
		prediction *= knn.StdDev[leftIndex]
		prediction += knn.LeftMean[leftIndex]
	} else if knn._type == base.Baseline {
		prediction += knn.Bias[leftIndex]
	}
	return prediction
}

// Fit the KNN model.
func (knn *KNN) Fit(trainSet core.DataSetInterface, options *base.RuntimeOptions) {
	knn.Init(trainSet)
	// Set global GlobalMean for new users (items)
	knn.GlobalMean = trainSet.GlobalMean()
	// Retrieve user (item) iRatings
	if knn.userBased {
		knn.LeftRatings = trainSet.Users()
		knn.RightRatings = trainSet.Items()
	} else {
		knn.LeftRatings = trainSet.Items()
		knn.RightRatings = trainSet.Users()
	}
	// Retrieve user (item) Mean
	if knn._type == base.Centered || knn._type == base.ZScore {
		options.Logf("compute mean")
		knn.LeftMean = make([]float64, len(knn.LeftRatings))
		for i := 0; i < len(knn.LeftRatings); i++ {
			knn.LeftRatings[i].ForEachIndex(func(_, index int, value float64) {
				knn.LeftMean[i] += value
			})
			knn.LeftMean[i] /= float64(knn.LeftRatings[i].Len())
		}
	}
	// Retrieve user (item) standard deviation
	if knn._type == base.ZScore {
		options.Logf("compute standard deviation")
		knn.StdDev = make([]float64, len(knn.LeftRatings))
		for i := range knn.LeftMean {
			sum, count := 0.0, 0.0
			knn.LeftRatings[i].ForEachIndex(func(_, index int, value float64) {
				sum += (value - knn.LeftMean[i]) * (value - knn.LeftMean[i])
				count++
			})
			knn.StdDev[i] = math.Sqrt(sum / count)
		}
	}
	if knn._type == base.Baseline {
		options.Logf("fit baseline model")
		baseLine := NewBaseLine(knn.Params)
		baseLine.Fit(trainSet, options)
		if knn.userBased {
			knn.Bias = baseLine.UserBias
		} else {
			knn.Bias = baseLine.ItemBias
		}
	}
	// Pairwise similarity
	options.Logf("compute similarity matrix")
	knn.SimMatrix = base.NewMatrix(len(knn.LeftRatings), len(knn.LeftRatings))
	base.Parallel(len(knn.LeftRatings), options.GetJobs(), func(begin, end int) {
		for iIndex := begin; iIndex < end; iIndex++ {
			iRatings := knn.LeftRatings[iIndex]
			for jIndex := 0; jIndex < len(knn.LeftRatings); jIndex++ {
				jRatings := knn.LeftRatings[jIndex]
				if iIndex != jIndex {
					ret := knn.similarity(iRatings, jRatings)
					// Get the number of common
					common := 0.0
					iRatings.ForIntersection(jRatings, func(index int, a, b float64) {
						common += 1
					})
					if !math.IsNaN(ret) {
						knn.SimMatrix[iIndex][jIndex] = (common - 1) / (common - 1 + float64(knn.shrinkage)) * ret
					}
				}
			}
		}
	})
}
