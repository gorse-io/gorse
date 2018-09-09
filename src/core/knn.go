package core

import (
	"math"
	"sort"
)

const (
	basic    = 0
	centered = 1
	zscore   = 2
	baseline = 3
)

type KNN struct {
	option     parameterReader
	tpe        int
	globalMean float64
	sims       [][]float64
	ratings    [][]float64
	trainSet   TrainSet
	means      []float64 // Centered KNN: user (item) mean
	bias       []float64 // KNN Baseline: bias
	// Parameters
	userBased bool
	k         int
	minK      int
}

type CandidateSet struct {
	similarities []float64
	candidates   []int
}

func NewCandidateSet(sim []float64, candidates []int) *CandidateSet {
	neighbors := CandidateSet{}
	neighbors.similarities = sim
	neighbors.candidates = candidates
	return &neighbors
}

func (n *CandidateSet) Len() int {
	return len(n.candidates)
}

func (n *CandidateSet) Less(i, j int) bool {
	return n.similarities[n.candidates[i]] > n.similarities[n.candidates[j]]
}

func (n *CandidateSet) Swap(i, j int) {
	n.candidates[i], n.candidates[j] = n.candidates[j], n.candidates[i]
}

func NewKNN() *KNN {
	knn := new(KNN)
	knn.tpe = basic
	return knn
}

func NewKNNWithMean() *KNN {
	knn := new(KNN)
	knn.tpe = centered
	return knn
}

func NewKNNWithZScore() *KNN {
	knn := new(KNN)
	knn.tpe = zscore
	return knn
}

func NewKNNBaseLine() *KNN {
	knn := new(KNN)
	knn.tpe = baseline
	return knn
}

func (knn *KNN) Predict(userId int, itemId int) float64 {
	innerUserId := knn.trainSet.ConvertUserId(userId)
	innerItemId := knn.trainSet.ConvertItemId(itemId)
	// Set user based or item based
	var leftId, rightId int
	if knn.userBased {
		leftId, rightId = innerUserId, innerItemId
	} else {
		leftId, rightId = innerItemId, innerUserId
	}
	if leftId == newId || rightId == newId {
		return knn.globalMean
	}
	// Find user (item) interacted with item (user)
	candidates := make([]int, 0)
	for otherId := range knn.ratings {
		if !math.IsNaN(knn.ratings[otherId][rightId]) && !math.IsNaN(knn.sims[leftId][otherId]) {
			candidates = append(candidates, otherId)
		}
	}
	// Set global globalMean for a user (item) with the number of neighborhoods less than min k
	if len(candidates) <= knn.minK {
		return knn.globalMean
	}
	// Sort users (items) by similarity
	candidateSet := NewCandidateSet(knn.sims[leftId], candidates)
	sort.Sort(candidateSet)
	// Find neighborhoods
	numNeighbors := knn.k
	if numNeighbors > candidateSet.Len() {
		numNeighbors = candidateSet.Len()
	}
	// Predict the rating by weighted globalMean
	weightSum := 0.0
	weightRating := 0.0
	for _, otherId := range candidateSet.candidates[0:numNeighbors] {
		weightSum += knn.sims[leftId][otherId]
		weightRating += knn.sims[leftId][otherId] * knn.ratings[otherId][rightId]
		if knn.tpe == centered {
			weightRating -= knn.sims[leftId][otherId] * knn.means[otherId]
		} else if knn.tpe == baseline {
			weightRating -= knn.sims[leftId][otherId] * knn.bias[otherId]
		}
	}
	prediction := weightRating / weightSum
	if knn.tpe == centered {
		prediction += knn.means[leftId]
	} else if knn.tpe == baseline {
		prediction += knn.bias[leftId]
	}
	return prediction
}

func (knn *KNN) Fit(trainSet TrainSet, params Parameters) {
	// Setup parameters
	reader := newParameterReader(params)
	sim := reader.getSim("sim", MSD)
	knn.userBased = reader.getBool("userBased", true)
	knn.k = reader.getInt("k", 40)
	knn.minK = reader.getInt("minK", 1)
	// Set global globalMean for new users (items)
	knn.trainSet = trainSet
	knn.globalMean = trainSet.GlobalMean()
	// Retrieve user (item) ratings
	if knn.userBased {
		knn.ratings = trainSet.UserRatings()
		knn.sims = newNanMatrix(trainSet.UserCount(), trainSet.UserCount())
	} else {
		knn.ratings = trainSet.ItemRatings()
		knn.sims = newNanMatrix(trainSet.ItemCount(), trainSet.ItemCount())
	}
	// Retrieve user (item) mean
	if knn.tpe == centered {
		knn.means = make([]float64, len(knn.ratings))
		for i := range knn.means {
			sum, count := 0.0, 0.0
			for j := range knn.ratings[i] {
				if !math.IsNaN(knn.ratings[i][j]) {
					sum += knn.ratings[i][j]
					count++
				}
			}
			knn.means[i] = sum / count
		}
	} else if knn.tpe == baseline {
		baseLine := NewBaseLine()
		baseLine.Fit(trainSet, params)
		if knn.userBased {
			knn.bias = baseLine.userBias
		} else {
			knn.bias = baseLine.itemBias
		}
	}
	// Pairwise similarity
	for leftId, leftRatings := range knn.ratings {
		for rightId, rightRatings := range knn.ratings {
			if leftId != rightId {
				if math.IsNaN(knn.sims[leftId][rightId]) {
					ret := sim(leftRatings, rightRatings)
					if !math.IsNaN(ret) {
						knn.sims[leftId][rightId] = ret
						knn.sims[rightId][leftId] = ret
					}
				}
			}
		}
	}
}
