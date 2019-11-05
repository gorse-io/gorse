package main

import (
	"fmt"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
)

func main() {
	// Load dataset
	data := core.LoadDataFromCSV("steam-100k.csv", ",", true)
	// Split dataset
	train, test := core.Split(data, 0.2)
	// Create model
	svd := model.NewBPR(base.Params{
		base.NFactors:   10,
		base.Reg:        0.01,
		base.Lr:         0.05,
		base.NEpochs:    100,
		base.InitMean:   0,
		base.InitStdDev: 0.001,
	})
	// Fit model
	svd.Fit(train, nil)
	// Evaluate model
	scores := core.EvaluateRank(svd, test, train, 10, core.Precision, core.Recall, core.NDCG)
	fmt.Printf("Precision@10 = %.5f\n", scores[0])
	fmt.Printf("Recall@10 = %.5f\n", scores[1])
	fmt.Printf("NDCG@10 = %.5f\n", scores[1])
	// Predict a rating
	fmt.Printf("Predict(4,8) = %.5f\n", svd.Predict(4, 8))
}
