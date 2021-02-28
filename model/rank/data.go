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
package rank

import (
	"bufio"
	log "github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/storage/data"
	"os"
	"strconv"
	"strings"
)

const batchSize = 1024

type Dataset struct {
	// users/items/features
	UnifiedIndex   UnifiedIndex
	UserItemLabels [][]int
	FeedbackInputs [][]int
	FeedbackTarget []float32
	// used for negative sampling
	PositiveCount      int
	UserFeedbackItems  [][]int
	UserFeedbackTarget [][]float32
}

func (dataset *Dataset) UserCount() int {
	return dataset.UnifiedIndex.CountUsers()
}

func (dataset *Dataset) ItemCount() int {
	return dataset.UnifiedIndex.CountItems()
}

func (dataset *Dataset) LabelCount() int {
	return dataset.UnifiedIndex.CountLabels()
}

func (dataset *Dataset) Count() int {
	return len(dataset.FeedbackTarget)
}

func (dataset *Dataset) Get(i int) ([]int, float32) {
	return dataset.FeedbackInputs[i], dataset.FeedbackTarget[i]
}

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
				log.Error("load: support binary features only")
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

func LoadDataFromBuiltIn(name string) (train *Dataset, test *Dataset, err error) {
	trainFilePath, testFilePath, err := model.LocateBuiltInDataset(name, model.FormatLibFM)
	if err != nil {
		return nil, nil, err
	}
	train, test = &Dataset{}, &Dataset{}
	trainMaxLabel, testMaxLabel := 0, 0
	if train.FeedbackInputs, train.FeedbackTarget, trainMaxLabel, err = LoadLibFMFile(trainFilePath); err != nil {
		return nil, nil, err
	}
	if test.FeedbackInputs, test.FeedbackTarget, testMaxLabel, err = LoadLibFMFile(testFilePath); err != nil {
		return nil, nil, err
	}
	unifiedIndex := NewUnifiedDirectIndex(base.Max(trainMaxLabel, testMaxLabel) + 1)
	train.UnifiedIndex = unifiedIndex
	test.UnifiedIndex = unifiedIndex
	return
}

func LoadDataFromDatabase(database data.Database, feedbackTypes []string) (*Dataset, error) {
	unifiedIndex := NewUnifiedMapIndexBuilder()
	cursor := ""
	var err error
	users := make([]data.User, 0)
	items := make([]data.Item, 0)
	// pull users
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
				unifiedIndex.AddLabel(label)
			}
		}
		if cursor == "" {
			break
		}
	}
	// pull items
	for {
		var batchItems []data.Item
		cursor, batchItems, err = database.GetItems(cursor, batchSize)
		if err != nil {
			return nil, err
		}
		for _, item := range batchItems {
			items = append(items, item)
			unifiedIndex.AddItem(item.ItemId)
			for _, label := range item.Labels {
				unifiedIndex.AddLabel(label)
			}
		}
		if cursor == "" {
			break
		}
	}
	// create dataset
	dataSet := &Dataset{
		UnifiedIndex:   unifiedIndex.Build(),
		FeedbackTarget: make([]float32, 0),
	}
	// insert users
	dataSet.UserItemLabels = make([][]int, dataSet.UnifiedIndex.CountItems()+dataSet.UnifiedIndex.CountUsers())
	for _, user := range users {
		userId := dataSet.UnifiedIndex.EncodeUser(user.UserId)
		dataSet.UserItemLabels[userId] = make([]int, len(user.Labels))
		for i := range user.Labels {
			dataSet.UserItemLabels[userId][i] = dataSet.UnifiedIndex.EncodeLabel(user.Labels[i])
		}
	}
	// insert items
	for _, item := range items {
		itemId := dataSet.UnifiedIndex.EncodeItem(item.ItemId)
		dataSet.UserItemLabels[itemId] = make([]int, len(item.Labels))
		for i := range item.Labels {
			dataSet.UserItemLabels[itemId][i] = dataSet.UnifiedIndex.EncodeLabel(item.Labels[i])
		}
	}
	// insert feedback
	dataSet.UserFeedbackItems = base.NewMatrixInt(dataSet.UnifiedIndex.CountUsers(), 0)
	dataSet.UserFeedbackTarget = base.NewMatrix32(dataSet.UnifiedIndex.CountUsers(), 0)
	for {
		var batchFeedback []data.Feedback
		cursor, batchFeedback, err = database.GetFeedback(feedbackTypes[0], cursor, batchSize)
		if err != nil {
			return nil, err
		}
		for _, v := range batchFeedback {
			userId := dataSet.UnifiedIndex.EncodeUser(v.UserId)
			if userId == base.NotId {
				log.Warnf("user (%v) not found", v.UserId)
				continue
			}
			itemId := dataSet.UnifiedIndex.EncodeItem(v.ItemId)
			if itemId == base.NotId {
				log.Warnf("item (%v) not found", v.ItemId)
				continue
			}
			dataSet.PositiveCount++
			dataSet.UserFeedbackItems[userId] = append(dataSet.UserFeedbackItems[userId], itemId)
			dataSet.UserFeedbackTarget[userId] = append(dataSet.UserFeedbackTarget[userId], 1)
		}
		if cursor == "" {
			break
		}
	}
	return dataSet, nil
}

