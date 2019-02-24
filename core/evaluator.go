package core

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/floats"
	"math"
)

type CVEvaluator func(estimator Model, testSet *DataSet, trainSet *DataSet) []float64

func NewRatingEvaluator(metrics ...RatingMetric) CVEvaluator {
	return func(model Model, testSet *DataSet, trainSet *DataSet) []float64 {
		return EvaluateRating(model, testSet, metrics...)
	}
}

func NewRankEvaluator(n int, metrics ...RankMetric) CVEvaluator {
	return func(model Model, testSet *DataSet, trainSet *DataSet) []float64 {
		return EvaluateRank(model, testSet, trainSet, n, metrics...)
	}
}

/* Evaluate Rating Prediction */

// RatingMetric is used by evaluators in rating prediction tasks.
type RatingMetric func(groundTruth []float64, prediction []float64) float64

// EvaluateRating evaluates a model in rating prediction tasks.
func EvaluateRating(estimator Model, testSet *DataSet, metrics ...RatingMetric) []float64 {
	groundTruth := make([]float64, testSet.Len())
	predictions := make([]float64, testSet.Len())
	scores := make([]float64, len(metrics))
	for j := 0; j < testSet.Len(); j++ {
		userId, itemId, rating := testSet.Get(j)
		groundTruth[j] = rating
		predictions[j] = estimator.Predict(userId, itemId)
	}
	for i, metric := range metrics {
		scores[i] = metric(groundTruth, predictions)
	}
	return scores
}

// RMSE is root mean square error.
func RMSE(groundTruth []float64, prediction []float64) float64 {
	sum := 0.0
	for j := 0; j < len(groundTruth); j++ {
		sum += (prediction[j] - groundTruth[j]) * (prediction[j] - groundTruth[j])
	}
	return math.Sqrt(sum / float64(len(groundTruth)))
}

// MAE is mean absolute error.
func MAE(groundTruth []float64, prediction []float64) float64 {
	sum := 0.0
	for j := 0; j < len(groundTruth); j++ {
		sum += math.Abs(prediction[j] - groundTruth[j])
	}
	return sum / float64(len(groundTruth))
}

/* Evaluate Item Ranking */

// RatingMetric is used by evaluators in rating prediction tasks.
type RankMetric func(targetSet map[int]float64, rankList []int) float64

// EvaluateRank evaluates a model in top-n tasks.
func EvaluateRank(estimator Model, testSet *DataSet, excludeSet *DataSet, n int, metrics ...RankMetric) []float64 {
	sum := make([]float64, len(metrics))
	items := Items(testSet, excludeSet)
	// For all users
	for denseUserId := 0; denseUserId < testSet.UserCount(); denseUserId++ {
		userId := testSet.UserIdSet.ToSparseId(denseUserId)
		// Find top-n items in test set
		targetSet := testSet.GetUserRatingsSet(userId)
		// Find top-n items in predictions
		rankList := Top(items, userId, n, excludeSet.GetUserRatingsSet(userId), estimator)
		// MRR
		for i, metric := range metrics {
			sum[i] += metric(targetSet, rankList)
		}
	}
	floats.MulConst(sum, 1/float64(testSet.UserCount()))
	return sum
}

// NDCG means Normalized Discounted Cumulative Gain.
func NDCG(targetSet map[int]float64, rankList []int) float64 {
	// IDCG = \sum^{|REL|}_{i=1} \frac {1} {\log_2(i+1)}
	idcg := 0.0
	for i := 0; i < len(targetSet) && i < len(rankList); i++ {
		idcg += 1.0 / math.Log2(float64(i)+2.0)
	}
	// DCG = \sum^{N}_{i=1} \frac {2^{rel_i}-1} {\log_2(i+1)}
	dcg := 0.0
	for i, itemId := range rankList {
		if _, exist := targetSet[itemId]; exist {
			dcg += 1.0 / math.Log2(float64(i)+2.0)
		}
	}
	return dcg / idcg
}

// Precision:
//   \frac{|relevant documents| \cap |retrieved documents|} {|{retrieved documents}|}
func Precision(targetSet map[int]float64, rankList []int) float64 {
	hit := 0
	for _, itemId := range rankList {
		if _, exist := targetSet[itemId]; exist {
			hit++
		}
	}
	return float64(hit) / float64(len(rankList))
}

// Recall:
//   \frac{|relevant documents| \cap |retrieved documents|} {|{relevant documents}|}
func Recall(targetSet map[int]float64, rankList []int) float64 {
	hit := 0
	for _, itemId := range rankList {
		if _, exist := targetSet[itemId]; exist {
			hit++
		}
	}
	return float64(hit) / float64(len(targetSet))
}

// MAP means Mean Average Precision.
// mAP: http://sdsawtelle.github.io/blog/output/mean-average-precision-MAP-for-recommender-systems.html
func MAP(targetSet map[int]float64, rankList []int) float64 {
	sumPrecision := 0.0
	hit := 0
	for i, itemId := range rankList {
		if _, exist := targetSet[itemId]; exist {
			hit++
			sumPrecision += float64(hit) / float64(i+1)
		}
	}
	return float64(sumPrecision) / float64(len(targetSet))
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
func MRR(targetSet map[int]float64, rankList []int) float64 {
	for i, itemId := range rankList {
		if _, exist := targetSet[itemId]; exist {
			return 1 / float64(i+1)
		}
	}
	return 0
}

// AUC evaluator.
func AUC(estimator Model, testSet *DataSet, excludeSet *DataSet) float64 {
	sum := 0.0
	// Find all userIds
	for denseUserIdInTest, userRating := range testSet.DenseUserRatings {
		userId := testSet.UserIdSet.ToSparseId(denseUserIdInTest)
		// Find all <userId, j>s in training Data set and test Data set.
		positiveSet := make(map[int]float64)
		if excludeSet != nil {
			denseUserIdInTrain := excludeSet.UserIdSet.ToDenseId(userId)
			if denseUserIdInTrain != base.NotId {
				excludeSet.DenseUserRatings[denseUserIdInTrain].ForEach(func(i, index int, value float64) {
					itemId := excludeSet.ItemIdSet.ToSparseId(index)
					positiveSet[itemId] = value
				})
			}
		}
		testSet.DenseUserRatings[denseUserIdInTest].ForEach(func(i, index int, value float64) {
			itemId := testSet.ItemIdSet.ToSparseId(index)
			positiveSet[itemId] = value
		})
		// Find all <userId, i>s in test Data set
		correctCount, pairCount := 0.0, 0.0
		userRating.ForEach(func(i, index int, value float64) {
			posItemId := testSet.ItemIdSet.ToSparseId(index)
			// Find all <userId, j>s not in full Data set
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
	}
	// \frac{1}{|U|} \sum_u \frac{1}{|E(u)|} \sum_{(i,j)\in{E(u)}} I(\hat{x}_{ui} - \hat{x}_{uj})
	return sum / float64(testSet.UserCount())
}
