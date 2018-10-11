package core

import "math/rand"

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
			trainIndex := concatenate(perm[0:begin], perm[end:dataSet.Length()])
			trainFolds[i] = NewTrainSet(dataSet.SubSet(trainIndex))
			begin = end
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
				make([]int, 0, trainSet.Length()-trainSet.UserCount),
				make([]int, 0, trainSet.Length()-trainSet.UserCount),
				make([]float64, 0, trainSet.Length()-trainSet.UserCount)
			testUsers, testItems, testRatings :=
				make([]int, 0, trainSet.UserCount),
				make([]int, 0, trainSet.UserCount),
				make([]float64, 0, trainSet.UserCount)
			for innerUserId, irs := range trainSet.UserRatings() {
				userId := trainSet.outerUserIds[innerUserId]
				out := rand.Intn(len(irs))
				for index, ir := range irs {
					itemId := trainSet.outerItemIds[ir.Id]
					if index == out {
						testUsers = append(testUsers, userId)
						testItems = append(testItems, itemId)
						testRatings = append(testRatings, ir.Rating)
					} else {
						trainUsers = append(trainUsers, userId)
						trainItems = append(trainItems, itemId)
						trainRatings = append(trainRatings, ir.Rating)
					}
				}
			}
			trainFolds[i] = NewTrainSet(NewRawSet(trainUsers, trainItems, trainRatings))
			testFolds[i] = NewRawSet(testUsers, testItems, testRatings)
		}
		return trainFolds, testFolds
	}
}
