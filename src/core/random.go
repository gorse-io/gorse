// Algorithm predicting a random rating based on the distribution of
// the training set, which is assumed to be normal. The prediction
// \hat{r}_{ui} is generated from a normal distribution N(\hat{μ},\hat{σ}^2)
// where \hat{μ} and \hat{σ}^2 are estimated from the training data
// using Maximum Likelihood Estimation

package core

import (
	"github.com/gonum/stat"
	"math/rand"
)

type Random struct {
	mean   float64 // mu
	stdDev float64 // sigma
	low    float64
	high   float64
}

func NewRandom() *Random {
	return new(Random)
}

func (random *Random) Predict(userId int, itemId int) float64 {
	ret := rand.NormFloat64()*random.stdDev + random.mean
	// Crop prediction
	if ret < random.low {
		ret = random.low
	} else if ret > random.high {
		ret = random.high
	}
	return ret
}

func (random *Random) Fit(trainSet TrainSet, options Options) {
	_, _, ratings := trainSet.Interactions()
	random.mean = stat.Mean(ratings, nil)
	random.stdDev = stat.StdDev(ratings, nil)
	random.low, random.high = trainSet.RatingRange()
}
