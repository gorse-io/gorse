package model

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
)

// SlopeOne, a collaborative filtering algorithm[4].
type SlopeOne struct {
	BaseModel
	GlobalMean  float64
	UserRatings []base.SparseVector
	UserMeans   []float64
	Dev         [][]float64 // The average differences between the LeftRatings of i and those of j
}

// NewSlopOne creates a slop one model. Params:
//	 nJobs		- The number of goroutines to compute deviation. Default is the number of CPUs.
func NewSlopOne(params base.Params) *SlopeOne {
	so := new(SlopeOne)
	so.SetParams(params)
	return so
}

func (so *SlopeOne) Predict(userId, itemId int) float64 {
	// Convert to inner Id
	innerUserId := so.UserIdSet.ToDenseId(userId)
	innerItemId := so.ItemIdSet.ToDenseId(itemId)
	prediction := 0.0
	if innerUserId != base.NotId {
		prediction = so.UserMeans[innerUserId]
	} else {
		prediction = so.GlobalMean
	}
	if innerItemId != base.NotId {
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

func (so *SlopeOne) Fit(trainSet core.DataSet, setters ...base.FitOption) {
	so.Init(trainSet, setters)
	so.GlobalMean = trainSet.GlobalMean
	so.UserRatings = trainSet.DenseUserRatings
	so.UserMeans = base.SparseVectorsMean(so.UserRatings)
	so.Dev = base.MakeMatrix(trainSet.ItemCount(), trainSet.ItemCount())
	itemRatings := trainSet.DenseItemRatings
	base.Parallel(len(itemRatings), so.rtOptions.NJobs, func(begin, end int) {
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
