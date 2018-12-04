package core

import . "github.com/zhenghaoz/gorse/base"

/* Model */

// Model is the interface for all models. Any model in this
// package should implement it.
type Model interface {
	// Set parameters.
	SetParams(params Params)
	// Get parameters.
	GetParams() Params
	// Predict the rating given by a user (userId) to a item (itemId).
	Predict(userId, itemId int) float64
	// Fit a model with a train set and parameters.
	Fit(trainSet TrainSet, setters ...RuntimeOptionSetter)
}
