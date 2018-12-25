package main

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
)

func main() {
	data := core.LoadDataFromBuiltIn("ml-100k")
	cv := core.GridSearchCV(model.NewSVDpp(nil), data, []core.Evaluator{core.RMSE},
		core.NewKFoldSplitter(5), core.ParameterGrid{
			base.NEpochs:    {100},
			base.Reg:        {0.05, 0.07},
			base.Lr:         {0.005},
			base.NFactors:   {50},
			base.InitMean:   {0},
			base.InitStdDev: {0.001},
		})
	cv[0].Summary()
}
