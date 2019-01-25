package core

import "github.com/zhenghaoz/gorse/base"

// Model is the interface for all models. Any model in this
// package should implement it.
type Model interface {
	// Set parameters.
	SetParams(params base.Params)
	// Get parameters.
	GetParams() base.Params
	// Predict the rating given by a user (userId) to a item (itemId).
	Predict(userId, itemId int) float64
	// Fit a model with a train set and parameters.
	Fit(trainSet *DataSet, setters ...RuntimeOption)
}

// Table is the interface for all kinds of data table.
type Table interface {
	// Len returns the length of dataset.
	Len() int
	// Get the i-th entry in dataset.
	Get(i int) (int, int, float64)
	// Mean of ratings.
	Mean() float64
	// StdDev of ratings.
	StdDev() float64
	// Minimum of ratings.
	Min() float64
	// Maximum of ratings.
	Max() float64
	// ForEach iterates entries in dataset.
	ForEach(f func(userId, itemId int, rating float64))
	// Subset returns a subset of dataset.
	SubSet(indices []int) Table
}
