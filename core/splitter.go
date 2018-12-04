package core

import "math/rand"
import . "github.com/zhenghaoz/gorse/core/base"

// Splitter split data to train set and test set.
type Splitter func(set DataSet, seed int64) ([]TrainSet, []DataSet)

// NewKFoldSplitter creates a k-fold splitter.
func NewKFoldSplitter(k int) Splitter {
	return func(dataSet DataSet, seed int64) ([]TrainSet, []DataSet) {
		trainFolds := make([]TrainSet, k)
		testFolds := make([]DataSet, k)
		rand.Seed(seed)
		perm := rand.Perm(dataSet.Length())
		foldSize := dataSet.Length() / k
		begin, end := 0, 0
		for i := 0; i < k; i++ {
			end += foldSize
			if i < dataSet.Length()%k {
				end++
			}
			// Test Data
			testIndex := perm[begin:end]
			testFolds[i] = dataSet.SubSet(testIndex)
			// Train Data
			trainIndex := Concatenate(perm[0:begin], perm[end:dataSet.Length()])
			trainFolds[i] = NewTrainSet(dataSet.SubSet(trainIndex))
			begin = end
		}
		return trainFolds, testFolds
	}
}

func NewRatioSplitter(repeat int, testRatio float64) Splitter {
	return func(set DataSet, seed int64) ([]TrainSet, []DataSet) {
		trainFolds := make([]TrainSet, repeat)
		testFolds := make([]DataSet, repeat)
		testSize := int(float64(set.Length()) * testRatio)
		rand.Seed(seed)
		for i := 0; i < repeat; i++ {
			perm := rand.Perm(set.Length())
			// Test Data
			testIndex := perm[:testSize]
			testFolds[i] = set.SubSet(testIndex)
			// Train Data
			trainIndex := perm[testSize:]
			trainFolds[i] = NewTrainSet(set.SubSet(trainIndex))
		}
		return trainFolds, testFolds
	}
}

// NewUserLOOSplitter creates a per-user leave-one-out data splitter.
func NewUserLOOSplitter(repeat int) Splitter {
	return func(dataSet DataSet, seed int64) ([]TrainSet, []DataSet) {
		trainFolds := make([]TrainSet, repeat)
		testFolds := make([]DataSet, repeat)
		rand.Seed(seed)
		trainSet := NewTrainSet(dataSet)
		for i := 0; i < repeat; i++ {
			trainUsers, trainItems, trainRatings :=
				make([]int, 0, trainSet.Length()-trainSet.UserCount()),
				make([]int, 0, trainSet.Length()-trainSet.UserCount()),
				make([]float64, 0, trainSet.Length()-trainSet.UserCount())
			testUsers, testItems, testRatings :=
				make([]int, 0, trainSet.UserCount()),
				make([]int, 0, trainSet.UserCount()),
				make([]float64, 0, trainSet.UserCount())
			for innerUserId, irs := range trainSet.UserRatings {
				userId := trainSet.UserIdSet.ToSparseId(innerUserId)
				out := rand.Intn(irs.Length())
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
			trainFolds[i] = NewTrainSet(NewRawDataSet(trainUsers, trainItems, trainRatings))
			testFolds[i] = NewRawDataSet(testUsers, testItems, testRatings)
		}
		return trainFolds, testFolds
	}
}

// NewUserKeepNSplitter splits users to a training set and a test set. Then,
// add all ratings of train users and n ratings of test users to the training
// set. The rest ratings of test set are added to the test set.
func NewUserKeepNSplitter(repeat int, n int, testRatio float64) Splitter {
	return func(set DataSet, seed int64) ([]TrainSet, []DataSet) {
		trainFolds := make([]TrainSet, repeat)
		testFolds := make([]DataSet, repeat)
		rand.Seed(seed)
		trainSet := NewTrainSet(set)
		testSize := int(float64(trainSet.UserCount()) * testRatio)
		for i := 0; i < repeat; i++ {
			trainUsers, trainItems, trainRatings :=
				make([]int, 0, trainSet.Length()-trainSet.UserCount()),
				make([]int, 0, trainSet.Length()-trainSet.UserCount()),
				make([]float64, 0, trainSet.Length()-trainSet.UserCount())
			testUsers, testItems, testRatings :=
				make([]int, 0, trainSet.UserCount()),
				make([]int, 0, trainSet.UserCount()),
				make([]float64, 0, trainSet.UserCount())
			userPerm := rand.Perm(trainSet.UserCount())
			userTest := userPerm[:testSize]
			userTrain := userPerm[testSize:]
			userRatings := trainSet.UserRatings
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
				ratingPerm := rand.Perm(userRatings[userId].Length())
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
			trainFolds[i] = NewTrainSet(NewRawDataSet(trainUsers, trainItems, trainRatings))
			testFolds[i] = NewRawDataSet(testUsers, testItems, testRatings)
		}
		return trainFolds, testFolds
	}
}
