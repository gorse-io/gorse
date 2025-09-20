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

	"github.com/c-bata/goptuna"
	"github.com/c-bata/goptuna/tpe"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
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

func (m *mockMatrixFactorizationForSearch) SuggestParams(trial goptuna.Trial) model.Params {
	return model.Params{
		model.NFactors:   lo.Must(trial.SuggestDiscreteFloat(string(model.NFactors), 1, 4, 1)),
		model.InitMean:   lo.Must(trial.SuggestDiscreteFloat(string(model.InitMean), 1, 4, 1)),
		model.InitStdDev: lo.Must(trial.SuggestDiscreteFloat(string(model.InitStdDev), 4, 4, 1)),
	}
}

func TestTPE(t *testing.T) {
	search := NewModelSearch(map[string]ModelCreator{
		"mock": func() MatrixFactorization {
			return newMockMatrixFactorizationForSearch(10)
		},
	}, nil, nil, nil)
	study, err := goptuna.CreateStudy("TestTPE",
		goptuna.StudyOptionDirection(goptuna.StudyDirectionMaximize),
		goptuna.StudyOptionSampler(tpe.NewSampler()))
	assert.NoError(t, err)
	err = study.Optimize(search.Objective, 10)
	assert.NoError(t, err)
	v, _ := study.GetBestValue()
	assert.Equal(t, float64(12), v)
	result := search.Result()
	assert.Equal(t, "mock", result.Type)
	assert.Equal(t, model.Params{
		model.NFactors:   float64(4),
		model.InitMean:   float64(4),
		model.InitStdDev: float64(4),
	}, result.Params)
	assert.Equal(t, Score{NDCG: 12}, result.Score)
}
