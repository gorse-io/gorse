package core

import (
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"math"
)

type ParameterGrid map[string][]interface{}

// The return data structure of cross validate
type CrossValidateResult struct {
	Trains []float64
	Tests  []float64
}

func CrossValidate(estimator Estimator, dataSet DataSet, metrics []Evaluator, cv int, seed int64,
	params Parameters) []CrossValidateResult {
	// Create return structures
	ret := make([]CrossValidateResult, len(metrics))
	for i := 0; i < len(ret); i++ {
		ret[i].Trains = make([]float64, cv)
		ret[i].Tests = make([]float64, cv)
	}
	// Split data set
	trainFolds, testFolds := dataSet.KFold(cv, seed)
	for i := 0; i < cv; i++ {
		trainFold := trainFolds[i]
		testFold := testFolds[i]
		estimator.Fit(trainFold, params)
		predictions := make([]float64, testFold.Length())
		interactionUsers, interactionItems := testFold.Users, testFold.Items
		for j := 0; j < testFold.Length(); j++ {
			userId := interactionUsers[j]
			itemId := interactionItems[j]
			predictions[j] = estimator.Predict(userId, itemId)
		}
		truth := testFold.Ratings
		// Evaluate on test set
		for j := 0; j < len(ret); j++ {
			ret[j].Tests[i] = metrics[j](predictions, truth)
		}
	}
	return ret
}

/* Model Selection */

// The return structure of grid search
type GridSearchResult struct {
	BestScore  float64
	BestParams Parameters
	BestIndex  int
	CVResults  []CrossValidateResult
	AllParams  []Parameters
}

// Tune algorithm parameters with GridSearchCV
func GridSearchCV(estimator Estimator, dataSet DataSet, paramGrid ParameterGrid,
	evaluators []Evaluator, cv int, seed int64) []GridSearchResult {
	// Retrieve parameter names and length
	params := make([]string, 0, len(paramGrid))
	count := 1
	for param, values := range paramGrid {
		params = append(params, param)
		count *= len(values)
	}
	// Create GridSearch result
	results := make([]GridSearchResult, len(evaluators))
	for i := range results {
		results[i] = GridSearchResult{}
		results[i].BestScore = math.Inf(1)
		results[i].CVResults = make([]CrossValidateResult, 0, count)
		results[i].AllParams = make([]Parameters, 0, count)
	}
	// Construct DFS procedure
	var dfs func(deep int, options Parameters)
	dfs = func(deep int, options Parameters) {
		if deep == len(params) {
			// Cross validate
			cvResults := CrossValidate(estimator, dataSet, evaluators, cv, seed, options)
			for i := range cvResults {
				results[i].CVResults = append(results[i].CVResults, cvResults[i])
				results[i].AllParams = append(results[i].AllParams, options.Copy())
				score := stat.Mean(cvResults[i].Tests, nil)
				if score < results[i].BestScore {
					results[i].BestScore = score
					results[i].BestParams = options.Copy()
					results[i].BestIndex = len(results[i].AllParams) - 1
				}
			}
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
	return results
}

/* Evaluator */

// Evaluator function
type Evaluator func([]float64, []float64) float64

// Root mean square error.
func RMSE(predictions []float64, truth []float64) float64 {
	temp := make([]float64, len(predictions))
	floats.SubTo(temp, predictions, truth)
	floats.Mul(temp, temp)
	return math.Sqrt(stat.Mean(temp, nil))
}

// Mean absolute error.
func MAE(predictions []float64, truth []float64) float64 {
	temp := make([]float64, len(predictions))
	floats.SubTo(temp, predictions, truth)
	abs(temp)
	return stat.Mean(temp, nil)
}
