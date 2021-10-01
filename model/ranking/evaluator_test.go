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
package ranking

import (
	"github.com/scylladb/go-set/i32set"
	"io"
	"strconv"
	"testing"

	"github.com/scylladb/go-set"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
)

const evalEpsilon = 0.00001

func TestNDCG(t *testing.T) {
	targetSet := set.NewInt32Set(1, 3, 5, 7)
	rankList := []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	assert.InDelta(t, 0.6766372989, NDCG(targetSet, rankList), evalEpsilon)
}

func TestPrecision(t *testing.T) {
	targetSet := set.NewInt32Set(1, 3, 5, 7)
	rankList := []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	assert.InDelta(t, 0.4, Precision(targetSet, rankList), evalEpsilon)
}

func TestRecall(t *testing.T) {
	targetSet := set.NewInt32Set(1, 3, 15, 17, 19)
	rankList := []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	assert.InDelta(t, 0.4, Recall(targetSet, rankList), evalEpsilon)
}

func TestAP(t *testing.T) {
	targetSet := set.NewInt32Set(1, 3, 7, 9)
	rankList := []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	assert.InDelta(t, 0.44375, MAP(targetSet, rankList), evalEpsilon)
}

func TestRR(t *testing.T) {
	targetSet := set.NewInt32Set(3)
	rankList := []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	assert.InDelta(t, 0.25, MRR(targetSet, rankList), evalEpsilon)
}

func TestHR(t *testing.T) {
	rankList := []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	assert.InDelta(t, 1, HR(set.NewInt32Set(3), rankList), evalEpsilon)
	assert.InDelta(t, 0, HR(set.NewInt32Set(30), rankList), evalEpsilon)
}

type mockMatrixFactorizationForEval struct {
	model.BaseModel
	positive []*i32set.Set
	negative []*i32set.Set
}

func (m *mockMatrixFactorizationForEval) Marshal(_ io.Writer) error {
	panic("implement me")
}

func (m *mockMatrixFactorizationForEval) Unmarshal(_ io.Reader) error {
	panic("implement me")
}

func (m *mockMatrixFactorizationForEval) Invalid() bool {
	panic("implement me")
}

func (m *mockMatrixFactorizationForEval) GetUserIndex() base.Index {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForEval) GetItemIndex() base.Index {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForEval) Fit(_, _ *DataSet, _ *FitConfig) Score {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForEval) Predict(_, _ string) float32 {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForEval) InternalPredict(userId, itemId int32) float32 {
	if m.positive[userId].Has(itemId) {
		return 1
	}
	if m.negative[userId].Has(itemId) {
		return -1
	}
	return 0
}

func (m *mockMatrixFactorizationForEval) Clear() {
	// do nothing
}

func (m *mockMatrixFactorizationForEval) GetParamsGrid() model.ParamsGrid {
	panic("don't call me")
}

func TestEvaluate(t *testing.T) {
	// create dataset
	train, test := NewDirectIndexDataset(), NewDirectIndexDataset()
	train.UserFeedback = make([][]int32, 4)
	for i := 0; i < 16; i++ {
		test.AddFeedback(strconv.Itoa(i/4), strconv.Itoa(i), true)
	}
	assert.Equal(t, 16, test.Count())
	assert.Equal(t, 4, test.UserCount())
	assert.Equal(t, 16, test.ItemCount())
	// create model
	m := &mockMatrixFactorizationForEval{
		positive: []*i32set.Set{
			set.NewInt32Set(0, 1, 2, 3),
			set.NewInt32Set(4, 5, 6),
			set.NewInt32Set(8, 9),
			set.NewInt32Set(12),
		},
		negative: []*i32set.Set{
			set.NewInt32Set(),
			set.NewInt32Set(7),
			set.NewInt32Set(10, 11),
			set.NewInt32Set(13, 14, 15),
		},
	}
	// evaluate model
	s := Evaluate(m, test, train, 4, test.ItemCount(), 4, Precision)
	assert.Equal(t, 1, len(s))
	assert.Equal(t, float32(0.625), s[0])
}

func TestSnapshotManger_AddSnapshot(t *testing.T) {
	a := []int{0}
	b := [][]int{{0}}
	snapshots := SnapshotManger{}
	a[0] = 1
	b[0][0] = 1
	snapshots.AddSnapshot(Score{NDCG: 1}, a, b)
	a[0] = 3
	b[0][0] = 3
	snapshots.AddSnapshot(Score{NDCG: 3}, a, b)
	a[0] = 2
	b[0][0] = 2
	snapshots.AddSnapshot(Score{NDCG: 2}, a, b)
	assert.Equal(t, float32(3), snapshots.BestScore.NDCG)
	assert.Equal(t, []int{3}, snapshots.BestWeights[0])
	assert.Equal(t, [][]int{{3}}, snapshots.BestWeights[1])
}
