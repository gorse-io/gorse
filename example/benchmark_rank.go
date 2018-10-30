package main

import (
	"fmt"
	"github.com/zhenghaoz/gorse/core"
	"gonum.org/v1/gonum/stat"
	"os"
	"runtime"
	"time"
)

type Model struct {
	name      string
	doc       string
	estimator core.Model
}

const goDoc = "https://goDoc.org/github.com/ZhangZhenghao/gorse/core"

func benchmark(dataSet string) {
	// Cross validation
	estimators := []Model{
		{"SVD", "#SVD", core.NewSVD(nil)},
		{"Random", "#Random", core.NewRandom(nil)},
	}
	set := core.LoadDataFromBuiltIn(dataSet)
	var start time.Time
	fmt.Printf("| %s | AUC | Time |\n", dataSet)
	fmt.Println("| - | - | - |")
	for _, model := range estimators {
		start = time.Now()
		out := core.CrossValidate(model.estimator, set, []core.Evaluator{core.NewAUCEvaluator(set)},
			core.NewUserLOOSplitter(1), 0, core.Parameters{
				"randState": 0,
				"optimizer": core.BPROptimizer,
			}, runtime.NumCPU())
		tm := time.Since(start)
		fmt.Printf("| [%s](%s%s) | %.3f | %d:%02d:%02d |\n",
			model.name, goDoc, model.doc,
			stat.Mean(out[0].Tests, nil),
			int(tm.Hours()), int(tm.Minutes())%60, int(tm.Seconds())%60)
	}
}

func main() {
	dataSet := "ml-1m"
	if len(os.Args) > 1 {
		dataSet = os.Args[1]
	}
	benchmark(dataSet)
}
