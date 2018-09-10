package core

import "math"

// A simple yet accurate collaborative filtering algorithm[1].
//
// [1] Lemire, Daniel, and Anna Maclachlan. "Slope one predictors
// for online rating-based collaborative filtering." Proceedings
// of the 2005 SIAM International Conference on Data Mining.
// Society for Industrial and Applied Mathematics, 2005.
type SlopeOne struct {
	globalMean  float64
	userRatings [][]float64
	userMeans   []float64
	dev         [][]float64 // The average differences between the ratings of i and those of j
	trainSet    TrainSet
}

func NewSlopOne() *SlopeOne {
	return new(SlopeOne)
}

func (so *SlopeOne) Predict(userId, itemId int) float64 {
	// Convert to inner Id
	innerUserId := so.trainSet.ConvertUserId(userId)
	innerItemId := so.trainSet.ConvertItemId(itemId)
	prediction := 0.0
	if innerUserId != newId {
		prediction = so.userMeans[innerUserId]
	} else {
		prediction = so.globalMean
	}
	if innerItemId != newId {
		sum, count := 0.0, 0.0
		for j := range so.userRatings[innerUserId] {
			if !math.IsNaN(so.userRatings[innerUserId][j]) {
				sum += so.dev[innerItemId][j]
				count++
			}
		}
		if count > 0 {
			prediction += sum / count
		}
	}
	return prediction
}

func (so *SlopeOne) Fit(trainSet TrainSet, params Parameters) {
	so.trainSet = trainSet
	so.globalMean = trainSet.GlobalMean()
	so.userRatings = trainSet.UserRatings()
	so.userMeans = means(so.userRatings)
	so.dev = newZeroMatrix(trainSet.ItemCount(), trainSet.ItemCount())
	ratings := trainSet.ItemRatings()
	for i := 0; i < len(ratings); i++ {
		for j := 0; j < i; j++ {
			count, sum := 0.0, 0.0
			for k := 0; k < len(ratings[i]); k++ {
				if !math.IsNaN(ratings[i][k]) && !math.IsNaN(ratings[j][k]) {
					count++
					sum += ratings[i][k] - ratings[j][k]
				}
			}
			if count > 0 {
				so.dev[i][j] = sum / count
				so.dev[j][i] = -so.dev[i][j]
			}
		}
	}
}
