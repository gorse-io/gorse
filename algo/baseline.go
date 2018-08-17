// Algorithm [Kor10] predicting the baseline estimate for given user and item.
//
//                   \hat{r}_{ui} = b_{ui} = μ + b_u + b_i
//
// If user u is unknown, then the bias bu is assumed to be zero. The same
// applies for item i with bi.

package algo

import (
	"../data"
)

type BaseLine struct {
	userBias   map[int]float64
	itemBias   map[int]float64
	globalBias float64
}

func NewBaseLineRecommend() *BaseLine {
	return &BaseLine{}
}

func (baseLine *BaseLine) Predict(userId int, itemId int) float64 {
	userBias, _ := baseLine.userBias[userId]
	itemBias, _ := baseLine.itemBias[itemId]
	return userBias + itemBias + baseLine.globalBias
}

func (baseLine *BaseLine) Fit(trainSet data.Set, options ...OptionEditor) {
	// Initialize parameters
	baseLine.userBias = make(map[int]float64)
	baseLine.itemBias = make(map[int]float64)
	for _, userId := range trainSet.AllUsers() {
		baseLine.userBias[userId] = 0
	}
	for _, itemId := range trainSet.AllItems() {
		baseLine.itemBias[itemId] = 0
	}
	// Setup options
	option := Option{
		regularization: 0.02,
		learningRate:   0.005,
		nEpoch:         20,
	}
	for _, editor := range options {
		editor(&option)
	}
	// Stochastic Gradient Descent
	users, items, ratings := trainSet.AllInteraction()
	for epoch := 0; epoch < option.nEpoch; epoch++ {
		for i := 0; i < trainSet.NRow(); i++ {
			userId := users[i]
			itemId := items[i]
			rating := ratings[i]
			userBias, _ := baseLine.userBias[userId]
			itemBias, _ := baseLine.itemBias[itemId]
			// Compute gradient
			diff := rating - baseLine.globalBias - userBias - itemBias
			gradGlobalBias := -2 * diff
			gradUserBias := -2*diff + 2*option.regularization*userBias
			gradItemBias := -2*diff + 2*option.regularization*itemBias
			// Update parameters
			baseLine.globalBias -= option.learningRate * gradGlobalBias
			baseLine.userBias[userId] -= option.learningRate * gradUserBias
			baseLine.itemBias[itemId] -= option.learningRate * gradItemBias
		}
	}
}
