// Copyright 2020 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package model

type VerboseInfo struct {
	Epoch     int
	NDCG      float32
	Precision float32
	Recall    float32
}

// Model is the interface for all models. Any model in this
// package should implement it.
type Model interface {
	// Set parameters.
	SetParams(params Params)
	// Get parameters.
	GetParams() Params
	// Predict the rating given by a user (userId) to a item (itemId).
	Predict(userId, itemId string) float32
	// Fit a model with a train set and parameters.
	Fit(trainSet *DataSet, validateSet *DataSet, config *FitConfig)
}

// Base model must be included by every recommendation model. Hyper-parameters,
// ID sets, random generator and fitting options are managed the Base model.
type Base struct {
	Params    Params          // Hyper-parameters
	UserIndex *Index          // Users' ID set
	ItemIndex *Index          // Items' ID set
	rng       RandomGenerator // Random generator
	randState int64           // Random seed
	// Tracker
	isSetParamsCalled bool // Check whether SetParams called
}

// SetParams sets hyper-parameters for the Base model.
func (model *Base) SetParams(params Params) {
	model.isSetParamsCalled = true
	model.Params = params
	model.randState = model.Params.GetInt64(RandomState, 0)
}

// GetParams returns all hyper-parameters.
func (model *Base) GetParams() Params {
	return model.Params
}

// Predict has not been implemented.
func (model *Base) Predict(userId, itemId string) float32 {
	panic("Predict() not implemented")
}

// Fit has not been implemented,
func (model *Base) Fit(trainSet *DataSet, validateSet *DataSet, config *FitConfig) {
	panic("Fit() not implemented")
}

// Init the Base model. The method must be called at the beginning of Fit.
func (model *Base) Init(trainSet *DataSet) {
	// Check Base.SetParams() called
	if !model.isSetParamsCalled {
		panic("Base.SetParams() not called")
	}
	// Setup ID set
	model.UserIndex = trainSet.UserIndex
	model.ItemIndex = trainSet.ItemIndex
	// Setup random state
	model.rng = NewRandomGenerator(model.randState)
}
