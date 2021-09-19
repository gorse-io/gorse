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
	"github.com/chewxy/math32"
	"github.com/juju/errors"
	"github.com/scylladb/go-set"
	"github.com/scylladb/go-set/i32set"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/storage/data"
)

const batchSize = 10000

// Dataset for click-through-rate models.
type Dataset struct {
	Index         UnifiedIndex
	Labels        [][]int32
	Features      [][]int32
	Values        [][]float32
	Target        []float32
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
	if len(dataset.Features) != len(dataset.Values) {
		panic("len(dataset.Features) != len(dataset.Values)")
	}
	if len(dataset.Features) != len(dataset.Target) {
		panic("len(dataset.Features) != len(dataset.Target)")
	}
	return len(dataset.Target)
}

// Get returns the i-th sample.
func (dataset *Dataset) Get(i int) ([]int32, []float32, float32) {
	return dataset.Features[i], dataset.Values[i], dataset.Target[i]
}

// LoadLibFMFile loads libFM format file.
func LoadLibFMFile(path string) (features [][]int32, values [][]float32, targets []float32, maxLabel int32, err error) {
	// open file
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, 0, errors.Trace(err)
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
			return nil, nil, nil, 0, errors.Trace(err)
		}
		targets = append(targets, float32(target))
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
					return nil, nil, nil, 0, errors.Trace(err)
				}
				lineFeatures = append(lineFeatures, int32(feature))
				// append value
				value, err := strconv.ParseFloat(v, 32)
				if err != nil {
					return nil, nil, nil, 0, errors.Trace(err)
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
		return nil, nil, nil, 0, errors.Trace(err)
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
	if train.Features, train.Values, train.Target, trainMaxLabel, err = LoadLibFMFile(trainFilePath); err != nil {
		return nil, nil, err
	}
	if test.Features, test.Values, test.Target, testMaxLabel, err = LoadLibFMFile(testFilePath); err != nil {
		return nil, nil, err
	}
	unifiedIndex := NewUnifiedDirectIndex(base.Max(trainMaxLabel, testMaxLabel) + 1)
	train.Index = unifiedIndex
	test.Index = unifiedIndex
	return
}

