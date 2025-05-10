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

package cf

import (
	"github.com/samber/lo"
)

// DataSet contains preprocessed data structures for recommendation models.
type DataSet struct {
	ItemFeatures [][]lo.Tuple2[int32, float32]
	UserFeatures [][]lo.Tuple2[int32, float32]
}

// NewMapIndexDataset creates a data set.
func NewMapIndexDataset() *DataSet {
	s := new(DataSet)
	return s
}
