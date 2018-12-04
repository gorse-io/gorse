package model

import (
	. "github.com/zhenghaoz/gorse/core"
	. "github.com/zhenghaoz/gorse/core/base"
	"runtime"
)

// SlopeOne, a collaborative filtering algorithm[1].
//
// [1] Lemire, Daniel, and Anna Maclachlan. "Slope one predictors
// for online rating-based collaborative filtering." Proceedings
// of the 2005 SIAM International Conference on Data Mining.
// Society for Industrial and Applied Mathematics, 2005.
type SlopeOne struct {
	Base
	GlobalMean  float64
	UserRatings []SparseVector
	UserMeans   []float64
	Dev         [][]float64 // The average differences between the LeftRatings of i and those of j
}

// NewSlopOne creates a slop one model. Params:
//	 nJobs		- The number of goroutines to compute deviation. Default is the number of CPUs.
func NewSlopOne(params Params) *SlopeOne {
	so := new(SlopeOne)
	so.Params = params
	return so
}

// Predict by a SlopOne model.
func (so *SlopeOne) Predict(userId, itemId int) float64 {
	// Convert to inner Id
	innerUserId := so.UserIdSet.ToSparseId(userId)
	innerItemId := so.ItemIdSet.ToSparseId(itemId)
	prediction := 0.0
	if innerUserId != NewId {
		prediction = so.UserMeans[innerUserId]
	} else {
		prediction = so.GlobalMean
	}
	if innerItemId != NewId {
		sum, count := 0.0, 0.0
		so.UserRatings[innerUserId].ForEach(func(i, index int, value float64) {
			sum += so.Dev[innerItemId][index]
			count++
		})
		if count > 0 {
			prediction += sum / count
		}
	}
	return prediction
}

// Fit a SlopeOne model.
func (so *SlopeOne) Fit(trainSet TrainSet, setters ...RuntimeOptionSetter) {
	so.Init(trainSet, setters)
	nJobs := runtime.NumCPU()
	so.GlobalMean = trainSet.GlobalMean
	so.UserRatings = trainSet.UserRatings
	so.UserMeans = means(so.UserRatings)
	so.Dev = zeros(trainSet.ItemCount(), trainSet.ItemCount())
	itemRatings := trainSet.ItemRatings
	parallel(len(itemRatings), nJobs, func(begin, end int) {
		for i := begin; i < end; i++ {
			for j := 0; j < i; j++ {
				count, sum := 0.0, 0.0
				// Find common user's ratings
				itemRatings[i].ForIntersection(&itemRatings[j], func(index int, a float64, b float64) {
					sum += a - b
					count++
				})
				if count > 0 {
					so.Dev[i][j] = sum / count
					so.Dev[j][i] = -so.Dev[i][j]
				}
			}
		}
	})
}
