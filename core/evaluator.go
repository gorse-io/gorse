package core

import (
	"github.com/zhenghaoz/gorse/base"
	"math"
)

// Evaluator evaluates the performance of a estimator on the test set.
type Evaluator func(estimator Model, trainSet DataSet, testSet DataSet) float64

// RMSE is root mean square error.
func RMSE(estimator Model, _ DataSet, testSet DataSet) float64 {
	sum := 0.0
	for j := 0; j < testSet.Len(); j++ {
		userId, itemId, rating := testSet.Get(j)
		prediction := estimator.Predict(userId, itemId)
		sum += (prediction - rating) * (prediction - rating)
	}
	return math.Sqrt(sum / float64(testSet.Len()))
}

// MAE is mean absolute error.
func MAE(estimator Model, _ DataSet, testSet DataSet) float64 {
	sum := 0.0
	for j := 0; j < testSet.Len(); j++ {
		userId, itemId, rating := testSet.Get(j)
		prediction := estimator.Predict(userId, itemId)
		sum += math.Abs(prediction - rating)
	}
	return sum / float64(testSet.Len())
}

// AUC evaluator.
func AUC(estimator Model, trainSet DataSet, testSet DataSet) float64 {
	sum, count := 0.0, 0.0
	// Find all userIds
	for denseUserIdInTest, userRating := range testSet.DenseUserRatings {
		userId := testSet.UserIdSet.ToSparseId(denseUserIdInTest)
		// Find all <userId, j>s in training data set and test data set.
		denseUserIdInTrain := trainSet.UserIdSet.ToDenseId(userId)
		positiveSet := make(map[int]float64)
		if denseUserIdInTrain != base.NotId {
			trainSet.DenseUserRatings[denseUserIdInTrain].ForEach(func(i, index int, value float64) {
				itemId := trainSet.ItemIdSet.ToSparseId(index)
				positiveSet[itemId] = value
			})
		}
		testSet.DenseUserRatings[denseUserIdInTest].ForEach(func(i, index int, value float64) {
			itemId := testSet.ItemIdSet.ToSparseId(index)
			positiveSet[itemId] = value
		})
		// Find all <userId, i>s in test data set
		correctCount, pairCount := 0.0, 0.0
		userRating.ForEach(func(i, index int, value float64) {
			posItemId := testSet.ItemIdSet.ToSparseId(index)
			// Find all <userId, j>s not in full data set
			for j := 0; j < testSet.ItemCount(); j++ {
				negItemId := testSet.ItemIdSet.ToSparseId(j)
				if _, exist := positiveSet[negItemId]; !exist {
					// I(\hat{x}_{ui} - \hat{x}_{uj})
					if estimator.Predict(userId, posItemId) > estimator.Predict(userId, negItemId) {
						correctCount++
					}
					pairCount++
				}
			}
		})
		// += \frac{1}{|E(u)|} \sum_{(i,j)\in{E(u)}} I(\hat{x}_{ui} - \hat{x}_{uj})
		sum += correctCount / pairCount
		count++
	}
	// \frac{1}{|U|} \sum_u \frac{1}{|E(u)|} \sum_{(i,j)\in{E(u)}} I(\hat{x}_{ui} - \hat{x}_{uj})
	return sum / count
}

// NewNDCG creates a Normalized Discounted Cumulative Gain evaluator.
func NewNDCG(n int) Evaluator {
	return func(model Model, trainSet DataSet, testSet DataSet) float64 {
		sum := 0.0
		// For all users
		for u := 0; u < testSet.UserCount(); u++ {
			// Find top-n items in test set
			targetSet := GetRelevantSet(testSet, u)
			// Find top-n items in predictions
			rankList := Top(testSet, u, n, trainSet, model)
			// IDCG = \sum^{|REL|}_{i=1} \frac {1} {\log_2(i+1)}
			idcg := 0.0
			for i := 0; i < len(targetSet); i++ {
				if i < n {
					idcg += 1.0 / math.Log2(float64(i)+2.0)
				}
			}
			// DCG = \sum^{N}_{i=1} \frac {2^{rel_i}-1} {\log_2(i+1)}
			dcg := 0.0
			for i, itemId := range rankList {
				if _, exist := targetSet[itemId]; exist {
					dcg += 1.0 / math.Log2(float64(i)+2.0)
				}
			}
			// NDCG = DCG / IDCG
			sum += dcg / idcg
		}
		return sum / float64(testSet.UserCount())
	}
}