// LoadDataFromDatabase load dataset from database.
func LoadDataFromDatabase(database data.Database, clickTypes []string, readType string) (*Dataset, error) {
	unifiedIndex := NewUnifiedMapIndexBuilder()
	var err error
	// pull users
	cursor := ""
	users := make([]data.User, 0)
	start := time.Now()
	for {
		var batchUsers []data.User
		cursor, batchUsers, err = database.GetUsers(cursor, batchSize)
		if err != nil {
			return nil, err
		}
		users = append(users, batchUsers...)
		for _, user := range batchUsers {
			unifiedIndex.AddUser(user.UserId)
			for _, label := range user.Labels {
				unifiedIndex.AddUserLabel(label)
			}
		}
		if cursor == "" {
			break
		}
	}
	base.Logger().Debug("pulled users from database",
		zap.Int32("n_users", unifiedIndex.UserIndex.Len()),
		zap.Duration("used_time", time.Since(start)))
	// pull items
	cursor = ""
	items := make([]data.Item, 0)
	start = time.Now()
	for {
		var batchItems []data.Item
		cursor, batchItems, err = database.GetItems(cursor, batchSize, nil)
		if err != nil {
			return nil, err
		}
		items = append(items, batchItems...)
		for _, item := range batchItems {
			unifiedIndex.AddItem(item.ItemId)
			for _, label := range item.Labels {
				unifiedIndex.AddItemLabel(label)
			}
		}
		if cursor == "" {
			break
		}
	}
	base.Logger().Debug("pulled items from database",
		zap.Int32("n_items", unifiedIndex.ItemIndex.Len()),
		zap.Duration("used_time", time.Since(start)))
	// create dataset
	dataSet := &Dataset{
		Index:  unifiedIndex.Build(),
		Target: make([]float32, 0),
	}
	// insert users
	dataSet.Labels = make([][]int32, dataSet.Index.CountItems()+dataSet.Index.CountUsers())
	for _, user := range users {
		userId := dataSet.Index.EncodeUser(user.UserId)
		dataSet.Labels[userId] = make([]int32, len(user.Labels))
		for i := range user.Labels {
			dataSet.Labels[userId][i] = dataSet.Index.EncodeUserLabel(user.Labels[i])
		}
	}
	// insert items
	for _, item := range items {
		itemIndex := dataSet.Index.EncodeItem(item.ItemId)
		dataSet.Labels[itemIndex] = make([]int32, 0, len(item.Labels))
		for _, label := range item.Labels {
			labelIndex := dataSet.Index.EncodeItemLabel(label)
			if labelIndex != base.NotId {
				dataSet.Labels[itemIndex] = append(dataSet.Labels[itemIndex], labelIndex)
			} else {
				base.Logger().Warn("label not used",
					zap.String("item_id", item.ItemId),
					zap.String("label", label))
			}
		}
	}
	// insert feedback
	cursor = ""
	feedbackTypes := append([]string{readType}, clickTypes...)
	positiveSet := make([]i32set.Set, len(users))
	start = time.Now()
	for {
		var batchFeedback []data.Feedback
		cursor, batchFeedback, err = database.GetFeedback(cursor, batchSize, nil, feedbackTypes...)
		if err != nil {
			return nil, err
		}
		for _, v := range batchFeedback {
			userId := dataSet.Index.EncodeUser(v.UserId)
			if userId == base.NotId {
				base.Logger().Warn("user not found", zap.String("user_id", v.UserId))
				continue
			}
			itemId := dataSet.Index.EncodeItem(v.ItemId)
			if itemId == base.NotId {
				base.Logger().Warn("item not found", zap.String("item_id", v.ItemId))
				continue
			}
			if positiveSet[userId].IsEmpty() {
				positiveSet[userId].Clear()
			}
			if !positiveSet[userId].Has(int32(itemId)) {
				// build features vector
				features := []int32{userId, itemId}
				features = append(features, dataSet.Labels[userId]...)
				features = append(features, dataSet.Labels[itemId]...)
				dataSet.Features = append(dataSet.Features, features)
				// build values vector
				values := []float32{1, 1}
				values = append(values, base.RepeatFloat32s(len(features)-2, 1/math32.Sqrt(float32(len(features)-2)))...)
				dataSet.Values = append(dataSet.Values, values)
				// positive or negative
				if v.FeedbackType == readType {
					dataSet.Target = append(dataSet.Target, -1)
					dataSet.NegativeCount++
				} else {
					positiveSet[userId].Add(int32(itemId))
					dataSet.Target = append(dataSet.Target, 1)
					dataSet.PositiveCount++
				}
			}
		}
		if cursor == "" {
			break
		}
	}
	base.Logger().Debug("pull feedback from database",
		zap.Int("n_positive", dataSet.PositiveCount),
		zap.Int("n_negative", dataSet.NegativeCount),
		zap.Duration("used_time", time.Since(start)))
	return dataSet, nil
}

// Split a dataset to training set and test set.
func (dataset *Dataset) Split(ratio float32, seed int64) (*Dataset, *Dataset) {
	// create train/test dataset
	trainSet := &Dataset{
		Index:  dataset.Index,
		Labels: dataset.Labels,
	}
	testSet := &Dataset{
		Index:  dataset.Index,
		Labels: dataset.Labels,
	}
	// split by random
	numTestSize := int(float32(dataset.Count()) * ratio)
	rng := base.NewRandomGenerator(seed)
	sampledIndex := set.NewIntSet(rng.Sample(0, dataset.Count(), numTestSize)...)
	for i := range dataset.Features {
		if sampledIndex.Has(i) {
			// add samples into test set
			testSet.Features = append(testSet.Features, dataset.Features[i])
			testSet.Values = append(testSet.Values, dataset.Values[i])
			testSet.Target = append(testSet.Target, dataset.Target[i])
			if dataset.Target[i] > 0 {
				testSet.PositiveCount++
			} else {
				testSet.NegativeCount++
			}
		} else {
			// add samples into train set
			trainSet.Features = append(trainSet.Features, dataset.Features[i])
			trainSet.Values = append(trainSet.Values, dataset.Values[i])
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
