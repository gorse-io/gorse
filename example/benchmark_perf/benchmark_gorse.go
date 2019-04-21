package main

import (
	"fmt"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
	"log"
	"os"
	"time"
)

var models = map[string]core.ModelInterface{
	"ml-100k": model.NewSVD(base.Params{
		base.NEpochs:    100,
		base.Reg:        0.1,
		base.Lr:         0.01,
		base.NFactors:   50,
		base.InitMean:   0,
		base.InitStdDev: 0.001,
	}),
	"ml-1m": model.NewSVD(base.Params{
		base.NEpochs:    100,
		base.Reg:        0.05,
		base.Lr:         0.005,
		base.NFactors:   80,
		base.InitMean:   0,
		base.InitStdDev: 0.001,
	}),
}

func main() {
	if len(os.Args) == 1 {
		// Show usage
		fmt.Printf("usage: %s [ml-100k|ml-1m]\n", os.Args[0])
		os.Exit(0)
	}
	dataset := os.Args[1]
	fmt.Printf("benchmarks on %s\n", dataset)
	// Load data
	data := core.LoadDataFromBuiltIn(dataset)
	// Cross Validation
	start := time.Now()
	cv := core.CrossValidate(models[dataset], data, core.NewKFoldSplitter(5), 0, nil,
		core.NewRatingEvaluator(core.RMSE, core.MAE))
	tm := time.Since(start)
	meanRMSE, marginRMSE := cv[0].MeanAndMargin()
	meanMAE, marginMAE := cv[1].MeanAndMargin()
	log.Printf("RMSE = %.5f(±%.5f), MAE = %.5f(±%.5f), time = %s",
		meanRMSE, marginRMSE, meanMAE, marginMAE, tm)
}
