// A collaborative filtering algorithm based on Non-negative
// Matrix Factorization.  This algorithm is very similar to
// SVD. The prediction \hat{r}_{ui} is set as:

package core

import (
	"github.com/gonum/floats"
)

type NMF struct {
	userFactor map[int][]float64 // p_u
	itemFactor map[int][]float64 // q_i
}

func NewNMF() *NMF {
	return new(NMF)
}

func (nmf *NMF) Predict(userId int, itemId int) float64 {
	userFactor, _ := nmf.userFactor[userId]
	itemFactor, _ := nmf.itemFactor[itemId]
	if len(userFactor) == len(itemFactor) {
		return floats.Dot(userFactor, itemFactor)
	}
	return 0
}

func (nmf *NMF) Fit(trainSet Set, options ...OptionSetter) {
	option := Option{
		nFactors: 15,
		nEpochs:  50,
		initLow:  0,
		initHigh: 1,
		reg:      0.06,
		lr:       0.005,
	}
	for _, editor := range options {
		editor(&option)
	}
	// Initialize parameters
	nmf.userFactor = make(map[int][]float64)
	nmf.itemFactor = make(map[int][]float64)
	for _, userId := range trainSet.AllUsers() {
		nmf.userFactor[userId] = NewNormalVector(option.nFactors, option.initMean, option.initStdDev)
	}
	for _, itemId := range trainSet.AllItems() {
		nmf.itemFactor[itemId] = NewNormalVector(option.nFactors, option.initMean, option.initStdDev)
	}
}
