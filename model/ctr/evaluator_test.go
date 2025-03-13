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
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPrecision(t *testing.T) {
	posPrediction := []float32{1, 1, 1}
	negPrediction := []float32{1}
	precision := Precision(posPrediction, negPrediction)
	assert.Equal(t, float32(0.75), precision)
	precision = Precision(nil, nil)
	assert.Zero(t, precision)
}

func TestRecall(t *testing.T) {
	posPrediction := []float32{1, -1, -1, -1}
	recall := Recall(posPrediction, nil)
	assert.Equal(t, float32(0.25), recall)
	recall = Recall(nil, nil)
	assert.Zero(t, recall)
}

func TestAccuracy(t *testing.T) {
	posPrediction := []float32{1, 1, -1, -1}
	negPrediction := []float32{1, 1, -1, -1}
	accuracy := Accuracy(posPrediction, negPrediction)
	assert.Equal(t, float32(0.5), accuracy)
	accuracy = Accuracy(nil, nil)
	assert.Zero(t, accuracy)
}
