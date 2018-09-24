package core

import (
	"path/filepath"
	"testing"
)

func TestSave(t *testing.T) {
	// Fit a SVD model
	dataSet := LoadDataFromBuiltIn("ml-100k")
	estimator1 := NewSVD(nil)
	estimator1.Fit(NewTrainSet(dataSet))
	err1 := RMSE(dataSet.Predict(estimator1), dataSet.Ratings)
	// Save the model
	if err := Save(filepath.Join(tempDir, "/svd.m"), estimator1); err != nil {
		t.Fatal(err)
	}
	// Load the model
	estimator2 := NewSVD(nil)
	if err := Load(filepath.Join(tempDir, "/svd.m"), &estimator2); err != nil {
		t.Fatal(err)
	}
	err2 := RMSE(dataSet.Predict(estimator2), dataSet.Ratings)
	if err1 != err2 {
		t.Fatalf("The model restored from the file has different accuracy: %v != %v", err1, err2)
	}
}
