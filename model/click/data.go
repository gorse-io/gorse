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
	"github.com/scylladb/go-set/i32set"
	"os"
	"strconv"
	"strings"

	"github.com/scylladb/go-set"
	"go.uber.org/zap"

	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/storage/data"
)

const batchSize = 1024

// Dataset for click-through-rate models.
type Dataset struct {
	Index  UnifiedIndex
	Labels [][]int
	Inputs [][]int
	Target []float32
}

// UserCount returns the number of users.
func (dataset *Dataset) UserCount() int {
	return dataset.Index.CountUsers()
}

// ItemCount returns the number of items.
func (dataset *Dataset) ItemCount() int {
	return dataset.Index.CountItems()
}

// Count returns the number of samples.
func (dataset *Dataset) Count() int {
	if len(dataset.Inputs) != len(dataset.Target) {
		panic("len(dataset.Inputs) != len(dataset.Target)")
	}
	return len(dataset.Target)
}

// Get returns the i-th sample.
func (dataset *Dataset) Get(i int) ([]int, float32) {
	return dataset.Inputs[i], dataset.Target[i]
}

// LoadLibFMFile loads libFM format file.
func LoadLibFMFile(path string) (labels [][]int, targets []float32, maxLabel int, err error) {
	labels = make([][]int, 0)
	targets = make([]float32, 0)
	// open file
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, 0, err
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
			return nil, nil, 0, err
		}
		targets = append(targets, float32(target))
		// fetch labels
		lineLabels := make([]int, len(fields[1:]))
		for i, field := range fields[1:] {
			kv := strings.Split(field, ":")
			k, v := kv[0], kv[1]
			if v != "1" {
				base.Logger().Error("support binary features only", zap.String("field", field))
			}
			label, err := strconv.Atoi(k)
			if err != nil {
				return nil, nil, 0, err
			}
			lineLabels[i] = label
			maxLabel = base.Max(maxLabel, base.Max(lineLabels...))
		}
		labels = append(labels, lineLabels)
	}
	// check error
	if err = scanner.Err(); err != nil {
		return nil, nil, 0, err
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
	trainMaxLabel, testMaxLabel := 0, 0
	if train.Inputs, train.Target, trainMaxLabel, err = LoadLibFMFile(trainFilePath); err != nil {
		return nil, nil, err
	}
	if test.Inputs, test.Target, testMaxLabel, err = LoadLibFMFile(testFilePath); err != nil {
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
	for {
		var batchUsers []data.User
		cursor, batchUsers, err = database.GetUsers(cursor, batchSize)
		if err != nil {
			return nil, err
		}
		for _, user := range batchUsers {
			users = append(users, user)
			unifiedIndex.AddUser(user.UserId)
			for _, label := range user.Labels {
				unifiedIndex.AddUserLabel(label)
			}
		}
		if cursor == "" {
			break
		}
	}
	// pull items
	cursor = ""
	items := make([]data.Item, 0)
	for {
		var batchItems []data.Item
		cursor, batchItems, err = database.GetItems(cursor, batchSize, nil)
		if err != nil {
			return nil, err
		}
		for _, item := range batchItems {
			items = append(items, item)
			unifiedIndex.AddItem(item.ItemId)
			for _, label := range item.Labels {
				unifiedIndex.AddItemLabel(label)
			}
		}
		if cursor == "" {
			break
		}
	}
	// create dataset
	dataSet := &Dataset{
		Index:  unifiedIndex.Build(),
		Target: make([]float32, 0),
	}
	// insert users
	dataSet.Labels = make([][]int, dataSet.Index.CountItems()+dataSet.Index.CountUsers())
	for _, user := range users {
		userId := dataSet.Index.EncodeUser(user.UserId)
		dataSet.Labels[userId] = make([]int, len(user.Labels))
		for i := range user.Labels {
			dataSet.Labels[userId][i] = dataSet.Index.EncodeUserLabel(user.Labels[i])
		}
	}
	// insert items
	for _, item := range items {
		itemIndex := dataSet.Index.EncodeItem(item.ItemId)
		dataSet.Labels[itemIndex] = make([]int, 0, len(item.Labels))
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
				// build input vector
				input := []int{userId, itemId}
				input = append(input, dataSet.Labels[userId]...)
				input = append(input, dataSet.Labels[itemId]...)
				dataSet.Inputs = append(dataSet.Inputs, input)
				// positive or negative
				if v.FeedbackType == readType {
					dataSet.Target = append(dataSet.Target, -1)
				} else {
					positiveSet[userId].Add(int32(itemId))
					dataSet.Target = append(dataSet.Target, 1)
				}
			}
		}
		if cursor == "" {
			break
		}
	}
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
	for i := range dataset.Inputs {
		if sampledIndex.Has(i) {
			// add samples into test set
			testSet.Inputs = append(testSet.Inputs, dataset.Inputs[i])
			testSet.Target = append(testSet.Target, dataset.Target[i])
		} else {
			// add samples into train set
			trainSet.Inputs = append(trainSet.Inputs, dataset.Inputs[i])
			trainSet.Target = append(trainSet.Target, dataset.Target[i])
		}
	}
	return trainSet, testSet
}
