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
	"github.com/chewxy/math32"
	"github.com/zhenghaoz/gorse/base/copier"
	"modernc.org/sortutil"
	"sort"
)

// EvaluateRegression evaluates factorization machines in regression task.
func EvaluateRegression(estimator FactorizationMachine, testSet *Dataset) Score {
	sum := float32(0)
	// For all UserFeedback
	for i := 0; i < testSet.Count(); i++ {
		features, values, target := testSet.Get(i)
		prediction := estimator.InternalPredict(features, values)
		sum += (target - prediction) * (target - prediction)
	}
	if 0 == testSet.Count() {
		return Score{
			Task: FMRegression,
			RMSE: 0,
		}
	}
	return Score{
		Task: FMRegression,
		RMSE: math32.Sqrt(sum / float32(testSet.Count())),
	}
}

// EvaluateClassification evaluates factorization machines in classification task.
func EvaluateClassification(estimator FactorizationMachine, testSet *Dataset) Score {
	// For all UserFeedback
	var posPrediction, negPrediction []float32
	for i := 0; i < testSet.Count(); i++ {
		features, values, target := testSet.Get(i)
		prediction := estimator.InternalPredict(features, values)
		if target > 0 {
			posPrediction = append(posPrediction, prediction)
		} else {
			negPrediction = append(negPrediction, prediction)
		}
	}
	if 0 == testSet.Count() {
		return Score{
			Task:      FMClassification,
			Precision: 0,
		}
	}
	return Score{
		Task:      FMClassification,
		Precision: Precision(posPrediction, negPrediction),
		Recall:    Recall(posPrediction, negPrediction),
		Accuracy:  Accuracy(posPrediction, negPrediction),
		AUC:       AUC(posPrediction, negPrediction),
	}
}

func Precision(posPrediction, negPrediction []float32) float32 {
	var tp, fp float32
	for _, p := range posPrediction {
		if p > 0 { // true positive
			tp++
		}
	}
	for _, p := range negPrediction {
		if p > 0 { // false positive
			fp++
		}
	}
	if tp+fp == 0 {
		return 0
	}
	return tp / (tp + fp)
}

func Recall(posPrediction, _ []float32) float32 {
	var tp, fn float32
	for _, p := range posPrediction {
		if p > 0 { // true positive
			tp++
		} else { // false negative
			fn++
		}
	}
	if tp+fn == 0 {
		return 0
	}
	return tp / (tp + fn)
}

func Accuracy(posPrediction, negPrediction []float32) float32 {
	var correct float32
	for _, p := range posPrediction {
		if p > 0 {
			correct++
		}
	}
	for _, p := range negPrediction {
		if p < 0 {
			correct++
		}
	}
	if len(posPrediction)+len(negPrediction) == 0 {
		return 0
	}
	return correct / float32(len(posPrediction)+len(negPrediction))
}

func AUC(posPrediction, negPrediction []float32) float32 {
	sort.Sort(sortutil.Float32Slice(posPrediction))
	sort.Sort(sortutil.Float32Slice(negPrediction))
	var sum float32
	var nPos int
	for pPos := range posPrediction {
		// find the negative sample with the greatest prediction less than current positive sample
		for nPos < len(negPrediction) && negPrediction[nPos] < posPrediction[pPos] {
			nPos++
		}
		// add the number of negative samples have less prediction than current positive sample
		sum += float32(nPos)
	}
	if len(posPrediction)*len(negPrediction) == 0 {
		return 0
	}
	return sum / float32(len(posPrediction)*len(negPrediction))
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
		if err := copier.Copy(&sm.BestWeights, weights); err != nil {
			panic(err)
		}
	}
}
