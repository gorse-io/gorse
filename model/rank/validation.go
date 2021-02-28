// Copyright 2020 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package rank

import (
	log "github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
)

// ParamsSearchResult contains the return of grid search.
type ParamsSearchResult struct {
	BestScore  Score
	BestParams model.Params
	BestIndex  int
	Scores     []Score
	Params     []model.Params
}

func NewParamsSearchResult() *ParamsSearchResult {
	return &ParamsSearchResult{
		Scores: make([]Score, 0),
		Params: make([]model.Params, 0),
	}
}

func (r *ParamsSearchResult) AddScore(params model.Params, score Score) {
	r.Scores = append(r.Scores, score)
	r.Params = append(r.Params, params.Copy())
	if len(r.Scores) == 0 || score.BetterThan(r.BestScore) {
		r.BestScore = score
		r.BestParams = params.Copy()
		r.BestIndex = len(r.Params) - 1
	}
}

// GridSearchCV finds the best parameters for a model.
func GridSearchCV(estimator FactorizationMachine, trainSet *Dataset, testSet *Dataset, paramGrid model.ParamsGrid,
	seed int64, fitConfig *FitConfig) ParamsSearchResult {
	// Retrieve parameter names and length
	paramNames := make([]model.ParamName, 0, len(paramGrid))
	count := 1
	for paramName, values := range paramGrid {
		paramNames = append(paramNames, paramName)
		count *= len(values)
	}
	// Construct DFS procedure
	results := ParamsSearchResult{
		Scores: make([]Score, 0, count),
		Params: make([]model.Params, 0, count),
	}
	var dfs func(deep int, params model.Params)
	progress := 0
	dfs = func(deep int, params model.Params) {
		if deep == len(paramNames) {
			progress++
			log.Infof("grid search %v/%v: %v", progress, count, params)
			// Cross validate
			estimator.Clear()
			estimator.SetParams(estimator.GetParams().Overwrite(params))
			score := estimator.Fit(trainSet, testSet, fitConfig)
			// Create GridSearch result
			results.Scores = append(results.Scores, score)
			results.Params = append(results.Params, params.Copy())
			if len(results.Scores) == 0 || score.BetterThan(results.BestScore) {
				results.BestScore = score
				results.BestParams = params.Copy()
				results.BestIndex = len(results.Params) - 1
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
	params := make(map[model.ParamName]interface{})
	dfs(0, params)
	return results
}

// RandomSearchCV searches hyper-parameters by random.
func RandomSearchCV(estimator FactorizationMachine, trainSet *Dataset, testSet *Dataset, paramGrid model.ParamsGrid,
	numTrials int, seed int64, fitConfig *FitConfig) ParamsSearchResult {
	rng := base.NewRandomGenerator(seed)
	results := ParamsSearchResult{
		Scores: make([]Score, 0, numTrials),
		Params: make([]model.Params, 0, numTrials),
	}
	for i := 1; i <= numTrials; i++ {
		// Make parameters
		params := model.Params{}
		for paramName, values := range paramGrid {
			value := values[rng.Intn(len(values))]
			params[paramName] = value
		}
		// Cross validate
		log.Infof("random search (%v/%v): %v", i, numTrials, params)
		estimator.Clear()
		estimator.SetParams(estimator.GetParams().Overwrite(params))
		score := estimator.Fit(trainSet, testSet, fitConfig)
		results.Scores = append(results.Scores, score)
		results.Params = append(results.Params, params.Copy())
		if len(results.Scores) == 0 || score.BetterThan(results.BestScore) {
			results.BestScore = score
			results.BestParams = params.Copy()
			results.BestIndex = len(results.Params) - 1
		}
	}
	return results
}
