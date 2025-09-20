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

	"github.com/c-bata/goptuna"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/storage/meta"
	"github.com/juju/errors"
	"golang.org/x/exp/maps"
)

type ModelCreator func() FactorizationMachines

type ModelSearch struct {
	modelCreators map[string]ModelCreator
	modelTypes    []string
	trainSet      dataset.CTRSplit
	testSet       dataset.CTRSplit
	config        *FitConfig
	result        meta.Model[Score]
}

func NewModelSearch(models map[string]ModelCreator, trainSet, testSet dataset.CTRSplit, config *FitConfig) *ModelSearch {
	return &ModelSearch{
		modelCreators: models,
		modelTypes:    maps.Keys(models),
		trainSet:      trainSet,
		testSet:       testSet,
		config:        config,
	}
}

func (ms *ModelSearch) Objective(trial goptuna.Trial) (float64, error) {
	if len(ms.modelCreators) == 0 {
		return 0, errors.New("no model to search")
	}
	modelType, err := trial.SuggestCategorical("Model", ms.modelTypes)
	if err != nil {
		return 0, errors.Trace(err)
	}
	m := ms.modelCreators[modelType]()
	m.SetParams(m.SuggestParams(trial))
	score := m.Fit(context.Background(), ms.trainSet, ms.testSet, ms.config)
	if score.AUC > ms.result.Score.AUC {
		ms.result = meta.Model[Score]{
			Type:   modelType,
			Params: m.GetParams(),
			Score:  score,
		}
	}
	return float64(score.AUC), nil
}

func (ms *ModelSearch) Result() meta.Model[Score] {
	return ms.result
}
