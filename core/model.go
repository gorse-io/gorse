package core

import "github.com/zhenghaoz/gorse/base"

// Model is the interface for all models. Any model in this
// package should implement it.
type ModelInterface interface {
	// Set parameters.
	SetParams(params base.Params)
	// Get parameters.
	GetParams() base.Params
	// Predict the rating given by a user (userId) to a item (itemId).
	Predict(userId, itemId int) float64
	// Fit a model with a train set and parameters.
	Fit(trainSet DataSetInterface, options *base.RuntimeOptions)
}
