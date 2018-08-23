package core

import (
	"core/algo"
	"core/data"
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

func CrossValidate(recommend algo.Algorithm, dataSet data.Set, measures []string, cv int, seed int64,
	options ...algo.OptionSetter) [][]float64 {
	a := make([][]float64, 2)
	a[0] = make([]float64, 0, cv)
	a[1] = make([]float64, 0, cv)
	// Split data set
	trainFolds, testFolds := dataSet.KFold(cv, seed)
	for i := 0; i < cv; i++ {
		trainFold := trainFolds[i]
		testFold := testFolds[i]
		recommend.Fit(data.NewDataSet(trainFold), options...)
		result := make([]float64, testFold.Nrow())
		for j := 0; j < testFold.Nrow(); j++ {
			userId, _ := testFold.Elem(j, 0).Int()
			itemId, _ := testFold.Elem(j, 1).Int()
			result[j] = recommend.Predict(userId, itemId)
		}
		tr := testFold.Col("X2").Float()
		a[0] = append(a[0], RootMeanSquareError(result, tr))
		a[1] = append(a[1], MeanAbsoluteError(result, tr))
	}
	return a
}
