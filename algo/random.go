// Random Algorithm predicting a random rating based on the distribution
// of the training set, which is assumed to be normal.

package algo

import (
	"../data"
	"math/rand"
	"github.com/gonum/stat"
)

type Random struct {
	mean float64
	stdDev float64
}

func (random *Random) Predict(userId int, itemId int) float64 {
	return rand.NormFloat64() * random.stdDev + random.mean
}

func (random *Random) Fit(trainSet data.Set) {
	ratings := trainSet.AllRatings()
	random.mean = stat.Mean(ratings, nil)
	random.stdDev = stat.StdDev(ratings, nil)
}
