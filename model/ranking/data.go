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

package ranking

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/model"
	"go.uber.org/zap"
)

// DataSet contains preprocessed data structures for recommendation models.
type DataSet struct {
	UserIndex      base.Index
	ItemIndex      base.Index
	FeedbackUsers  base.Array[int32]
	FeedbackItems  base.Array[int32]
	UserFeedback   [][]int32
	ItemFeedback   [][]int32
	Negatives      [][]int32
	ItemFeatures   [][]lo.Tuple2[int32, float32]
	UserFeatures   [][]lo.Tuple2[int32, float32]
	HiddenItems    []bool
	ItemCategories [][]string
	CategorySet    mapset.Set[string]
	// statistics
	NumItemLabels    int32
	NumUserLabels    int32
	NumItemLabelUsed int
	NumUserLabelUsed int
}

// NewMapIndexDataset creates a data set.
func NewMapIndexDataset() *DataSet {
	s := new(DataSet)
	s.CategorySet = mapset.NewSet[string]()
	// Create index
	s.UserIndex = base.NewMapIndex()
	s.ItemIndex = base.NewMapIndex()
	// Initialize slices
	s.UserFeedback = make([][]int32, 0)
	s.ItemFeedback = make([][]int32, 0)
	return s
}

func NewDirectIndexDataset() *DataSet {
	dataset := new(DataSet)
	// Create index
	dataset.UserIndex = base.NewDirectIndex()
	dataset.ItemIndex = base.NewDirectIndex()
	// Initialize slices
	dataset.UserFeedback = make([][]int32, 0)
	dataset.ItemFeedback = make([][]int32, 0)
	dataset.Negatives = make([][]int32, 0)
	return dataset
}

func (dataset *DataSet) AddUser(userId string) {
	dataset.UserIndex.Add(userId)
	userIndex := dataset.UserIndex.ToNumber(userId)
	for int(userIndex) >= len(dataset.UserFeedback) {
		dataset.UserFeedback = append(dataset.UserFeedback, make([]int32, 0))
	}
}

func (dataset *DataSet) AddItem(itemId string) {
	dataset.ItemIndex.Add(itemId)
	itemIndex := dataset.ItemIndex.ToNumber(itemId)
	for int(itemIndex) >= len(dataset.ItemFeedback) {
		dataset.ItemFeedback = append(dataset.ItemFeedback, make([]int32, 0))
	}
}

func (dataset *DataSet) AddFeedback(userId, itemId string, insertUserItem bool) {
	if insertUserItem {
		dataset.UserIndex.Add(userId)
	}
	if insertUserItem {
		dataset.ItemIndex.Add(itemId)
	}
	userIndex := dataset.UserIndex.ToNumber(userId)
	itemIndex := dataset.ItemIndex.ToNumber(itemId)
	if userIndex != base.NotId && itemIndex != base.NotId {
		dataset.AddRawFeedback(userIndex, itemIndex)
	}
}

func (dataset *DataSet) AddRawFeedback(userIndex, itemIndex int32) {
	dataset.FeedbackUsers.Append(userIndex)
	dataset.FeedbackItems.Append(itemIndex)
	for int(itemIndex) >= len(dataset.ItemFeedback) {
		dataset.ItemFeedback = append(dataset.ItemFeedback, make([]int32, 0))
	}
	dataset.ItemFeedback[itemIndex] = append(dataset.ItemFeedback[itemIndex], userIndex)
	for int(userIndex) >= len(dataset.UserFeedback) {
		dataset.UserFeedback = append(dataset.UserFeedback, make([]int32, 0))
	}
	dataset.UserFeedback[userIndex] = append(dataset.UserFeedback[userIndex], itemIndex)
}

func (dataset *DataSet) SetNegatives(userId string, negatives []string) {
	userIndex := dataset.UserIndex.ToNumber(userId)
	if userIndex != base.NotId {
		for int(userIndex) >= len(dataset.Negatives) {
			dataset.Negatives = append(dataset.Negatives, make([]int32, 0))
		}
		dataset.Negatives[userIndex] = make([]int32, 0, len(negatives))
		for _, itemId := range negatives {
			itemIndex := dataset.ItemIndex.ToNumber(itemId)
			if itemIndex != base.NotId {
				dataset.Negatives[userIndex] = append(dataset.Negatives[userIndex], itemIndex)
			}
		}
	}
}

