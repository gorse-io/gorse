package main

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
)

func main() {
	data := core.LoadDataFromBuiltIn("ml-100k")
	cv := core.GridSearchCV(model.NewSVDpp(nil), data, []core.Evaluator{core.RMSE},
		core.NewRatioSplitter(1, 0.2), core.ParameterGrid{
			base.NEpochs:  {100},
			base.Reg:      {0.1},
			base.Lr:       {0.007},
			base.NFactors: {80},
		})
	cv[0].Summary()
}
