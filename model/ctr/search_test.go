// Copyright 2021 gorse Project Authors
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
	"io"
	"testing"

	"github.com/gorse-io/gorse/base"
	"github.com/gorse-io/gorse/base/task"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model"
	"github.com/stretchr/testify/assert"
)

// NewMapIndexDataset creates a data set.
func NewMapIndexDataset() *Dataset {
	s := new(Dataset)
	s.Index = base.NewUnifiedDirectIndex(0)
	return s
}

type mockFactorizationMachineForSearch struct {
	model.BaseModel
}

func (m *mockFactorizationMachineForSearch) Marshal(_ io.Writer) error {
	panic("implement me")
}

func (m *mockFactorizationMachineForSearch) Invalid() bool {
	panic("implement me")
}

func (m *mockFactorizationMachineForSearch) GetUserIndex() base.Index {
	panic("don't call me")
}

func (m *mockFactorizationMachineForSearch) GetItemIndex() base.Index {
	panic("don't call me")
}

func (m *mockFactorizationMachineForSearch) Fit(_ context.Context, _, _ dataset.CTRSplit, cfg *FitConfig) Score {
	score := float32(0)
	score += m.Params.GetFloat32(model.NFactors, 0.0)
	score += m.Params.GetFloat32(model.InitMean, 0.0)
	score += m.Params.GetFloat32(model.InitStdDev, 0.0)
	return Score{AUC: score}
}

func (m *mockFactorizationMachineForSearch) Predict(_, _ string, _, _ []Feature) float32 {
	panic("don't call me")
}

func (m *mockFactorizationMachineForSearch) InternalPredict(_ []int32, _ []float32) float32 {
	panic("don't call me")
}

func (m *mockFactorizationMachineForSearch) Clear() {
	// do nothing
}

func (m *mockFactorizationMachineForSearch) GetParamsGrid(_ bool) model.ParamsGrid {
	return model.ParamsGrid{
		model.NFactors:   []interface{}{1, 2, 3, 4},
		model.InitMean:   []interface{}{4, 3, 2, 1},
		model.InitStdDev: []interface{}{4, 4, 4, 4},
	}
}

func newFitConfigForSearch() *FitConfig {
	return &FitConfig{
		Verbose: 1,
	}
}

func TestGridSearchCV(t *testing.T) {
	m := &mockFactorizationMachineForSearch{}
	fitConfig := newFitConfigForSearch()
	r := GridSearchCV(context.Background(), m, nil, nil, m.GetParamsGrid(false), 0, fitConfig)
	assert.Equal(t, float32(12), r.BestScore.AUC)
	assert.Equal(t, model.Params{
		model.NFactors:   4,
		model.InitMean:   4,
		model.InitStdDev: 4,
	}, r.BestParams)
}

func TestRandomSearchCV(t *testing.T) {
	m := &mockFactorizationMachineForSearch{}
	fitConfig := newFitConfigForSearch()
	r := RandomSearchCV(context.Background(), m, nil, nil, m.GetParamsGrid(false), 63, 0, fitConfig)
	assert.Equal(t, float32(12), r.BestScore.AUC)
	assert.Equal(t, model.Params{
		model.NFactors:   4,
		model.InitMean:   4,
		model.InitStdDev: 4,
	}, r.BestParams)
}

func TestModelSearcher_RandomSearch(t *testing.T) {
	searcher := NewModelSearcher(2, 63, false)
	searcher.model = &mockFactorizationMachineForSearch{model.BaseModel{Params: model.Params{model.NEpochs: 2}}}
	err := searcher.Fit(context.Background(), NewMapIndexDataset(), NewMapIndexDataset(), task.NewConstantJobsAllocator(1))
	assert.NoError(t, err)
	m, score := searcher.GetBestModel()
	assert.Equal(t, float32(12), score.AUC)
	assert.Equal(t, model.Params{
		model.NEpochs:    2,
		model.NFactors:   4,
		model.InitMean:   4,
		model.InitStdDev: 4,
	}, m.GetParams())
}

func TestModelSearcher_GridSearch(t *testing.T) {
	searcher := NewModelSearcher(2, 64, false)
	searcher.model = &mockFactorizationMachineForSearch{model.BaseModel{Params: model.Params{model.NEpochs: 2}}}
	err := searcher.Fit(context.Background(), NewMapIndexDataset(), NewMapIndexDataset(), task.NewConstantJobsAllocator(1))
	assert.NoError(t, err)
	m, score := searcher.GetBestModel()
	assert.Equal(t, float32(12), score.AUC)
	assert.Equal(t, model.Params{
		model.NEpochs:    2,
		model.NFactors:   4,
		model.InitMean:   4,
		model.InitStdDev: 4,
	}, m.GetParams())
}
