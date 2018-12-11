package benchmark

import (
	"fmt"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
	"gonum.org/v1/gonum/stat"
	"os"
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
	models := []Model{
		{"SVD", "#SVD", model.NewSVD(nil)},
		//{"SVD++", "#SVDpp", model.NewSVDpp(nil)},
		//{"NMF[3]", "#NMF", core.NewNMF(nil)},
		//{"Slope One[4]", "#SlopeOne", model.NewSlopOne(nil)},
		//{"KNN", "#NewKNN", model.NewKNN(nil)},
		//{"Centered k-NN", "#NewKNNWithMean", model.NewKNN(base.Params{base.KNNType: base.Centered})},
		//{"k-NN Baseline", "#NewKNNBaseLine", model.NewKNN(nil)},
		//{"k-NN Z-Score", "#NewKNNWithZScore", model.NewKNN(nil)},
		//{"Co-Clustering[5]", "#CoClustering", model.NewCoClustering(nil)},
		//{"BaseLine", "#BaseLine", model.NewBaseLine(nil)},
		//{"Random", "#Random", model.NewRandom(nil)},
	}
	set := core.NewTrainSet(core.LoadDataFromBuiltIn(dataSet))
	var start time.Time
	fmt.Printf("| %s | RMSE | MAE | NDCG@5 | Time |\n", dataSet)
	fmt.Println("| - | - | - | - | - |")
	for _, model := range models {
		start = time.Now()
		out := core.CrossValidate(model.estimator, set, []core.Evaluator{core.RMSE, core.MAE, core.NewNDCG(10)},
			core.NewKFoldSplitter(5))
		tm := time.Since(start)
		fmt.Printf("| [%s](%s%s) | %.3f | %.3f | %.3f | %d:%02d:%02d |\n",
			model.name, goDoc, model.doc,
			stat.Mean(out[0].Tests, nil),
			stat.Mean(out[1].Tests, nil),
			stat.Mean(out[2].Tests, nil),
			int(tm.Hours()), int(tm.Minutes())%60, int(tm.Seconds())%60)
	}
}

func main() {
	dataSet := "ml-100k"
	if len(os.Args) > 1 {
		dataSet = os.Args[1]
	}
	benchmark(dataSet)
}
