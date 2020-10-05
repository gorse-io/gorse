// Copyright 2020 Zhenghao Zhang
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
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/floats"
	"math"
	"math/rand"
)

// Evaluator is the evaluator for cross-validation.
type Evaluator func(estimator ModelInterface, testSet, trainSet DataSetInterface) []float64

// NewEvaluator creates a evaluator for personalized ranking cross-validation.
func NewEvaluator(n int, numCandidates int, metrics ...Scorer) Evaluator {
	return func(model ModelInterface, testSet DataSetInterface, trainSet DataSetInterface) []float64 {
		return EvaluateRank(model, testSet, trainSet, n, numCandidates, metrics...)
	}
}

/* Evaluate Item Ranking */

// Scorer is used by evaluators in personalized ranking tasks.
type Scorer func(targetSet *base.MarginalSubSet, rankList []string) float64

// EvaluateRank evaluates a model in top-n tasks.
func EvaluateRank(estimator ModelInterface, testSet DataSetInterface, excludeSet DataSetInterface, n int, numCandidates int, scorers ...Scorer) []float64 {
	sum := make([]float64, len(scorers))
	count := 0.0
	items := Items(testSet, excludeSet)
	// Convert to slice
	var itemIds []string
	if numCandidates > 0 {
		itemIds = make([]string, 0, len(items))
		for itemId := range items {
			itemIds = append(itemIds, itemId)
		}
	}
	// For all users
	for userIndex := 0; userIndex < testSet.UserCount(); userIndex++ {
		userId := testSet.UserIndexer().ToID(userIndex)
		// Find top-n items in test set
		targetSet := testSet.UserByIndex(userIndex)
		if targetSet.Len() > 0 {
			if numCandidates > 0 {
				// Sample items
				items = make(map[string]bool)
				targetSet.ForEach(func(i int, id string, value float64) {
					items[id] = true
				})
				exclude := excludeSet.User(userId)
				for len(items) < numCandidates+n {
					index := rand.Intn(len(itemIds))
					itemId := itemIds[index]
					if _, isSampled := items[itemId]; !isSampled && !exclude.Contain(itemId) {
						items[itemId] = true
					}
				}
			}
			// Find top-n items in predictions
			rankList, _ := Top(items, userId, n, excludeSet.User(userId), estimator)
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
func NDCG(targetSet *base.MarginalSubSet, rankList []string) float64 {
	// IDCG = \sum^{|REL|}_{i=1} \frac {1} {\log_2(i+1)}
	idcg := 0.0
	for i := 0; i < targetSet.Len() && i < len(rankList); i++ {
		idcg += 1.0 / math.Log2(float64(i)+2.0)
	}
	// DCG = \sum^{N}_{i=1} \frac {2^{rel_i}-1} {\log_2(i+1)}
	dcg := 0.0
	for i, itemId := range rankList {
		if targetSet.Contain(itemId) {
			dcg += 1.0 / math.Log2(float64(i)+2.0)
		}
	}
	return dcg / idcg
}

// Precision is the fraction of relevant items among the recommended items.
//   \frac{|relevant documents| \cap |retrieved documents|} {|{retrieved documents}|}
func Precision(targetSet *base.MarginalSubSet, rankList []string) float64 {
	hit := 0.0
	for _, itemId := range rankList {
		if targetSet.Contain(itemId) {
			hit++
		}
	}
	return float64(hit) / float64(len(rankList))
}

// Recall is the fraction of relevant items that have been recommended over the total
// amount of relevant items.
//   \frac{|relevant documents| \cap |retrieved documents|} {|{relevant documents}|}
func Recall(targetSet *base.MarginalSubSet, rankList []string) float64 {
	hit := 0
	for _, itemId := range rankList {
		if targetSet.Contain(itemId) {
			hit++
		}
	}
	return float64(hit) / float64(targetSet.Len())
}

// HR means Hit Ratio.
func HR(targetSet *base.MarginalSubSet, rankList []string) float64 {
	for _, itemId := range rankList {
		if targetSet.Contain(itemId) {
			return 1
		}
	}
	return 0
}

// MAP means Mean Average Precision.
// mAP: http://sdsawtelle.github.io/blog/output/mean-average-precision-MAP-for-recommender-systems.html
func MAP(targetSet *base.MarginalSubSet, rankList []string) float64 {
	sumPrecision := 0.0
	hit := 0
	for i, itemId := range rankList {
		if targetSet.Contain(itemId) {
			hit++
			sumPrecision += float64(hit) / float64(i+1)
		}
	}
	return float64(sumPrecision) / float64(targetSet.Len())
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
func MRR(targetSet *base.MarginalSubSet, rankList []string) float64 {
	for i, itemId := range rankList {
		if targetSet.Contain(itemId) {
			return 1 / float64(i+1)
		}
	}
	return 0
}

// EvaluateAUC evaluates a model by AUC.
func EvaluateAUC(estimator ModelInterface, testSet, excludeSet DataSetInterface) float64 {
	sum := 0.0
	userCount := 0.0
	// Find all userIds
	for userTestIndex, userRating := range testSet.Users() {
		if userRating.Len() > 0 {
			userCount++
			userId := testSet.UserIndexer().ToID(userTestIndex)
			// Find all <userId, j>s in training Data set and test Data set.
			positiveSet := make(map[string]float64)
			if excludeSet != nil {
				userExcludeIndex := excludeSet.UserIndexer().ToIndex(userId)
				if userExcludeIndex != base.NotId {
					excludeSet.UserByIndex(userExcludeIndex).ForEachIndex(func(i, index int, value float64) {
						itemId := excludeSet.ItemIndexer().ToID(index)
						positiveSet[itemId] = value
					})
				}
			}
			testSet.UserByIndex(userTestIndex).ForEachIndex(func(i, index int, value float64) {
				itemId := testSet.ItemIndexer().ToID(index)
				positiveSet[itemId] = value
			})
			// Generate scores for all items
			predictions := make([]float64, testSet.ItemCount())
			for itemTestIndex := range predictions {
				itemId := testSet.ItemIndexer().ToID(itemTestIndex)
				predictions[itemTestIndex] = estimator.Predict(userId, itemId)
			}
			// Find all <userId, i>s in test Data set
			correctCount, pairCount := 0.0, 0.0
			userRating.ForEachIndex(func(i, posTestIndex int, value float64) {
				// Find all <userId, j>s not in full Data set
				for j := 0; j < testSet.ItemCount(); j++ {
					negId := testSet.ItemIndexer().ToID(j)
					if _, exist := positiveSet[negId]; !exist {
						// I(\hat{x}_{ui} - \hat{x}_{uj})
						if predictions[posTestIndex] > predictions[j] {
							correctCount++
						}
						pairCount++
					}
				}
			})
			// += \frac{1}{|E(u)|} \sum_{(i,j)\in{E(u)}} I(\hat{x}_{ui} - \hat{x}_{uj})
			sum += correctCount / pairCount
		}
	}
	// \frac{1}{|U|} \sum_u \frac{1}{|E(u)|} \sum_{(i,j)\in{E(u)}} I(\hat{x}_{ui} - \hat{x}_{uj})
	return sum / userCount
}

// Top gets the ranking
func Top(items map[string]bool, userId string, n int, exclude *base.MarginalSubSet, model ModelInterface) ([]string, []float64) {
	// Get top-n list
	itemsHeap := base.NewMaxHeap(n)
	for itemId := range items {
		if !exclude.Contain(itemId) {
			itemsHeap.Add(itemId, model.Predict(userId, itemId))
		}
	}
	elem, scores := itemsHeap.ToSorted()
	recommends := make([]string, len(elem))
	for i := range recommends {
		recommends[i] = elem[i].(string)
	}
	return recommends, scores
}

// Items gets all items from the test set and the training set.
func Items(dataSet ...DataSetInterface) map[string]bool {
	items := make(map[string]bool)
	for _, data := range dataSet {
		for i := 0; i < data.ItemCount(); i++ {
			itemId := data.ItemIndexer().ToID(i)
			items[itemId] = true
		}
	}
	return items
}
