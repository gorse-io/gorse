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

func main() {
	fmt.Println("Benchmarks on MovieLens 1M")
	// Models for benchmarks
	models := []core.Model{
		model.NewItemPop(nil),
		model.NewSVD(base.Params{
			base.Target:     base.BPR,
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
	}
	// Load data
	data := core.LoadDataFromBuiltIn("ml-100k")
	// Cross Validation
	lines := make([][]string, 0)
	for _, m := range models {
		start := time.Now()
		cv := core.CrossValidate(m, data, []core.Evaluator{
			core.NewPrecision(10),
			core.NewRecall(10),
			core.NewMAP(10),
			core.NewNDCG(10),
			core.NewMRR(10),
		}, core.NewKFoldSplitter(5))
		tm := time.Since(start)
		line := []string{fmt.Sprint(reflect.TypeOf(m))}
		for i := range cv {
			mean, margin := cv[i].MeanMarginScore()
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
