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
	"io"
	"testing"
	"time"

	"github.com/c-bata/goptuna"
	"github.com/c-bata/goptuna/tpe"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base/task"
	"github.com/zhenghaoz/gorse/dataset"
	"github.com/zhenghaoz/gorse/model"
)

type mockMatrixFactorizationForSearch struct {
	model.BaseModel
}

func newMockMatrixFactorizationForSearch(numEpoch int) *mockMatrixFactorizationForSearch {
	return &mockMatrixFactorizationForSearch{model.BaseModel{Params: model.Params{model.NEpochs: numEpoch}}}
}

func (m *mockMatrixFactorizationForSearch) GetUserFactor(_ int32) []float32 {
	panic("implement me")
}

func (m *mockMatrixFactorizationForSearch) GetItemFactor(_ int32) []float32 {
	panic("implement me")
}

func (m *mockMatrixFactorizationForSearch) IsUserPredictable(_ int32) bool {
	panic("implement me")
}

func (m *mockMatrixFactorizationForSearch) IsItemPredictable(_ int32) bool {
	panic("implement me")
}

func (m *mockMatrixFactorizationForSearch) Marshal(_ io.Writer) error {
	panic("implement me")
}

func (m *mockMatrixFactorizationForSearch) Unmarshal(_ io.Reader) error {
	panic("implement me")
}

func (m *mockMatrixFactorizationForSearch) Invalid() bool {
	panic("implement me")
}

func (m *mockMatrixFactorizationForSearch) GetUserIndex() *dataset.FreqDict {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForSearch) GetItemIndex() *dataset.FreqDict {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForSearch) Fit(_ context.Context, _, _ dataset.CFSplit, _ *FitConfig) Score {
	score := float32(0)
	score += m.Params.GetFloat32(model.NFactors, 0.0)
	score += m.Params.GetFloat32(model.InitMean, 0.0)
	score += m.Params.GetFloat32(model.InitStdDev, 0.0)
	return Score{NDCG: score}
}

func (m *mockMatrixFactorizationForSearch) Predict(_, _ string) float32 {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForSearch) internalPredict(_, _ int32) float32 {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForSearch) Clear() {
	// do nothing
}

func (m *mockMatrixFactorizationForSearch) GetParamsGrid(_ bool) model.ParamsGrid {
	return model.ParamsGrid{
		model.NFactors:   []interface{}{1, 2, 3, 4},
		model.InitMean:   []interface{}{1, 2, 3, 4},
		model.InitStdDev: []interface{}{4},
	}
}

func (m *mockMatrixFactorizationForSearch) SuggestParams(trial goptuna.Trial) model.Params {
	return model.Params{
		model.NFactors:   lo.Must(trial.SuggestDiscreteFloat(string(model.NFactors), 1, 4, 1)),
		model.InitMean:   lo.Must(trial.SuggestDiscreteFloat(string(model.InitMean), 1, 4, 1)),
		model.InitStdDev: lo.Must(trial.SuggestDiscreteFloat(string(model.InitStdDev), 4, 4, 1)),
	}
}

func newFitConfigForSearch() *FitConfig {
	return &FitConfig{
		Verbose: 1,
	}
}

func TestGridSearchCV(t *testing.T) {
	m := &mockMatrixFactorizationForSearch{}
	fitConfig := newFitConfigForSearch()
	r := GridSearchCV(context.Background(), m, nil, nil, m.GetParamsGrid(false), 0, fitConfig)
	assert.Equal(t, float32(12), r.BestScore.NDCG)
	assert.Equal(t, model.Params{
		model.NFactors:   4,
		model.InitMean:   4,
		model.InitStdDev: 4,
	}, r.BestParams)
}

func TestRandomSearchCV(t *testing.T) {
	m := &mockMatrixFactorizationForSearch{}
	fitConfig := newFitConfigForSearch()
	r := RandomSearchCV(context.Background(), m, nil, nil, m.GetParamsGrid(false), 17, 0, fitConfig)
	assert.Equal(t, float32(12), r.BestScore.NDCG)
	assert.Equal(t, model.Params{
		model.NFactors:   4,
		model.InitMean:   4,
		model.InitStdDev: 4,
	}, r.BestParams)
}

func TestTPE(t *testing.T) {
	search := NewModelSearch(nil, nil, &mockMatrixFactorizationForSearch{})
	study, err := goptuna.CreateStudy("TestTPE",
		goptuna.StudyOptionDirection(goptuna.StudyDirectionMaximize),
		goptuna.StudyOptionSampler(tpe.NewSampler()))
	assert.NoError(t, err)
	err = study.Optimize(search.Objective, 10)
	assert.NoError(t, err)
	v, _ := study.GetBestValue()
	p, _ := study.GetBestParams()
	assert.Equal(t, float64(12), v)
	assert.Equal(t, model.Params{
		model.NFactors:   float64(4),
		model.InitMean:   float64(4),
		model.InitStdDev: float64(4),
	}, search.New(p).GetParams())
}

func TestModelSearcher(t *testing.T) {
	searcher := NewModelSearcher(2, 63, false)
	searcher.models = []MatrixFactorization{newMockMatrixFactorizationForSearch(2)}
	err := searcher.Fit(context.Background(),
		dataset.NewDataset(time.Now(), 0, 0),
		dataset.NewDataset(time.Now(), 0, 0),
		task.NewConstantJobsAllocator(1))
	assert.NoError(t, err)
	_, m, score := searcher.GetBestModel()
	assert.Equal(t, float32(12), score.NDCG)
	assert.Equal(t, model.Params{
		model.NEpochs:    2,
		model.NFactors:   4,
		model.InitMean:   4,
		model.InitStdDev: 4,
	}, m.GetParams())
}
