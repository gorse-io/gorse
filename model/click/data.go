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

package click

import (
	"bufio"
	"github.com/juju/errors"
	"github.com/scylladb/go-set"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"os"
	"strconv"
	"strings"
)

// Dataset for click-through-rate models.
type Dataset struct {
	Index UnifiedIndex

	UserFeatures [][]int32 // features of users
	ItemFeatures [][]int32 // features of items

	Users       base.Integers
	Items       base.Integers
	CtxFeatures [][]int32
	CtxValues   [][]float32
	NormValues  base.Floats
	Target      base.Floats

	PositiveCount int
	NegativeCount int
}

// UserCount returns the number of users.
func (dataset *Dataset) UserCount() int {
	return int(dataset.Index.CountUsers())
}

// ItemCount returns the number of items.
func (dataset *Dataset) ItemCount() int {
	return int(dataset.Index.CountItems())
}

// Count returns the number of samples.
func (dataset *Dataset) Count() int {
	if dataset.Users.Len() != dataset.Items.Len() {
		panic("dataset.Users.Len() != dataset.Items.Len()")
	}
	if dataset.Users.Len() != dataset.NormValues.Len() {
		panic("dataset.Users.Len() != dataset.NormValues.Len()")
	}
	if len(dataset.CtxFeatures) != len(dataset.CtxValues) {
		panic("len(dataset.CtxFeatures) != len(dataset.CtxValues)")
	}
	if dataset.Users.Len() > 0 && dataset.Users.Len() != dataset.Target.Len() {
		panic("dataset.Users.Len() != dataset.Target.Len()")
	}
	if dataset.CtxFeatures != nil && len(dataset.CtxFeatures) != dataset.Target.Len() {
		panic("len(dataset.CtxFeatures) != len(dataset.Target)")
	}
	return dataset.Target.Len()
}

// Get returns the i-th sample.
func (dataset *Dataset) Get(i int) ([]int32, []float32, float32) {
	var features []int32
	var values []float32
	var position int32
	// append user id
	if dataset.Users.Len() > 0 {
		features = append(features, dataset.Users.Get(i))
		values = append(values, 1)
		position += int32(dataset.UserCount())
	}
	// append item id
	if dataset.Items.Len() > 0 {
		features = append(features, position+dataset.Items.Get(i))
		values = append(values, 1)
		position += int32(dataset.ItemCount())
	}
	// append user features
	if dataset.Users.Len() > 0 {
		userFeatures := dataset.UserFeatures[dataset.Users.Get(i)]
		for _, feature := range userFeatures {
			features = append(features, position+feature)
		}
		values = append(values, base.RepeatFloat32s(len(userFeatures), dataset.NormValues.Get(i))...)
		position += dataset.Index.CountUserLabels()
	}
	// append item features
	if dataset.Items.Len() > 0 {
		itemFeatures := dataset.ItemFeatures[dataset.Items.Get(i)]
		for _, feature := range itemFeatures {
			features = append(features, position+feature)
		}
		values = append(values, base.RepeatFloat32s(len(itemFeatures), dataset.NormValues.Get(i))...)
		position += dataset.Index.CountItemLabels()
	}
	// append context features
	if dataset.CtxFeatures != nil {
		features = append(features, dataset.CtxFeatures[i]...)
		values = append(values, dataset.CtxValues[i]...)
	}
	return features, values, dataset.Target.Get(i)
}

