package main

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
	"log"
	"os"
	"reflect"
	"time"
)

var models = map[string][]core.ModelInterface{
	"ml-100k": {
		// SlopOne
		model.NewSlopOne(nil),
		// CoClustering
		model.NewCoClustering(base.Params{
			base.NUserClusters: 5,
			base.NItemClusters: 3,
			base.NEpochs:       20,
		}),
		// KNN
		model.NewKNN(base.Params{
			base.Type:       base.Baseline,
			base.UserBased:  false,
			base.Similarity: base.Pearson,
			base.K:          30,
			base.Shrinkage:  90,
		}),
		// SVD
		model.NewSVD(base.Params{
			base.NEpochs:    100,
			base.Reg:        0.1,
			base.Lr:         0.01,
			base.NFactors:   50,
			base.InitMean:   0,
			base.InitStdDev: 0.001,
		}),
		//SVD++
		model.NewSVDpp(base.Params{
			base.NEpochs:    100,
			base.Reg:        0.05,
			base.Lr:         0.005,
			base.NFactors:   50,
			base.InitMean:   0,
			base.InitStdDev: 0.001,
		}),
	},
	"ml-1m": {
		// SlopOne
		model.NewSlopOne(nil),
		// CoClustering
		model.NewCoClustering(base.Params{
			base.NUserClusters: 5,
			base.NItemClusters: 5,
			base.NEpochs:       30,
		}),
		// KNN
		model.NewKNN(base.Params{
			base.Type:       base.Baseline,
			base.UserBased:  false,
			base.Similarity: base.Pearson,
			base.K:          20,
			base.Shrinkage:  50,
		}),
		// SVD
		model.NewSVD(base.Params{
			base.NEpochs:    100,
			base.Reg:        0.05,
			base.Lr:         0.005,
			base.NFactors:   80,
			base.InitMean:   0,
			base.InitStdDev: 0.001,
		}),
		//SVD++
		model.NewSVDpp(base.Params{
			base.NFactors:   80,
			base.Reg:        0.05,
			base.Lr:         0.005,
			base.NEpochs:    100,
			base.InitMean:   0,
			base.InitStdDev: 0.001,
		}),
	},
}

func main() {
	if len(os.Args) == 1 {
		// Show usage
		fmt.Printf("usage: %s <dataset>\n\n", os.Args[0])
		fmt.Println("support dataset:")
		fmt.Println()
		fmt.Println("\tml-100k")
		fmt.Println("\tml-1m")
		os.Exit(0)
	}
	dataset := os.Args[1]
	fmt.Printf("benchmarks on %s\n", dataset)
	// Load data
	data := core.LoadDataFromBuiltIn(dataset)
	// Cross Validation
	lines := make([][]string, 0)
	for _, m := range models[dataset] {
		start := time.Now()
		cv := core.CrossValidate(m, data, core.NewKFoldSplitter(5), 0, nil,
			core.NewRatingEvaluator(core.RMSE, core.MAE))
		tm := time.Since(start)
		meanRMSE, marginRMSE := cv[0].MeanAndMargin()
		meanMAE, marginMAE := cv[1].MeanAndMargin()
		lines = append(lines, []string{
			fmt.Sprint(reflect.TypeOf(m)),
			fmt.Sprintf("%.5f(±%.5f)", meanRMSE, marginRMSE),
			fmt.Sprintf("%.5f(±%.5f)", meanMAE, marginMAE),
			fmt.Sprintf("%d:%02d:%02d", int(tm.Hours()), int(tm.Minutes())%60, int(tm.Seconds())%60),
		})
		log.Printf("%s: RMSE = %.5f(±%.5f), MAE = %.5f(±%.5f)",
			reflect.TypeOf(m), meanRMSE, marginRMSE, meanMAE, marginMAE)
	}
	// Print table
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Model", "RMSE", "MAE", "Time"})
	for _, v := range lines {
		table.Append(v)
	}
	table.Render()
}
