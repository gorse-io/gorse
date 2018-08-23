// Algorithm predicting a random rating based on the distribution of
// the training set, which is assumed to be normal. The prediction
// \hat{r}_{ui} is generated from a normal distribution N(\hat{μ},\hat{σ}^2)
// where \hat{μ} and \hat{σ}^2 are estimated from the training data
// using Maximum Likelihood Estimation

package algo

import (
	"core/data"
	"github.com/gonum/stat"
	"math/rand"
)

type Random struct {
	mean   float64 // mu
	stdDev float64 // sigma
}

func NewRandom() *Random {
	return new(Random)
}

func (random *Random) Predict(userId int, itemId int) float64 {
	return rand.NormFloat64()*random.stdDev + random.mean
}

func (random *Random) Fit(trainSet data.Set, options ...OptionSetter) {
	ratings := trainSet.AllRatings()
	random.mean = stat.Mean(ratings, nil)
	random.stdDev = stat.StdDev(ratings, nil)
}
