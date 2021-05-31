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
package ranking

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestKNN_MovieLens(t *testing.T) {
	trainSet, testSet, err := LoadDataFromBuiltIn("ml-1m")
	assert.Nil(t, err)
	m := NewKNN(nil)
	score := m.Fit(trainSet, testSet, fitConfig)
	assertEpsilon(t, 0.311, score.NDCG, benchEpsilon)
	assert.Equal(t, m.Predict([]string{"1"}, "1"), m.InternalPredict([]int{1}, 1))
}

//func TestKNN_Pinterest(t *testing.T) {
//	trainSet, testSet, err := LoadDataFromBuiltIn("pinterest-20")
//	assert.Nil(t, err)
//	m := NewKNN(nil)
//	score := m.Fit(trainSet, testSet, fitConfig)
//	assertEpsilon(t, 0.570, score.NDCG, benchEpsilon)
//}
