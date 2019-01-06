package main

import (
	"fmt"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
	"log"
	"os"
	"runtime/pprof"
)

func main() {
	f, err := os.Create("cpuperf")
	if err != nil {
		log.Fatal(err)
	}
	if err = pprof.StartCPUProfile(f); err != nil {
		log.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	data := core.LoadDataFromBuiltIn("ml-100k")
	svd := model.NewSVD(base.Params{
		base.Target:     base.BPR,
		base.NFactors:   10,
		base.Reg:        0.01,
		base.Lr:         0.05,
		base.NEpochs:    100,
		base.InitMean:   0,
		base.InitStdDev: 0.001,
	})
	out := core.CrossValidate(svd, data, []core.Evaluator{
		core.NewPrecision(10),
		core.NewRecall(10),
		core.NewMAP(10),
		core.NewNDCG(10),
		core.NewMRR(10),
	}, core.NewKFoldSplitter(5))
	fmt.Println(out)

	// Save memory profile
	f, err = os.Create("memprof")
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
