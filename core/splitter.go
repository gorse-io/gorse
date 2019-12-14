package core

import "math/rand"
import "github.com/zhenghaoz/gorse/base"

// Split dataset to a training set and a test set with ratio.
func Split(data DataSetInterface, testRatio float64) (train, test DataSetInterface) {
	testSize := int(float64(data.Count()) * testRatio)
	perm := rand.Perm(data.Count())
	// Test Data
	testIndex := perm[:testSize]
	test = data.SubSet(testIndex)
	// Train Data
	trainIndex := perm[testSize:]
	train = data.SubSet(trainIndex)
	return
}

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
