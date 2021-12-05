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
	"github.com/chewxy/math32"
	"github.com/scylladb/go-set"
	"github.com/scylladb/go-set/i32set"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/copier"
	"github.com/zhenghaoz/gorse/floats"
)

/* Evaluate Item Ranking */

// Metric is used by evaluators in personalized ranking tasks.
type Metric func(targetSet *i32set.Set, rankList []int32) float32

// Evaluate evaluates a model in top-n tasks.
func Evaluate(estimator MatrixFactorization, testSet, trainSet *DataSet, topK, numCandidates, nJobs int, scorers ...Metric) []float32 {
	partSum := make([][]float32, nJobs)
	partCount := make([]float32, nJobs)
	for i := 0; i < nJobs; i++ {
		partSum[i] = make([]float32, len(scorers))
	}
	//rng := NewRandomGenerator(0)
	// For all UserFeedback
	negatives := testSet.NegativeSample(trainSet, numCandidates)
	_ = base.Parallel(testSet.UserCount(), nJobs, func(workerId, userIndex int) error {
		// Find top-n ItemFeedback in test set
		targetSet := set.NewInt32Set(testSet.UserFeedback[userIndex]...)
		if targetSet.Size() > 0 {
			// Sample negative samples
			//userTrainSet := NewSet(trainSet.UserFeedback[userIndex])
			negativeSample := negatives[userIndex]
			candidates := make([]int32, 0, targetSet.Size()+len(negativeSample))
			candidates = append(candidates, testSet.UserFeedback[userIndex]...)
			candidates = append(candidates, negativeSample...)
			// Find top-n ItemFeedback in predictions
			rankList, _ := Rank(estimator, int32(userIndex), candidates, topK)
			partCount[workerId]++
			for i, metric := range scorers {
				partSum[workerId][i] += metric(targetSet, rankList)
			}
		}
		return nil
	})
	sum := make([]float32, len(scorers))
	for i := 0; i < nJobs; i++ {
		for j := range partSum[i] {
			sum[j] += partSum[i][j]
		}
	}
	count := floats.Sum(partCount)
	floats.MulConst(sum, 1/count)
	return sum
}

// NDCG means Normalized Discounted Cumulative Gain.
func NDCG(targetSet *i32set.Set, rankList []int32) float32 {
	// IDCG = \sum^{|REL|}_{i=1} \frac {1} {\log_2(i+1)}
	idcg := float32(0)
	for i := 0; i < targetSet.Size() && i < len(rankList); i++ {
		idcg += 1.0 / math32.Log2(float32(i)+2.0)
	}
	// DCG = \sum^{N}_{i=1} \frac {2^{rel_i}-1} {\log_2(i+1)}
	dcg := float32(0)
	for i, itemId := range rankList {
		if targetSet.Has(itemId) {
			dcg += 1.0 / math32.Log2(float32(i)+2.0)
		}
	}
	return dcg / idcg
}

// Precision is the fraction of relevant ItemFeedback among the recommended ItemFeedback.
//   \frac{|relevant documents| \cap |retrieved documents|} {|{retrieved documents}|}
func Precision(targetSet *i32set.Set, rankList []int32) float32 {
	hit := float32(0)
	for _, itemId := range rankList {
		if targetSet.Has(itemId) {
			hit++
		}
	}
	return hit / float32(len(rankList))
}

// Recall is the fraction of relevant ItemFeedback that have been recommended over the total
// amount of relevant ItemFeedback.
//   \frac{|relevant documents| \cap |retrieved documents|} {|{relevant documents}|}
func Recall(targetSet *i32set.Set, rankList []int32) float32 {
	hit := 0
	for _, itemId := range rankList {
		if targetSet.Has(itemId) {
			hit++
		}
	}
	return float32(hit) / float32(targetSet.Size())
}

// HR means Hit Ratio.
func HR(targetSet *i32set.Set, rankList []int32) float32 {
	for _, itemId := range rankList {
		if targetSet.Has(itemId) {
			return 1
		}
	}
	return 0
}

// MAP means Mean Average Precision.
// mAP: http://sdsawtelle.github.io/blog/output/mean-average-precision-MAP-for-recommender-systems.html
func MAP(targetSet *i32set.Set, rankList []int32) float32 {
	sumPrecision := float32(0)
	hit := 0
	for i, itemId := range rankList {
		if targetSet.Has(itemId) {
			hit++
			sumPrecision += float32(hit) / float32(i+1)
		}
	}
	return sumPrecision / float32(targetSet.Size())
}

// MRR means Mean Reciprocal Rank.
//
// The mean reciprocal rank is a statistic measure for evaluating any process
// that produces a list of possible responses to a sample of queries, ordered
// by probability of correctness. The reciprocal rank of a query response is
// the multiplicative inverse of the rank of the first correct answer: 1 for
// first place, ​1⁄2 for second place, ​1⁄3 for third place and so on. The
// mean reciprocal rank is the average of the reciprocal ranks of results for
// a sample of queries Q:
//
//   MRR = \frac{1}{Q} \sum^{|Q|}_{i=1} \frac{1}{rank_i}
func MRR(targetSet *i32set.Set, rankList []int32) float32 {
	for i, itemId := range rankList {
		if targetSet.Has(itemId) {
			return 1 / float32(i+1)
		}
	}
	return 0
}

func Rank(model MatrixFactorization, userId int32, candidates []int32, topN int) ([]int32, []float32) {
	// Get top-n list
	itemsHeap := base.NewTopKFilter(topN)
	for _, itemId := range candidates {
		itemsHeap.Push(itemId, model.InternalPredict(userId, itemId))
	}
	elem, scores := itemsHeap.PopAll()
	recommends := make([]int32, len(elem))
	for i := range recommends {
		recommends[i] = elem[i]
	}
	return recommends, scores
}

// SnapshotManger manages the best snapshot.
type SnapshotManger struct {
	BestWeights []interface{}
	BestScore   Score
}

// AddSnapshot adds a copied snapshot.
func (sm *SnapshotManger) AddSnapshot(score Score, weights ...interface{}) {
	if sm.BestWeights == nil || score.NDCG > sm.BestScore.NDCG {
		sm.BestScore = score
		if err := copier.Copy(&sm.BestWeights, weights); err != nil {
			panic(err)
		}
	}
}

// AddSnapshotNoCopy adds a snapshot without copy.
func (sm *SnapshotManger) AddSnapshotNoCopy(score Score, weights ...interface{}) {
	if sm.BestWeights == nil || score.NDCG > sm.BestScore.NDCG {
		sm.BestScore = score
		if err := copier.Copy(&sm.BestWeights, weights); err != nil {
			panic(err)
		}
	}
}