// NewPrecision creates a Precision@N evaluator.
//   Precision = \frac{|relevant documents| \cap |retrieved documents|}
//                    {|{retrieved documents}|}
func NewPrecision(n int) Evaluator {
	return func(model Model, trainSet DataSet, testSet DataSet) float64 {
		sum := 0.0
		// For all users
		for u := 0; u < testSet.UserCount(); u++ {
			// Find top-n items in test set
			targetSet := GetRelevantSet(testSet, u)
			// Find top-n items in predictions
			rankList := Top(testSet, u, n, trainSet, model)
			// Precision
			hit := 0
			for _, itemId := range rankList {
				if _, exist := targetSet[itemId]; exist {
					hit++
				}
			}
			sum += float64(hit) / float64(len(rankList))
		}
		return sum / float64(testSet.UserCount())
	}
}

// NewRecall creates a Recall@N evaluator.
//   Recall = \frac{|relevant documents| \cap |retrieved documents|}
//                 {|{relevant documents}|}
func NewRecall(n int) Evaluator {
	return func(model Model, trainSet DataSet, testSet DataSet) float64 {
		sum := 0.0
		// For all users
		for u := 0; u < testSet.UserCount(); u++ {
			// Find top-n items in test set
			targetSet := GetRelevantSet(testSet, u)
			// Find top-n items in predictions
			rankList := Top(testSet, u, n, trainSet, model)
			// Precision
			hit := 0
			for _, itemId := range rankList {
				if _, exist := targetSet[itemId]; exist {
					hit++
				}
			}
			sum += float64(hit) / float64(len(targetSet))
		}
		return sum / float64(testSet.UserCount())
	}
}

// NewMAP creates a mean average precision evaluator.
// mAP: http://sdsawtelle.github.io/blog/output/mean-average-precision-MAP-for-recommender-systems.html
func NewMAP(n int) Evaluator {
	return func(estimator Model, trainSet DataSet, testSet DataSet) float64 {
		sum := 0.0
		// For all users
		for u := 0; u < testSet.UserCount(); u++ {
			// Find top-n items in test set
			targetSet := GetRelevantSet(testSet, u)
			// Find top-n items in predictions
			rankList := Top(testSet, u, n, trainSet, estimator)
			// MAP
			sumPrecision := 0.0
			hit := 0
			for i, itemId := range rankList {
				if _, exist := targetSet[itemId]; exist {
					hit++
					sumPrecision += float64(hit) / float64(i+1)
				}
			}
			sum += float64(sumPrecision) / float64(len(targetSet))
		}
		return sum / float64(testSet.UserCount())
	}
}

// NewMRR creates a mean reciprocal rank evaluator.
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
func NewMRR(n int) Evaluator {
	return func(model Model, trainSet DataSet, testSet DataSet) float64 {
		sum := 0.0
		// For all users
		for u := 0; u < testSet.UserCount(); u++ {
			// Find top-n items in test set
			targetSet := GetRelevantSet(testSet, u)
			// Find top-n items in predictions
			rankList := Top(testSet, u, n, trainSet, model)
			// MRR
			for i, itemId := range rankList {
				if _, exist := targetSet[itemId]; exist {
					sum += 1 / float64(i+1)
					break
				}
			}
		}
		return sum / float64(testSet.UserCount())
	}
}
