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

import "github.com/zhenghaoz/gorse/config"

type FISM struct {
	BaseItemBased
}

func (fism *FISM) SetParams(params Params) {
	panic("not implemented")
}

func (fism *FISM) GetParams() Params {
	panic("not implemented")
}

func (fism *FISM) Fit(trainSet *DataSet, validateSet *DataSet, config *config.FitConfig) Score {
	panic("not implemented")
}

func (fism *FISM) Predict(supportItems []string, itemId string) float32 {
	panic("not implemented")
}

func (fism *FISM) InternalPredict(supportItems []int, itemId int) float32 {
	panic("not implemented")
}
