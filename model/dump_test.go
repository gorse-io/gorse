package model

import (
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/core"
	"path/filepath"
	"reflect"
	"testing"
)

func ModelDumpTest(t *testing.T, models ...core.Model) {
	data := core.LoadDataFromBuiltIn("ml-100k")
	splitter := core.NewRatioSplitter(1, 0.2)
	trains, tests := splitter(data, 0)
	for _, model := range models {
		t.Log("Checking", reflect.TypeOf(model))
		// Fit model
		model.Fit(trains[0])
		rmse := core.RMSE(model, tests[0], trains[0])
		// Save model
		if err := core.Save(filepath.Join(core.TempDir, "/model.m"), model); err != nil {
			t.Fatal(err)
		}
		// Load the model
		cp := reflect.New(reflect.TypeOf(model).Elem()).Interface().(core.Model)
		if err := core.Load(filepath.Join(core.TempDir, "/model.m"), cp); err != nil {
			t.Fatal(err)
		}
		rmse2 := core.RMSE(cp, tests[0], trains[0])
		assert.Equal(t, rmse, rmse2)
	}
}

func TestModelDump(t *testing.T) {
	ModelDumpTest(t,
		NewBaseLine(nil),
		NewSVD(nil),
		NewNMF(nil),
		NewSlopOne(nil),
		NewCoClustering(nil),
		NewKNN(nil))
}
