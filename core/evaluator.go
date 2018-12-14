package core

import (
	"math"
)

// Evaluator evaluates the performance of a estimator on the test set.
type Evaluator func(Model, TrainSet, TrainSet) float64

// RMSE is root mean square error.
func RMSE(estimator Model, trainSet TrainSet, testSet TrainSet) float64 {
	sum := 0.0
	for j := 0; j < testSet.Len(); j++ {
		userId, itemId, rating := testSet.Get(j)
		prediction := estimator.Predict(userId, itemId)
		sum += (prediction - rating) * (prediction - rating)
	}
	return math.Sqrt(sum / float64(testSet.Len()))
}

// MAE is mean absolute error.
func MAE(estimator Model, trainSet TrainSet, testSet TrainSet) float64 {
	sum := 0.0
	for j := 0; j < testSet.Len(); j++ {
		userId, itemId, rating := testSet.Get(j)
		prediction := estimator.Predict(userId, itemId)
		sum += math.Abs(prediction - rating)
	}
	return sum / float64(testSet.Len())
}

// NewAUCEvaluator creates a AUC evaluator.
func NewAUCEvaluator(fullSet DataSet) Evaluator {
	return func(estimator Model, trainSet TrainSet, testSet TrainSet) float64 {
		return 0
		//sum, count := 0.0, 0.0
		//// Find all userIds
		//for denseUserIdOfTest, userRating := range testSet.UserRatings {
		//	userId := testSet.UserIdSet.ToSparseId(denseUserIdOfTest)
		//	// Find all <userId, j>s in full data set
		//	denseUserIdFull := full.UserIdSet.ToDenseId(userId)
		//	fullRatedItem := make(map[int]float64)
		//	full.UserRatings[denseUserIdFull].ForEach(func(i, index int, value float64) {
		//		itemId := full.ItemIdSet.ToSparseId(index)
		//		fullRatedItem[itemId] = value
		//	})
		//	// Find all <userId, i>s in test data set
		//	indicatorSum, ratedCount := 0.0, 0.0
		//	userRating.ForEach(func(i, index int, value float64) {
		//		iItemId := test.ItemIdSet.ToSparseId(index)
		//		// Find all <userId, j>s not in full data set
		//		for j := 0; j < full.ItemCount(); j++ {
		//			jItemId := full.ItemIdSet.ToSparseId(j)
		//			if _, exist := fullRatedItem[jItemId]; !exist {
		//				// I(\hat{x}_{ui} - \hat{x}_{uj})
		//				if estimator.Predict(userId, iItemId) > estimator.Predict(userId, jItemId) {
		//					indicatorSum++
		//				}
		//				ratedCount++
		//			}
		//		}
		//	})
		//	// += \frac{1}{|E(u)|} \sum_{(i,j)\in{E(u)}} I(\hat{x}_{ui} - \hat{x}_{uj})
		//	sum += indicatorSum / ratedCount
		//	count++
		//}
		//// \frac{1}{|U|} \sum_u \frac{1}{|E(u)|} \sum_{(i,j)\in{E(u)}} I(\hat{x}_{ui} - \hat{x}_{uj})
		//return sum / count
	}
}

// NewNDCG creates a Normalized Discounted Cumulative Gain evaluator.
func NewNDCG(n int) Evaluator {
	return func(model Model, trainSet TrainSet, testSet TrainSet) float64 {
		sum := 0.0
		// For all users
		for u := 0; u < testSet.UserCount(); u++ {
			// Find top-n items in test set
			targetSet := GetRelevantSet(testSet, u)
			// Find top-n items in predictions
			rankList := GetTopN(testSet, u, n, trainSet, model)
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
	return func(model Model, trainSet TrainSet, testSet TrainSet) float64 {
		sum := 0.0
		// For all users
		for u := 0; u < testSet.UserCount(); u++ {
			// Find top-n items in test set
			targetSet := GetRelevantSet(testSet, u)
			// Find top-n items in predictions
			rankList := GetTopN(testSet, u, n, trainSet, model)
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
	return func(model Model, trainSet TrainSet, testSet TrainSet) float64 {
		sum := 0.0
		// For all users
		for u := 0; u < testSet.UserCount(); u++ {
			// Find top-n items in test set
			targetSet := GetRelevantSet(testSet, u)
			// Find top-n items in predictions
			rankList := GetTopN(testSet, u, n, trainSet, model)
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
