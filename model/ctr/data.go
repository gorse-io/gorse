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
	"github.com/gorse-io/gorse/common/jsonutil"
	"github.com/gorse-io/gorse/common/util"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"modernc.org/mathutil"
)

type Label struct {
	Name  string
	Value float32
}

func ConvertLabels(o any) []Label {
	features := make([]Label, 0)
	return convertLabels(features, "", o)
}

func convertLabels(result []Label, prefix string, o any) []Label {
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
					result = append(result, Label{
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
			result = convertLabels(result, prefix+key+".", val)
		}
	case string:
		result = append(result, Label{
			Name:  prefix + labels,
			Value: 1,
		})
	case json.Number:
		value, _ := labels.Float64()
		result = append(result, Label{
			Name:  prefix,
			Value: float32(value),
		})
	}
	return result
}

type Embedding struct {
	Name  string
	Value []float32
}

func ConvertEmbeddings(o any) []Embedding {
	embeddings := make([]Embedding, 0)
	return convertEmbeddings(embeddings, "", o)
}

func convertEmbeddings(result []Embedding, prefix string, o any) []Embedding {
	if o == nil {
		return nil
	}
	switch embeddings := o.(type) {
	case []any:
		if len(embeddings) == 0 {
			return nil
		}
		var value []float32
		for _, val := range embeddings {
			switch v := val.(type) {
			case float64:
				value = append(value, float32(v))
			case float32:
				value = append(value, v)
			default:
				return result
			}
		}
		result = append(result, Embedding{
			Name:  prefix,
			Value: value,
		})
	case []float64:
		result = append(result, Embedding{
			Name:  prefix,
			Value: lo.Map(embeddings, func(f float64, _ int) float32 { return float32(f) }),
		})
	case []float32:
		result = append(result, Embedding{
			Name:  prefix,
			Value: embeddings,
		})
	case map[string]any:
		for key, val := range embeddings {
			if prefix == "" {
				result = convertEmbeddings(result, key, val)
			} else {
				result = convertEmbeddings(result, prefix+"."+key, val)
			}
		}
	}
	return result
}

