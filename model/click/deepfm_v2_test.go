// Copyright 2023 gorse Project Authors
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
	"bytes"
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/model"
)

func TestDeepFMV2_Classification_Frappe(t *testing.T) {
	t.Skip()
	train, test, err := LoadDataFromBuiltIn("frappe")
	assert.NoError(t, err)
	m := NewDeepFMV2(model.Params{
		model.InitStdDev: 0.01,
		model.NFactors:   8,
		model.NEpochs:    10,
		model.Lr:         0.01,
		model.Reg:        0.0001,
		model.BatchSize:  1024,
	})
	fitConfig := newFitConfigWithTestTracker(20)
	score := m.Fit(context.Background(), train, test, fitConfig)
	//assert.InDelta(t, 0.9439709, score.Accuracy, classificationDelta)
	_ = score
}

func TestDeepFMV2_Classification_Criteo(t *testing.T) {
	t.Skip()
	train, test, err := LoadDataFromBuiltIn("criteo")
	assert.NoError(t, err)
	m := NewDeepFM(model.Params{
		model.InitStdDev: 0.01,
		model.NFactors:   8,
		model.NEpochs:    10,
		model.Lr:         0.01,
		model.Reg:        0.0001,
		model.BatchSize:  1024,
	})
	fitConfig := newFitConfigWithTestTracker(10)
	score := m.Fit(context.Background(), train, test, fitConfig)
	assert.InDelta(t, 0.77, score.Accuracy, classificationDelta)

	// test prediction
	assert.Equal(t, m.BatchInternalPredict([]lo.Tuple2[[]int32, []float32]{{A: []int32{1, 2, 3, 4, 5, 6}, B: []float32{1, 1, 0.3, 0.4, 0.5, 0.6}}}),
		m.BatchPredict([]lo.Tuple4[string, string, []Feature, []Feature]{{
			A: "1",
			B: "2",
			C: []Feature{
				{Name: "3", Value: 0.3},
				{Name: "4", Value: 0.4},
			},
			D: []Feature{
				{Name: "5", Value: 0.5},
				{Name: "6", Value: 0.6},
			}}}))

	// test marshal and unmarshal
	buf := bytes.NewBuffer(nil)
	err = MarshalModel(buf, m)
	assert.NoError(t, err)
	tmp, err := UnmarshalModel(buf)
	assert.NoError(t, err)
	scoreClone := EvaluateClassification(tmp, test)
	assert.InDelta(t, 0.77, scoreClone.Accuracy, regressionDelta)

	// test clear
	assert.False(t, m.Invalid())
	m.Clear()
	assert.True(t, m.Invalid())
}
