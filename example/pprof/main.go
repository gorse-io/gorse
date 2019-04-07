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

	data := core.LoadDataFromBuiltIn("ml-1m")
	svd := model.NewSVD(base.Params{
		base.Optimizer:  base.BPR,
		base.NFactors:   10,
		base.Reg:        0.01,
		base.Lr:         0.05,
		base.NEpochs:    100,
		base.InitMean:   0,
		base.InitStdDev: 0.001,
	})
	core.CrossValidate(svd, data, core.NewKFoldSplitter(5), 0,
		core.NewRankEvaluator(10, core.Precision, core.Recall, core.MAP, core.NDCG, core.MRR))

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
