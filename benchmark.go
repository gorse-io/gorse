package main

import (
	"fmt"
	"github.com/ZhangZhenghao/gorse/core"
	"gonum.org/v1/gonum/stat"
	"os"
	"runtime"
	"time"
)

type Model struct {
	name      string
	doc       string
	estimator core.Estimator
}

const goDoc = "https://goDoc.org/github.com/ZhangZhenghao/gorse/core"

func main() {
	// Parse arguments
	dataSet := "ml-1m"
	if len(os.Args) > 1 {
		dataSet = os.Args[1]
	}
	// Cross validation
	estimators := []Model{
		{"SVD", "#SVD", core.NewSVD(nil)},
		{"SVD++", "#SVDpp", core.NewSVDpp(nil)},
		{"NMF[3]", "#NMF", core.NewNMF(nil)},
		{"Slope One[4]", "#SlopeOne", core.NewSlopOne(nil)},
		{"KNN", "#NewKNN", core.NewKNN(nil)},
		{"Centered k-NN", "#NewKNNWithMean", core.NewKNNWithMean(nil)},
		{"k-NN Baseline", "#NewKNNBaseLine", core.NewKNNBaseLine(nil)},
		{"k-NN Z-Score", "#NewKNNWithZScore", core.NewKNNWithZScore(nil)},
		{"Co-Clustering[5]", "#CoClustering", core.NewCoClustering(nil)},
		{"BaseLine", "#BaseLine", core.NewBaseLine(nil)},
		{"Random", "#Random", core.NewRandom(nil)},
	}
	set := core.LoadDataFromBuiltIn(dataSet)
	var start time.Time
	fmt.Printf("| %s | RMSE | MAE | Time |\n", dataSet)
	fmt.Println("| - | - | - | - |")
	for _, model := range estimators {
		start = time.Now()
		out := core.CrossValidate(model.estimator, set, []core.Evaluator{core.RMSE, core.MAE},
			5, 0, nil, runtime.NumCPU())
		tm := time.Since(start)
		fmt.Printf("| [%s](%s%s) | %.3f | %.3f | %d:%02d:%02d |\n",
			model.name, goDoc, model.doc,
			stat.Mean(out[0].Tests, nil),
			stat.Mean(out[1].Tests, nil),
			int(tm.Hours()), int(tm.Minutes())%60, int(tm.Seconds())%60)
	}
}
