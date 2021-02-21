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
package rank

import (
	"github.com/chewxy/math32"
)

func EvaluateRegression(estimator FactorizationMachine, testSet *Dataset, trainSet *Dataset) float32 {
	sum := float32(0)
	// For all UserFeedback
	for i := 0; i < testSet.Count(); i++ {
		labels, target := testSet.Get(i)
		prediction := estimator.InternalPredict(labels)
		sum += (target - prediction) * (target - prediction)
	}
	return math32.Sqrt(sum / float32(testSet.Count()))
}

func EvaluateClassification(estimator FactorizationMachine, testSet *Dataset, trainSet *Dataset) float32 {
	correct := float32(0)
	// For all UserFeedback
	for i := 0; i < testSet.Count(); i++ {
		labels, target := testSet.Get(i)
		prediction := estimator.InternalPredict(labels)
		if target*prediction > 0 {
			correct++
		}
	}
	return correct / float32(testSet.Count())
}
