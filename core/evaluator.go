package core

import (
	"gonum.org/v1/gonum/floats"
	"math"
)

// Evaluator evaluates the performance of a estimator on the test set.
type Evaluator func(Model, DataSet) float64

// RMSE is root mean square error.
func RMSE(estimator Model, testSet DataSet) float64 {
	sum := 0.0
	for j := 0; j < testSet.Len(); j++ {
		userId, itemId, rating := testSet.Get(j)
		prediction := estimator.Predict(userId, itemId)
		sum += (prediction - rating) * (prediction - rating)
	}
	return math.Sqrt(sum / float64(testSet.Len()))
}

// MAE is mean absolute error.
func MAE(estimator Model, testSet DataSet) float64 {
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
	return func(estimator Model, testSet DataSet) float64 {
		full := NewTrainSet(fullSet)
		test := NewTrainSet(testSet)
		sum, count := 0.0, 0.0
		// Find all userIds
		for innerUserIdTest, irs := range test.UserRatings {
			userId := test.UserIdSet.ToSparseId(innerUserIdTest)
			// Find all <userId, j>s in full data set
			denseUserIdFull := full.UserIdSet.ToDenseId(userId)
			fullRatedItem := make(map[int]float64)
			full.UserRatings[denseUserIdFull].ForEach(func(i, index int, value float64) {
				itemId := full.ItemIdSet.ToSparseId(index)
				fullRatedItem[itemId] = value
			})
			// Find all <userId, i>s in test data set
			indicatorSum, ratedCount := 0.0, 0.0
			irs.ForEach(func(i, index int, value float64) {
				iItemId := test.ItemIdSet.ToSparseId(index)
				// Find all <userId, j>s not in full data set
				for j := 0; j < full.ItemCount(); j++ {
					jItemId := full.ItemIdSet.ToSparseId(j)
					if _, exist := fullRatedItem[jItemId]; !exist {
						// I(\hat{x}_{ui} - \hat{x}_{uj})
						if estimator.Predict(userId, iItemId) > estimator.Predict(userId, jItemId) {
							indicatorSum++
						}
						ratedCount++
					}
				}
			})
			// += \frac{1}{|E(u)|} \sum_{(i,j)\in{E(u)}} I(\hat{x}_{ui} - \hat{x}_{uj})
			sum += indicatorSum / ratedCount
			count++
		}
		// \frac{1}{|U|} \sum_u \frac{1}{|E(u)|} \sum_{(i,j)\in{E(u)}} I(\hat{x}_{ui} - \hat{x}_{uj})
		return sum / count
	}
}

// NewNDCG creates a Normalized Discounted Cumulative Gain evaluator.
func NewNDCG(n int) Evaluator {
	return func(model Model, data DataSet) float64 {
		testSet := NewTrainSet(data)
		sum := 0.0
		// For all users
		for innerUserIdTest, irs := range testSet.UserRatings {
			userId := testSet.UserIdSet.ToSparseId(innerUserIdTest)
			// Find top-n items in test set
			relSet := make(map[int]float64)
			relIndex := make([]int, irs.Len())
			relRating := make([]float64, irs.Len())
			irs.ForEach(func(i, index int, value float64) {
				relIndex[i] = i
				relRating[i] = -value
			})
			floats.Argsort(relRating, relIndex)
			for i := 0; i < n && i < irs.Len(); i++ {
				index := relIndex[i]
				relSet[irs.Indices[index]] = irs.Values[index]
			}
			// Find top-n items in predictions
			topIndex := make([]int, irs.Len())
			topRating := make([]float64, irs.Len())
			irs.ForEach(func(i, index int, value float64) {
				itemId := testSet.ItemIdSet.ToSparseId(index)
				topIndex[i] = i
				topRating[i] = -model.Predict(userId, itemId)
			})
			floats.Argsort(topRating, topIndex)
			// IDCG = \sum^{|REL|}_{i=1} \frac {1} {\log_2(i+1)}
			idcg := 0.0
			for i := 0; i < n && i < irs.Len(); i++ {
				idcg += 1.0 / math.Log2(float64(i)+2.0)
			}
			// DCG = \sum^{N}_{i=1} \frac {2^{rel_i}-1} {\log_2(i+1)}
			dcg := 0.0
			for i := 0; i < n && i < irs.Len(); i++ {
				index := topIndex[i]
				if _, exist := relSet[irs.Indices[index]]; exist {
					dcg += 1.0 / math.Log2(float64(i)+2.0)
				}
			}
			// NDCG = DCG / IDCG
			sum += dcg / idcg
		}
		return sum / float64(testSet.UserCount())
	}
}

func NewPrecision(n int) Evaluator {

}

func NewRecall(n int) Evaluator {

}

func NewMAP() Evaluator {

}

func NewMRR() Evaluator {

}
