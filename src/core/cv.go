package core

func CrossValidate(algorithm Algorithm, dataSet TrainSet, metrics []Metrics, cv int, seed int64,
	options Options) [][]float64 {
	ret := make([][]float64, len(metrics))
	for i := 0; i < len(ret); i++ {
		ret[i] = make([]float64, cv)
	}
	// Split data set
	trainFolds, testFolds := dataSet.KFold(cv, seed)
	for i := 0; i < cv; i++ {
		trainFold := trainFolds[i]
		testFold := testFolds[i]
		algorithm.Fit(trainFold, options)
		predictions := make([]float64, testFold.Length())
		interactionUsers, interactionItems, _ := testFold.Interactions()
		for j := 0; j < testFold.Length(); j++ {
			userId := interactionUsers[j]
			itemId := interactionItems[j]
			predictions[j] = algorithm.Predict(userId, itemId)
		}
		_, _, truth := testFold.Interactions()
		// Metrics
		for j := 0; j < len(ret); j++ {
			ret[j][i] = metrics[j](predictions, truth)
		}
	}
	return ret
}

// TODO: Tune algorithm parameters with GridSearchCV
//func GridSearchCV(algo Algorithm, paramGrid map[string][]interface{}, measures []Metrics, cv int) Algorithm {
//	// Retrieve parameter names
//	names := make([]string, len(paramGrid))
//	return NewBaseLine()
//}
