package core

import (
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
	UserRatings [][]IdRating
	UserMeans   []float64
	Dev         [][]float64 // The average differences between the LeftRatings of i and those of j
}

// NewSlopOne creates a slop one model. Parameters:
//	 nJobs		- The number of goroutines to compute deviation. Default is the number of CPUs.
func NewSlopOne(params Parameters) *SlopeOne {
	so := new(SlopeOne)
	so.Params = params
	return so
}

// Predict by a SlopOne model.
func (so *SlopeOne) Predict(userId, itemId int) float64 {
	// Convert to inner Id
	innerUserId := so.Data.ConvertUserId(userId)
	innerItemId := so.Data.ConvertItemId(itemId)
	prediction := 0.0
	if innerUserId != NewId {
		prediction = so.UserMeans[innerUserId]
	} else {
		prediction = so.GlobalMean
	}
	if innerItemId != NewId {
		sum, count := 0.0, 0.0
		for _, ir := range so.UserRatings[innerUserId] {
			sum += so.Dev[innerItemId][ir.Id]
			count++
		}
		if count > 0 {
			prediction += sum / count
		}
	}
	return prediction
}

// Fit a SlopeOne model.
func (so *SlopeOne) Fit(trainSet TrainSet) {
	nJobs := runtime.NumCPU()
	so.Data = trainSet
	so.GlobalMean = trainSet.GlobalMean
	so.UserRatings = trainSet.UserRatings()
	so.UserMeans = means(so.UserRatings)
	so.Dev = newZeroMatrix(trainSet.ItemCount, trainSet.ItemCount)
	itemRatings := trainSet.ItemRatings()
	sorts(itemRatings)
	parallel(len(itemRatings), nJobs, func(begin, end int) {
		for i := begin; i < end; i++ {
			for j := 0; j < i; j++ {
				count, sum, ptr := 0.0, 0.0, 0
				// Find common user's ratings
				for k := 0; k < len(itemRatings[i]) && ptr < len(itemRatings[j]); k++ {
					ur := itemRatings[i][k]
					for ptr < len(itemRatings[j]) && itemRatings[j][ptr].Id < ur.Id {
						ptr++
					}
					if ptr < len(itemRatings[j]) && itemRatings[j][ptr].Id == ur.Id {
						count++
						sum += ur.Rating - itemRatings[j][ptr].Rating
					}
				}
				if count > 0 {
					so.Dev[i][j] = sum / count
					so.Dev[j][i] = -so.Dev[i][j]
				}
			}
		}
	})
}
