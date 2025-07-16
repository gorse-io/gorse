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
package cf

import (
	"context"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/dataset"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/storage/data"
	"io"
	"strconv"
	"testing"
	"time"
)

const evalEpsilon = 0.00001

func TestNDCG(t *testing.T) {
	targetSet := mapset.NewSet[int32](1, 3, 5, 7)
	rankList := []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	assert.InDelta(t, 0.6766372989, NDCG(targetSet, rankList), evalEpsilon)
}

func TestPrecision(t *testing.T) {
	targetSet := mapset.NewSet[int32](1, 3, 5, 7)
	rankList := []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	assert.InDelta(t, 0.4, Precision(targetSet, rankList), evalEpsilon)
}

func TestRecall(t *testing.T) {
	targetSet := mapset.NewSet[int32](1, 3, 15, 17, 19)
	rankList := []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	assert.InDelta(t, 0.4, Recall(targetSet, rankList), evalEpsilon)
}

func TestAP(t *testing.T) {
	targetSet := mapset.NewSet[int32](1, 3, 7, 9)
	rankList := []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	assert.InDelta(t, 0.44375, MAP(targetSet, rankList), evalEpsilon)
}

func TestRR(t *testing.T) {
	targetSet := mapset.NewSet[int32](3)
	rankList := []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	assert.InDelta(t, 0.25, MRR(targetSet, rankList), evalEpsilon)
}

func TestHR(t *testing.T) {
	rankList := []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	assert.InDelta(t, 1, HR(mapset.NewSet[int32](3), rankList), evalEpsilon)
	assert.InDelta(t, 0, HR(mapset.NewSet[int32](30), rankList), evalEpsilon)
}

type mockMatrixFactorizationForEval struct {
	model.BaseModel
	positive []mapset.Set[int32]
	negative []mapset.Set[int32]
}

func (m *mockMatrixFactorizationForEval) GetUserFactor(_ int32) []float32 {
	panic("implement me")
}

func (m *mockMatrixFactorizationForEval) GetItemFactor(_ int32) []float32 {
	panic("implement me")
}

func (m *mockMatrixFactorizationForEval) IsUserPredictable(_ int32) bool {
	panic("implement me")
}

func (m *mockMatrixFactorizationForEval) IsItemPredictable(_ int32) bool {
	panic("implement me")
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

func (m *mockMatrixFactorizationForEval) GetUserIndex() *dataset.FreqDict {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForEval) GetItemIndex() *dataset.FreqDict {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForEval) Fit(_ context.Context, _, _ dataset.CFSplit, _ *FitConfig) Score {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForEval) Predict(_, _ string) float32 {
	panic("don't call me")
}

func (m *mockMatrixFactorizationForEval) internalPredict(userId, itemId int32) float32 {
	if m.positive[userId].Contains(itemId) {
		return 1
	}
	if m.negative[userId].Contains(itemId) {
		return -1
	}
	return 0
}

func (m *mockMatrixFactorizationForEval) Clear() {
	// do nothing
}

func (m *mockMatrixFactorizationForEval) GetParamsGrid(_ bool) model.ParamsGrid {
	panic("don't call me")
}

func TestEvaluate(t *testing.T) {
	// create dataset
	train, test := dataset.NewDataset(time.Now(), 0, 0), dataset.NewDataset(time.Now(), 0, 0)
	//train.UserFeedback = make([][]int32, 4)
	for i := 0; i < 4; i++ {
		train.AddUser(data.User{UserId: strconv.Itoa(i)})
		test.AddUser(data.User{UserId: strconv.Itoa(i / 4)})
	}
	for i := 0; i < 16; i++ {
		test.AddItem(data.Item{ItemId: strconv.Itoa(i)})
		test.AddFeedback(strconv.Itoa(i/4), strconv.Itoa(i))
	}
	assert.Equal(t, 16, test.CountFeedback())
	assert.Equal(t, 4, test.CountUsers())
	assert.Equal(t, 16, test.CountItems())
	// create model
	m := &mockMatrixFactorizationForEval{
		positive: []mapset.Set[int32]{
			mapset.NewSet[int32](0, 1, 2, 3),
			mapset.NewSet[int32](4, 5, 6),
			mapset.NewSet[int32](8, 9),
			mapset.NewSet[int32](12),
		},
		negative: []mapset.Set[int32]{
			mapset.NewSet[int32](),
			mapset.NewSet[int32](7),
			mapset.NewSet[int32](10, 11),
			mapset.NewSet[int32](13, 14, 15),
		},
	}
	// evaluate model
	s := Evaluate(m, test, train, 4, test.CountItems(), 4, Precision)
	assert.Equal(t, 1, len(s))
	assert.Equal(t, float32(0.625), s[0])
}
