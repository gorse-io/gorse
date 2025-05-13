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

package cf

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/progress"
	"github.com/zhenghaoz/gorse/base/task"
	"github.com/zhenghaoz/gorse/dataset"
	"github.com/zhenghaoz/gorse/model"
	"go.uber.org/zap"
)

// ParamsSearchResult contains the return of grid search.
type ParamsSearchResult struct {
	BestModel  MatrixFactorization
	BestScore  Score
	BestParams model.Params
	BestIndex  int
	Scores     []Score
	Params     []model.Params
}

func (r *ParamsSearchResult) AddScore(params model.Params, score Score) {
	r.Scores = append(r.Scores, score)
	r.Params = append(r.Params, params.Copy())
	if len(r.Scores) == 0 || score.NDCG > r.BestScore.NDCG {
		r.BestScore = score
		r.BestParams = params.Copy()
		r.BestIndex = len(r.Params) - 1
	}
}

// GridSearchCV finds the best parameters for a model.
func GridSearchCV(ctx context.Context, estimator MatrixFactorization, trainSet, testSet dataset.CFSplit, paramGrid model.ParamsGrid,
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
			log.Logger().Info(fmt.Sprintf("grid search (%v/%v)", span.Count(), total),
				zap.Any("params", params))
			// Cross validate
			estimator.Clear()
			estimator.SetParams(estimator.GetParams().Overwrite(params))
			score := estimator.Fit(newCtx, trainSet, testSet, fitConfig)
			// Create GridSearch result
			results.Scores = append(results.Scores, score)
			results.Params = append(results.Params, params.Copy())
			if len(results.Scores) == 0 || score.NDCG > results.BestScore.NDCG {
				results.BestModel = Clone(estimator)
				results.BestScore = score
				results.BestParams = params.Copy()
				results.BestIndex = len(results.Params) - 1
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
func RandomSearchCV(ctx context.Context, estimator MatrixFactorization, trainSet, testSet dataset.CFSplit, paramGrid model.ParamsGrid,
	numTrials int, seed int64, fitConfig *FitConfig) ParamsSearchResult {
	// if the number of combination is less than number of trials, use grid search
	if paramGrid.NumCombinations() < numTrials {
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
		log.Logger().Info(fmt.Sprintf("random search (%v/%v)", i, numTrials),
			zap.Any("params", params))
		estimator.Clear()
		estimator.SetParams(estimator.GetParams().Overwrite(params))
		score := estimator.Fit(newCtx, trainSet, testSet, fitConfig)
		results.Scores = append(results.Scores, score)
		results.Params = append(results.Params, params.Copy())
		if len(results.Scores) == 0 || score.NDCG > results.BestScore.NDCG {
			results.BestModel = Clone(estimator)
			results.BestScore = score
			results.BestParams = params.Copy()
			results.BestIndex = len(results.Params) - 1
		}
		span.Add(1)
	}
	span.End()
	return results
}

// ModelSearcher is a thread-safe personal ranking model searcher.
type ModelSearcher struct {
	models []MatrixFactorization
	// arguments
	numEpochs  int
	numTrials  int
	searchSize bool
	// results
	bestMutex     sync.Mutex
	bestModelName string
	bestModel     MatrixFactorization
	bestScore     Score
}

// NewModelSearcher creates a thread-safe personal ranking model searcher.
func NewModelSearcher(nEpoch, nTrials int, searchSize bool) *ModelSearcher {
	searcher := &ModelSearcher{
		numTrials:  nTrials,
		numEpochs:  nEpoch,
		searchSize: searchSize,
	}
	searcher.models = append(searcher.models, NewBPR(model.Params{model.NEpochs: searcher.numEpochs}))
	searcher.models = append(searcher.models, NewALS(model.Params{model.NEpochs: searcher.numEpochs}))
	return searcher
}

// GetBestModel returns the optimal personal ranking model.
func (searcher *ModelSearcher) GetBestModel() (string, MatrixFactorization, Score) {
	searcher.bestMutex.Lock()
	defer searcher.bestMutex.Unlock()
	return searcher.bestModelName, searcher.bestModel, searcher.bestScore
}

func (searcher *ModelSearcher) Fit(ctx context.Context, trainSet, valSet dataset.CFSplit, j *task.JobsAllocator) error {
	log.Logger().Info("ranking model search",
		zap.Int("n_users", trainSet.CountUsers()),
		zap.Int("n_items", trainSet.CountItems()))
	startTime := time.Now()
	for _, m := range searcher.models {
		r := RandomSearchCV(ctx, m, trainSet, valSet, m.GetParamsGrid(searcher.searchSize), searcher.numTrials, 0, NewFitConfig())
		searcher.bestMutex.Lock()
		if searcher.bestModel == nil || r.BestScore.NDCG > searcher.bestScore.NDCG {
			searcher.bestModel = r.BestModel
			searcher.bestScore = r.BestScore
		}
		searcher.bestMutex.Unlock()
	}
	searchTime := time.Since(startTime)
	log.Logger().Info("complete ranking model search",
		zap.Float32("NDCG@10", searcher.bestScore.NDCG),
		zap.Float32("Precision@10", searcher.bestScore.Precision),
		zap.Float32("Recall@10", searcher.bestScore.Recall),
		zap.String("model", GetModelName(searcher.bestModel)),
		zap.Any("params", searcher.bestModel.GetParams()),
		zap.String("search_time", searchTime.String()))
	return nil
}
