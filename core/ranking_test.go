package core

import (
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"strconv"
	"testing"
)

func TestTop(t *testing.T) {
	model := NewEvaluatorTesterModel([]int{0, 0, 0, 0, 0, 0, 0, 0, 0},
		[]int{0, 1, 2, 3, 4, 5, 6, 7, 8},
		[]float64{1, 9, 2, 8, 3, 7, 4, 6, 5})
	testSet := NewDataSet([]string{"0", "0", "0", "0", "0", "0", "0", "0", "0"},
		[]string{"0", "1", "2", "3", "4", "5", "6", "7", "8"},
		[]float64{1, 9, 2, 8, 3, 7, 4, 6, 5})
	exclude := base.NewMarginalSubSet(NewTestIndexer([]string{"7", "8"}), []int{0, 1}, []float64{0, 0}, []int{0, 1})
	items := Items(testSet)
	top, _ := Top(items, "0", 5, exclude, model)
	assert.Equal(t, []string{"1", "3", "5", "6", "4"}, top)
}

func TestNeighbors(t *testing.T) {
	// Generate test data
	//     1 2 3 4 5 6 7 8 9 (item)
	//   +--------------------
	// 1 | 1 1 1 1 1 1 1 1 1
	// 2 | 1 1 1 1 1 1 1 1 0
	// 3 | 1 1 1 1 1 1 1 0 0
	// 4 | 1 1 1 1 1 1 0 0 0
	// 5 | 1 1 1 1 1 0 0 0 0
	// 6 | 1 1 1 1 0 0 0 0 0
	// 7 | 1 1 1 0 0 0 0 0 0
	// 8 | 1 1 0 0 0 0 0 0 0
	// 9 | 1 0 0 0 0 0 0 0 0
	// (user)
	users := make([]string, 0, 81)
	items := make([]string, 0, 81)
	ratings := make([]float64, 0, 81)
	for i := 1; i < 10; i++ {
		for j := 1; j < 10; j++ {
			users = append(users, strconv.Itoa(i))
			items = append(items, strconv.Itoa(j))
			if i+j < 9 {
				ratings = append(ratings, 1)
			} else {
				ratings = append(ratings, 0)
			}
		}
	}
	dataSet := NewDataSet(users, items, ratings)
	// Find N nearest neighbors
	neighbors, _ := Neighbors(dataSet, "1", 5, base.MSDSimilarity)
	assert.Equal(t, []string{"2", "3", "4", "5", "6"}, neighbors)
}

func TestPopular(t *testing.T) {
	users := []string{"1", "1", "2", "1", "2", "3", "1", "2", "3", "4", "1", "2", "3", "4", "5"}
	items := []string{"1", "2", "2", "3", "3", "3", "4", "4", "4", "4", "5", "5", "5", "5", "5"}
	ratings := []float64{1, 2, 2, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 5}
	dataSet := NewDataSet(users, items, ratings)
	items, scores := Popular(dataSet, 3)
	assert.Equal(t, []string{"5", "4", "3"}, items)
	assert.Equal(t, []float64{5, 4, 3}, scores)
}
