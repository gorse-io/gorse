package pr

import (
	"fmt"
	"github.com/chewxy/math32"
	"github.com/scylladb/go-set"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"go.uber.org/zap"
	"sort"
	"sync"
	"time"
)

type KNN interface {
	Model
	// Predict the rating given by a user (userId) to a item (itemId).
	Predict(userProfile []string, itemId string) float32
	// InternalPredict predicts rating given by a user index and a item index
	InternalPredict(userProfile []int, itemIndex int) float32
}

type BaseKNN struct {
	model.BaseModel
	similarity string
	ItemIndex  base.Index
	Similarity []ConcurrentMap
}

func (knn *BaseKNN) SetParams(params model.Params) {
	knn.BaseModel.SetParams(params)
	knn.similarity = params.GetString(model.Similarity, model.SimilarityCosine)
}

func (knn *BaseKNN) GetParamsGrid() model.ParamsGrid {
	return model.ParamsGrid{
		model.Similarity: []interface{}{model.SimilarityCosine, model.SimilarityDot},
	}
}

func (knn *BaseKNN) Clear() {
	// do nothing
}

func (knn *BaseKNN) Predict(userProfile []string, itemId string) float32 {
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

func (knn *BaseKNN) InternalPredict(userProfile []int, itemIndex int) float32 {
	sum := float32(0)
	for _, supportIndex := range userProfile {
		sum += knn.Similarity[supportIndex].Get(itemIndex)
	}
	return sum
}

func (knn *BaseKNN) GetItemIndex() base.Index {
	return knn.ItemIndex
}

type CollaborativeKNN struct {
	BaseKNN
}

func NewCollaborativeKNN(params model.Params) *CollaborativeKNN {
	knn := new(CollaborativeKNN)
	knn.SetParams(params)
	return knn
}

func (knn *CollaborativeKNN) Fit(trainSet *DataSet, valSet *DataSet, config *FitConfig) Score {
	config = config.LoadDefaultIfNil()
	knn.ItemIndex = trainSet.ItemIndex
	base.Logger().Info("fit collaborative knn",
		zap.String("similarity", knn.similarity),
		zap.Int("n_users", trainSet.UserCount()),
		zap.Int("n_items", trainSet.ItemCount()),
		zap.Int("n_feedback", trainSet.Count()),
		zap.Int("n_jobs", config.Jobs),
		zap.Int("n_candidates", config.Candidates))
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
	base.Logger().Info("eval collaborative knn",
		zap.String("fit_time", fitTime.String()),
		zap.String("eval_time", evalTime.String()),
		zap.Float32(fmt.Sprintf("NDCG@%v", config.TopK), scores[0]),
		zap.Float32(fmt.Sprintf("Precision@%v", config.TopK), scores[1]),
		zap.Float32(fmt.Sprintf("Recall@%v", config.TopK), scores[2]))
	return Score{NDCG: scores[0], Precision: scores[1], Recall: scores[2]}
}

type ContentKNN struct {
	BaseKNN
}

func NewContentKNN(params model.Params) *ContentKNN {
	knn := new(ContentKNN)
	knn.SetParams(params)
	return knn
}

func (knn *ContentKNN) Fit(trainSet *DataSet, valSet *DataSet, config *FitConfig) Score {
	config = config.LoadDefaultIfNil()
	knn.ItemIndex = trainSet.ItemIndex
	base.Logger().Info("fit content knn",
		zap.String("similarity", knn.similarity),
		zap.Int("n_users", trainSet.UserCount()),
		zap.Int("n_items", trainSet.ItemCount()),
		zap.Int("n_feedback", trainSet.Count()),
		zap.Int("n_jobs", config.Jobs),
		zap.Int("n_candidates", config.Candidates))
	// init similarity
	knn.Similarity = make([]ConcurrentMap, trainSet.ItemCount())
	for i := range knn.Similarity {
		knn.Similarity[i] = NewConcurrentMap()
	}
	// sort item labels and build reverse index
	labelPairCount := 0
	labelItems := base.NewMatrixInt(trainSet.NumItemLabels, 0)
	for itemIndex := range trainSet.ItemLabels {
		sort.Ints(trainSet.ItemLabels[itemIndex])
		for _, labelIndex := range trainSet.ItemLabels[itemIndex] {
			labelPairCount++
			labelItems[labelIndex] = append(labelItems[labelIndex], itemIndex)
		}
	}
	// execute plan
	var items []int
	var sparseDataset bool
	if labelPairCount*labelPairCount/trainSet.NumItemLabels/trainSet.ItemCount() > trainSet.ItemCount() {
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
			for _, labelIndex := range trainSet.ItemLabels[itemIndex] {
				neighborSet.Add(labelItems[labelIndex]...)
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
					similarity = dot(trainSet.ItemLabels[itemIndex], trainSet.ItemLabels[neighborId])
					if similarity != 0 {
						similarity /= math32.Sqrt(float32(len(trainSet.ItemLabels[itemIndex])))
						similarity /= math32.Sqrt(float32(len(trainSet.ItemLabels[neighborId])))
					}
				case model.SimilarityDot:
					similarity = dot(trainSet.ItemLabels[itemIndex], trainSet.ItemLabels[neighborId])
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
	base.Logger().Info("eval content knn",
		zap.String("fit_time", fitTime.String()),
		zap.String("eval_time", evalTime.String()),
		zap.Float32(fmt.Sprintf("NDCG@%v", config.TopK), scores[0]),
		zap.Float32(fmt.Sprintf("Precision@%v", config.TopK), scores[1]),
		zap.Float32(fmt.Sprintf("Recall@%v", config.TopK), scores[2]))
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
