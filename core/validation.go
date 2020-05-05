package core

import (
	"bytes"
	"encoding/gob"
	"github.com/zhenghaoz/gorse/base"
	"gonum.org/v1/gonum/stat"
	"math"
	"reflect"
)

// ParameterGrid contains candidate for grid search.
type ParameterGrid map[base.ParamName][]interface{}

/* Cross Validation */

// CrossValidateResult contains the result of cross validate
type CrossValidateResult struct {
	TestScore []float64
}

// MeanAndMargin returns the mean and the margin of cross validation scores.
func (sv CrossValidateResult) MeanAndMargin() (float64, float64) {
	mean := stat.Mean(sv.TestScore, nil)
	margin := 0.0
	for _, score := range sv.TestScore {
		temp := math.Abs(score - mean)
		if temp > margin {
			margin = temp
		}
	}
	return mean, margin
}

// CrossValidate evaluates a model by k-fold cross validation.
func CrossValidate(model ModelInterface, dataSet DataSetInterface, splitter Splitter, seed int64,
	options *base.RuntimeOptions, evaluators ...Evaluator) []CrossValidateResult {
	// Split data set
	trainFolds, testFolds := splitter(dataSet, seed)
	length := len(trainFolds)
	// Cross validation
	scores := make([][]float64, length)
	params := model.GetParams()
	base.Parallel(length, options.GetCVJobs(), func(begin, end int) {
		cp := reflect.New(reflect.TypeOf(model).Elem()).Interface().(ModelInterface)
		Copy(cp, model)
		cp.SetParams(params)
		for i := begin; i < end; i++ {
			trainFold := trainFolds[i]
			testFold := testFolds[i]
			cp.Fit(trainFold, options)
			// Evaluate on test set
			for _, evaluator := range evaluators {
				tempScore := evaluator(cp, testFold, trainFold)
				scores[i] = append(scores[i], tempScore...)
			}
		}
	})
	// Create return structures
	ret := make([]CrossValidateResult, len(scores[0]))
	for i := 0; i < len(ret); i++ {
		ret[i].TestScore = make([]float64, length)
		for j := range ret[i].TestScore {
			ret[i].TestScore[j] = scores[j][i]
		}
	}
	return ret
}

/* Model Selection */

// ModelSelectionResult contains the return of grid search.
type ModelSelectionResult struct {
	BestScore  float64
	BestParams base.Params
	BestIndex  int
	CVResults  []CrossValidateResult
	AllParams  []base.Params
}

// GridSearchCV finds the best parameters for a model.
func GridSearchCV(estimator ModelInterface, dataSet DataSetInterface, paramGrid ParameterGrid,
	splitter Splitter, seed int64, options *base.RuntimeOptions, evaluators ...Evaluator) []ModelSelectionResult {
	// Retrieve parameter names and length
	paramNames := make([]base.ParamName, 0, len(paramGrid))
	count := 1
	for paramName, values := range paramGrid {
		paramNames = append(paramNames, paramName)
		count *= len(values)
	}
	// Construct DFS procedure
	var results []ModelSelectionResult
	var dfs func(deep int, params base.Params)
	progress := 0
	dfs = func(deep int, params base.Params) {
		if deep == len(paramNames) {
			progress++
			options.Logf("grid search (%v/%v): %v", progress, count, params)
			// Cross validate
			estimator.SetParams(estimator.GetParams().Merge(params))
			cvResults := CrossValidate(estimator, dataSet, splitter, seed, options, evaluators...)
			// Create GridSearch result
			if results == nil {
				results = make([]ModelSelectionResult, len(cvResults))
				for i := range results {
					results[i] = ModelSelectionResult{}
					results[i].BestScore = 0
					results[i].CVResults = make([]CrossValidateResult, 0, count)
					results[i].AllParams = make([]base.Params, 0, count)
				}
			}
			for i := range cvResults {
				results[i].CVResults = append(results[i].CVResults, cvResults[i])
				results[i].AllParams = append(results[i].AllParams, params.Copy())
				score := stat.Mean(cvResults[i].TestScore, nil)
				if score > results[i].BestScore {
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
	params := make(map[base.ParamName]interface{})
	dfs(0, params)
	return results
}

// RandomSearchCV searches hyper-parameters by random.
func RandomSearchCV(estimator ModelInterface, dataSet DataSetInterface, paramGrid ParameterGrid,
	splitter Splitter, trial int, seed int64, options *base.RuntimeOptions, evaluators ...Evaluator) []ModelSelectionResult {
	rng := base.NewRandomGenerator(seed)
	var results []ModelSelectionResult
	for i := 0; i < trial; i++ {
		// Make parameters
		params := base.Params{}
		for paramName, values := range paramGrid {
			value := values[rng.Intn(len(values))]
			params[paramName] = value
		}
		// Cross validate
		options.Logf("random search (%v/%v): %v", i+1, trial, params)
		estimator.SetParams(estimator.GetParams().Merge(params))
		cvResults := CrossValidate(estimator, dataSet, splitter, seed, options, evaluators...)
		if results == nil {
			results = make([]ModelSelectionResult, len(cvResults))
			for i := range results {
				results[i] = ModelSelectionResult{}
				results[i].BestScore = 0
				results[i].CVResults = make([]CrossValidateResult, trial)
				results[i].AllParams = make([]base.Params, trial)
			}
		}
		for j := range cvResults {
			results[j].CVResults[i] = cvResults[j]
			results[j].AllParams[i] = params.Copy()
			score := stat.Mean(cvResults[j].TestScore, nil)
			if score > results[j].BestScore {
				results[j].BestScore = score
				results[j].BestParams = params.Copy()
				results[j].BestIndex = len(results[j].AllParams) - 1
			}
		}
	}
	return results
}

// Copy a object from src to dst.
func Copy(dst, src interface{}) error {
	buffer := new(bytes.Buffer)
	encoder := gob.NewEncoder(buffer)
	if err := encoder.Encode(src); err != nil {
		return err
	}
	decoder := gob.NewDecoder(buffer)
	err := decoder.Decode(dst)
	return err
}