func (dataset *DataSet) Count() int {
	if dataset.FeedbackUsers.Len() != dataset.FeedbackItems.Len() {
		panic("dataset.FeedbackUsers.Len() != dataset.FeedbackItems.Len()")
	}
	return dataset.FeedbackUsers.Len()
}

// UserCount returns the number of UserFeedback.
func (dataset *DataSet) UserCount() int {
	return int(dataset.UserIndex.Len())
}

// ItemCount returns the number of ItemFeedback.
func (dataset *DataSet) ItemCount() int {
	return int(dataset.ItemIndex.Len())
}

func createSliceOfSlice(n int) [][]int32 {
	x := make([][]int32, n)
	for i := range x {
		x[i] = make([]int32, 0)
	}
	return x
}

func (dataset *DataSet) NegativeSample(excludeSet *DataSet, numCandidates int) [][]int32 {
	if len(dataset.Negatives) == 0 {
		rng := base.NewRandomGenerator(0)
		dataset.Negatives = make([][]int32, dataset.UserCount())
		for userIndex := 0; userIndex < dataset.UserCount(); userIndex++ {
			s1 := mapset.NewSet(dataset.UserFeedback[userIndex]...)
			s2 := mapset.NewSet(excludeSet.UserFeedback[userIndex]...)
			dataset.Negatives[userIndex] = rng.SampleInt32(0, int32(dataset.ItemCount()), numCandidates, s1, s2)
		}
	}
	return dataset.Negatives
}

// Split dataset by user-leave-one-out method. The argument `numTestUsers` determines the number of users in the test
// set. If numTestUsers is equal or greater than the number of total users or numTestUsers <= 0, all users are presented
// in the test set.
func (dataset *DataSet) Split(numTestUsers int, seed int64) (*DataSet, *DataSet) {
	trainSet, testSet := new(DataSet), new(DataSet)
	trainSet.NumItemLabels, testSet.NumItemLabels = dataset.NumItemLabels, dataset.NumItemLabels
	trainSet.NumUserLabels, testSet.NumUserLabels = dataset.NumUserLabels, dataset.NumUserLabels
	trainSet.HiddenItems, testSet.HiddenItems = dataset.HiddenItems, dataset.HiddenItems
	trainSet.ItemCategories, testSet.ItemCategories = dataset.ItemCategories, dataset.ItemCategories
	trainSet.CategorySet, testSet.CategorySet = dataset.CategorySet, dataset.CategorySet
	trainSet.ItemFeatures, testSet.ItemFeatures = dataset.ItemFeatures, dataset.ItemFeatures
	trainSet.UserFeatures, testSet.UserFeatures = dataset.UserFeatures, dataset.UserFeatures
	trainSet.NumItemLabelUsed, testSet.NumItemLabelUsed = dataset.NumItemLabelUsed, dataset.NumItemLabelUsed
	trainSet.NumUserLabelUsed, testSet.NumUserLabelUsed = dataset.NumUserLabelUsed, dataset.NumUserLabelUsed
	trainSet.UserIndex, testSet.UserIndex = dataset.UserIndex, dataset.UserIndex
	trainSet.ItemIndex, testSet.ItemIndex = dataset.ItemIndex, dataset.ItemIndex
	trainSet.UserFeedback, testSet.UserFeedback = createSliceOfSlice(dataset.UserCount()), createSliceOfSlice(dataset.UserCount())
	trainSet.ItemFeedback, testSet.ItemFeedback = createSliceOfSlice(dataset.ItemCount()), createSliceOfSlice(dataset.ItemCount())
	rng := base.NewRandomGenerator(seed)
	if numTestUsers >= dataset.UserCount() || numTestUsers <= 0 {
		for userIndex := int32(0); userIndex < int32(dataset.UserCount()); userIndex++ {
			if len(dataset.UserFeedback[userIndex]) > 0 {
				k := rng.Intn(len(dataset.UserFeedback[userIndex]))
				testSet.FeedbackUsers.Append(userIndex)
				testSet.FeedbackItems.Append(dataset.UserFeedback[userIndex][k])
				testSet.UserFeedback[userIndex] = append(testSet.UserFeedback[userIndex], dataset.UserFeedback[userIndex][k])
				testSet.ItemFeedback[dataset.UserFeedback[userIndex][k]] = append(testSet.ItemFeedback[dataset.UserFeedback[userIndex][k]], userIndex)
				for i, itemIndex := range dataset.UserFeedback[userIndex] {
					if i != k {
						trainSet.FeedbackUsers.Append(userIndex)
						trainSet.FeedbackItems.Append(itemIndex)
						trainSet.UserFeedback[userIndex] = append(trainSet.UserFeedback[userIndex], itemIndex)
						trainSet.ItemFeedback[itemIndex] = append(trainSet.ItemFeedback[itemIndex], userIndex)
					}
				}
			}
		}
	} else {
		testUsers := rng.SampleInt32(0, int32(dataset.UserCount()), numTestUsers)
		for _, userIndex := range testUsers {
			if len(dataset.UserFeedback[userIndex]) > 0 {
				k := rng.Intn(len(dataset.UserFeedback[userIndex]))
				testSet.FeedbackUsers.Append(userIndex)
				testSet.FeedbackItems.Append(dataset.UserFeedback[userIndex][k])
				testSet.UserFeedback[userIndex] = append(testSet.UserFeedback[userIndex], dataset.UserFeedback[userIndex][k])
				testSet.ItemFeedback[dataset.UserFeedback[userIndex][k]] = append(testSet.ItemFeedback[dataset.UserFeedback[userIndex][k]], userIndex)
				for i, itemIndex := range dataset.UserFeedback[userIndex] {
					if i != k {
						trainSet.FeedbackUsers.Append(userIndex)
						trainSet.FeedbackItems.Append(itemIndex)
						trainSet.UserFeedback[userIndex] = append(trainSet.UserFeedback[userIndex], itemIndex)
						trainSet.ItemFeedback[itemIndex] = append(trainSet.ItemFeedback[itemIndex], userIndex)
					}
				}
			}
		}
		testUserSet := mapset.NewSet(testUsers...)
		for userIndex := int32(0); userIndex < int32(dataset.UserCount()); userIndex++ {
			if !testUserSet.Contains(userIndex) {
				for _, itemIndex := range dataset.UserFeedback[userIndex] {
					trainSet.FeedbackUsers.Append(userIndex)
					trainSet.FeedbackItems.Append(itemIndex)
					trainSet.UserFeedback[userIndex] = append(trainSet.UserFeedback[userIndex], itemIndex)
					trainSet.ItemFeedback[itemIndex] = append(trainSet.ItemFeedback[itemIndex], userIndex)
				}
			}
		}
	}
	return trainSet, testSet
}