// LoadLibFMFile loads libFM format file.
func LoadLibFMFile(path string) (features [][]int32, values [][]float32, targets base.Floats, maxLabel int32, err error) {
	// open file
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, base.Floats{}, 0, errors.Trace(err)
	}
	defer file.Close()
	// read lines
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, " ")
		// fetch target
		target, err := strconv.ParseFloat(fields[0], 32)
		if err != nil {
			return nil, nil, base.Floats{}, 0, errors.Trace(err)
		}
		targets.Append(float32(target))
		// fetch features
		lineFeatures := make([]int32, 0, len(fields[1:]))
		lineValues := make([]float32, 0, len(fields[1:]))
		for _, field := range fields[1:] {
			if len(strings.TrimSpace(field)) > 0 {
				kv := strings.Split(field, ":")
				k, v := kv[0], kv[1]
				// append feature
				feature, err := strconv.Atoi(k)
				if err != nil {
					return nil, nil, base.Floats{}, 0, errors.Trace(err)
				}
				lineFeatures = append(lineFeatures, int32(feature))
				// append value
				value, err := strconv.ParseFloat(v, 32)
				if err != nil {
					return nil, nil, base.Floats{}, 0, errors.Trace(err)
				}
				lineValues = append(lineValues, float32(value))
				maxLabel = base.Max(maxLabel, lineFeatures...)
			}
		}
		features = append(features, lineFeatures)
		values = append(values, lineValues)
	}
	// check error
	if err = scanner.Err(); err != nil {
		return nil, nil, base.Floats{}, 0, errors.Trace(err)
	}
	return
}

// LoadDataFromBuiltIn loads built-in dataset.
func LoadDataFromBuiltIn(name string) (train, test *Dataset, err error) {
	trainFilePath, testFilePath, err := model.LocateBuiltInDataset(name, model.FormatLibFM)
	if err != nil {
		return nil, nil, err
	}
	train, test = &Dataset{}, &Dataset{}
	var trainMaxLabel, testMaxLabel int32
	if train.CtxFeatures, train.CtxValues, train.Target, trainMaxLabel, err = LoadLibFMFile(trainFilePath); err != nil {
		return nil, nil, err
	}
	if test.CtxFeatures, test.CtxValues, test.Target, testMaxLabel, err = LoadLibFMFile(testFilePath); err != nil {
		return nil, nil, err
	}
	unifiedIndex := NewUnifiedDirectIndex(base.Max(trainMaxLabel, testMaxLabel) + 1)
	train.Index = unifiedIndex
	test.Index = unifiedIndex
	return
}

// Split a dataset to training set and test set.
func (dataset *Dataset) Split(ratio float32, seed int64) (*Dataset, *Dataset) {
	// create train/test dataset
	trainSet := &Dataset{
		Index:        dataset.Index,
		UserFeatures: dataset.UserFeatures,
		ItemFeatures: dataset.ItemFeatures,
	}
	testSet := &Dataset{
		Index:        dataset.Index,
		UserFeatures: dataset.UserFeatures,
		ItemFeatures: dataset.ItemFeatures,
	}
	// split by random
	numTestSize := int(float32(dataset.Count()) * ratio)
	rng := base.NewRandomGenerator(seed)
	sampledIndex := set.NewIntSet(rng.Sample(0, dataset.Count(), numTestSize)...)
	for i := 0; i < dataset.Target.Len(); i++ {
		if sampledIndex.Has(i) {
			// add samples into test set
			testSet.Users.Append(dataset.Users.Get(i))
			testSet.Items.Append(dataset.Items.Get(i))
			if dataset.CtxFeatures != nil {
				testSet.CtxFeatures = append(testSet.CtxFeatures, dataset.CtxFeatures[i])
			}
			if dataset.CtxValues != nil {
				testSet.CtxValues = append(testSet.CtxValues, dataset.CtxValues[i])
			}
			testSet.NormValues.Append(dataset.NormValues.Get(i))
			testSet.Target.Append(dataset.Target.Get(i))
			if dataset.Target.Get(i) > 0 {
				testSet.PositiveCount++
			} else {
				testSet.NegativeCount++
			}
		} else {
			// add samples into train set
			trainSet.Users.Append(dataset.Users.Get(i))
			trainSet.Items.Append(dataset.Items.Get(i))
			if dataset.CtxFeatures != nil {
				trainSet.CtxFeatures = append(trainSet.CtxFeatures, dataset.CtxFeatures[i])
			}
			if dataset.CtxValues != nil {
				trainSet.CtxValues = append(trainSet.CtxValues, dataset.CtxValues[i])
			}
			trainSet.NormValues.Append(dataset.NormValues.Get(i))
			trainSet.Target.Append(dataset.Target.Get(i))
			if dataset.Target.Get(i) > 0 {
				trainSet.PositiveCount++
			} else {
				trainSet.NegativeCount++
			}
		}
	}
	return trainSet, testSet
}
