package core

type KNN struct {
}

func NewKNN() *KNN {
	return new(KNN)
}

func (knn *KNN) Predict(userId int, itemId int) float64 {
	return 0
}

func (knn *KNN) Fit(trainSet TrainSet, options ...OptionSetter) {
	// Setup options
	option := Option{
		k:    40, // the (max) number of neighbors to take into account for aggregation
		minK: 40, // The minimum number of neighbors to take into account for aggregation.
		// If there are not enough neighbors, the prediction is set the global
		// mean of all interactionRatings
	}
	for _, setter := range options {
		setter(&option)
	}
	// User based
	//history := trainSet.UserHistory()
	//
}
