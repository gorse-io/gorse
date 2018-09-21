package main

import (
	"fmt"
	"github.com/ZhangZhenghao/gorse/core"
	"github.com/olekukonko/tablewriter"
	"gonum.org/v1/gonum/stat"
	"os"
	"time"
)

func main() {
	// Parse arguments
	dataSet := "ml-100k"
	if len(os.Args) > 1 {
		dataSet = os.Args[1]
	}
	// Cross validation
	estimators := map[string]core.Estimator{
		"Random":        core.NewRandom(nil),
		"BaseLine":      core.NewBaseLine(nil),
		"SVD":           core.NewSVD(nil),
		"SVD++":         core.NewSVDpp(nil),
		"NMF":           core.NewNMF(nil),
		"Slope One":     core.NewSlopOne(nil),
		"KNN":           core.NewKNN(nil),
		"Centered k-NN": core.NewKNNWithMean(nil),
		"k-NN Baseline": core.NewKNNBaseLine(nil),
		"k-NN Z-Score":  core.NewKNNWithZScore(nil),
		"Co-Clustering": core.NewCoClustering(nil),
	}
	set := core.LoadDataFromBuiltIn(dataSet)
	var start time.Time
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "RMSE", "MAE", "Time"})
	for name, algo := range estimators {
		start = time.Now()
		out := core.CrossValidate(algo, set, []core.Evaluator{core.RMSE, core.MAE},
			5, 0, nil)
		tm := time.Since(start)
		table.Append([]string{name,
			fmt.Sprintf("%.3f", stat.Mean(out[0].Tests, nil)),
			fmt.Sprintf("%.3f", stat.Mean(out[1].Tests, nil)),
			fmt.Sprint(tm),
		})
	}
	table.Render()
}
