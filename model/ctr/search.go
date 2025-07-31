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

package ctr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gorse-io/gorse/dataset"

	"github.com/gorse-io/gorse/base"
	"github.com/gorse-io/gorse/base/log"
	"github.com/gorse-io/gorse/base/progress"
	"github.com/gorse-io/gorse/base/task"
	"github.com/gorse-io/gorse/model"
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
func GridSearchCV(ctx context.Context, estimator FactorizationMachine, trainSet, testSet dataset.CTRSplit, paramGrid model.ParamsGrid,
	_ int64, fitConfig *FitConfig) ParamsSearchResult {
	// Retrieve parameter names and length
	paramNames := make([]model.ParamName, 0, len(paramGrid))
	total := 1
	for paramName, values := range paramGrid {
		paramNames = append(paramNames, paramName)
		total *= len(values)
	}
	// Construct DFS procedure
	results := ParamsSearchResult{
		Scores: make([]Score, 0, total),
		Params: make([]model.Params, 0, total),
	}
	var dfs func(deep int, params model.Params)
	newCtx, span := progress.Start(ctx, "GridSearchCV", total)
	dfs = func(deep int, params model.Params) {
		if deep == len(paramNames) {
			log.Logger().Info(fmt.Sprintf("grid search %v/%v", span.Count(), total),
				zap.Any("params", params))
			// Cross validate
			estimator.Clear()
			estimator.SetParams(estimator.GetParams().Overwrite(params))
			score := estimator.Fit(newCtx, trainSet, testSet, fitConfig)
			// Create GridSearch result
			results.Scores = append(results.Scores, score)
			results.Params = append(results.Params, params.Copy())
			if len(results.Scores) == 0 || score.BetterThan(results.BestScore) {
				results.BestScore = score
				results.BestParams = params.Copy()
				results.BestIndex = len(results.Params) - 1
				results.BestModel = Clone(estimator)
			}
			span.Add(1)
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
	span.End()
	return results
}

// RandomSearchCV searches hyper-parameters by random.
func RandomSearchCV(ctx context.Context, estimator FactorizationMachine, trainSet, testSet dataset.CTRSplit, paramGrid model.ParamsGrid,
	numTrials int, seed int64, fitConfig *FitConfig) ParamsSearchResult {
	// if the number of combination is less than number of trials, use grid search
	if paramGrid.NumCombinations() <= numTrials {
		return GridSearchCV(ctx, estimator, trainSet, testSet, paramGrid, seed, fitConfig)
	}
	rng := base.NewRandomGenerator(seed)
	results := ParamsSearchResult{
		Scores: make([]Score, 0, numTrials),
		Params: make([]model.Params, 0, numTrials),
	}
	newCtx, span := progress.Start(ctx, "RandomSearchCV", numTrials)
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
		score := estimator.Fit(newCtx, trainSet, testSet, fitConfig)
		results.Scores = append(results.Scores, score)
		results.Params = append(results.Params, params.Copy())
		if len(results.Scores) == 0 || score.BetterThan(results.BestScore) {
			results.BestScore = score
			results.BestParams = params.Copy()
			results.BestIndex = len(results.Params) - 1
			results.BestModel = Clone(estimator)
		}
		span.Add(1)
	}
	span.End()
	return results
}

// ModelSearcher is a thread-safe click model searcher.
type ModelSearcher struct {
	model FactorizationMachine
	// arguments
	numEpochs  int
	numTrials  int
	searchSize bool
	// results
	bestMutex sync.Mutex
	bestModel FactorizationMachine
	bestScore Score
}

// NewModelSearcher creates a thread-safe personal ranking model searcher.
func NewModelSearcher(nEpoch, nTrials int, searchSize bool) *ModelSearcher {
	return &ModelSearcher{
		model:      NewFM(model.Params{model.NEpochs: nEpoch}),
		numTrials:  nTrials,
		numEpochs:  nEpoch,
		searchSize: searchSize,
	}
}

// GetBestModel returns the best click model with its score.
func (searcher *ModelSearcher) GetBestModel() (FactorizationMachine, Score) {
	searcher.bestMutex.Lock()
	defer searcher.bestMutex.Unlock()
	return searcher.bestModel, searcher.bestScore
}

func (searcher *ModelSearcher) Fit(ctx context.Context, trainSet, valSet dataset.CTRSplit, j *task.JobsAllocator) error {
	log.Logger().Info("click model search",
		zap.Int("n_users", trainSet.CountUsers()),
		zap.Int("n_items", trainSet.CountItems()),
		zap.Int("n_user_labels", trainSet.CountUserLabels()),
		zap.Int("n_item_labels", trainSet.CountItemLabels()))
	startTime := time.Now()

	// Random search
	grid := searcher.model.GetParamsGrid(searcher.searchSize)
	r := RandomSearchCV(ctx, searcher.model, trainSet, valSet, grid, searcher.numTrials, 0, NewFitConfig())
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
