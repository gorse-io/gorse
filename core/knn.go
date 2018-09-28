package core

import (
	"math"
	"runtime"
	"sort"
)

// KNN for collaborate filtering.
type KNN struct {
	Base
	KNNType      string
	GlobalMean   float64
	Sims         [][]float64
	LeftRatings  [][]IdRating
	RightRatings [][]IdRating
	Means        []float64 // Centered KNN: user (item) Mean
	StdDevs      []float64 // KNN with Z Score: user (item) standard deviation
	Bias         []float64 // KNN Baseline: Bias
}

// KNN type
const (
	basic    = "basic"
	centered = "centered"
	zScore   = "zscore"
	baseline = "baseline"
)

// NewKNN creates a KNN model. Parameters:
//   sim       - The similarity function. Default is MSD.
//   userBased - User based or item based? Default is true.
//   k         - The maximum k neighborhoods to predict the rating. Default is 40.
//   minK      - The minimum k neighborhoods to predict the rating. Default is 1.
//   nJobs     - The number of goroutines to compute similarity. Default is the number of CPUs.
func NewKNN(params Parameters) *KNN {
	knn := new(KNN)
	knn.Params = params
	knn.KNNType = knn.Params.GetString("type", basic)
	return knn
}

// NewKNNWithMean creates a KNN model with Mean. Parameters:
//   sim       - The similarity function. Default is MSD.
//   userBased - User based or item based? Default is true.
//   k         - The maximum k neighborhoods to predict the rating. Default is 40.
//   minK      - The minimum k neighborhoods to predict the rating. Default is 1.
//   nJobs     - The number of goroutines to compute similarity. Default is the number of CPUs.
func NewKNNWithMean(params Parameters) *KNN {
	knn := new(KNN)
	knn.Params = params
	knn.KNNType = knn.Params.GetString("type", centered)
	return knn
}

// NewKNNWithZScore creates a KNN model with Z-Score. Parameters:
//   sim       - The similarity function. Default is MSD.
//   userBased - User based or item based? Default is true.
//   k         - The maximum k neighborhoods to predict the rating. Default is 40.
//   minK      - The minimum k neighborhoods to predict the rating. Default is 1.
//   nJobs     - The number of goroutines to compute similarity. Default is the number of CPUs.
func NewKNNWithZScore(params Parameters) *KNN {
	knn := new(KNN)
	knn.Params = params
	knn.KNNType = knn.Params.GetString("type", zScore)
	return knn
}

// NewKNNBaseLine creates a KNN model with baseline. Parameters:
//   sim       - The similarity function. Default is MSD.
//   userBased - User based or item based? Default is true.
//   k         - The maximum k neighborhoods to predict the rating. Default is 40.
//   minK      - The minimum k neighborhoods to predict the rating. Default is 1.
//   nJobs     - The number of goroutines to compute similarity. Default is the number of CPUs.
func NewKNNBaseLine(params Parameters) *KNN {
	knn := new(KNN)
	knn.Params = params
	knn.KNNType = knn.Params.GetString("type", baseline)
	return knn
}

