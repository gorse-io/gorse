package model

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
)

// SlopeOne [4] predicts ratings by the form f(x) = x + b, which precompute
// the average difference between the ratings of one item and another for
// users who rated both.
//
// First, deviations between pairs of items are computed. Given a training
// set χ, and any two items j and i with ratings u_j and u_i respectively
// in some user evaluation u (annotated as u∈S_{j,i}(χ)), the average
// deviation of item i with respect to item j is computed by:
//  dev_{j,i} = \sum_{u∈S_{j,i}(χ)} \frac{u_j-u_i} {card(S_{j,i}(χ)}
// The computation on deviations could be parallelized.
//
// In the predicting stage, Given that dev_{j,i} + u_i is a prediction for
// u_j given u_i, a reasonable predictor might be the average of all such
// predictions
//  P(u)_j = \frac{1}{card(R_j) \sum_{i∈R_j}(dev_{j,i} + u_i)
// where R_j = {i|i ∈ S(u), i \ne j, card(S_{j,i}(χ)) > 0} is the set of
// all relevant items. The subset of the set of items consisting of all
// those items which are rated in u is S(u).
type SlopeOne struct {
	Base
	GlobalMean  float64                // Mean of ratings in training set
	UserRatings []*base.MarginalSubSet // Ratings by each user
	UserMeans   []float64              // Mean of each user's ratings
	Dev         [][]float64            // Deviations
}

// NewSlopOne creates a SlopeOne model.
func NewSlopOne(params base.Params) *SlopeOne {
	so := new(SlopeOne)
	so.SetParams(params)
	return so
}

// Predict by the SlopeOne model.
func (so *SlopeOne) Predict(userId, itemId int) float64 {
	// Convert to index
	userIndex := so.UserIndexer.ToIndex(userId)
	itemIndex := so.ItemIndexer.ToIndex(itemId)
	prediction := 0.0
	if userIndex != base.NotId {
		prediction = so.UserMeans[userIndex]
	} else {
		// Use global mean for new user
		prediction = so.GlobalMean
	}
	if itemIndex != base.NotId {
		sum, count := 0.0, 0.0
		so.UserRatings[userIndex].ForEachIndex(func(i, index int, value float64) {
			sum += so.Dev[itemIndex][index]
			count++
		})
		if count > 0 {
			prediction += sum / count
		}
	}
	return prediction
}

// Fit the SlopeOne model.
func (so *SlopeOne) Fit(trainSet core.DataSetInterface, setters ...core.RuntimeOption) {
	// Initialize
	so.Init(trainSet, setters)
	so.GlobalMean = trainSet.GlobalMean()
	so.UserRatings = trainSet.Users()
	so.UserMeans = make([]float64, trainSet.UserCount())
	for i := 0; i < trainSet.UserCount(); i++ {
		ratings := trainSet.UserByIndex(i)
		so.UserMeans[i] = ratings.Mean()
	}
	so.Dev = base.NewMatrix(trainSet.ItemCount(), trainSet.ItemCount())
	// Compute deviations
	base.ParallelFor(0, trainSet.ItemCount(), func(i int) {
		for j := 0; j < i; j++ {
			count, sum := 0.0, 0.0
			// Find common user's ratings
			trainSet.ItemByIndex(i).ForIntersection(trainSet.ItemByIndex(j), func(index int, a float64, b float64) {
				sum += a - b
				count++
			})
			if count > 0 {
				so.Dev[i][j] = sum / count
				so.Dev[j][i] = -so.Dev[i][j]
			}
		}
	})
}
