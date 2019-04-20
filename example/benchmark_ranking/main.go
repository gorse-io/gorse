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
		model.NewItemPop(nil),
		model.NewSVD(base.Params{
			base.Optimizer:  base.BPR,
			base.NFactors:   10,
			base.Reg:        0.01,
			base.Lr:         0.05,
			base.NEpochs:    100,
			base.InitMean:   0,
			base.InitStdDev: 0.001,
		}),
		model.NewWRMF(base.Params{
			base.NFactors: 20,
			base.Reg:      0.015,
			base.Alpha:    1.0,
			base.NEpochs:  10,
		}),
	},
	"ml-1m": {
		model.NewItemPop(nil),
		model.NewSVD(base.Params{
			base.Optimizer:  base.BPR,
			base.NFactors:   10,
			base.Reg:        0.01,
			base.Lr:         0.05,
			base.NEpochs:    100,
			base.InitMean:   0,
			base.InitStdDev: 0.001,
		}),
		model.NewWRMF(base.Params{
			base.NFactors: 20,
			base.Reg:      0.015,
			base.Alpha:    1.0,
			base.NEpochs:  10,
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
			core.NewRankEvaluator(10, core.Precision, core.Recall, core.MAP, core.NDCG, core.MRR))
		tm := time.Since(start)
		line := []string{fmt.Sprint(reflect.TypeOf(m))}
		for i := range cv {
			mean, margin := cv[i].MeanAndMargin()
			line = append(line, fmt.Sprintf("%.5f(±%.5f)", mean, margin))
		}
		line = append(line, fmt.Sprintf("%d:%02d:%02d", int(tm.Hours()), int(tm.Minutes())%60, int(tm.Seconds())%60))
		lines = append(lines, line)
		log.Printf("%s completed", reflect.TypeOf(m))
	}
	// Print table
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Model", "Precision@10", "Recall@10", "MAP@10", "NDCG@10", "MRR@10", "Time"})
	for _, v := range lines {
		table.Append(v)
	}
	table.Render()
}
