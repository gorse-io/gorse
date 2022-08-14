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

package click

import (
	"fmt"
	"sync"
	"time"

	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/task"
	"github.com/zhenghaoz/gorse/model"
	"go.uber.org/zap"
)

// ParamsSearchResult contains the return of grid search.
type ParamsSearchResult struct {
	BestScore  Score
	BestModel  FactorizationMachine
	BestParams model.Params
	BestIndex  int
	Scores     []Score
	Params     []model.Params
}

// GridSearchCV finds the best parameters for a model.
func GridSearchCV(estimator FactorizationMachine, trainSet *Dataset, testSet *Dataset, paramGrid model.ParamsGrid,
	_ int64, fitConfig *FitConfig) ParamsSearchResult {
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
			log.Logger().Info(fmt.Sprintf("grid search %v/%v", progress, count),
				zap.Any("params", params))
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
				results.BestModel = Clone(estimator)
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
	// if the number of combination is less than number of trials, use grid search
	if paramGrid.NumCombinations() <= numTrials {
		return GridSearchCV(estimator, trainSet, testSet, paramGrid, seed, fitConfig)
	}
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
		log.Logger().Info(fmt.Sprintf("random search %v/%v", i, numTrials),
			zap.Any("params", params))
		estimator.Clear()
		estimator.SetParams(estimator.GetParams().Overwrite(params))
		score := estimator.Fit(trainSet, testSet, fitConfig)
		results.Scores = append(results.Scores, score)
		results.Params = append(results.Params, params.Copy())
		if len(results.Scores) == 0 || score.BetterThan(results.BestScore) {
			results.BestScore = score
			results.BestParams = params.Copy()
			results.BestIndex = len(results.Params) - 1
			results.BestModel = Clone(estimator)
		}
	}
	return results
}

// ModelSearcher is a thread-safe click model searcher.
type ModelSearcher struct {
	model FactorizationMachine
	// arguments
	numEpochs  int
	numTrials  int
	jobsAlloc  *task.JobsAllocator
	searchSize bool
	// results
	bestMutex sync.Mutex
	bestModel FactorizationMachine
	bestScore Score
}

// NewModelSearcher creates a thread-safe personal ranking model searcher.
func NewModelSearcher(nEpoch, nTrials int, jobsAlloc *task.JobsAllocator, searchSize bool) *ModelSearcher {
	return &ModelSearcher{
		model:      NewFM(FMClassification, model.Params{model.NEpochs: nEpoch}),
		numTrials:  nTrials,
		numEpochs:  nEpoch,
		jobsAlloc:  jobsAlloc,
		searchSize: searchSize,
	}
}

// GetBestModel returns the best click model with its score.
func (searcher *ModelSearcher) GetBestModel() (FactorizationMachine, Score) {
	searcher.bestMutex.Lock()
	defer searcher.bestMutex.Unlock()
	return searcher.bestModel, searcher.bestScore
}

func (searcher *ModelSearcher) Complexity() int {
	return searcher.numTrials * searcher.numEpochs
}

func (searcher *ModelSearcher) Fit(trainSet, valSet *Dataset, t *task.Task) error {
	log.Logger().Info("click model search",
		zap.Int("n_users", trainSet.UserCount()),
		zap.Int("n_items", trainSet.ItemCount()),
		zap.Int32("n_user_labels", trainSet.Index.CountUserLabels()),
		zap.Int32("n_item_labels", trainSet.Index.CountItemLabels()))
	startTime := time.Now()

	// Random search
	grid := searcher.model.GetParamsGrid(searcher.searchSize)
	r := RandomSearchCV(searcher.model, trainSet, valSet, grid, searcher.numTrials, 0, NewFitConfig().
		SetJobsAllocator(searcher.jobsAlloc).
		SetTask(t))
	searcher.bestMutex.Lock()
	defer searcher.bestMutex.Unlock()
	searcher.bestModel = r.BestModel
	searcher.bestScore = r.BestScore

	searchTime := time.Since(startTime)
	log.Logger().Info("complete ranking model search",
		zap.Float32("auc", searcher.bestScore.AUC),
		zap.Any("params", searcher.bestModel.GetParams()),
		zap.String("search_time", searchTime.String()))
	return nil
}
