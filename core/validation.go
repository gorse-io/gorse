package core

import (
	. "github.com/zhenghaoz/gorse/base"
	"gonum.org/v1/gonum/stat"
	"math"
	"reflect"
)

// ParameterGrid contains candidate for grid search.
type ParameterGrid map[ParamName][]interface{}

/* Cross Validation */

// CrossValidateResult contains the result of cross validate
type CrossValidateResult struct {
	Tests []float64
}

// CrossValidation evaluates a model by k-fold cross validation.
func CrossValidate(estimator Model, dataSet DataSet, metrics []Evaluator,
	splitter Splitter, options ...CVOption) []CrossValidateResult {
	cvOptions := NewCVOptions(options)
	// Split data set
	trainFolds, testFolds := splitter(dataSet, cvOptions.Seed)
	length := len(trainFolds)
	// Create return structures
	ret := make([]CrossValidateResult, len(metrics))
	for i := 0; i < len(ret); i++ {
		ret[i].Tests = make([]float64, length)
	}
	// Cross validation
	params := estimator.GetParams()
	Parallel(length, cvOptions.NJobs, func(begin, end int) {
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

// ModelSelectionResult contains the return of grid search.
type ModelSelectionResult struct {
	BestScore  float64
	BestParams Params
	BestIndex  int
	CVResults  []CrossValidateResult
	AllParams  []Params
}

// GridSearchCV finds the best parameters for a model.
func GridSearchCV(estimator Model, dataSet DataSet, paramGrid ParameterGrid,
	evaluators []Evaluator, options ...CVOption) []ModelSelectionResult {
	// Retrieve parameter names and length
	paramNames := make([]ParamName, 0, len(paramGrid))
	count := 1
	for paramName, values := range paramGrid {
		paramNames = append(paramNames, paramName)
		count *= len(values)
	}
	// Create GridSearch result
	results := make([]ModelSelectionResult, len(evaluators))
	for i := range results {
		results[i] = ModelSelectionResult{}
		results[i].BestScore = math.Inf(1)
		results[i].CVResults = make([]CrossValidateResult, 0, count)
		results[i].AllParams = make([]Params, 0, count)
	}
	// Construct DFS procedure
	var dfs func(deep int, params Params)
	dfs = func(deep int, params Params) {
		if deep == len(paramNames) {
			// Cross validate
			cvResults := CrossValidate(estimator, dataSet, evaluators, NewKFoldSplitter(5), options...)
			for i := range cvResults {
				results[i].CVResults = append(results[i].CVResults, cvResults[i])
				results[i].AllParams = append(results[i].AllParams, params.Copy())
				score := stat.Mean(cvResults[i].Tests, nil)
				if score < results[i].BestScore {
					results[i].BestScore = score
					results[i].BestParams = params.Copy()
					results[i].BestIndex = len(results[i].AllParams) - 1
				}
			}
		} else {
			paramName := paramNames[deep]
			values := paramGrid[paramName]
			for _, val := range values {
				params[paramName] = val
				dfs(deep+1, params)
			}
		}
	}
	params := make(map[ParamName]interface{})
	dfs(0, params)
	return results
}

func RandomSearchCV(estimator Model, dataSet DataSet, paramGrid ParameterGrid,
	evaluators []Evaluator, trial int, options ...CVOption) []ModelSelectionResult {
	cvOptions := NewCVOptions(options)
	rng := NewRandomGenerator(cvOptions.Seed)
	//
	for i := 0; i < trial; i++ {
		params := Params{}
		for paramName, values := range paramGrid {
			value := values[rng.Intn(len(values))]
			params[paramName] = value
		}
	}
}
