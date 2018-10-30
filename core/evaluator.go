package core

import "math"

// Evaluator evaluates the performance of a estimator on the test set.
type Evaluator func(Model, RawDataSet) float64

// RMSE is root mean square error.
func RMSE(estimator Model, testSet RawDataSet) float64 {
	sum := 0.0
	for j := 0; j < testSet.Length(); j++ {
		userId, itemId, rating := testSet.Index(j)
		prediction := estimator.Predict(userId, itemId)
		sum += (prediction - rating) * (prediction - rating)
	}
	return math.Sqrt(sum / float64(testSet.Length()))
}

// MAE is mean absolute error.
func MAE(estimator Model, testSet RawDataSet) float64 {
	sum := 0.0
	for j := 0; j < testSet.Length(); j++ {
		userId, itemId, rating := testSet.Index(j)
		prediction := estimator.Predict(userId, itemId)
		sum += math.Abs(prediction - rating)
	}
	return sum / float64(testSet.Length())
}

// NewAUCEvaluator creates a AUC evaluator.
func NewAUCEvaluator(fullSet RawDataSet) Evaluator {
	return func(estimator Model, testSet RawDataSet) float64 {
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

func NewNDCG(n int) Evaluator {
	return func(model Model, dataSet RawDataSet) float64 {
		return 0
	}
}

func NewHR() Evaluator {
	return func(model Model, dataSet RawDataSet) float64 {
		return 0
	}
}

func NewRecall() Evaluator {
	return func(model Model, set RawDataSet) float64 {
		return 0
	}
}
