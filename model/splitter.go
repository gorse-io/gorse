// Copyright 2020 Zhenghao Zhang
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

import "math/rand"
import "github.com/zhenghaoz/gorse/base"

// Splitter split Data to train set and test set.
type Splitter func(set DataSetInterface, seed int64) ([]DataSetInterface, []DataSetInterface)

// NewKFoldSplitter creates a k-fold splitter.
func NewKFoldSplitter(k int) Splitter {
	return func(dataSet DataSetInterface, seed int64) (trainFolds, testFolds []DataSetInterface) {
		// Create folds
		trainFolds = make([]DataSetInterface, k)
		testFolds = make([]DataSetInterface, k)
		// Check nil
		if dataSet == nil {
			return
		}
		// Generate permutation
		rand.Seed(seed)
		perm := rand.Perm(dataSet.Count())
		// Split folds
		foldSize := dataSet.Count() / k
		begin, end := 0, 0
		for i := 0; i < k; i++ {
			end += foldSize
			if i < dataSet.Count()%k {
				end++
			}
			// Test Data
			testIndex := perm[begin:end]
			testFolds[i] = dataSet.SubSet(testIndex)
			// Train Data
			trainIndex := base.Concatenate(perm[0:begin], perm[end:dataSet.Count()])
			trainFolds[i] = dataSet.SubSet(trainIndex)
			begin = end
		}
		return trainFolds, testFolds
	}
}

// NewRatioSplitter creates a ratio splitter.
func NewRatioSplitter(repeat int, testRatio float64) Splitter {
	return func(dataSet DataSetInterface, seed int64) (trainFolds, testFolds []DataSetInterface) {
		trainFolds = make([]DataSetInterface, repeat)
		testFolds = make([]DataSetInterface, repeat)
		// Check nil
		if dataSet == nil {
			return
		}
		testSize := int(float64(dataSet.Count()) * testRatio)
		rand.Seed(seed)
		for i := 0; i < repeat; i++ {
			perm := rand.Perm(dataSet.Count())
			// Test Data
			testIndex := perm[:testSize]
			testFolds[i] = dataSet.SubSet(testIndex)
			// Train Data
			trainIndex := perm[testSize:]
			trainFolds[i] = dataSet.SubSet(trainIndex)
		}
		return trainFolds, testFolds
	}
}

// NewUserLOOSplitter creates a per-user leave-one-out Data splitter.
func NewUserLOOSplitter(repeat int) Splitter {
	return func(dataSet DataSetInterface, seed int64) (trainFolds, testFolds []DataSetInterface) {
		trainFolds = make([]DataSetInterface, repeat)
		testFolds = make([]DataSetInterface, repeat)
		// Check nil
		if dataSet == nil {
			return
		}
		rand.Seed(seed)
		for i := 0; i < repeat; i++ {
			trainUsers, trainItems, trainRatings :=
				make([]string, 0, dataSet.Count()-dataSet.UserCount()),
				make([]string, 0, dataSet.Count()-dataSet.UserCount()),
				make([]float64, 0, dataSet.Count()-dataSet.UserCount())
			testUsers, testItems, testRatings :=
				make([]string, 0, dataSet.UserCount()),
				make([]string, 0, dataSet.UserCount()),
				make([]float64, 0, dataSet.UserCount())
			for innerUserId := 0; innerUserId < dataSet.UserCount(); innerUserId++ {
				irs := dataSet.UserByIndex(innerUserId)
				userId := dataSet.UserIndexer().ToID(innerUserId)
				out := rand.Intn(irs.Len())
				irs.ForEachIndex(func(i, index int, value float64) {
					itemId := dataSet.ItemIndexer().ToID(index)
					if i == out {
						testUsers = append(testUsers, userId)
						testItems = append(testItems, itemId)
						testRatings = append(testRatings, value)
					} else {
						trainUsers = append(trainUsers, userId)
						trainItems = append(trainItems, itemId)
						trainRatings = append(trainRatings, value)
					}
				})
			}
			trainFolds[i] = NewDataSet(trainUsers, trainItems, trainRatings)
			testFolds[i] = NewDataSet(testUsers, testItems, testRatings)
		}
		return trainFolds, testFolds
	}
}

func NewRatioUserLOOSplitter(repeat int, ratio float64) Splitter {
	return func(dataSet DataSetInterface, seed int64) (trainFolds, testFolds []DataSetInterface) {
		trainFolds = make([]DataSetInterface, repeat)
		testFolds = make([]DataSetInterface, repeat)
		// Check nil
		if dataSet == nil {
			return
		}
		rand.Seed(seed)
		testSize := int(float64(dataSet.UserCount()) * ratio)
		for i := 0; i < repeat; i++ {
			perm := rand.Perm(dataSet.UserCount())
			testSet := make(map[int]interface{})
			for _, userIndex := range perm[:testSize] {
				testSet[userIndex] = nil
			}
			trainUsers, trainItems, trainRatings :=
				make([]string, 0, dataSet.Count()-testSize),
				make([]string, 0, dataSet.Count()-testSize),
				make([]float64, 0, dataSet.Count()-testSize)
			testUsers, testItems, testRatings :=
				make([]string, 0, testSize),
				make([]string, 0, testSize),
				make([]float64, 0, testSize)
			for innerUserId := 0; innerUserId < dataSet.UserCount(); innerUserId++ {
				irs := dataSet.UserByIndex(innerUserId)
				if _, isInTestSet := testSet[innerUserId]; isInTestSet {
					userId := dataSet.UserIndexer().ToID(innerUserId)
					out := rand.Intn(irs.Len())
					irs.ForEachIndex(func(i, index int, value float64) {
						itemId := dataSet.ItemIndexer().ToID(index)
						if i == out {
							testUsers = append(testUsers, userId)
							testItems = append(testItems, itemId)
							testRatings = append(testRatings, value)
						} else {
							trainUsers = append(trainUsers, userId)
							trainItems = append(trainItems, itemId)
							trainRatings = append(trainRatings, value)
						}
					})
				} else {
					userId := dataSet.UserIndexer().ToID(innerUserId)
					irs.ForEachIndex(func(i, index int, value float64) {
						itemId := dataSet.ItemIndexer().ToID(index)
						trainUsers = append(trainUsers, userId)
						trainItems = append(trainItems, itemId)
						trainRatings = append(trainRatings, value)
					})
				}
			}
			trainFolds[i] = NewDataSet(trainUsers, trainItems, trainRatings)
			testFolds[i] = NewDataSet(testUsers, testItems, testRatings)
		}
		return trainFolds, testFolds
	}
}
