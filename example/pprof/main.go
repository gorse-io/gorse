package main

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
	"log"
	"os"
	"runtime/pprof"
)

func main() {
	f, err := os.Create("cpu_prof")
	if err != nil {
		log.Fatal(err)
	}
	if err = pprof.StartCPUProfile(f); err != nil {
		log.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	data := core.LoadDataFromBuiltIn("ml-100k")
	svd := model.NewSVD(base.Params{
		base.NEpochs:    100,
		base.Reg:        0.1,
		base.Lr:         0.01,
		base.NFactors:   50,
		base.InitMean:   0,
		base.InitStdDev: 0.001,
	})
	core.CrossValidate(svd, data, core.NewKFoldSplitter(5), 0, nil,
		core.NewRatingEvaluator(core.RMSE, core.MAE))

	// Save memory profile
	f, err = os.Create("mem_prof")
	if err != nil {
		log.Fatal(err)
	}
	if err = pprof.WriteHeapProfile(f); err != nil {
		log.Fatal(err)
	}
	if err = f.Close(); err != nil {
		log.Fatal(err)
	}
}
