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
package model

import (
	"github.com/chewxy/math32"
	"github.com/zhenghaoz/gorse/base"
	"testing"
)

const evalEpsilon = 0.00001

func EqualEpsilon(t *testing.T, expect float32, actual float32, epsilon float32) {
	if math32.Abs(expect-actual) > evalEpsilon {
		t.Fatalf("Expect %f ± %f, Actual: %f\n", expect, epsilon, actual)
	}
}

func TestNDCG(t *testing.T) {
	targetSet := base.NewSet([]int{1, 3, 5, 7})
	rankList := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	EqualEpsilon(t, 0.6766372989, NDCG(targetSet, rankList), evalEpsilon)
}

func TestPrecision(t *testing.T) {
	targetSet := base.NewSet([]int{1, 3, 5, 7})
	rankList := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	EqualEpsilon(t, 0.4, Precision(targetSet, rankList), evalEpsilon)
}

func TestRecall(t *testing.T) {
	targetSet := base.NewSet([]int{1, 3, 15, 17, 19})
	rankList := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	EqualEpsilon(t, 0.4, Recall(targetSet, rankList), evalEpsilon)
}

func TestAP(t *testing.T) {
	targetSet := base.NewSet([]int{1, 3, 7, 9})
	rankList := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	EqualEpsilon(t, 0.44375, MAP(targetSet, rankList), evalEpsilon)
}

func TestRR(t *testing.T) {
	targetSet := base.NewSet([]int{3})
	rankList := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	EqualEpsilon(t, 0.25, MRR(targetSet, rankList), evalEpsilon)
}