// Predict by a KNN model.
func (knn *KNN) Predict(userId, itemId int) float64 {
	innerUserId := knn.Data.ConvertUserId(userId)
	innerItemId := knn.Data.ConvertItemId(itemId)
	// Retrieve parameters
	userBased := knn.Params.GetBool("userBased", true)
	k := knn.Params.GetInt("k", 40)
	minK := knn.Params.GetInt("minK", 1)
	// Set user based or item based
	var leftId, rightId int
	if userBased {
		leftId, rightId = innerUserId, innerItemId
	} else {
		leftId, rightId = innerItemId, innerUserId
	}
	if leftId == NewId || rightId == NewId {
		return knn.GlobalMean
	}
	// Find user (item) interacted with item (user)
	candidates := make([]IdRating, 0)
	for _, ir := range knn.RightRatings[rightId] {
		if !math.IsNaN(knn.Sims[leftId][ir.Id]) {
			candidates = append(candidates, ir)
		}
	}
	// Set global GlobalMean for a user (item) with the number of neighborhoods less than min k
	if len(candidates) <= minK {
		return knn.GlobalMean
	}
	// Sort users (items) by similarity
	candidateSet := newCandidateSet(knn.Sims[leftId], candidates)
	sort.Sort(candidateSet)
	// Find neighborhoods
	numNeighbors := k
	if numNeighbors > candidateSet.Len() {
		numNeighbors = candidateSet.Len()
	}
	// Predict the rating by weighted GlobalMean
	weightSum := 0.0
	weightRating := 0.0
	for _, or := range candidateSet.candidates[0:numNeighbors] {
		weightSum += knn.Sims[leftId][or.Id]
		rating := or.Rating
		if knn.KNNType == centered {
			rating -= knn.Means[or.Id]
		} else if knn.KNNType == zScore {
			rating = (rating - knn.Means[or.Id]) / knn.StdDevs[or.Id]
		} else if knn.KNNType == baseline {
			rating -= knn.Bias[or.Id]
		}
		weightRating += knn.Sims[leftId][or.Id] * rating
	}
	prediction := weightRating / weightSum
	if knn.KNNType == centered {
		prediction += knn.Means[leftId]
	} else if knn.KNNType == zScore {
		prediction *= knn.StdDevs[leftId]
		prediction += knn.Means[leftId]
	} else if knn.KNNType == baseline {
		prediction += knn.Bias[leftId]
	}
	return prediction
}

// Fit a KNN model.
func (knn *KNN) Fit(trainSet TrainSet) {
	knn.Base.Fit(trainSet)
	// Setup parameters
	sim := knn.Params.GetSim("sim", MSD)
	userBased := knn.Params.GetBool("userBased", true)
	nJobs := knn.Params.GetInt("nJobs", runtime.NumCPU())
	// Set global GlobalMean for new users (items)
	knn.GlobalMean = trainSet.GlobalMean
	// Retrieve user (item) iRatings
	if userBased {
		knn.LeftRatings = trainSet.UserRatings()
		knn.RightRatings = trainSet.ItemRatings()
		knn.Sims = newNanMatrix(trainSet.UserCount, trainSet.UserCount)
	} else {
		knn.LeftRatings = trainSet.ItemRatings()
		knn.RightRatings = trainSet.UserRatings()
		knn.Sims = newNanMatrix(trainSet.ItemCount, trainSet.ItemCount)
	}
	// Retrieve user (item) Mean
	if knn.KNNType == centered || knn.KNNType == zScore {
		knn.Means = means(knn.LeftRatings)
	}
	// Retrieve user (item) standard deviation
	if knn.KNNType == zScore {
		knn.StdDevs = make([]float64, len(knn.LeftRatings))
		for i := range knn.Means {
			sum, count := 0.0, 0.0
			for _, ir := range knn.LeftRatings[i] {
				sum += (ir.Rating - knn.Means[i]) * (ir.Rating - knn.Means[i])
				count++
			}
			knn.StdDevs[i] = math.Sqrt(sum/count) + 1e-5
		}
	}
	if knn.KNNType == baseline {
		baseLine := NewBaseLine(knn.Params)
		baseLine.Fit(trainSet)
		if userBased {
			knn.Bias = baseLine.UserBias
		} else {
			knn.Bias = baseLine.ItemBias
		}
	}
	// Pairwise similarity
	sortedLeftRatings := sorts(knn.LeftRatings)
	parallel(len(sortedLeftRatings), nJobs, func(begin, end int) {
		for iId := begin; iId < end; iId++ {
			iRatings := sortedLeftRatings[iId]
			for jId, jRatings := range sortedLeftRatings {
				if iId != jId {
					if math.IsNaN(knn.Sims[iId][jId]) {
						ret := sim(iRatings, jRatings)
						if !math.IsNaN(ret) {
							knn.Sims[iId][jId] = ret
							knn.Sims[jId][iId] = ret
						}
					}
				}
			}
		}
	})
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
