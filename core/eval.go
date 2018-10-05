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
	Trains []float64
	Tests  []float64
}

// CrossValidation evaluates a model by k-fold cross validation.
func CrossValidate(estimator Estimator, dataSet DataSet, metrics []Evaluator, splitter Splitter, seed int64,
	params Parameters, nJobs int) []CrossValidateResult {
	// Split data set
	trainFolds, testFolds := splitter(dataSet, seed)
	length := len(trainFolds)
	// Create return structures
	ret := make([]CrossValidateResult, len(metrics))
	for i := 0; i < len(ret); i++ {
		ret[i].Trains = make([]float64, length)
		ret[i].Tests = make([]float64, length)
	}
	// Cross validation
	parallel(length, nJobs, func(begin, end int) {
		cp := reflect.New(reflect.TypeOf(estimator).Elem()).Interface().(Estimator)
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
			// Evaluate on train set
			for j := 0; j < len(ret); j++ {
				ret[j].Trains[i] = metrics[j](cp, trainFold.DataSet)
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
func GridSearchCV(estimator Estimator, dataSet DataSet, paramGrid ParameterGrid,
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

/* Evaluator */

// Evaluator function type.
type Evaluator func(Estimator, DataSet) float64

// RMSE is root mean square error.
func RMSE(estimator Estimator, testSet DataSet) float64 {
	sum := 0.0
	for j := 0; j < testSet.Length(); j++ {
		userId, itemId, rating := testSet.Index(j)
		prediction := estimator.Predict(userId, itemId)
		sum += (prediction - rating) * (prediction - rating)
	}
	return math.Sqrt(sum / float64(testSet.Length()))
}

// MAE is mean absolute error.
func MAE(estimator Estimator, testSet DataSet) float64 {
	sum := 0.0
	for j := 0; j < testSet.Length(); j++ {
		userId, itemId, rating := testSet.Index(j)
		prediction := estimator.Predict(userId, itemId)
		sum += math.Abs(prediction - rating)
	}
	return sum / float64(testSet.Length())
}

// NewAUC creates a AUC evaluator.
func NewAUC(fullSet DataSet) Evaluator {
	return func(estimator Estimator, testSet DataSet) float64 {
		full := NewTrainSet(fullSet)
		test := NewTrainSet(testSet)
		sum, count := 0.0, 0.0
		// Find all userIds
		for innerUserIdTest, irs := range test.UserRatings() {
			userId := test.outerUserIds[innerUserIdTest]
			// Find all <userId, j>s in full data set
			innerUserIdFull := full.ConvertUserId(userId)
			fullRatedItem := make(map[int]float64)
			for _, jr := range full.UserRatings()[innerUserIdFull] {
				itemId := full.outerItemIds[jr.Id]
				fullRatedItem[itemId] = jr.Rating
			}
			// Find all <userId, i>s in test data set
			indicatorSum, ratedCount := 0.0, 0.0
			for _, ir := range irs {
				iItemId := test.outerItemIds[ir.Id]
				// Find all <userId, j>s not in full data set
				for j := 0; j < full.ItemCount; j++ {
					jItemId := full.outerItemIds[j]
					if _, exist := fullRatedItem[jItemId]; !exist {
						// I(\hat{x}_{ui} - \hat{x}_{uj})
						if estimator.Predict(userId, iItemId) > estimator.Predict(userId, jItemId) {
							indicatorSum++
						}
						ratedCount++
					}
				}
			}
			// += \frac{1}{|E(u)|} \sum_{(i,j)\in{E(u)}} I(\hat{x}_{ui} - \hat{x}_{uj})
			sum += indicatorSum / ratedCount
			count++
		}
		// \frac{1}{|U|} \sum_u \frac{1}{|E(u)|} \sum_{(i,j)\in{E(u)}} I(\hat{x}_{ui} - \hat{x}_{uj})
		return sum / count
	}
}
