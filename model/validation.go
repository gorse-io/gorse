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

// GridSearchResult contains the return of grid search.
type GridSearchResult struct {
	BestScore  Score
	BestParams Params
	BestIndex  int
	Scores     []Score
	Params     []Params
}

// RandomSearchCV searches hyper-parameters by random.
func RandomSearchCV(estimator Model, trainSet *DataSet, testSet *DataSet, paramGrid ParameterGrid,
	numTrials int, seed int64) GridSearchResult {
	rng := base.NewRandomGenerator(seed)
	results := GridSearchResult{
		Scores: make([]Score, numTrials),
		Params: make([]Params, numTrials),
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
		estimator.SetParams(estimator.GetParams().Merge(params))
		score := estimator.Fit(trainSet, testSet, nil)
		results.Scores[i] = score
		results.Params[i] = params.Copy()
		if score.NDCG > results.BestScore.NDCG {
			results.BestScore = score
			results.BestParams = params.Copy()
			results.BestIndex = len(results.Params) - 1
		}
	}
	return results
}
