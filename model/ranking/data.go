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
	"github.com/juju/errors"
	"github.com/scylladb/go-set"
	"github.com/scylladb/go-set/i32set"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"go.uber.org/zap"
	"os"
	"strings"
)

// DataSet contains preprocessed data structures for recommendation models.
type DataSet struct {
	UserIndex     base.Index
	ItemIndex     base.Index
	FeedbackUsers base.Integers
	FeedbackItems base.Integers
	UserFeedback  [][]int32
	ItemFeedback  [][]int32
	Negatives     [][]int32
	ItemLabels    [][]int32
	UserLabels    [][]int32
	// statistics
	NumItemLabels int32
	NumUserLabels int32
}

// NewMapIndexDataset creates a data set.
func NewMapIndexDataset() *DataSet {
	s := new(DataSet)
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
			s1 := set.NewInt32Set(dataset.UserFeedback[userIndex]...)
			s2 := set.NewInt32Set(excludeSet.UserFeedback[userIndex]...)
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
	trainSet.ItemLabels, testSet.ItemLabels = dataset.ItemLabels, dataset.ItemLabels
	trainSet.UserLabels, testSet.UserLabels = dataset.UserLabels, dataset.UserLabels
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
		testUserSet := i32set.New(testUsers...)
		for userIndex := int32(0); userIndex < int32(dataset.UserCount()); userIndex++ {
			if !testUserSet.Has(userIndex) {
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

// LoadDataFromCSV loads Data from a CSV file. The CSV file should be:
//   [optional header]
//   <userId 1> <sep> <itemId 1> <sep> <rating 1> <sep> <extras>
//   <userId 2> <sep> <itemId 2> <sep> <rating 2> <sep> <extras>
//   <userId 3> <sep> <itemId 3> <sep> <rating 3> <sep> <extras>
//   ...
// For example, the `u.Data` from MovieLens 100K is:
//  196\t242\t3\t881250949
//  186\t302\t3\t891717742
//  22\t377\t1\t878887116
func LoadDataFromCSV(fileName, sep string, hasHeader bool) *DataSet {
	dataset := NewMapIndexDataset()
	// Open file
	file, err := os.Open(fileName)
	if err != nil {
		base.Logger().Fatal("failed to open csv file", zap.Error(err),
			zap.String("csv_file", fileName))
	}
	defer file.Close()
	// Read CSV file
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Ignore header
		if hasHeader {
			hasHeader = false
			continue
		}
		fields := strings.Split(line, sep)
		// Ignore empty line
		if len(fields) < 2 {
			continue
		}
		dataset.AddFeedback(fields[0], fields[1], true)
	}
	return dataset
}

func loadTest(dataset *DataSet, path string) error {
	// Open
	file, err := os.Open(path)
	if err != nil {
		return errors.Trace(err)
	}
	defer file.Close()
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
	defer file.Close()
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
