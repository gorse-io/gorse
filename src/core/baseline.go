// Algorithm predicting the baseline estimate for given user and item.
//
//                   \hat{r}_{ui} = b_{ui} = μ + b_u + b_i
//
// If user u is unknown, then the bias bu is assumed to be zero. The same
// applies for item i with bi.

package core

type BaseLine struct {
	userBias   map[int]float64 // b_u
	itemBias   map[int]float64 // b_i
	globalBias float64         // mu
}

func NewBaseLine() *BaseLine {
	return new(BaseLine)
}

func (baseLine *BaseLine) Predict(userId int, itemId int) float64 {
	userBias, _ := baseLine.userBias[userId]
	itemBias, _ := baseLine.itemBias[itemId]
	ret := userBias + itemBias + baseLine.globalBias
	return ret
}

func (baseLine *BaseLine) Fit(trainSet TrainSet, options ...OptionSetter) {
	// Setup options
	option := Option{
		reg:     0.02,
		lr:      0.005,
		nEpochs: 20,
	}
	for _, editor := range options {
		editor(&option)
	}
	// Initialize parameters
	baseLine.userBias = make(map[int]float64)
	baseLine.itemBias = make(map[int]float64)
	for _, userId := range trainSet.Users() {
		baseLine.userBias[userId] = 0
	}
	for _, itemId := range trainSet.Items() {
		baseLine.itemBias[itemId] = 0
	}
	// Stochastic Gradient Descent
	users, items, ratings := trainSet.Interactions()
	for epoch := 0; epoch < option.nEpochs; epoch++ {
		for i := 0; i < trainSet.Length(); i++ {
			userId := users[i]
			itemId := items[i]
			rating := ratings[i]
			userBias, _ := baseLine.userBias[userId]
			itemBias, _ := baseLine.itemBias[itemId]
			// Compute gradient
			diff := baseLine.Predict(userId, itemId) - rating
			gradGlobalBias := diff
			gradUserBias := diff + option.reg*userBias
			gradItemBias := diff + option.reg*itemBias
			// Update parameters
			baseLine.globalBias -= option.lr * gradGlobalBias
			baseLine.userBias[userId] -= option.lr * gradUserBias
			baseLine.itemBias[itemId] -= option.lr * gradItemBias
		}
	}
}
