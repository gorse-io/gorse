package core

import (
	"github.com/zhenghaoz/gorse/base"
	"math"
)

// Top gets the ranking
func Top(items map[int]bool, userId int, n int, exclude *base.MarginalSubSet, model ModelInterface) ([]int, []float64) {
	// Get top-n list
	itemsHeap := base.NewMaxHeap(n)
	for itemId := range items {
		if !exclude.Contain(itemId) {
			itemsHeap.Add(itemId, model.Predict(userId, itemId))
		}
	}
	elem, scores := itemsHeap.ToSorted()
	recommends := make([]int, len(elem))
	for i := range recommends {
		recommends[i] = elem[i].(int)
	}
	return recommends, scores
}

// Items gets all items from the test set and the training set.
func Items(dataSet ...DataSetInterface) map[int]bool {
	items := make(map[int]bool)
	for _, data := range dataSet {
		for i := 0; i < data.ItemCount(); i++ {
			itemId := data.ItemIndexer().ToID(i)
			items[itemId] = true
		}
	}
	return items
}

// Neighbors finds N nearest neighbors of a item. It returns a unordered slice of items (sparse ID) and
// corresponding similarities.
func Neighbors(dataSet DataSetInterface, itemId int, n int, similarity base.FuncSimilarity) ([]int, []float64) {
	// Convert sparse ID to dense ID
	itemIndex := dataSet.ItemIndexer().ToIndex(itemId)
	itemRatings := dataSet.ItemByIndex(itemIndex)
	// Find nearest neighbors
	neighbors := base.NewMaxHeap(n)
	for neighborIndex := 0; neighborIndex < dataSet.ItemCount(); neighborIndex++ {
		if neighborIndex != itemIndex {
			neighborRatings := dataSet.ItemByIndex(neighborIndex)
			neighborId := dataSet.ItemIndexer().ToID(neighborIndex)
			sim := similarity(itemRatings, neighborRatings)
			if !math.IsNaN(sim) {
				neighbors.Add(neighborId, sim)
			}
		}
	}
	elem, scores := neighbors.ToSorted()
	items := make([]int, neighbors.Len())
	for i := range items {
		items[i] = elem[i].(int)
	}
	return items, scores
}

// Popular finds popular items in the dataset.
func Popular(dataSet DataSetInterface, n int) ([]int, []float64) {
	popItems := base.NewMaxHeap(n)
	for itemIndex := 0; itemIndex < dataSet.ItemCount(); itemIndex++ {
		itemScore := dataSet.ItemByIndex(itemIndex).Len()
		itemId := dataSet.ItemIndexer().ToID(itemIndex)
		popItems.Add(itemId, float64(itemScore))
	}
	elem, scores := popItems.ToSorted()
	recommends := make([]int, len(elem))
	for i := range recommends {
		recommends[i] = elem[i].(int)
	}
	return recommends, scores
}
