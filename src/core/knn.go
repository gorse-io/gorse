package core

import (
	"math"
	"sort"
)

type KNN struct {
	option  Option
	mean    float64
	sims    map[int]map[int]float64
	ratings map[int]map[int]float64
}

type CandidateSet struct {
	similarities map[int]float64
	candidates   []int
}

func NewCandidateSet(sim map[int]float64, candidates []int) *CandidateSet {
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
	// Set user based or item based
	var leftId, rightId int
	if knn.option.userBased {
		leftId, rightId = userId, itemId
	} else {
		leftId, rightId = itemId, userId
	}
	// Find user (item) interacted with item (user)
	candidates := make([]int, 0)
	for otherId := range knn.ratings {
		if _, exist := knn.ratings[otherId][rightId]; exist && !math.IsNaN(knn.sims[leftId][otherId]) {
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
		weightRating += knn.sims[userId][otherId] * knn.ratings[otherId][itemId]
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
	knn.mean = trainSet.GlobalMean()
	// Retrieve user (item) ratings
	if knn.option.userBased {
		knn.ratings = trainSet.UserRatings()
	} else {
		knn.ratings = trainSet.ItemRatings()
	}
	// Pairwise similarity
	knn.sims = make(map[int]map[int]float64)
	for leftId, leftRatings := range knn.ratings {
		for rightId, rightRatings := range knn.ratings {
			if leftId != rightId {
				// Create secondary map
				if _, exist := knn.sims[leftId]; !exist {
					knn.sims[leftId] = make(map[int]float64)
				}
				if _, exist := knn.sims[rightId]; !exist {
					knn.sims[rightId] = make(map[int]float64)
				}
				// Set similarity values
				if _, exist := knn.sims[leftId][rightId]; !exist {
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
