package core

import (
	"github.com/zhenghaoz/gorse/base"
	"gonum.org/v1/gonum/floats"
	"math/rand"
)

func GetRelevantSet(test *DataSet, denseUserId int) map[int]float64 {
	set := make(map[int]float64)
	test.DenseUserRatings[denseUserId].ForEach(func(i, index int, value float64) {
		itemId := test.ItemIdSet.ToSparseId(index)
		set[itemId] = value
	})
	return set
}

// Top gets the ranking
func Top(test *DataSet, denseUserId int, n int, train *DataSet, model Model) []int {
	// Find ratings in training set
	trainSet := make(map[int]float64)
	userId := test.UserIdSet.ToSparseId(denseUserId)
	denseUserIdInTrain := train.UserIdSet.ToDenseId(userId)
	if denseUserIdInTrain != base.NotId {
		train.DenseUserRatings[denseUserIdInTrain].ForEach(func(i, index int, value float64) {
			itemId := train.ItemIdSet.ToSparseId(index)
			trainSet[itemId] = value
		})
	}
	// Get top-n list
	list := make([]int, 0)
	ids := make([]int, 0)
	indices := make([]int, 0)
	ratings := make([]float64, 0)
	for i := 0; i < test.ItemCount(); i++ {
		itemId := test.ItemIdSet.ToSparseId(i)
		if _, exist := trainSet[itemId]; !exist {
			indices = append(indices, i)
			ids = append(ids, itemId)
			ratings = append(ratings, -model.Predict(userId, itemId))
		}
	}
	floats.Argsort(ratings, indices)
	for i := 0; i < n && i < len(indices); i++ {
		index := indices[i]
		list = append(list, ids[index])
	}
	return list
}

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
