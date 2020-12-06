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
package model

import (
	log "github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/base"
)

// ParamsSearchResult contains the return of grid search.
type ParamsSearchResult struct {
	BestScore  Score
	BestParams Params
	BestIndex  int
	Scores     []Score
	Params     []Params
}

func NewParamsSearchResult() *ParamsSearchResult {
	return &ParamsSearchResult{
		Scores: make([]Score, 0),
		Params: make([]Params, 0),
	}
}

func (r *ParamsSearchResult) AddScore(params Params, score Score) {
	r.Scores = append(r.Scores, score)
	r.Params = append(r.Params, params.Copy())
	if len(r.Scores) == 0 || score.NDCG > r.BestScore.NDCG {
		r.BestScore = score
		r.BestParams = params.Copy()
		r.BestIndex = len(r.Params) - 1
	}
}

// GridSearchCV finds the best parameters for a model.
func GridSearchCV(estimator Model, trainSet *DataSet, testSet *DataSet, paramGrid ParamsGrid, seed int64) ParamsSearchResult {
	// Retrieve parameter names and length
	paramNames := make([]ParamName, 0, len(paramGrid))
	count := 1
	for paramName, values := range paramGrid {
		paramNames = append(paramNames, paramName)
		count *= len(values)
	}
	// Construct DFS procedure
	results := ParamsSearchResult{
		Scores: make([]Score, 0, count),
		Params: make([]Params, 0, count),
	}
	var dfs func(deep int, params Params)
	progress := 0
	dfs = func(deep int, params Params) {
		if deep == len(paramNames) {
			progress++
			log.Infof("grid search %v/%v: %v", progress, count, params)
			// Cross validate
			estimator.Clear()
			estimator.SetParams(estimator.GetParams().Merge(params))
			score := estimator.Fit(trainSet, testSet, nil)
			// Create GridSearch result
			results.Scores = append(results.Scores, score)
			results.Params = append(results.Params, params.Copy())
			if len(results.Scores) == 0 || score.NDCG > results.BestScore.NDCG {
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
	params := make(map[ParamName]interface{})
	dfs(0, params)
	return results
}

// RandomSearchCV searches hyper-parameters by random.
func RandomSearchCV(estimator Model, trainSet *DataSet, testSet *DataSet, paramGrid ParamsGrid,
	numTrials int, seed int64) ParamsSearchResult {
	rng := base.NewRandomGenerator(seed)
	results := ParamsSearchResult{
		Scores: make([]Score, 0, numTrials),
		Params: make([]Params, 0, numTrials),
	}
	for i := 1; i <= numTrials; i++ {
		// Make parameters
		params := Params{}
		for paramName, values := range paramGrid {
			value := values[rng.Intn(len(values))]
			params[paramName] = value
		}
		// Cross validate
		log.Infof("random search (%v/%v): %v", i, numTrials, params)
		estimator.Clear()
		estimator.SetParams(estimator.GetParams().Merge(params))
		score := estimator.Fit(trainSet, testSet, nil)
		results.Scores = append(results.Scores, score)
		results.Params = append(results.Params, params.Copy())
		if len(results.Scores) == 0 || score.NDCG > results.BestScore.NDCG {
			results.BestScore = score
			results.BestParams = params.Copy()
			results.BestIndex = len(results.Params) - 1
		}
	}
	return results
}
