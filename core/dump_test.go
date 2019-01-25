package core

import (
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"path/filepath"
	"testing"
)

type DumpTestModel struct {
	Public     []float64
	private    []float64
	paramsFlag bool
}

func (model *DumpTestModel) SetParams(params base.Params) {
	model.paramsFlag = true
}

func (model *DumpTestModel) GetParams() base.Params {
	return nil
}

func (model *DumpTestModel) Predict(userId, itemId int) float64 {
	panic("Predict() not implemented")
}

func (model *DumpTestModel) Fit(trainSet *DataSet, setters ...RuntimeOption) {
	panic("Fit() not implemented")
}

func TestSave(t *testing.T) {
	// Create a model
	estimator1 := new(DumpTestModel)
	estimator1.Public = []float64{1, 3, 0, 1, 7, 8, 7, 0, 3, 0, 0}
	estimator1.private = []float64{1, 7, 8, 1, 6, 8, 7, 6, 2, 2, 3}
	// Save the model
	if err := Save(filepath.Join(TempDir, "/svd.m"), estimator1); err != nil {
		t.Fatal(err)
	}
	// Load the model
	estimator2 := new(DumpTestModel)
	if err := Load(filepath.Join(TempDir, "/svd.m"), estimator2); err != nil {
		t.Fatal(err)
	}
	// Check the model
	estimator1.Public[0] = 0
	assert.Equal(t, []float64{1, 3, 0, 1, 7, 8, 7, 0, 3, 0, 0}, estimator2.Public)
	assert.Equal(t, 0, len(estimator2.private))
}

func TestCopy(t *testing.T) {
	// Create a model
	estimator1 := new(DumpTestModel)
	estimator1.Public = []float64{1, 3, 0, 1, 7, 8, 7, 0, 3, 0, 0}
	estimator1.private = []float64{1, 7, 8, 1, 6, 8, 7, 6, 2, 2, 3}
	// Copy the model
	estimator2 := new(DumpTestModel)
	if err := Copy(estimator2, estimator1); err != nil {
		t.Fatal(err)
	}
	// Check the model
	estimator1.Public[0] = 0
	assert.Equal(t, []float64{1, 3, 0, 1, 7, 8, 7, 0, 3, 0, 0}, estimator2.Public)
	assert.Equal(t, 0, len(estimator2.private))
}
