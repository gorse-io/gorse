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
)

func main() {
	fmt.Println("Benchmarks on MovieLens 1M")
	// Models for benchmarks
	models := []core.Model{
		//// KNN
		//model.NewKNN(base.Params{
		//	base.Type:   base.Baseline,
		//	base.UserBased: false,
		//	base.K:         80,
		//	base.Shrinkage: 10,
		//}),
		// SVD
		model.NewSVD(base.Params{
			base.Lr:       0.001,
			base.NEpochs:  100,
			base.NFactors: 80,
			base.Reg:      0.1,
		}),
		//// SlopOne
		//model.NewSlopOne(nil),
		//// CoClustering
		//model.NewCoClustering(base.Params{
		//	base.NUserClusters: 5,
		//	base.NItemClusters: 5,
		//	base.NEpochs:       30,
		//}),
		// SVD++
		//model.NewSVDpp(base.Params{
		//	base.NFactors: 10,
		//	base.Reg: 0.05,
		//	base.Lr: 0.002,
		//	base.NEpochs: 80,
		//}),
	}
	// Load data
	data := core.LoadDataFromBuiltIn("ml-1m")
	// Cross Validation
	lines := make([][]string, 0)
	for _, m := range models {
		cv := core.CrossValidate(m, data, []core.Evaluator{core.RMSE}, core.NewRatioSplitter(1, 0.2))
		lines = append(lines, []string{
			fmt.Sprint(reflect.TypeOf(m)),
			cv[0].Summary(),
			fmt.Sprint(m.GetParams()),
		})
		log.Println(fmt.Sprint(reflect.TypeOf(m)), ":", cv[0].Summary())
	}
	// Print table
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Model", "RMSE", "Hyper-parameters"})
	for _, v := range lines {
		table.Append(v)
	}
	table.Render()
}
