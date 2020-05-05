package model

import (
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"reflect"
	"testing"
)

func ModelParallelTest(t *testing.T, models ...core.ModelInterface) {
	data := core.LoadDataFromBuiltIn("ml-100k")
	splitter := core.NewUserLOOSplitter(1)
	trains, tests := splitter(data, 0)
	for _, model := range models {
		t.Log("Checking", reflect.TypeOf(model))
		// Fit model
		model.Fit(trains[0], &base.RuntimeOptions{FitJobs: 1})
		rmse := core.EvaluateRank(model, tests[0], trains[0], 10, core.NDCG)[0]
		// Refit model
		model.Fit(trains[0], &base.RuntimeOptions{FitJobs: 3})
		rmse2 := core.EvaluateRank(model, tests[0], trains[0], 10, core.NDCG)[0]
		assert.Equal(t, rmse, rmse2)
	}
}

func TestModelParallel(t *testing.T) {
	ModelParallelTest(t)
}
