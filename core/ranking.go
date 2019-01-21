package core

import (
	"gonum.org/v1/gonum/floats"
)

// Top gets the ranking
func Top(test *DataSet, denseUserId int, n int, exclude map[int]float64, model Model) []int {
	userId := test.UserIdSet.ToSparseId(denseUserId)
	// Get top-n list
	list := make([]int, 0)
	ids := make([]int, 0)
	indices := make([]int, 0)
	ratings := make([]float64, 0)
	for i := 0; i < test.ItemCount(); i++ {
		itemId := test.ItemIdSet.ToSparseId(i)
		if _, exist := exclude[itemId]; !exist {
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
