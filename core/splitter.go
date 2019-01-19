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
	return func(dataSet Table, seed int64) ([]*DataSet, []*DataSet) {
		// Create folds
		trainFolds := make([]*DataSet, k)
		testFolds := make([]*DataSet, k)
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
	return func(set Table, seed int64) ([]*DataSet, []*DataSet) {
		trainFolds := make([]*DataSet, repeat)
		testFolds := make([]*DataSet, repeat)
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

// NewUserKeepNSplitter splits users to a training set and a test set. Then,
// add all ratings of train users and n ratings of test users to the training
// set. The rest ratings of test set are added to the test set.
func NewUserKeepNSplitter(repeat int, n int, testRatio float64) Splitter {
	return func(set Table, seed int64) ([]*DataSet, []*DataSet) {
		trainFolds := make([]*DataSet, repeat)
		testFolds := make([]*DataSet, repeat)
		rand.Seed(seed)
		trainSet := NewDataSet(set)
		testSize := int(float64(trainSet.UserCount()) * testRatio)
		for i := 0; i < repeat; i++ {
			trainUsers, trainItems, trainRatings :=
				make([]int, 0, trainSet.Len()-trainSet.UserCount()),
				make([]int, 0, trainSet.Len()-trainSet.UserCount()),
				make([]float64, 0, trainSet.Len()-trainSet.UserCount())
			testUsers, testItems, testRatings :=
				make([]int, 0, trainSet.UserCount()),
				make([]int, 0, trainSet.UserCount()),
				make([]float64, 0, trainSet.UserCount())
			userPerm := rand.Perm(trainSet.UserCount())
			userTest := userPerm[:testSize]
			userTrain := userPerm[testSize:]
			userRatings := trainSet.DenseUserRatings
			// Add all train user's ratings to train set
			for _, userId := range userTrain {
				userRatings[userId].ForEach(func(i, index int, value float64) {
					trainUsers = append(trainUsers, userId)
					trainItems = append(trainItems, index)
					trainRatings = append(trainRatings, value)
				})
			}
			// Add test user's ratings to train set and test set
			for _, userId := range userTest {
				ratingPerm := rand.Perm(userRatings[userId].Len())
				for i, index := range ratingPerm {
					if i < n {
						trainUsers = append(trainUsers, userId)
						trainItems = append(trainItems, userRatings[userId].Indices[index])
						trainRatings = append(trainRatings, userRatings[userId].Values[index])
					} else {
						testUsers = append(testUsers, userId)
						testItems = append(testItems, userRatings[userId].Indices[index])
						testRatings = append(testRatings, userRatings[userId].Values[index])
					}
				}
			}
			trainFolds[i] = NewDataSet(NewDataTable(trainUsers, trainItems, trainRatings))
			testFolds[i] = NewDataSet(NewDataTable(testUsers, testItems, testRatings))
		}
		return trainFolds, testFolds
	}
}
