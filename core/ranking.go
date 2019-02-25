package core

import (
	"gonum.org/v1/gonum/floats"
)

// Top gets the ranking
func Top(items map[int]bool, userId int, n int, exclude map[int]float64, model Model) []int {
	// Get top-n list
	list := make([]int, 0)
	ids := make([]int, 0)
	indices := make([]int, 0)
	ratings := make([]float64, 0)
	for itemId := range items {
		if _, exist := exclude[itemId]; !exist {
			indices = append(indices, len(indices))
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

// Items gets all items from the test set and the training set.
func Items(dataSet ...*DataSet) map[int]bool {
	items := make(map[int]bool)
	for _, data := range dataSet {
		for i := 0; i < data.ItemCount(); i++ {
			itemId := data.ItemIdSet.ToSparseId(i)
			items[itemId] = true
		}
	}
	return items
}
