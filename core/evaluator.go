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
	for j := 0; j < testSet.Length(); j++ {
		userId, itemId, rating := testSet.Index(j)
		prediction := estimator.Predict(userId, itemId)
		sum += (prediction - rating) * (prediction - rating)
	}
	return math.Sqrt(sum / float64(testSet.Length()))
}

// MAE is mean absolute error.
func MAE(estimator Model, testSet DataSet) float64 {
	sum := 0.0
	for j := 0; j < testSet.Length(); j++ {
		userId, itemId, rating := testSet.Index(j)
		prediction := estimator.Predict(userId, itemId)
		sum += math.Abs(prediction - rating)
	}
	return sum / float64(testSet.Length())
}

// NewAUCEvaluator creates a AUC evaluator.
func NewAUCEvaluator(fullSet DataSet) Evaluator {
	return func(estimator Model, testSet DataSet) float64 {
		full := NewTrainSet(fullSet)
		test := NewTrainSet(testSet)
		sum, count := 0.0, 0.0
		// Find all userIds
		for innerUserIdTest, irs := range test.UserRatings() {
			userId := test.outerUserIds[innerUserIdTest]
			// Find all <userId, j>s in full data set
			innerUserIdFull := full.ConvertUserId(userId)
			fullRatedItem := make(map[int]float64)
			for _, jr := range full.UserRatings()[innerUserIdFull] {
				itemId := full.outerItemIds[jr.Id]
				fullRatedItem[itemId] = jr.Rating
			}
			// Find all <userId, i>s in test data set
			indicatorSum, ratedCount := 0.0, 0.0
			for _, ir := range irs {
				iItemId := test.outerItemIds[ir.Id]
				// Find all <userId, j>s not in full data set
				for j := 0; j < full.ItemCount; j++ {
					jItemId := full.outerItemIds[j]
					if _, exist := fullRatedItem[jItemId]; !exist {
						// I(\hat{x}_{ui} - \hat{x}_{uj})
						if estimator.Predict(userId, iItemId) > estimator.Predict(userId, jItemId) {
							indicatorSum++
						}
						ratedCount++
					}
				}
			}
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
		for innerUserIdTest, irs := range testSet.UserRatings() {
			userId := testSet.outerUserIds[innerUserIdTest]
			// Find top-n items in test set
			relSet := make(map[int]float64)
			relIndex := make([]int, len(irs))
			relRating := make([]float64, len(irs))
			for i, ir := range irs {
				relIndex[i] = i
				relRating[i] = -ir.Rating
			}
			floats.Argsort(relRating, relIndex)
			for i := 0; i < n && i < len(irs); i++ {
				index := relIndex[i]
				ir := irs[index]
				relSet[ir.Id] = ir.Rating
			}
			// Find top-n items in predictions
			topIndex := make([]int, len(irs))
			topRating := make([]float64, len(irs))
			for i, ir := range irs {
				itemId := testSet.outerItemIds[ir.Id]
				topIndex[i] = i
				topRating[i] = -model.Predict(userId, itemId)
			}
			floats.Argsort(topRating, topIndex)
			// IDCG = \sum^{|REL|}_{i=1} \frac {1} {\log_2(i+1)}
			idcg := 0.0
			for i := 0; i < n && i < len(irs); i++ {
				idcg += 1.0 / math.Log2(float64(i)+2.0)
			}
			// DCG = \sum^{N}_{i=1} \frac {2^{rel_i}-1} {\log_2(i+1)}
			dcg := 0.0
			for i := 0; i < n && i < len(irs); i++ {
				index := topIndex[i]
				ir := irs[index]
				if _, exist := relSet[ir.Id]; exist {
					dcg += 1.0 / math.Log2(float64(i)+2.0)
				}
			}
			// NDCG = DCG / IDCG
			sum += dcg / idcg
		}
		return sum / float64(testSet.UserCount)
	}
}

func NewHR() Evaluator {
	return func(model Model, dataSet DataSet) float64 {
		return 0
	}
}

func NewRecall() Evaluator {
	return func(model Model, set DataSet) float64 {
		return 0
	}
}
