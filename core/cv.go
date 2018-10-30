package core

import (
	"gonum.org/v1/gonum/stat"
	"math"
	"reflect"
)

// ParameterGrid contains candidate for grid search.
type ParameterGrid map[string][]interface{}

/* Cross Validation */

// CrossValidateResult contains the result of cross validate
type CrossValidateResult struct {
	Tests []float64
}

// CrossValidation evaluates a model by k-fold cross validation.
func CrossValidate(estimator Model, dataSet RawDataSet, metrics []Evaluator, splitter Splitter, seed int64,
	params Parameters, nJobs int) []CrossValidateResult {
	// Split data set
	trainFolds, testFolds := splitter(dataSet, seed)
	length := len(trainFolds)
	// Create return structures
	ret := make([]CrossValidateResult, len(metrics))
	for i := 0; i < len(ret); i++ {
		ret[i].Tests = make([]float64, length)
	}
	// Cross validation
	parallel(length, nJobs, func(begin, end int) {
		cp := reflect.New(reflect.TypeOf(estimator).Elem()).Interface().(Model)
		Copy(cp, estimator)
		for i := begin; i < end; i++ {
			trainFold := trainFolds[i]
			testFold := testFolds[i]
			cp.SetParams(params)
			cp.Fit(trainFold)
			// Evaluate on test set
			for j := 0; j < len(ret); j++ {
				ret[j].Tests[i] = metrics[j](cp, testFold)
			}
		}
	})
	return ret
}

/* Model Selection */

// GridSearchResult contains the return of grid search.
type GridSearchResult struct {
	BestScore  float64
	BestParams Parameters
	BestIndex  int
	CVResults  []CrossValidateResult
	AllParams  []Parameters
}

// GridSearchCV finds the best parameters for a model.
func GridSearchCV(estimator Model, dataSet RawDataSet, paramGrid ParameterGrid,
	evaluators []Evaluator, cv int, seed int64, nJobs int) []GridSearchResult {
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
			cvResults := CrossValidate(estimator, dataSet, evaluators, NewKFoldSplitter(5), seed, options, nJobs)
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
