package core

import (
	"math"
	"sort"
)

// KNN for collaborate filtering.
type KNN struct {
	_type         int
	globalMean    float64
	sims          [][]float64
	leftRatings   [][]IdRating
	rightRatings  [][]IdRating
	means         []float64 // Centered KNN: user (item) mean
	stdDeviations []float64 // KNN with Z Score: user (item) standard deviation
	bias          []float64 // KNN Baseline: bias
	trainSet      TrainSet
	// Parameters
	userBased bool
	k         int
	minK      int
}

// KNN type
const (
	basic    = 0
	centered = 1
	zScore   = 2
	baseline = 3
)

func NewKNN() *KNN {
	knn := new(KNN)
	knn._type = basic
	return knn
}

func NewKNNWithMean() *KNN {
	knn := new(KNN)
	knn._type = centered
	return knn
}

func NewKNNWithZScore() *KNN {
	knn := new(KNN)
	knn._type = zScore
	return knn
}

func NewKNNBaseLine() *KNN {
	knn := new(KNN)
	knn._type = baseline
	return knn
}

func (knn *KNN) Predict(userId, itemId int) float64 {
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
	candidates := make([]IdRating, 0)
	for _, ir := range knn.rightRatings[rightId] {
		if !math.IsNaN(knn.sims[leftId][ir.Id]) {
			candidates = append(candidates, ir)
		}
	}
	// Set global globalMean for a user (item) with the number of neighborhoods less than min k
	if len(candidates) <= knn.minK {
		return knn.globalMean
	}
	// Sort users (items) by similarity
	candidateSet := newCandidateSet(knn.sims[leftId], candidates)
	sort.Sort(candidateSet)
	// Find neighborhoods
	numNeighbors := knn.k
	if numNeighbors > candidateSet.Len() {
		numNeighbors = candidateSet.Len()
	}
	// Predict the rating by weighted globalMean
	weightSum := 0.0
	weightRating := 0.0
	for _, or := range candidateSet.candidates[0:numNeighbors] {
		weightSum += knn.sims[leftId][or.Id]
		rating := or.Rating
		if knn._type == centered {
			rating -= knn.means[or.Id]
		} else if knn._type == zScore {
			rating = (rating - knn.means[or.Id]) / knn.stdDeviations[or.Id]
		} else if knn._type == baseline {
			rating -= knn.bias[or.Id]
		}
		weightRating += knn.sims[leftId][or.Id] * rating
	}
	prediction := weightRating / weightSum
	if knn._type == centered {
		prediction += knn.means[leftId]
	} else if knn._type == zScore {
		prediction *= knn.stdDeviations[leftId]
		prediction += knn.means[leftId]
	} else if knn._type == baseline {
		prediction += knn.bias[leftId]
	}
	return prediction
}

// Fit a KNN model. Parameters:
//   sim		- The similarity function. Default is MSD.
//   userBased	- User based or item based? Default is true.
//	 k			- The maximum k neighborhoods to predict the rating. Default is 40.
//	 minK		- The minimum k neighborhoods to predict the rating. Default is 1.
func (knn *KNN) Fit(trainSet TrainSet, params Parameters) {
	// Setup parameters
	sim := params.GetSim("sim", MSD)
	knn.userBased = params.GetBool("userBased", true)
	knn.k = params.GetInt("k", 40)
	knn.minK = params.GetInt("minK", 1)
	// Set global globalMean for new users (items)
	knn.trainSet = trainSet
	knn.globalMean = trainSet.GlobalMean
	// Retrieve user (item) iRatings
	if knn.userBased {
		knn.leftRatings = trainSet.UserRatings()
		knn.rightRatings = trainSet.ItemRatings()
		knn.sims = newNanMatrix(trainSet.UserCount, trainSet.UserCount)
	} else {
		knn.leftRatings = trainSet.ItemRatings()
		knn.rightRatings = trainSet.UserRatings()
		knn.sims = newNanMatrix(trainSet.ItemCount, trainSet.ItemCount)
	}
	// Retrieve user (item) mean
	if knn._type == centered || knn._type == zScore {
		knn.means = means(knn.leftRatings)
	}
	// Retrieve user (item) standard deviation
	if knn._type == zScore {
		knn.stdDeviations = make([]float64, len(knn.leftRatings))
		for i := range knn.means {
			sum, count := 0.0, 0.0
			for _, ir := range knn.leftRatings[i] {
				sum += (ir.Rating - knn.means[i]) * (ir.Rating - knn.means[i])
				count++
			}
			knn.stdDeviations[i] = math.Sqrt(sum/count) + 1e-5
		}
	}
	if knn._type == baseline {
		baseLine := NewBaseLine()
		baseLine.Fit(trainSet, params)
		if knn.userBased {
			knn.bias = baseLine.userBias
		} else {
			knn.bias = baseLine.itemBias
		}
	}
	// Pairwise similarity
	sortedLeftRatings := sorts(knn.leftRatings)
	for iId, iRatings := range sortedLeftRatings {
		for jId, jRatings := range sortedLeftRatings {
			if iId != jId {
				if math.IsNaN(knn.sims[iId][jId]) {
					ret := sim(iRatings, jRatings)
					if !math.IsNaN(ret) {
						knn.sims[iId][jId] = ret
						knn.sims[jId][iId] = ret
					}
				}
			}
		}
	}
}

// A data structure used to sort candidates by similarity.
type _CandidateSet struct {
	similarities []float64
	candidates   []IdRating
}

func newCandidateSet(sim []float64, candidates []IdRating) *_CandidateSet {
	neighbors := _CandidateSet{}
	neighbors.similarities = sim
	neighbors.candidates = candidates
	return &neighbors
}

func (cs *_CandidateSet) Len() int {
	return len(cs.candidates)
}

func (cs *_CandidateSet) Less(i, j int) bool {
	return cs.similarities[cs.candidates[i].Id] > cs.similarities[cs.candidates[j].Id]
}

func (cs *_CandidateSet) Swap(i, j int) {
	cs.candidates[i], cs.candidates[j] = cs.candidates[j], cs.candidates[i]
}
