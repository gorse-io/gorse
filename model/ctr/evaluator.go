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

package ctr

import (
	"sort"

	"github.com/gorse-io/gorse/dataset"

	"github.com/chewxy/math32"
	"github.com/samber/lo"
	"modernc.org/sortutil"
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
	if testSet.Count() == 0 {
		return Score{
			RMSE: 0,
		}
	}
	return Score{
		RMSE: math32.Sqrt(sum / float32(testSet.Count())),
	}
}

// EvaluateClassification evaluates factorization machines in classification task.
func EvaluateClassification(estimator FactorizationMachine, testSet dataset.CTRSplit) Score {
	// For all UserFeedback
	var posFeatures, negFeatures []lo.Tuple2[[]int32, []float32]
	for i := 0; i < testSet.Count(); i++ {
		indices, values, target := testSet.Get(i)
		if target > 0 {
			posFeatures = append(posFeatures, lo.Tuple2[[]int32, []float32]{A: indices, B: values})
		} else {
			negFeatures = append(negFeatures, lo.Tuple2[[]int32, []float32]{A: indices, B: values})
		}
	}
	var posPrediction, negPrediction []float32
	if batchInference, ok := estimator.(BatchInference); ok {
		posPrediction = batchInference.BatchInternalPredict(posFeatures)
		negPrediction = batchInference.BatchInternalPredict(negFeatures)
	} else {
		for _, features := range posFeatures {
			posPrediction = append(posPrediction, estimator.InternalPredict(features.A, features.B))
		}
		for _, features := range negFeatures {
			negPrediction = append(negPrediction, estimator.InternalPredict(features.A, features.B))
		}
	}
	if testSet.Count() == 0 {
		return Score{
			Precision: 0,
		}
	}
	return Score{
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
