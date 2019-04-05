package core

import "math/rand"
import "github.com/zhenghaoz/gorse/base"

// Split dataset to a training set and a test set with ratio.
func Split(data *DataSet, testRatio float64) (train, test *DataSet) {
	testSize := int(float64(data.Len()) * testRatio)
	perm := rand.Perm(data.Len())
	// Test Data
	testIndex := perm[:testSize]
	test = NewDataSet(data.SubSet(testIndex))
	// Train Data
	trainIndex := perm[testSize:]
	train = NewDataSet(data.SubSet(trainIndex))
	return
}

// Splitter split Data to train set and test set.
type Splitter func(set Table, seed int64) ([]*DataSet, []*DataSet)

// NewKFoldSplitter creates a k-fold splitter.
func NewKFoldSplitter(k int) Splitter {
	return func(dataSet Table, seed int64) (trainFolds, testFolds []*DataSet) {
		// Create folds
		trainFolds = make([]*DataSet, k)
		testFolds = make([]*DataSet, k)
		// Check nil
		if dataSet == nil {
			return
		}
		// Generate permutation
		rand.Seed(seed)
		perm := rand.Perm(dataSet.Len())
		// Split folds
		foldSize := dataSet.Len() / k
		begin, end := 0, 0
		for i := 0; i < k; i++ {
			end += foldSize
			if i < dataSet.Len()%k {
				end++
			}
			// Test Data
			testIndex := perm[begin:end]
			testFolds[i] = NewDataSet(dataSet.SubSet(testIndex))
			// Train Data
			trainIndex := base.Concatenate(perm[0:begin], perm[end:dataSet.Len()])
			trainFolds[i] = NewDataSet(dataSet.SubSet(trainIndex))
			begin = end
		}
		return trainFolds, testFolds
	}
}

// NewRatioSplitter creates a ratio splitter.
func NewRatioSplitter(repeat int, testRatio float64) Splitter {
	return func(set Table, seed int64) (trainFolds, testFolds []*DataSet) {
		trainFolds = make([]*DataSet, repeat)
		testFolds = make([]*DataSet, repeat)
		testSize := int(float64(set.Len()) * testRatio)
		rand.Seed(seed)
		for i := 0; i < repeat; i++ {
			perm := rand.Perm(set.Len())
			// Test Data
			testIndex := perm[:testSize]
			testFolds[i] = NewDataSet(set.SubSet(testIndex))
			// Train Data
			trainIndex := perm[testSize:]
			trainFolds[i] = NewDataSet(set.SubSet(trainIndex))
		}
		return trainFolds, testFolds
	}
}

// NewUserLOOSplitter creates a per-user leave-one-out Data splitter.
func NewUserLOOSplitter(repeat int) Splitter {
	return func(dataSet Table, seed int64) ([]*DataSet, []*DataSet) {
		trainFolds := make([]*DataSet, repeat)
		testFolds := make([]*DataSet, repeat)
		rand.Seed(seed)
		trainSet := NewDataSet(dataSet)
		for i := 0; i < repeat; i++ {
			trainUsers, trainItems, trainRatings :=
				make([]int, 0, trainSet.Len()-trainSet.UserCount()),
				make([]int, 0, trainSet.Len()-trainSet.UserCount()),
				make([]float64, 0, trainSet.Len()-trainSet.UserCount())
			testUsers, testItems, testRatings :=
				make([]int, 0, trainSet.UserCount()),
				make([]int, 0, trainSet.UserCount()),
				make([]float64, 0, trainSet.UserCount())
			for innerUserId, irs := range trainSet.DenseUserRatings {
				userId := trainSet.UserIdSet.ToSparseId(innerUserId)
				out := rand.Intn(irs.Len())
				irs.ForEach(func(i, index int, value float64) {
					itemId := trainSet.ItemIdSet.ToSparseId(index)
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
			trainFolds[i] = NewDataSet(NewDataTable(trainUsers, trainItems, trainRatings))
			testFolds[i] = NewDataSet(NewDataTable(testUsers, testItems, testRatings))
		}
		return trainFolds, testFolds
	}
}
