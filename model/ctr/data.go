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

package ctr

import (
	"bufio"
	"encoding/json"
	"os"
	"strconv"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/base"
	"github.com/gorse-io/gorse/base/jsonutil"
	"github.com/gorse-io/gorse/model"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"modernc.org/mathutil"
)

type Feature struct {
	Name      string
	Value     float32
	Embedding []float32
}

func ConvertLabelsToFeatures(o any) []Feature {
	features := make([]Feature, 0)
	return convertLabelsToFeatures(features, "", o)
}

func convertLabelsToFeatures(result []Feature, prefix string, o any) []Feature {
	if o == nil {
		return nil
	}
	switch labels := o.(type) {
	case []any:
		if len(labels) == 0 {
			return nil
		}
		switch labels[0].(type) {
		case string:
			for _, val := range labels {
				if s, ok := val.(string); ok {
					result = append(result, Feature{
						Name:  prefix + s,
						Value: 1,
					})
				} else {
					panic("unsupported labels: " + jsonutil.MustMarshal(labels))
				}
			}
		}
	case map[string]any:
		for key, val := range labels {
			result = convertLabelsToFeatures(result, prefix+key+".", val)
		}
	case string:
		result = append(result, Feature{
			Name:  prefix + labels,
			Value: 1,
		})
	case json.Number:
		value, _ := labels.Float64()
		result = append(result, Feature{
			Name:  prefix,
			Value: float32(value),
		})
	}
	return result
}

// Dataset for click-through-rate models.
type Dataset struct {
	Index base.UnifiedIndex

	UserFeatures    [][]lo.Tuple2[int32, float32] // features of users
	ItemFeatures    [][]lo.Tuple2[int32, float32] // features of items
	ContextFeatures [][]lo.Tuple2[int32, float32] // features of context

	Users  base.Array[int32]
	Items  base.Array[int32]
	Target base.Array[float32]

	PositiveCount int
	NegativeCount int
}

// CountUsers returns the number of users.
func (dataset *Dataset) CountUsers() int {
	return int(dataset.Index.CountUsers())
}

// CountItems returns the number of items.
func (dataset *Dataset) CountItems() int {
	return int(dataset.Index.CountItems())
}

func (dataset *Dataset) CountUserLabels() int {
	return int(dataset.Index.CountUserLabels())
}

func (dataset *Dataset) CountItemLabels() int {
	return int(dataset.Index.CountItemLabels())
}

func (dataset *Dataset) CountContextLabels() int {
	return int(dataset.Index.CountContextLabels())
}

func (dataset *Dataset) CountPositive() int {
	return dataset.PositiveCount
}

func (dataset *Dataset) CountNegative() int {
	return dataset.NegativeCount
}

func (dataset *Dataset) GetIndex() base.UnifiedIndex {
	return dataset.Index
}

// Count returns the number of samples.
func (dataset *Dataset) Count() int {
	if dataset.Users.Len() != dataset.Items.Len() {
		panic("dataset.Users.Len() != dataset.Items.Len()")
	}
	if dataset.Users.Len() > 0 && dataset.Users.Len() != dataset.Target.Len() {
		panic("dataset.Users.Len() != dataset.Target.Len()")
	}
	if dataset.ContextFeatures != nil && len(dataset.ContextFeatures) != dataset.Target.Len() {
		panic("len(dataset.ContextFeatures) != len(dataset.Target)")
	}
	return dataset.Target.Len()
}

func (dataset *Dataset) GetTarget(i int) float32 {
	return dataset.Target.Get(i)
}

// Get returns the i-th sample.
func (dataset *Dataset) Get(i int) ([]int32, []float32, float32) {
	var (
		indices  []int32
		values   []float32
		position int32
	)
	// append user id
	if dataset.Users.Len() > 0 {
		indices = append(indices, dataset.Users.Get(i))
		values = append(values, 1)
		position += int32(dataset.CountUsers())
	}
	// append item id
	if dataset.Items.Len() > 0 {
		indices = append(indices, position+dataset.Items.Get(i))
		values = append(values, 1)
		position += int32(dataset.CountItems())
	}
	// append user indices
	if dataset.Users.Len() > 0 {
		userFeatures := dataset.UserFeatures[dataset.Users.Get(i)]
		for _, feature := range userFeatures {
			indices = append(indices, position+feature.A)
			values = append(values, feature.B)
		}
		position += dataset.Index.CountUserLabels()
	}
	// append item indices
	if dataset.Items.Len() > 0 {
		itemFeatures := dataset.ItemFeatures[dataset.Items.Get(i)]
		for _, feature := range itemFeatures {
			indices = append(indices, position+feature.A)
			values = append(values, feature.B)
		}
	}
	// append context indices
	if dataset.ContextFeatures != nil {
		contextIndices, contextValues := lo.Unzip2(dataset.ContextFeatures[i])
		indices = append(indices, contextIndices...)
		values = append(values, contextValues...)
	}
	return indices, values, dataset.Target.Get(i)
}

// LoadLibFMFile loads libFM format file.
func LoadLibFMFile(path string) (features [][]lo.Tuple2[int32, float32], targets base.Array[float32], maxLabel int32, err error) {
	// open file
	file, err := os.Open(path)
	if err != nil {
		return nil, base.Array[float32]{}, 0, errors.Trace(err)
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
			return nil, base.Array[float32]{}, 0, errors.Trace(err)
		}
		targets.Append(float32(target))
		// fetch features
		lineFeatures := make([]lo.Tuple2[int32, float32], 0, len(fields[1:]))
		for _, field := range fields[1:] {
			if len(strings.TrimSpace(field)) > 0 {
				kv := strings.Split(field, ":")
				k, v := kv[0], kv[1]
				// append feature
				feature, err := strconv.Atoi(k)
				if err != nil {
					return nil, base.Array[float32]{}, 0, errors.Trace(err)
				}
				value, err := strconv.ParseFloat(v, 32)
				if err != nil {
					return nil, base.Array[float32]{}, 0, errors.Trace(err)
				}
				lineFeatures = append(lineFeatures, lo.Tuple2[int32, float32]{
					A: int32(feature),
					B: float32(value),
				})
				maxLabel = mathutil.MaxInt32Val(maxLabel, int32(feature))
			}
		}
		features = append(features, lineFeatures)
	}
	// check error
	if err = scanner.Err(); err != nil {
		return nil, base.Array[float32]{}, 0, errors.Trace(err)
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
	if train.ContextFeatures, train.Target, trainMaxLabel, err = LoadLibFMFile(trainFilePath); err != nil {
		return nil, nil, err
	}
	if test.ContextFeatures, test.Target, testMaxLabel, err = LoadLibFMFile(testFilePath); err != nil {
		return nil, nil, err
	}
	unifiedIndex := base.NewUnifiedDirectIndex(mathutil.MaxInt32(trainMaxLabel, testMaxLabel) + 1)
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
	sampledIndex := mapset.NewSet(rng.Sample(0, dataset.Count(), numTestSize)...)
	for i := 0; i < dataset.Target.Len(); i++ {
		if sampledIndex.Contains(i) {
			// add samples into test set
			testSet.Users.Append(dataset.Users.Get(i))
			testSet.Items.Append(dataset.Items.Get(i))
			if dataset.ContextFeatures != nil {
				testSet.ContextFeatures = append(testSet.ContextFeatures, dataset.ContextFeatures[i])
			}
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
			if dataset.ContextFeatures != nil {
				trainSet.ContextFeatures = append(trainSet.ContextFeatures, dataset.ContextFeatures[i])
			}
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
