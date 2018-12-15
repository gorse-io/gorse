package main

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
)

func main() {
	data := core.LoadDataFromBuiltIn("ml-1m")
	cv := core.GridSearchCV(model.NewCoClustering(nil), data, []core.Evaluator{core.RMSE},
		core.NewKFoldSplitter(5), core.ParameterGrid{
			base.NItemClusters: {3, 5},
			base.NUserClusters: {3, 5},
			base.NEpochs:       {30},
		})
	cv[0].Summary()
}
