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

import (
	"github.com/zhenghaoz/gorse/base"
)

// Model is the interface for all models. Any model in this
// package should implement it.
type Model interface {
	// Set parameters.
	SetParams(params Params)
	// Get parameters.
	GetParams() Params
	// Get parameters grid
	GetParamsGrid() ParamsGrid
	// Clear model weights
	Clear()
}

// BaseModel model must be included by every recommendation model. Hyper-parameters,
// ID sets, random generator and fitting options are managed the BaseModel model.
type BaseModel struct {
	Params    Params               // Hyper-parameters
	rng       base.RandomGenerator // Random generator
	randState int64                // Random seed
}

// SetParams sets hyper-parameters for the BaseModel model.
func (model *BaseModel) SetParams(params Params) {
	model.Params = params
	model.randState = model.Params.GetInt64(RandomState, 0)
	model.rng = base.NewRandomGenerator(model.randState)
}

// GetParams returns all hyper-parameters.
func (model *BaseModel) GetParams() Params {
	return model.Params
}

func (model *BaseModel) GetRandomGenerator() base.RandomGenerator {
	return model.rng
}
