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
	"github.com/zhenghaoz/gorse/config"
)

type Score struct {
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
	// Fit a model with a train set and parameters.
	Fit(trainSet *DataSet, validateSet *DataSet, config *config.FitConfig) Score
}

type MatrixFactorization interface {
	// Predict the rating given by a user (userId) to a item (itemId).
	Predict(userId, itemId string) float32
	// InternalPredict
	InternalPredict(userId, itemId int) float32
}

type ItemBased interface {
	// Predict the rating given by a user (userId) to a item (itemId).
	Predict(supportItems []string, itemId string) float32
	// InternalPredict
	InternalPredict(supportItems []int, itemId int) float32
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

type BaseMatrixFactorization struct {
	BaseModel
	UserIndex base.Index
	ItemIndex base.Index
}

func (model *BaseMatrixFactorization) Init(trainSet *DataSet) {
	model.UserIndex = trainSet.UserIndex
	model.ItemIndex = trainSet.ItemIndex
}

type BaseItemBased struct {
	BaseModel
	ItemIndex base.Index
}

func (model *BaseItemBased) Init(trainSet *DataSet) {
	model.ItemIndex = trainSet.ItemIndex
}
