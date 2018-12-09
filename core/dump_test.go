package core

import (
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

type DumpTesterModel struct {
	Public  []float64
	private []float64
}

func TestSave(t *testing.T) {
	// Create a model
	estimator1 := new(DumpTesterModel)
	estimator1.Public = []float64{1, 3, 0, 1, 7, 8, 7, 0, 3, 0, 0}
	estimator1.private = []float64{1, 7, 8, 1, 6, 8, 7, 6, 2, 2, 3}
	// Save the model
	if err := Save(filepath.Join(tempDir, "/svd.m"), estimator1); err != nil {
		t.Fatal(err)
	}
	// Load the model
	estimator2 := new(DumpTesterModel)
	if err := Load(filepath.Join(tempDir, "/svd.m"), &estimator2); err != nil {
		t.Fatal(err)
	}
	// Check the model
	estimator1.Public[0] = 0
	assert.Equal(t, []float64{1, 3, 0, 1, 7, 8, 7, 0, 3, 0, 0}, estimator2.Public)
	assert.Equal(t, 0, len(estimator2.private))
}

func TestCopy(t *testing.T) {
	// Create a model
	estimator1 := new(DumpTesterModel)
	estimator1.Public = []float64{1, 3, 0, 1, 7, 8, 7, 0, 3, 0, 0}
	estimator1.private = []float64{1, 7, 8, 1, 6, 8, 7, 6, 2, 2, 3}
	// Copy the model
	estimator2 := new(DumpTesterModel)
	if err := Copy(estimator2, estimator1); err != nil {
		t.Fatal(err)
	}
	// Check the model
	estimator1.Public[0] = 0
	assert.Equal(t, []float64{1, 3, 0, 1, 7, 8, 7, 0, 3, 0, 0}, estimator2.Public)
	assert.Equal(t, 0, len(estimator2.private))
}