// Dataset for click-through-rate models.
type Dataset struct {
	Index                  dataset.UnifiedIndex
	UserLabels             [][]lo.Tuple2[int32, float32]
	ItemLabels             [][]lo.Tuple2[int32, float32]
	ContextLabels          [][]lo.Tuple2[int32, float32]
	Users                  []int32
	Items                  []int32
	Target                 []float32
	ItemEmbeddings         [][][]float32
	ItemEmbeddingDimension []int
	ItemEmbeddingIndex     *dataset.Index
	PositiveCount          int
	NegativeCount          int
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

func (dataset *Dataset) GetIndex() dataset.UnifiedIndex {
	return dataset.Index
}

// Count returns the number of samples.
func (dataset *Dataset) Count() int {
	if len(dataset.Users) != len(dataset.Items) {
		panic("len(dataset.Users) != len(dataset.Items)")
	}
	if len(dataset.Users) > 0 && len(dataset.Users) != len(dataset.Target) {
		panic("len(dataset.Users) != len(dataset.Target)")
	}
	if dataset.ContextLabels != nil && len(dataset.ContextLabels) != len(dataset.Target) {
		panic("len(dataset.ContextLabels) != len(dataset.Target)")
	}
	return len(dataset.Target)
}

func (dataset *Dataset) GetTarget(i int) float32 {
	return dataset.Target[i]
}

// Get returns the i-th sample.
func (dataset *Dataset) Get(i int) ([]int32, []float32, float32) {
	var (
		indices  []int32
		values   []float32
		position int32
	)
	// append user id
	if len(dataset.Users) > 0 {
		indices = append(indices, dataset.Users[i])
		values = append(values, 1)
		position += int32(dataset.CountUsers())
	}
	// append item id
	if len(dataset.Items) > 0 {
		indices = append(indices, position+dataset.Items[i])
		values = append(values, 1)
		position += int32(dataset.CountItems())
	}
	// append user indices
	if len(dataset.Users) > 0 {
		userFeatures := dataset.UserLabels[dataset.Users[i]]
		for _, feature := range userFeatures {
			indices = append(indices, position+feature.A)
			values = append(values, feature.B)
		}
		position += dataset.Index.CountUserLabels()
	}
	// append item indices
	if len(dataset.Items) > 0 {
		itemFeatures := dataset.ItemLabels[dataset.Items[i]]
		for _, feature := range itemFeatures {
			indices = append(indices, position+feature.A)
			values = append(values, feature.B)
		}
	}
	// append context indices
	if dataset.ContextLabels != nil {
		contextIndices, contextValues := lo.Unzip2(dataset.ContextLabels[i])
		indices = append(indices, contextIndices...)
		values = append(values, contextValues...)
	}
	return indices, values, dataset.Target[i]
}

// LoadLibFMFile loads libFM format file.
func LoadLibFMFile(path string) (features [][]lo.Tuple2[int32, float32], targets []float32, maxLabel int32, err error) {
	// open file
	file, err := os.Open(path)
	if err != nil {
		return nil, []float32{}, 0, errors.Trace(err)
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
			return nil, []float32{}, 0, errors.Trace(err)
		}
		targets = append(targets, float32(target))
		// fetch features
		lineFeatures := make([]lo.Tuple2[int32, float32], 0, len(fields[1:]))
		for _, field := range fields[1:] {
			if len(strings.TrimSpace(field)) > 0 {
				kv := strings.Split(field, ":")
				k, v := kv[0], kv[1]
				// append feature
				feature, err := strconv.Atoi(k)
				if err != nil {
					return nil, []float32{}, 0, errors.Trace(err)
				}
				value, err := strconv.ParseFloat(v, 32)
				if err != nil {
					return nil, []float32{}, 0, errors.Trace(err)
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
		return nil, []float32{}, 0, errors.Trace(err)
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
	if train.ContextLabels, train.Target, trainMaxLabel, err = LoadLibFMFile(trainFilePath); err != nil {
		return nil, nil, err
	}
	if test.ContextLabels, test.Target, testMaxLabel, err = LoadLibFMFile(testFilePath); err != nil {
		return nil, nil, err
	}
	unifiedIndex := dataset.NewUnifiedDirectIndex(mathutil.MaxInt32(trainMaxLabel, testMaxLabel) + 1)
	train.Index = unifiedIndex
	test.Index = unifiedIndex
	return
}

// Split a dataset to training set and test set.
func (dataset *Dataset) Split(ratio float32, seed int64) (*Dataset, *Dataset) {
	// create train/test dataset
	trainSet := &Dataset{
		Index:      dataset.Index,
		UserLabels: dataset.UserLabels,
		ItemLabels: dataset.ItemLabels,
	}
	testSet := &Dataset{
		Index:      dataset.Index,
		UserLabels: dataset.UserLabels,
		ItemLabels: dataset.ItemLabels,
	}
	// split by random
	numTestSize := int(float32(dataset.Count()) * ratio)
	rng := util.NewRandomGenerator(seed)
	sampledIndex := mapset.NewSet(rng.Sample(0, dataset.Count(), numTestSize)...)
	for i := 0; i < len(dataset.Target); i++ {
		if sampledIndex.Contains(i) {
			// add samples into test set
			testSet.Users = append(testSet.Users, dataset.Users[i])
			testSet.Items = append(testSet.Items, dataset.Items[i])
			if dataset.ContextLabels != nil {
				testSet.ContextLabels = append(testSet.ContextLabels, dataset.ContextLabels[i])
			}
			testSet.Target = append(testSet.Target, dataset.Target[i])
			if dataset.Target[i] > 0 {
				testSet.PositiveCount++
			} else {
				testSet.NegativeCount++
			}
		} else {
			// add samples into train set
			trainSet.Users = append(trainSet.Users, dataset.Users[i])
			trainSet.Items = append(trainSet.Items, dataset.Items[i])
			if dataset.ContextLabels != nil {
				trainSet.ContextLabels = append(trainSet.ContextLabels, dataset.ContextLabels[i])
			}
			trainSet.Target = append(trainSet.Target, dataset.Target[i])
			if dataset.Target[i] > 0 {
				trainSet.PositiveCount++
			} else {
				trainSet.NegativeCount++
			}
		}
	}
	return trainSet, testSet
}
