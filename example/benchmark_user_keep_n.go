package main

import (
	"fmt"
	"github.com/zhenghaoz/gorse/core"
	"gonum.org/v1/gonum/stat"
	"runtime"
	"time"
)

// Benchmark of models for users with few ratings.
func main() {
	// Setup all models
	type Model struct {
		name      string
		estimator core.Model
	}
	models := []Model{
		{"SVD", core.NewSVD(nil)},
		{"SVD++", core.NewSVDpp(nil)},
		//{"NMF[3]", core.NewNMF(nil)},
		//{"Slope One[4]", core.NewSlopOne(nil)},
		//{"KNN", core.NewKNN(nil)},
		//{"Centered k-NN", core.NewKNNWithMean(nil)},
		//{"k-NN Baseline", core.NewKNNBaseLine(nil)},
		//{"k-NN Z-Score", core.NewKNNWithZScore(nil)},
		//{"Co-Clustering[5]", core.NewCoClustering(nil)},
		{"BaseLine", core.NewBaseLine(nil)},
		{"Random", core.NewRandom(nil)},
	}
	// Load data dataSet
	dataSetName := "ml-1m"
	dataSet := core.NewTrainSet(core.LoadDataFromBuiltIn(dataSetName))
	fmt.Printf("The Number of Ratings per User of %s: %f\n", dataSetName, dataSet.Stat()["n_rating_per_user"])
	// Test models under different N
	for n := 0; n < 100; n += 10 {
		fmt.Printf("\nWhen only %d ratings of correspond users occurred in the training data:\n", n)
		var start time.Time
		fmt.Printf("| %s | RMSE | MAE | NDCG@5 | NDCG@10 | Time |\n", dataSetName)
		fmt.Println("| - | - | - | - | - |")
		for _, model := range models {
			start = time.Now()
			out := core.CrossValidate(model.estimator, dataSet,
				[]core.Evaluator{core.RMSE, core.MAE, core.NewNDCG(5), core.NewNDCG(10)},
				core.NewUserKeepNSplitter(5, n, 0.3), 0, core.Parameters{
					"randState": 0,
				}, runtime.NumCPU())
			tm := time.Since(start)
			fmt.Printf("| %s | %.3f | %.3f | %.3f | %.3f | %d:%02d:%02d |\n",
				model.name,
				stat.Mean(out[0].Tests, nil),
				stat.Mean(out[1].Tests, nil),
				stat.Mean(out[2].Tests, nil),
				stat.Mean(out[3].Tests, nil),
				int(tm.Hours()), int(tm.Minutes())%60, int(tm.Seconds())%60)
		}
	}
}