func (dataset *Dataset) Split(ratio float32, seed int64) (*Dataset, *Dataset) {
	// create train/test dataset
	trainSet := &Dataset{
		UnifiedIndex:       dataset.UnifiedIndex,
		UserItemLabels:     dataset.UserItemLabels,
		UserFeedbackItems:  base.NewMatrixInt(dataset.UserCount(), 0),
		UserFeedbackTarget: base.NewMatrix32(dataset.UserCount(), 0),
	}
	testSet := &Dataset{
		UnifiedIndex:       dataset.UnifiedIndex,
		UserItemLabels:     dataset.UserItemLabels,
		UserFeedbackItems:  base.NewMatrixInt(dataset.UserCount(), 0),
		UserFeedbackTarget: base.NewMatrix32(dataset.UserCount(), 0),
	}
	// split by random
	numTestSize := int(float32(dataset.PositiveCount) * ratio)
	rng := base.NewRandomGenerator(seed)
	sampledIndex := base.NewSet(rng.Sample(0, dataset.PositiveCount, numTestSize)...)
	cursor := 0
	for userId, items := range dataset.UserFeedbackItems {
		for i, itemId := range items {
			if sampledIndex.Contain(cursor) {
				// add samples into test set
				testSet.PositiveCount++
				testSet.UserFeedbackItems[userId] = append(testSet.UserFeedbackItems[userId], itemId)
				testSet.UserFeedbackTarget[userId] = append(testSet.UserFeedbackTarget[userId], dataset.UserFeedbackTarget[userId][i])
			} else {
				// add samples into train set
				trainSet.PositiveCount++
				trainSet.UserFeedbackItems[userId] = append(trainSet.UserFeedbackItems[userId], itemId)
				trainSet.UserFeedbackTarget[userId] = append(trainSet.UserFeedbackTarget[userId], dataset.UserFeedbackTarget[userId][i])
			}
			cursor++
		}
	}
	return trainSet, testSet
}

func (dataset *Dataset) NegativeSample(numNegatives int, trainSet *Dataset, seed int64) {
	if dataset.UserFeedbackItems != nil {
		rng := base.NewRandomGenerator(seed)
		// reset samples
		dataset.FeedbackInputs = make([][]int, 0)
		dataset.FeedbackTarget = make([]float32, 0)
		for userId, items := range dataset.UserFeedbackItems {
			// fill positive items
			for i, itemId := range items {
				x := make([]int, 0, 2+len(dataset.UserItemLabels[userId])+len(dataset.UserItemLabels[itemId]))
				x = append(x, userId)
				x = append(x, itemId)
				x = append(x, dataset.UserItemLabels[userId]...)
				x = append(x, dataset.UserItemLabels[itemId]...)
				dataset.FeedbackInputs = append(dataset.FeedbackInputs, x)
				dataset.FeedbackTarget = append(dataset.FeedbackTarget, dataset.UserFeedbackTarget[userId][i])
			}
			// fill negative samples
			posSet := base.NewSet(items...)
			if trainSet != nil {
				posSet.Add(trainSet.UserFeedbackItems[userId]...)
			}
			sampled := rng.Sample(dataset.UserCount(), dataset.ItemCount()+dataset.UserCount(), numNegatives*len(items), posSet)
			for _, negItemId := range sampled {
				x := make([]int, 0, 2+len(dataset.UserItemLabels[userId])+len(dataset.UserItemLabels[negItemId]))
				x = append(x, userId)
				x = append(x, negItemId)
				x = append(x, dataset.UserItemLabels[userId]...)
				x = append(x, dataset.UserItemLabels[negItemId]...)
				dataset.FeedbackInputs = append(dataset.FeedbackInputs, x)
				dataset.FeedbackTarget = append(dataset.FeedbackTarget, -1)
			}
		}
		// shuffle samples
		rng.Shuffle(len(dataset.FeedbackInputs), func(i, j int) {
			dataset.FeedbackInputs[i], dataset.FeedbackInputs[j] = dataset.FeedbackInputs[j], dataset.FeedbackInputs[i]
			dataset.FeedbackTarget[i], dataset.FeedbackTarget[j] = dataset.FeedbackTarget[j], dataset.FeedbackTarget[i]
		})
	}
}
