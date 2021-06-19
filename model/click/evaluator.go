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

package click

import (
	"github.com/barkimedes/go-deepcopy"
	"github.com/chewxy/math32"
)

// EvaluateRegression evaluates factorization machines in regression task.
func EvaluateRegression(estimator FactorizationMachine, testSet *Dataset) Score {
	sum := float32(0)
	// For all UserFeedback
	for i := 0; i < testSet.Count(); i++ {
		labels, target := testSet.Get(i)
		prediction := estimator.InternalPredict(labels)
		sum += (target - prediction) * (target - prediction)
	}
	return Score{
		Task: FMRegression,
		RMSE: math32.Sqrt(sum / float32(testSet.Count())),
	}
}

// EvaluateClassification evaluates factorization machines in classification task.
func EvaluateClassification(estimator FactorizationMachine, testSet *Dataset) Score {
	correct := float32(0)
	// For all UserFeedback
	for i := 0; i < testSet.Count(); i++ {
		labels, target := testSet.Get(i)
		prediction := estimator.InternalPredict(labels)
		if target*prediction > 0 {
			correct++
		}
	}
	return Score{
		Task:      FMClassification,
		Precision: correct / float32(testSet.Count()),
	}
}

// SnapshotManger manages the best snapshot.
type SnapshotManger struct {
	BestWeights []interface{}
	BestScore   Score
}

// AddSnapshot adds a copied snapshot.
func (sm *SnapshotManger) AddSnapshot(score Score, weights ...interface{}) {
	if sm.BestWeights == nil || score.BetterThan(sm.BestScore) {
		sm.BestScore = score
		if temp, err := deepcopy.Anything(weights); err != nil {
			panic(err)
		} else {
			sm.BestWeights = temp.([]interface{})
		}
	}
}
