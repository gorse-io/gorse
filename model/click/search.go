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
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"go.uber.org/zap"
	"sync"
	"time"
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
			base.Logger().Info(fmt.Sprintf("grid search %v/%v", progress, count),
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
	if paramGrid.NumCombinations() < numTrials {
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
		base.Logger().Info(fmt.Sprintf("random search %v/%v", i, numTrials),
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
	// arguments
	numEpochs int
	numTrials int
	numJobs   int
	// results
	bestMutex     sync.Mutex
	useClickModel bool
	bestModel     FactorizationMachine
	bestScore     Score
}

// NewModelSearcher creates a thread-safe personal ranking model searcher.
func NewModelSearcher(nEpoch, nTrials, nJobs int) *ModelSearcher {
	return &ModelSearcher{
		numTrials: nTrials,
		numEpochs: nEpoch,
		numJobs:   nJobs,
	}
}

// GetBestModel returns the best click model with its score.
func (searcher *ModelSearcher) GetBestModel() (FactorizationMachine, Score) {
	searcher.bestMutex.Lock()
	defer searcher.bestMutex.Unlock()
	return searcher.bestModel, searcher.bestScore
}

// IsClickModelHelpful check if click model helps to improve recommendation. Click model helps if:
// 1. There are item labels or user labels.
// 2. Labels improve test precision.
func (searcher *ModelSearcher) IsClickModelHelpful() bool {
	searcher.bestMutex.Lock()
	defer searcher.bestMutex.Unlock()
	return searcher.useClickModel
}

func (searcher *ModelSearcher) Fit(trainSet, valSet *Dataset) error {
	base.Logger().Info("click model search",
		zap.Int("n_users", trainSet.UserCount()),
		zap.Int("n_items", trainSet.ItemCount()),
		zap.Int("n_user_labels", trainSet.Index.CountUserLabels()),
		zap.Int("n_item_labels", trainSet.Index.CountItemLabels()))
	startTime := time.Now()

	// Check number of labels
	if trainSet.Index.CountItemLabels() == 0 && trainSet.Index.CountUserLabels() == 0 {
		base.Logger().Warn("click model doesn't work if there are no labels for items or users")
		return nil
	}

	// Random search
	fm := NewFM(FMClassification, nil)
	grid := fm.GetParamsGrid()
	grid[model.UseFeature] = []interface{}{true, false}
	r := RandomSearchCV(fm, trainSet, valSet, grid, searcher.numTrials*2, 0,
		NewFitConfig().SetJobs(searcher.numJobs))
	if !r.BestParams[model.UseFeature].(bool) {
		// If model searcher found it's better to ignore features, just don't use features.
		searcher.useClickModel = false
		base.Logger().Info("it seems worse to use features")
		return nil
	}
	searcher.bestMutex.Lock()
	defer searcher.bestMutex.Unlock()
	searcher.useClickModel = true
	searcher.bestModel = r.BestModel
	searcher.bestScore = r.BestScore

	searchTime := time.Since(startTime)
	base.Logger().Info("complete ranking model search",
		zap.Float32("precision", searcher.bestScore.Precision),
		zap.Any("params", searcher.bestModel.GetParams()),
		zap.String("search_time", searchTime.String()))
	return nil
}
