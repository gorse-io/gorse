package model

import (
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
	knnType      string
	userBased    bool
	simMetric    Similarity
	k            int
	minK         int
	target       string
	lr           float64
	reg          float64
	nEpochs      int
}

// NewKNN creates a KNN model. Params:
//   KNNType        - The type of KNN ('Basic', 'Centered', 'ZScore', 'Baseline').
//                    Default is 'basic'.
//   KNNSimilarity  - The similarity function. Default is MSD.
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
	knn.knnType = knn.Params.GetString(KNNType, Basic)
	knn.simMetric = knn.Params.GetSim(KNNSimilarity, MSD)
	knn.userBased = knn.Params.GetBool(UserBased, true)
	knn.k = knn.Params.GetInt(K, 40)
	knn.minK = knn.Params.GetInt(MinK, 1)
	knn.target = knn.Params.GetString(Target, Regression)
	knn.lr = knn.Params.GetFloat64(Lr, 0.005)
	knn.reg = knn.Params.GetFloat64(Reg, 0.02)
	knn.nEpochs = knn.Params.GetInt(NEpochs, 20)
}

func (knn *KNN) Predict(userId, itemId int) float64 {
	denseUserId := knn.UserIdSet.ToDenseId(userId)
	denseItemId := knn.ItemIdSet.ToDenseId(itemId)
	// Select predict function
	switch knn.target {
	case Regression:
		return knn.predictRegression(denseUserId, denseItemId)
	case BPR:
		return knn.predictBPR(denseUserId, denseItemId)
	default:
		panic("Unknown target")
	}
}

func (knn *KNN) predictRegression(denseUserId, denseItemId int) float64 {
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
		if knn.knnType == Centered {
			rating -= knn.LeftMean[index]
		} else if knn.knnType == ZScore {
			rating = (rating - knn.LeftMean[index]) / knn.StdDev[index]
		} else if knn.knnType == Baseline {
			rating -= knn.Bias[index]
		}
		weightRating += knn.SimMatrix[leftId][index] * rating
	})
	prediction := weightRating / weightSum
	if knn.knnType == Centered {
		prediction += knn.LeftMean[leftId]
	} else if knn.knnType == ZScore {
		prediction *= knn.StdDev[leftId]
		prediction += knn.LeftMean[leftId]
	} else if knn.knnType == Baseline {
		prediction += knn.Bias[leftId]
	}
	return prediction
}

func (knn *KNN) predictBPR(denseUserId, denseItemId int) float64 {
	prediction := 0.0
	knn.UserRatings[denseUserId].ForEach(func(i, index int, value float64) {
		if index != denseItemId {
			prediction += knn.SimMatrix[denseItemId][index]
		}
	})
	return prediction
}

// Fit a KNN model.
func (knn *KNN) Fit(trainSet TrainSet, options ...FitOption) {
	knn.Init(trainSet, options)
	// Select fit function
	switch knn.target {
	case Regression:
		knn.fitRegression(trainSet)
	case BPR:
		knn.fitBPR(trainSet)
	}
}

func (knn *KNN) fitRegression(trainSet TrainSet) {
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
	if knn.knnType == Centered || knn.knnType == ZScore {
		knn.LeftMean = SparseVectorsMean(knn.LeftRatings)
	}
	// Retrieve user (item) standard deviation
	if knn.knnType == ZScore {
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
	if knn.knnType == Baseline {
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
		// Call SortIndex() to make sure simMetric() reentrant
		knn.LeftRatings[i].SortIndex()
	}
	knn.SimMatrix = MakeMatrix(len(knn.LeftRatings), len(knn.LeftRatings))
	Parallel(len(knn.LeftRatings), knn.rtOptions.NJobs, func(begin, end int) {
		for iId := begin; iId < end; iId++ {
			iRatings := knn.LeftRatings[iId]
			for jId, jRatings := range knn.LeftRatings {
				if iId != jId {
					ret := knn.simMetric(&iRatings, &jRatings)
					if !math.IsNaN(ret) {
						knn.SimMatrix[iId][jId] = ret
					}
				}
			}
		}
	})
}

func (knn *KNN) fitBPR(trainSet TrainSet) {
	knn.UserRatings = trainSet.UserRatings
	// Create the set of positive feedback
	positiveSet := make([]map[int]bool, trainSet.UserCount())
	for denseUserId, userRating := range trainSet.UserRatings {
		positiveSet[denseUserId] = make(map[int]bool)
		userRating.ForEach(func(i, index int, value float64) {
			positiveSet[denseUserId][index] = true
		})
	}
	// Training
	for epoch := 0; epoch < knn.nEpochs; epoch++ {
		// Generate permutation
		perm := knn.rng.Perm(trainSet.Len())
		// Training epoch
		for _, i := range perm {
			// Select a positive sample
			denseUserId, densePosId, _ := trainSet.GetDense(i)
			// Select a negative sample
			denseNegId := -1
			for {
				temp := knn.rng.Intn(trainSet.ItemCount())
				if _, exist := positiveSet[denseUserId][temp]; !exist {
					denseNegId = temp
					break
				}
			}
			diff := knn.predictBPR(denseUserId, densePosId) - knn.predictBPR(denseUserId, denseNegId)
			grad := math.Exp(-diff) / (1.0 + math.Exp(-diff))
			// Update positive weights
			knn.UserRatings[denseUserId].ForEach(func(i, index int, value float64) {
				if index != densePosId {
					// TODO: Add regularization
					knn.SimMatrix[index][densePosId] += knn.lr * grad
					knn.SimMatrix[densePosId][index] += knn.lr * grad
				}
			})
			// Update negative weights
			knn.UserRatings[denseUserId].ForEach(func(i, index int, value float64) {
				if index != denseNegId {
					// TODO: Add regularization
					knn.SimMatrix[index][denseNegId] -= knn.lr * grad
					knn.SimMatrix[denseNegId][index] -= knn.lr * grad
				}
			})
		}
	}
}
