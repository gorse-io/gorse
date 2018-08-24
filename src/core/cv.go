package core

func CrossValidate(recommend Algorithm, dataSet Set, measures []string, cv int, seed int64,
	options ...OptionSetter) [][]float64 {
	a := make([][]float64, 2)
	a[0] = make([]float64, 0, cv)
	a[1] = make([]float64, 0, cv)
	// Split data set
	trainFolds, testFolds := dataSet.KFold(cv, seed)
	for i := 0; i < cv; i++ {
		trainFold := trainFolds[i]
		testFold := testFolds[i]
		recommend.Fit(NewDataSet(trainFold), options...)
		result := make([]float64, testFold.Nrow())
		for j := 0; j < testFold.Nrow(); j++ {
			userId, _ := testFold.Elem(j, 0).Int()
			itemId, _ := testFold.Elem(j, 1).Int()
			result[j] = recommend.Predict(userId, itemId)
		}
		tr := testFold.Col("X2").Float()
		a[0] = append(a[0], RootMeanSquareError(result, tr))
		a[1] = append(a[1], MeanAbsoluteError(result, tr))
	}
	return a
}
