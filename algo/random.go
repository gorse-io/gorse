// Algorithm predicting a random rating based on the distribution of
// the training set, which is assumed to be normal. The prediction
// \hat{r}_{ui} is generated from a normal distribution N(\hat{μ},\hat{σ}^2)
// where \hat{μ} and \hat{σ}^2 are estimated from the training data
// using Maximum Likelihood Estimation

package algo

import (
	"../data"
	"github.com/gonum/stat"
	"math/rand"
)

type Random struct {
	mean   float64
	stdDev float64
}

func NewRandomRecommend() *Random {
	return &Random{}
}

func (random *Random) Predict(userId int, itemId int) float64 {
	return rand.NormFloat64()*random.stdDev + random.mean
}

func (random *Random) Fit(trainSet data.Set, options ...OptionEditor) {
	ratings := trainSet.AllRatings()
	random.mean = stat.Mean(ratings, nil)
	random.stdDev = stat.StdDev(ratings, nil)
}