// GetIndex gets the i-th record by <user index, item index, rating>.
func (dataset *DataSet) GetIndex(i int) (int32, int32) {
	return dataset.FeedbackUsers.Get(i), dataset.FeedbackItems.Get(i)
}

func loadTest(dataset *DataSet, path string) error {
	// Open
	file, err := os.Open(path)
	if err != nil {
		return errors.Trace(err)
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			log.Logger().Error("failed to close file", zap.Error(err))
		}
	}(file)
	// Read lines
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		positive, negative := fields[0], fields[1:]
		if positive[0] != '(' || positive[len(positive)-1] != ')' {
			return fmt.Errorf("wrong foramt: %v", line)
		}
		positive = positive[1 : len(positive)-1]
		fields = strings.Split(positive, ",")
		userId, itemId := fields[0], fields[1]
		dataset.AddFeedback(userId, itemId, true)
		dataset.SetNegatives(userId, negative)
	}
	return scanner.Err()
}

func loadTrain(path string) (*DataSet, error) {
	// Open
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			log.Logger().Error("failed to close file", zap.Error(err))
		}
	}(file)
	// Read lines
	scanner := bufio.NewScanner(file)
	dataset := NewDirectIndexDataset()
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		userId, itemId := fields[0], fields[1]
		dataset.AddFeedback(userId, itemId, true)
	}
	return dataset, scanner.Err()
}

// LoadDataFromBuiltIn loads a built-in Data set. Now support:
func LoadDataFromBuiltIn(dataSetName string) (*DataSet, *DataSet, error) {
	// Extract Data set information
	trainFilePath, testFilePath, err := model.LocateBuiltInDataset(dataSetName, model.FormatNCF)
	if err != nil {
		return nil, nil, err
	}
	// Load dataset
	trainSet, err := loadTrain(trainFilePath)
	if err != nil {
		return nil, nil, err
	}
	testSet := NewDirectIndexDataset()
	testSet.UserIndex = trainSet.UserIndex
	testSet.ItemIndex = trainSet.ItemIndex
	err = loadTest(testSet, testFilePath)
	if err != nil {
		return nil, nil, err
	}
	return trainSet, testSet, nil
}
