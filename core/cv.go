package core

import (
	"fmt"
	"math"
)

type CrossValidateResult struct {
}

func CrossValidate(algorithm Algorithm, dataSet TrainSet, metrics []Metrics, cv int, seed int64,
	params Parameters) [][]float64 {
	ret := make([][]float64, len(metrics))
	for i := 0; i < len(ret); i++ {
		ret[i] = make([]float64, cv)
	}
	// Split data set
	trainFolds, testFolds := dataSet.KFold(cv, seed)
	for i := 0; i < cv; i++ {
		trainFold := trainFolds[i]
		testFold := testFolds[i]
		algorithm.Fit(trainFold, params)
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

type GridSearchResult struct {
	bestEstimator Algorithm
	bestScore     float64
	bestParams    map[string]interface{}
}

// TODO: Tune algorithm parameters with GridSearchCV
func GridSearchCV(algo Algorithm, dataSet TrainSet, paramGrid map[string][]interface{}, measures []Metrics, cv int, seed int64) GridSearchResult {
	// Retrieve parameter names
	params := make([]string, 0, len(paramGrid))
	for param := range paramGrid {
		params = append(params, param)
	}
	// Create GridSearch result
	result := GridSearchResult{}
	result.bestScore = math.Inf(1)
	// Construct DFS procedure
	var dfs func(deep int, options map[string]interface{})
	dfs = func(deep int, options map[string]interface{}) {
		if deep == len(params) {
			// Cross validate
			//CrossValidate(algo, dataSet, measures, cv, seed, newParameterReader(parameters))
			fmt.Println(options)
		} else {
			param := params[deep]
			values := paramGrid[param]
			for _, val := range values {
				options[param] = val
				dfs(deep+1, options)
			}
		}
	}
	options := make(map[string]interface{})
	dfs(0, options)
	return GridSearchResult{}
}
