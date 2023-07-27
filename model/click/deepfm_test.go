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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/model"
)

func TestDeepFM_Classification_Frappe(t *testing.T) {
	train, test, err := LoadDataFromBuiltIn("frappe")
	assert.NoError(t, err)
	m := NewDeepFM(model.Params{
		model.InitStdDev: 0.01,
		model.NFactors:   8,
		model.NEpochs:    10,
		model.Lr:         0.01,
		model.Reg:        0.0001,
	})
	fitConfig := newFitConfigWithTestTracker(20)
	score := m.Fit(context.Background(), train, test, fitConfig)
	assert.InDelta(t, 0.9271656, score.Accuracy, classificationDelta)
}
