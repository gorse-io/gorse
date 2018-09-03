// Algorithm predicting the baseline estimate for given user and item.
//
//                   \hat{r}_{ui} = b_{ui} = μ + b_u + b_i
//
// If user u is unknown, then the bias bu is assumed to be zero. The same
// applies for item i with bi.

package core

type BaseLine struct {
	userBias   []float64 // b_u
	itemBias   []float64 // b_i
	globalBias float64   // mu
	trainSet   TrainSet
}

func NewBaseLine() *BaseLine {
	return new(BaseLine)
}

func (baseLine *BaseLine) Predict(userId int, itemId int) float64 {
	// Convert to inner Id
	innerUserId := baseLine.trainSet.ConvertUserId(userId)
	innerItemId := baseLine.trainSet.ConvertItemId(itemId)
	ret := baseLine.globalBias
	if innerUserId != noBody {
		ret += baseLine.userBias[innerUserId]
	}
	if innerItemId != noBody {
		ret += baseLine.itemBias[innerItemId]
	}
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
	baseLine.trainSet = trainSet
	baseLine.userBias = make([]float64, trainSet.UserCount())
	baseLine.itemBias = make([]float64, trainSet.ItemCount())
	// Stochastic Gradient Descent
	users, items, ratings := trainSet.Interactions()
	for epoch := 0; epoch < option.nEpochs; epoch++ {
		for i := 0; i < trainSet.Length(); i++ {
			userId, itemId, rating := users[i], items[i], ratings[i]
			innerUserId := trainSet.ConvertUserId(userId)
			innerItemId := trainSet.ConvertItemId(itemId)
			userBias := baseLine.userBias[innerUserId]
			itemBias := baseLine.itemBias[innerItemId]
			// Compute gradient
			diff := baseLine.Predict(userId, itemId) - rating
			gradGlobalBias := diff
			gradUserBias := diff + option.reg*userBias
			gradItemBias := diff + option.reg*itemBias
			// Update parameters
			baseLine.globalBias -= option.lr * gradGlobalBias
			baseLine.userBias[innerUserId] -= option.lr * gradUserBias
			baseLine.itemBias[innerItemId] -= option.lr * gradItemBias
		}
	}
}
