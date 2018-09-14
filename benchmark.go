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
	dataset := "ml-100k"
	if len(os.Args) > 1 {
		dataset = os.Args[1]
	}
	// Cross validation
	estimators := map[string]core.Estimator{
		"Random":        core.NewRandom(),
		"BaseLine":      core.NewBaseLine(),
		"SVD":           core.NewSVD(),
		"SVD++":         core.NewSVDpp(),
		"NMF":           core.NewNMF(),
		"Slope One":     core.NewSlopOne(),
		"KNN":           core.NewKNN(),
		"Centered k-NN": core.NewKNNWithMean(),
		"k-NN Baseline": core.NewKNNBaseLine(),
		"k-NN Z-Score":  core.NewKNNWithZScore(),
		"Co-Clustering": core.NewCoClustering(),
	}
	set := core.LoadDataFromBuiltIn(dataset)
	var start time.Time
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "RMSE", "MAE", "Time"})
	for name, algo := range estimators {
		start = time.Now()
		out := core.CrossValidate(algo, set, []core.Evaluator{core.RMSE, core.MAE},
			5, 0, nil)
		tm := time.Since(start)
		table.Append([]string{name,
			fmt.Sprintf("%f", stat.Mean(out[0].Tests, nil)),
			fmt.Sprintf("%f", stat.Mean(out[1].Tests, nil)),
			fmt.Sprint(tm),
		})
	}
	table.Render()
}
