package core

// A simple yet accurate collaborative filtering algorithm[1].
//
// [1] Lemire, Daniel, and Anna Maclachlan. "Slope one predictors
// for online rating-based collaborative filtering." Proceedings
// of the 2005 SIAM International Conference on Data Mining.
// Society for Industrial and Applied Mathematics, 2005.
type SlopeOne struct {
	globalMean  float64
	userRatings [][]IdRating
	userMeans   []float64
	dev         [][]float64 // The average differences between the leftRatings of i and those of j
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
		for _, ir := range so.userRatings[innerUserId] {
			sum += so.dev[innerItemId][ir.Id]
			count++
		}
		if count > 0 {
			prediction += sum / count
		}
	}
	return prediction
}

// Fit a SlopeOne model
func (so *SlopeOne) Fit(trainSet TrainSet, params Parameters) {
	so.trainSet = trainSet
	so.globalMean = trainSet.GlobalMean
	so.userRatings = trainSet.UserRatings()
	so.userMeans = means(so.userRatings)
	so.dev = newZeroMatrix(trainSet.ItemCount, trainSet.ItemCount)
	itemRatings := trainSet.ItemRatings()
	sorts(itemRatings)
	for i := range itemRatings {
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
				so.dev[i][j] = sum / count
				so.dev[j][i] = -so.dev[i][j]
			}
		}
	}
}
