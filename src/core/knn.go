package core

import (
	"math"
	"sort"
)

type KNN struct {
	option   Option
	mean     float64
	sims     [][]float64
	ratings  [][]float64
	trainSet TrainSet
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
	return new(KNN)
}

func (knn *KNN) Predict(userId int, itemId int) float64 {
	innerUserId := knn.trainSet.ConvertUserId(userId)
	innerItemId := knn.trainSet.ConvertItemId(itemId)
	// Set user based or item based
	var leftId, rightId int
	if knn.option.userBased {
		leftId, rightId = innerUserId, innerItemId
	} else {
		leftId, rightId = innerItemId, innerUserId
	}
	if leftId == noBody || rightId == noBody {
		return knn.mean
	}
	// Find user (item) interacted with item (user)
	candidates := make([]int, 0)
	for otherId := range knn.ratings {
		if !math.IsNaN(knn.ratings[otherId][rightId]) && !math.IsNaN(knn.sims[leftId][otherId]) {
			candidates = append(candidates, otherId)
		}
	}
	// Set global mean for a user (item) with the number of neighborhoods less than min k
	if len(candidates) <= knn.option.minK {
		return knn.mean
	}
	// Sort users (items) by similarity
	candidateSet := NewCandidateSet(knn.sims[leftId], candidates)
	sort.Sort(candidateSet)
	// Find neighborhoods
	numNeighbors := knn.option.k
	if numNeighbors > candidateSet.Len() {
		numNeighbors = candidateSet.Len()
	}
	// Predict the rating by weighted mean
	weightSum := 0.0
	weightRating := 0.0
	for _, otherId := range candidateSet.candidates[0:numNeighbors] {
		weightSum += knn.sims[leftId][otherId]
		weightRating += knn.sims[leftId][otherId] * knn.ratings[otherId][rightId]
	}
	return weightRating / weightSum
}

func (knn *KNN) Fit(trainSet TrainSet, options ...OptionSetter) {
	// Setup options
	knn.option = Option{
		sim:       MSD,
		userBased: true,
		k:         40, // the (max) number of neighbors to take into account for aggregation
		minK:      1,  // The minimum number of neighbors to take into account for aggregation.
		// If there are not enough neighbors, the prediction is set the global
		// mean of all interactionRatings
	}
	for _, setter := range options {
		setter(&knn.option)
	}
	// Set global mean for new users (items)
	knn.trainSet = trainSet
	knn.mean = trainSet.GlobalMean()
	// Retrieve user (item) ratings
	if knn.option.userBased {
		knn.ratings = trainSet.UserRatings()
		knn.sims = newNanMatrix(trainSet.UserCount(), trainSet.UserCount())
	} else {
		knn.ratings = trainSet.ItemRatings()
		knn.sims = newNanMatrix(trainSet.ItemCount(), trainSet.ItemCount())
	}
	// Pairwise similarity
	for leftId, leftRatings := range knn.ratings {
		for rightId, rightRatings := range knn.ratings {
			if leftId != rightId {
				if math.IsNaN(knn.sims[leftId][rightId]) {
					ret := knn.option.sim(leftRatings, rightRatings)
					if !math.IsNaN(ret) {
						knn.sims[leftId][rightId] = ret
						knn.sims[rightId][leftId] = ret
					}
				}
			}
		}
	}
}
