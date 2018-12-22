package core

import (
	"fmt"
	. "github.com/zhenghaoz/gorse/base"
	"gonum.org/v1/gonum/stat"
	"gopkg.in/cheggaaa/pb.v1"
	"math"
	"reflect"
)

// ParameterGrid contains candidate for grid search.
type ParameterGrid map[ParamName][]interface{}

/* Cross Validation */

// CrossValidateResult contains the result of cross validate
type CrossValidateResult struct {
	TestScore []float64
	TestTime  []float64
	FitTime   []float64
}

func (sv CrossValidateResult) MeanMarginScore() (float64, float64) {
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

// CrossValidation evaluates a model by k-fold cross validation.
func CrossValidate(estimator Model, dataSet Table, metrics []Evaluator,
	splitter Splitter, options ...CVOption) []CrossValidateResult {
	cvOptions := NewCVOptions(options)
	// Split data set
	trainFolds, testFolds := splitter(dataSet, cvOptions.Seed)
	length := len(trainFolds)
	// Create return structures
	ret := make([]CrossValidateResult, len(metrics))
	for i := 0; i < len(ret); i++ {
		ret[i].TestScore = make([]float64, length)
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
				ret[j].TestScore[i] = metrics[j](cp, trainFold, NewDataSet(testFold))
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

func (cv ModelSelectionResult) Summary() {
	fmt.Printf("The best score is: %.5f\n", cv.BestScore)
	fmt.Printf("The best params is: %v\n", cv.BestParams)
}

// GridSearchCV finds the best parameters for a model.
func GridSearchCV(estimator Model, dataSet Table,
	evaluators []Evaluator, splitter Splitter, paramGrid ParameterGrid, options ...CVOption) []ModelSelectionResult {
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
	// Progress bar
	bar := pb.StartNew(count)
	// Construct DFS procedure
	var dfs func(deep int, params Params)
	dfs = func(deep int, params Params) {
		if deep == len(paramNames) {
			// Cross validate
			estimator.SetParams(params)
			cvResults := CrossValidate(estimator, dataSet, evaluators, splitter, options...)
			for i := range cvResults {
				results[i].CVResults = append(results[i].CVResults, cvResults[i])
				results[i].AllParams = append(results[i].AllParams, params.Copy())
				score := stat.Mean(cvResults[i].TestScore, nil)
				if score < results[i].BestScore {
					results[i].BestScore = score
					results[i].BestParams = params.Copy()
					results[i].BestIndex = len(results[i].AllParams) - 1
				}
				// Progress bar
				bar.Increment()
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
	bar.FinishPrint("Completed!")
	return results
}

func RandomSearchCV(estimator Model, dataSet Table, paramGrid ParameterGrid,
	evaluators []Evaluator, trial int, options ...CVOption) []ModelSelectionResult {
	cvOptions := NewCVOptions(options)
	rng := NewRandomGenerator(cvOptions.Seed)
	// Create results
	results := make([]ModelSelectionResult, len(evaluators))
	for i := range results {
		results[i] = ModelSelectionResult{}
		results[i].BestScore = math.Inf(1)
		results[i].CVResults = make([]CrossValidateResult, 0, trial)
		results[i].AllParams = make([]Params, 0, trial)
	}
	//
	for i := 0; i < trial; i++ {
		// Make parameters
		params := Params{}
		for paramName, values := range paramGrid {
			value := values[rng.Intn(len(values))]
			params[paramName] = value
		}
		// Cross validate
		cvResults := CrossValidate(estimator, dataSet, evaluators, NewKFoldSplitter(5), options...)
		for i := range cvResults {
			results[i].CVResults = append(results[i].CVResults, cvResults[i])
			results[i].AllParams = append(results[i].AllParams, params.Copy())
			score := stat.Mean(cvResults[i].TestScore, nil)
			if score < results[i].BestScore {
				results[i].BestScore = score
				results[i].BestParams = params.Copy()
				results[i].BestIndex = len(results[i].AllParams) - 1
			}
		}
	}
	return results
}
