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
	"github.com/zhenghaoz/gorse/floats"
)

/* Evaluate Item Ranking */

// Metric is used by evaluators in personalized ranking tasks.
type Metric func(targetSet base.Set, rankList []int) float32

// Evaluate evaluates a model in top-n tasks.
func Evaluate(estimator Model, testSet *DataSet, trainSet *DataSet, n int, numCandidates int, scorers ...Metric) []float32 {
	sum := make([]float32, len(scorers))
	count := float32(0.0)
	//rng := NewRandomGenerator(0)
	// For all UserFeedback
	for userIndex := 0; userIndex < testSet.UserCount(); userIndex++ {
		// Find top-n ItemFeedback in test set
		targetSet := base.NewSet(testSet.UserFeedback[userIndex])
		if targetSet.Len() > 0 {
			// Sample negative samples
			//userTrainSet := NewSet(trainSet.UserFeedback[userIndex])
			negativeSample := testSet.NegativeSample(trainSet, numCandidates)[userIndex]
			candidates := make([]int, 0, targetSet.Len()+len(negativeSample))
			candidates = append(candidates, testSet.UserFeedback[userIndex]...)
			candidates = append(candidates, negativeSample...)
			// Find top-n ItemFeedback in predictions
			var rankList []int
			switch estimator.(type) {
			case MatrixFactorization:
				rankList, _ = RankMatrixFactorization(estimator.(MatrixFactorization), userIndex, candidates, n)
			case ItemBased:
				rankList, _ = RankItemBased(estimator.(ItemBased), trainSet.UserFeedback[userIndex], candidates, n)
			default:
				panic("unsupported algorithm")
			}
			count++
			for i, metric := range scorers {
				sum[i] += metric(targetSet, rankList)
			}
		}
	}
	floats.MulConst(sum, 1/count)
	return sum
}

// NDCG means Normalized Discounted Cumulative Gain.
func NDCG(targetSet base.Set, rankList []int) float32 {
	// IDCG = \sum^{|REL|}_{i=1} \frac {1} {\log_2(i+1)}
	idcg := float32(0)
	for i := 0; i < targetSet.Len() && i < len(rankList); i++ {
		idcg += 1.0 / math32.Log2(float32(i)+2.0)
	}
	// DCG = \sum^{N}_{i=1} \frac {2^{rel_i}-1} {\log_2(i+1)}
	dcg := float32(0)
	for i, itemId := range rankList {
		if targetSet.Contain(itemId) {
			dcg += 1.0 / math32.Log2(float32(i)+2.0)
		}
	}
	return dcg / idcg
}

// Precision is the fraction of relevant ItemFeedback among the recommended ItemFeedback.
//   \frac{|relevant documents| \cap |retrieved documents|} {|{retrieved documents}|}
func Precision(targetSet base.Set, rankList []int) float32 {
	hit := float32(0)
	for _, itemId := range rankList {
		if targetSet.Contain(itemId) {
			hit++
		}
	}
	return hit / float32(len(rankList))
}

// Recall is the fraction of relevant ItemFeedback that have been recommended over the total
// amount of relevant ItemFeedback.
//   \frac{|relevant documents| \cap |retrieved documents|} {|{relevant documents}|}
func Recall(targetSet base.Set, rankList []int) float32 {
	hit := 0
	for _, itemId := range rankList {
		if targetSet.Contain(itemId) {
			hit++
		}
	}
	return float32(hit) / float32(targetSet.Len())
}

// HR means Hit Ratio.
func HR(targetSet base.Set, rankList []int) float32 {
	for _, itemId := range rankList {
		if targetSet.Contain(itemId) {
			return 1
		}
	}
	return 0
}

// MAP means Mean Average Precision.
// mAP: http://sdsawtelle.github.io/blog/output/mean-average-precision-MAP-for-recommender-systems.html
func MAP(targetSet base.Set, rankList []int) float32 {
	sumPrecision := float32(0)
	hit := 0
	for i, itemId := range rankList {
		if targetSet.Contain(itemId) {
			hit++
			sumPrecision += float32(hit) / float32(i+1)
		}
	}
	return sumPrecision / float32(targetSet.Len())
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
func MRR(targetSet base.Set, rankList []int) float32 {
	for i, itemId := range rankList {
		if targetSet.Contain(itemId) {
			return 1 / float32(i+1)
		}
	}
	return 0
}

// RankMatrixFactorization gets the ranking
func RankMatrixFactorization(model MatrixFactorization, userId int, candidates []int, topN int) ([]int, []float32) {
	// Get top-n list
	itemsHeap := base.NewMaxHeap(topN)
	for _, itemId := range candidates {
		itemsHeap.Add(itemId, model.InternalPredict(userId, itemId))
	}
	elem, scores := itemsHeap.ToSorted()
	recommends := make([]int, len(elem))
	for i := range recommends {
		recommends[i] = elem[i].(int)
	}
	return recommends, scores
}

func RankItemBased(model ItemBased, supportItems []int, candidates []int, topN int) ([]int, []float32) {
	// Get top-n list
	itemsHeap := base.NewMaxHeap(topN)
	for _, itemId := range candidates {
		itemsHeap.Add(itemId, model.InternalPredict(supportItems, itemId))
	}
	elem, scores := itemsHeap.ToSorted()
	recommends := make([]int, len(elem))
	for i := range recommends {
		recommends[i] = elem[i].(int)
	}
	return recommends, scores
}
