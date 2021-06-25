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
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/chewxy/math32"
	"github.com/scylladb/go-set"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"go.uber.org/zap"
)

type KNN struct {
	model.BaseModel
	similarity string
	ItemIndex  base.Index
	Similarity []ConcurrentMap
}

func (knn *KNN) SetParams(params model.Params) {
	knn.BaseModel.SetParams(params)
	knn.similarity = params.GetString(model.Similarity, model.SimilarityCosine)
}

func (knn *KNN) GetParamsGrid() model.ParamsGrid {
	return model.ParamsGrid{
		model.Similarity: []interface{}{model.SimilarityCosine, model.SimilarityDot},
	}
}

func (knn *KNN) Clear() {
	knn.ItemIndex = nil
	knn.Similarity = nil
}

func (knn *KNN) Invalid() bool {
	return knn == nil || knn.ItemIndex == nil || knn.Similarity == nil
}

func (knn *KNN) Predict(userProfile []string, itemId string) float32 {
	supportIndices := make([]int, 0, len(userProfile))
	for _, supportId := range userProfile {
		supportIndex := knn.ItemIndex.ToNumber(supportId)
		if supportIndex == base.NotId {
			base.Logger().Info("unknown item:", zap.String("item_id", supportId))
			return 0
		}
		supportIndices = append(supportIndices, supportIndex)
	}
	itemIndex := knn.ItemIndex.ToNumber(itemId)
	if itemIndex == base.NotId {
		base.Logger().Info("unknown item:", zap.String("item_id", itemId))
		return 0
	}
	return knn.InternalPredict(supportIndices, itemIndex)
}

func (knn *KNN) InternalPredict(userProfile []int, itemIndex int) float32 {
	sum := float32(0)
	for _, supportIndex := range userProfile {
		sum += knn.Similarity[supportIndex].Get(itemIndex)
	}
	return sum
}

func (knn *KNN) GetItemIndex() base.Index {
	return knn.ItemIndex
}

func NewKNN(params model.Params) *KNN {
	knn := new(KNN)
	knn.SetParams(params)
	return knn
}

func (knn *KNN) Fit(trainSet, valSet *DataSet, config *FitConfig) Score {
	config = config.LoadDefaultIfNil()
	knn.ItemIndex = trainSet.ItemIndex
	base.Logger().Info("fit knn",
		zap.Any("params", knn.GetParams()),
		zap.Any("config", config))
	// init similarity
	knn.Similarity = make([]ConcurrentMap, trainSet.ItemCount())
	for i := range knn.Similarity {
		knn.Similarity[i] = NewConcurrentMap()
	}
	// sort item feedback
	for i := range trainSet.ItemFeedback {
		sort.Ints(trainSet.ItemFeedback[i])
	}
	// execute plan
	var items []int
	var sparseDataset bool
	if trainSet.Count()*trainSet.Count()/trainSet.UserCount()/trainSet.ItemCount() > trainSet.ItemCount() {
		sparseDataset = false
		items = base.RangeInt(trainSet.ItemCount())
	} else {
		sparseDataset = true
	}
	fitStart := time.Now()
	_ = base.Parallel(trainSet.ItemCount(), config.Jobs, func(_, itemIndex int) error {
		// compute similarity
		var neighbors []int
		if sparseDataset {
			neighborSet := set.NewIntSet()
			for _, userIndex := range trainSet.ItemFeedback[itemIndex] {
				neighborSet.Add(trainSet.UserFeedback[userIndex]...)
			}
			neighbors = neighborSet.List()
		} else {
			neighbors = items
		}
		for _, neighborId := range neighbors {
			if neighborId < itemIndex {
				var similarity float32
				switch knn.similarity {
				case model.SimilarityCosine:
					similarity = dot(trainSet.ItemFeedback[itemIndex], trainSet.ItemFeedback[neighborId])
					if similarity != 0 {
						similarity /= math32.Sqrt(float32(len(trainSet.ItemFeedback[itemIndex])))
						similarity /= math32.Sqrt(float32(len(trainSet.ItemFeedback[neighborId])))
					}
				case model.SimilarityDot:
					similarity = dot(trainSet.ItemFeedback[itemIndex], trainSet.ItemFeedback[neighborId])
				default:
					panic("invalid similarity")
				}
				if similarity != 0 {
					knn.Similarity[itemIndex].Set(neighborId, similarity)
					knn.Similarity[neighborId].Set(itemIndex, similarity)
				}
			}
		}
		return nil
	})
	fitTime := time.Since(fitStart)
	evalStart := time.Now()
	scores := Evaluate(knn, valSet, trainSet, config.TopK, config.Candidates, config.Jobs, NDCG, Precision, Recall)
	evalTime := time.Since(evalStart)
	base.Logger().Info("fit knn complete",
		zap.Float32(fmt.Sprintf("NDCG@%v", config.TopK), scores[0]),
		zap.Float32(fmt.Sprintf("Precision@%v", config.TopK), scores[1]),
		zap.Float32(fmt.Sprintf("Recall@%v", config.TopK), scores[2]),
		zap.String("fit_time", fitTime.String()),
		zap.String("eval_time", evalTime.String()))
	return Score{NDCG: scores[0], Precision: scores[1], Recall: scores[2]}
}

func dot(a, b []int) float32 {
	i, j, sum := 0, 0, float32(0)
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			sum += 1
			i++
			j++
		} else if a[i] < b[j] {
			i++
		} else if a[i] > b[j] {
			j++
		}
	}
	return sum
}

type ConcurrentMap struct {
	Map   map[int]float32
	mutex sync.RWMutex
}

func NewConcurrentMap() ConcurrentMap {
	return ConcurrentMap{
		Map: make(map[int]float32),
	}
}

func (m *ConcurrentMap) Set(i int, v float32) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.Map[i] = v
}

func (m *ConcurrentMap) Get(i int) float32 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if v, ok := m.Map[i]; ok {
		return v
	}
	return 0
}
