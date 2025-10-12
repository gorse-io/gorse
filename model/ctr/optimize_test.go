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

	"github.com/c-bata/goptuna"
	"github.com/c-bata/goptuna/tpe"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

// NewMapIndexDataset creates a data set.
func NewMapIndexDataset() *Dataset {
	s := new(Dataset)
	s.Index = dataset.NewUnifiedDirectIndex(0)
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

func (m *mockFactorizationMachineForSearch) GetUserIndex() dataset.Index {
	panic("don't call me")
}

func (m *mockFactorizationMachineForSearch) GetItemIndex() dataset.Index {
	panic("don't call me")
}

func (m *mockFactorizationMachineForSearch) Fit(_ context.Context, _, _ dataset.CTRSplit, cfg *FitConfig) Score {
	score := float32(0)
	score += m.Params.GetFloat32(model.NFactors, 0.0)
	score += m.Params.GetFloat32(model.InitMean, 0.0)
	score += m.Params.GetFloat32(model.InitStdDev, 0.0)
	return Score{AUC: score}
}

func (m *mockFactorizationMachineForSearch) Predict(_, _ string, _, _ []Label) float32 {
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

func (m *mockFactorizationMachineForSearch) SuggestParams(trial goptuna.Trial) model.Params {
	return model.Params{
		model.NFactors:   lo.Must(trial.SuggestDiscreteFloat(string(model.NFactors), 1, 4, 1)),
		model.InitMean:   lo.Must(trial.SuggestDiscreteFloat(string(model.InitMean), 1, 4, 1)),
		model.InitStdDev: lo.Must(trial.SuggestDiscreteFloat(string(model.InitStdDev), 4, 4, 1)),
	}
}

func TestTPE(t *testing.T) {
	search := NewModelSearch(map[string]ModelCreator{
		"mock": func() FactorizationMachines {
			return &mockFactorizationMachineForSearch{}
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
	assert.Equal(t, Score{AUC: 12}, result.Score)
}
