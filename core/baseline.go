package core

// Algorithm predicting the baseline estimate for given user and item.
//
//                   \hat{r}_{ui} = b_{ui} = μ + b_u + b_i
//
// If user u is unknown, then the bias b_u is assumed to be zero. The same
// applies for item i with b_i.
type BaseLine struct {
	userBias   []float64 // b_u
	itemBias   []float64 // b_i
	globalBias float64   // mu
	trainSet   TrainSet
}

func NewBaseLine() *BaseLine {
	return new(BaseLine)
}

func (baseLine *BaseLine) Predict(userId, itemId int) float64 {
	// Convert to inner Id
	innerUserId := baseLine.trainSet.ConvertUserId(userId)
	innerItemId := baseLine.trainSet.ConvertItemId(itemId)
	ret := baseLine.globalBias
	if innerUserId != newId {
		ret += baseLine.userBias[innerUserId]
	}
	if innerItemId != newId {
		ret += baseLine.itemBias[innerItemId]
	}
	return ret
}

// Fit a baseline model.
// Parameters:
//	 reg 		- The regularization parameter of the cost function that is
// 				  optimized. Default is 0.02.
//	 lr 		- The learning rate of SGD. Default is 0.005.
//	 nEpochs	- The number of iteration of the SGD procedure. Default is 20.
func (baseLine *BaseLine) Fit(trainSet TrainSet, params Parameters) {
	// Setup parameters
	reader := newParameterReader(params)
	reg := reader.getFloat64("reg", 0.02)
	lr := reader.getFloat64("lr", 0.005)
	nEpochs := reader.getInt("nEpochs", 20)
	// Initialize parameters
	baseLine.trainSet = trainSet
	baseLine.userBias = make([]float64, trainSet.UserCount())
	baseLine.itemBias = make([]float64, trainSet.ItemCount())
	// Stochastic Gradient Descent
	for epoch := 0; epoch < nEpochs; epoch++ {
		for i := 0; i < trainSet.Length(); i++ {
			userId, itemId, rating := trainSet.Users[i], trainSet.Items[i], trainSet.Ratings[i]
			innerUserId := trainSet.ConvertUserId(userId)
			innerItemId := trainSet.ConvertItemId(itemId)
			userBias := baseLine.userBias[innerUserId]
			itemBias := baseLine.itemBias[innerItemId]
			// Compute gradient
			diff := baseLine.Predict(userId, itemId) - rating
			gradGlobalBias := diff
			gradUserBias := diff + reg*userBias
			gradItemBias := diff + reg*itemBias
			// Update parameters
			baseLine.globalBias -= lr * gradGlobalBias
			baseLine.userBias[innerUserId] -= lr * gradUserBias
			baseLine.itemBias[innerItemId] -= lr * gradItemBias
		}
	}
}
