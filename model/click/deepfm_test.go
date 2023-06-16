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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/model"
)

func TestDeepFM_Classification_Frappe(t *testing.T) {
	train, test, err := LoadDataFromBuiltIn("frappe")
	assert.NoError(t, err)
	m := NewDeepFM(model.Params{
		model.NFactors: 16,
		model.NEpochs:  1,
	})
	fitConfig := newFitConfigWithTestTracker(20)
	score := m.Fit(train, test, fitConfig)
	assert.InDelta(t, 0.91684, score.Accuracy, classificationDelta)
	assert.Equal(t, m.Complexity(), fitConfig.Task.Done)
	assert.Equal(t, m.InternalPredict([]int32{1, 2, 3, 4, 5, 6}, []float32{1, 1, 0.3, 0.4, 0.5, 0.6}),
		m.Predict("1", "2",
			[]Feature{
				{Name: "3", Value: 0.3},
				{Name: "4", Value: 0.4},
			},
			[]Feature{
				{Name: "5", Value: 0.5},
				{Name: "6", Value: 0.6},
			}))
}
