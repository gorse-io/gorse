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
package click

import (
	"github.com/stretchr/testify/mock"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
)

// NewMapIndexDataset creates a data set.
func NewMapIndexDataset() *Dataset {
	s := new(Dataset)
	s.Index = NewUnifiedDirectIndex(0)
	return s
}

type mockFactorizationMachineForSearch struct {
	model.BaseModel
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

func (m *mockFactorizationMachineForSearch) Fit(_, _ *Dataset, _ *FitConfig) Score {
	score := float32(0)
	score += m.Params.GetFloat32(model.NFactors, 0.0)
	score += m.Params.GetFloat32(model.NEpochs, 0.0)
	score += m.Params.GetFloat32(model.InitMean, 0.0)
	score += m.Params.GetFloat32(model.InitStdDev, 0.0)
	return Score{Task: FMClassification, AUC: score}
}

func (m *mockFactorizationMachineForSearch) Predict(_, _ string, _, _ []string) float32 {
	panic("don't call me")
}

func (m *mockFactorizationMachineForSearch) InternalPredict(_ []int32, _ []float32) float32 {
	panic("don't call me")
}

func (m *mockFactorizationMachineForSearch) Clear() {
	// do nothing
}

func (m *mockFactorizationMachineForSearch) GetParamsGrid() model.ParamsGrid {
	return model.ParamsGrid{
		model.NFactors:   []interface{}{1, 2, 3, 4},
		model.InitMean:   []interface{}{4, 3, 2, 1},
		model.InitStdDev: []interface{}{4, 4, 4, 4},
	}
}

type mockRunner struct {
	mock.Mock
}

func (r *mockRunner) Lock() {
	r.Called()
}

func (r *mockRunner) UnLock() {
	r.Called()
}

func newFitConfigForSearch() (*FitConfig, *mockTracker) {
	tracker := new(mockTracker)
	tracker.On("Suspend", mock.Anything)
	return &FitConfig{
		Jobs:    1,
		Verbose: 1,
		Tracker: tracker,
	}, tracker
}

func TestGridSearchCV(t *testing.T) {
	m := &mockFactorizationMachineForSearch{}
	fitConfig, tracker := newFitConfigForSearch()
	runner := new(mockRunner)
	runner.On("Lock")
	runner.On("UnLock")
	r := GridSearchCV(m, nil, nil, m.GetParamsGrid(), 0, fitConfig, runner)
	assert.Equal(t, float32(12), r.BestScore.AUC)
	tracker.AssertExpectations(t)
	runner.AssertCalled(t, "Lock")
	runner.AssertCalled(t, "UnLock")
	assert.Equal(t, model.Params{
		model.NFactors:   4,
		model.InitMean:   4,
		model.InitStdDev: 4,
	}, r.BestParams)
}

func TestRandomSearchCV(t *testing.T) {
	m := &mockFactorizationMachineForSearch{}
	fitConfig, tracker := newFitConfigForSearch()
	runner := new(mockRunner)
	runner.On("Lock")
	runner.On("UnLock")
	r := RandomSearchCV(m, nil, nil, m.GetParamsGrid(), 63, 0, fitConfig, runner)
	tracker.AssertExpectations(t)
	runner.AssertCalled(t, "Lock")
	runner.AssertCalled(t, "UnLock")
	assert.Equal(t, float32(12), r.BestScore.AUC)
	assert.Equal(t, model.Params{
		model.NFactors:   4,
		model.InitMean:   4,
		model.InitStdDev: 4,
	}, r.BestParams)
}

func TestModelSearcher(t *testing.T) {
	tracker := new(mockTracker)
	tracker.On("Start", 63*2)
	tracker.On("SubTracker")
	tracker.On("Finish")
	runner := new(mockRunner)
	runner.On("Lock")
	runner.On("UnLock")
	searcher := NewModelSearcher(2, 63, 1)
	searcher.model = &mockFactorizationMachineForSearch{}
	err := searcher.Fit(NewMapIndexDataset(), NewMapIndexDataset(), tracker, runner)
	assert.NoError(t, err)
	m, score := searcher.GetBestModel()
	assert.Equal(t, float32(12), score.AUC)
	assert.Equal(t, model.Params{
		model.NFactors:   4,
		model.InitMean:   4,
		model.InitStdDev: 4,
	}, m.GetParams())
}
