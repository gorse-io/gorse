package model

import (
	"fmt"
	. "github.com/zhenghaoz/gorse/base"
	. "github.com/zhenghaoz/gorse/core"
	"math"
)

// KNN for collaborate filtering.
type KNN struct {
	Base
	GlobalMean   float64
	SimMatrix    [][]float64
	LeftRatings  []SparseVector
	RightRatings []SparseVector
	UserRatings  []SparseVector
	LeftMean     []float64 // Centered KNN: user (item) Mean
	StdDev       []float64 // KNN with Z Score: user (item) standard deviation
	Bias         []float64 // KNN Baseline: Bias
	_type        ParamString
	userBased    bool
	similarity   FuncSimilarity
	k            int
	minK         int
	shrinkage    int
}

// NewKNN creates a KNN model. Params:
//   Type        - The type of KNN ('Basic', 'Centered', 'ZScore', 'Baseline').
//                    Default is 'basic'.
//   Similarity  - The similarity function. Default is MSD.
//   UserBased      - User based or item based? Default is true.
//   K              - The maximum k neighborhoods to predict the rating. Default is 40.
//   MinK           - The minimum k neighborhoods to predict the rating. Default is 1.
func NewKNN(params Params) *KNN {
	knn := new(KNN)
	knn.SetParams(params)
	return knn
}

func (knn *KNN) SetParams(params Params) {
	knn.Base.SetParams(params)
	// Setup parameters
	knn._type = knn.Params.GetString(Type, Basic)
	knn.userBased = knn.Params.GetBool(UserBased, true)
	knn.k = knn.Params.GetInt(K, 40)
	knn.minK = knn.Params.GetInt(MinK, 1)
	knn.shrinkage = knn.Params.GetInt(Shrinkage, 100)
	// Setup similarity function
	switch name := knn.Params.GetString(Similarity, MSD); name {
	case MSD:
		knn.similarity = MSDSimilarity
	case Cosine:
		knn.similarity = CosineSimilarity
	case Pearson:
		knn.similarity = PearsonSimilarity
	default:
		panic(fmt.Sprintf("Unknown similarity function: %v", name))
	}
}

func (knn *KNN) Predict(userId, itemId int) float64 {
	denseUserId := knn.UserIdSet.ToDenseId(userId)
	denseItemId := knn.ItemIdSet.ToDenseId(itemId)
	// Set user based or item based
	var leftId, rightId int
	if knn.userBased {
		leftId, rightId = denseUserId, denseItemId
	} else {
		leftId, rightId = denseItemId, denseUserId
	}
	if leftId == NotId || rightId == NotId {
		return knn.GlobalMean
	}
	// Find user (item) interacted with item (user)
	neighbors := MakeKNNHeap(knn.k)
	knn.RightRatings[rightId].ForEach(func(i, index int, value float64) {
		neighbors.Add(index, value, knn.SimMatrix[leftId][index])
	})
	// Set global GlobalMean for a user (item) with the number of neighborhoods less than min k
	if neighbors.Len() < knn.minK {
		return knn.GlobalMean
	}
	// Predict the rating by weighted GlobalMean
	weightSum := 0.0
	weightRating := 0.0
	neighbors.SparseVector.ForEach(func(i, index int, value float64) {
		weightSum += knn.SimMatrix[leftId][index]
		rating := value
		if knn._type == Centered {
			rating -= knn.LeftMean[index]
		} else if knn._type == ZScore {
			rating = (rating - knn.LeftMean[index]) / knn.StdDev[index]
		} else if knn._type == Baseline {
			rating -= knn.Bias[index]
		}
		weightRating += knn.SimMatrix[leftId][index] * rating
	})
	prediction := weightRating / weightSum
	if knn._type == Centered {
		prediction += knn.LeftMean[leftId]
	} else if knn._type == ZScore {
		prediction *= knn.StdDev[leftId]
		prediction += knn.LeftMean[leftId]
	} else if knn._type == Baseline {
		prediction += knn.Bias[leftId]
	}
	return prediction
}

// Fit a KNN model.
func (knn *KNN) Fit(trainSet DataSet, options ...FitOption) {
	knn.Init(trainSet, options)
	// Set global GlobalMean for new users (items)
	knn.GlobalMean = trainSet.GlobalMean
	// Retrieve user (item) iRatings
	if knn.userBased {
		knn.LeftRatings = trainSet.UserRatings
		knn.RightRatings = trainSet.ItemRatings
	} else {
		knn.LeftRatings = trainSet.ItemRatings
		knn.RightRatings = trainSet.UserRatings
	}
	// Retrieve user (item) Mean
	if knn._type == Centered || knn._type == ZScore {
		knn.LeftMean = SparseVectorsMean(knn.LeftRatings)
	}
	// Retrieve user (item) standard deviation
	if knn._type == ZScore {
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
	if knn._type == Baseline {
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
	knn.SimMatrix = MakeMatrix(len(knn.LeftRatings), len(knn.LeftRatings))
	Parallel(len(knn.LeftRatings), knn.rtOptions.NJobs, func(begin, end int) {
		for iId := begin; iId < end; iId++ {
			iRatings := knn.LeftRatings[iId]
			for jId, jRatings := range knn.LeftRatings {
				if iId != jId {
					ret := knn.similarity(&iRatings, &jRatings)
					// Get the number of common
					common := 0.0
					iRatings.ForIntersection(&jRatings, func(index int, a, b float64) {
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
