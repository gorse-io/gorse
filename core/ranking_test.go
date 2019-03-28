package core

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTop(t *testing.T) {
	model := NewEvaluatorTesterModel([]int{0, 0, 0, 0, 0, 0, 0, 0, 0},
		[]int{0, 1, 2, 3, 4, 5, 6, 7, 8},
		[]float64{1, 9, 2, 8, 3, 7, 4, 6, 5})
	testSet := NewDataSet(NewDataTable([]int{0, 0, 0, 0, 0, 0, 0, 0, 0},
		[]int{0, 1, 2, 3, 4, 5, 6, 7, 8},
		[]float64{1, 9, 2, 8, 3, 7, 4, 6, 5}))
	exclude := map[int]float64{7: 0, 8: 0}
	items := Items(testSet)
	top, _ := Top(items, 0, 5, exclude, model)
	assert.Equal(t, []int{1, 3, 5, 6, 4}, top)
}
