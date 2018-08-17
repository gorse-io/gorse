package cv

import "../algo"
import (
	"../data"
	"github.com/gonum/floats"
	"github.com/gonum/stat"
	"math"
)

func AbsTo(dst []float64, a []float64) {
	for i := 0; i < len(a); i++ {
		if a[i] < 0 {
			dst[i] = -a[i]
		} else {
			dst[i] = a[i]
		}
	}
}

func RootMeanSquareError(predictions []float64, truth []float64) float64 {
	temp := make([]float64, len(predictions))
	floats.SubTo(temp, predictions, truth)
	floats.MulTo(temp, temp, temp)
	return math.Sqrt(stat.Mean(temp, nil))
}

func MeanAbsoluteError(predictions []float64, truth []float64) float64 {
	temp := make([]float64, len(predictions))
	floats.SubTo(temp, predictions, truth)
	AbsTo(temp, temp)
	return stat.Mean(temp, nil)
}

func CrossValidate(recommender algo.Algorithm, dataSet data.Set, measures []string, cv int) [][]float64 {
	a := make([][]float64, cv)
	// Split data set
	trainFolds, testFolds := dataSet.KFold(cv)
	for i := 0; i < cv; i++ {
		trainFold := trainFolds[i]
		testFold := testFolds[i]
		recommender.Fit(data.NewDataSet(trainFold))
		result := make([]float64, testFold.Nrow())
		for j := 0; j < testFold.Nrow(); j++ {
			userId, _ := testFold.Elem(j, 0).Int()
			itemId, _ := testFold.Elem(j, 1).Int()
			result[j] = recommender.Predict(userId, itemId)
		}
		tr := testFold.Col("X2").Float()
		a[i] = []float64{RootMeanSquareError(result, tr), MeanAbsoluteError(result, tr)}
	}
	return a
}
