package base

import (
	"github.com/stretchr/testify/assert"
	"math"
	"sort"
	"testing"
)

func NewTestIndexer(id []string) *Indexer {
	indexer := NewIndexer()
	for _, v := range id {
		indexer.Add(v)
	}
	return indexer
}

func TestIndexer(t *testing.T) {
	// Create a indexer
	set := NewIndexer()
	assert.Equal(t, set.Len(), 0)
	// Add IDs
	set.Add("1")
	set.Add("2")
	set.Add("4")
	set.Add("8")
	assert.Equal(t, 4, set.Len())
	assert.Equal(t, 0, set.ToIndex("1"))
	assert.Equal(t, 1, set.ToIndex("2"))
	assert.Equal(t, 2, set.ToIndex("4"))
	assert.Equal(t, 3, set.ToIndex("8"))
	assert.Equal(t, NotId, set.ToIndex("1000"))
	assert.Equal(t, "1", set.ToID(0))
	assert.Equal(t, "2", set.ToID(1))
	assert.Equal(t, "4", set.ToID(2))
	assert.Equal(t, "8", set.ToID(3))
}

func TestStringIndexer(t *testing.T) {
	// Create a string indexer
	set := NewStringIndexer()
	assert.Equal(t, set.Len(), 0)
	// Add IDs
	set.Add("1")
	set.Add("2")
	set.Add("4")
	set.Add("8")
	assert.Equal(t, 4, set.Len())
	assert.Equal(t, 0, set.ToIndex("1"))
	assert.Equal(t, 1, set.ToIndex("2"))
	assert.Equal(t, 2, set.ToIndex("4"))
	assert.Equal(t, 3, set.ToIndex("8"))
	assert.Equal(t, NotId, set.ToIndex("1000"))
	assert.Equal(t, "1", set.ToName(0))
	assert.Equal(t, "2", set.ToName(1))
	assert.Equal(t, "4", set.ToName(2))
	assert.Equal(t, "8", set.ToName(3))
}

func TestNewMarginalSubSet(t *testing.T) {
	// Create a subset
	indexer := NewTestIndexer([]string{"2", "4", "6", "8", "9"})
	indices := []int{0, 1, 2, 3, 4}
	values := []float64{1, 1, 1, 1, 1}
	subset := []int{4, 2, 0}
	set := NewMarginalSubSet(indexer, indices, values, subset)
	assert.Equal(t, []int{0, 2, 4}, set.SubSet)
	// Check Count()
	assert.Equal(t, 3, set.Count())
	// Check Contain()
	assert.True(t, !set.Contain("0"))
	assert.True(t, set.Contain("2"))
	assert.True(t, !set.Contain("4"))
	assert.True(t, set.Contain("6"))
	assert.True(t, !set.Contain("8"))
	assert.True(t, set.Contain("9"))
	assert.True(t, !set.Contain("12"))
	// Check ForEach()
	subID := make([]string, set.Len())
	subIndices := make([]int, set.Len())
	subValues := make([]float64, set.Len())
	set.ForEach(func(i int, id string, value float64) {
		subID[i] = id
		subValues[i] = value
	})
	assert.Equal(t, []string{"2", "6", "9"}, subID)
	assert.Equal(t, []float64{1, 1, 1}, subValues)
	// Check ForEachIndex()
	set.ForEachIndex(func(i, index int, value float64) {
		subIndices[i] = index
		subValues[i] = value
	})
	assert.Equal(t, []int{0, 2, 4}, subIndices)
	assert.Equal(t, []float64{1, 1, 1}, subValues)
}

func TestMarginalSubSet_ForIntersection(t *testing.T) {
	// Create two subsets
	indexer := NewTestIndexer([]string{"2", "4", "6", "8", "10"})
	indices := []int{0, 1, 2, 3, 4}
	a := NewMarginalSubSet(indexer, indices, []float64{1, 1, 1, 1, 1}, []int{0, 1, 2, 3})
	b := NewMarginalSubSet(indexer, indices, []float64{2, 2, 2, 2, 2}, []int{1, 2, 3, 4})
	intersectIDs := make([]string, 0)
	intersectA := make([]float64, 0)
	intersectB := make([]float64, 0)
	a.ForIntersection(b, func(id string, a, b float64) {
		intersectIDs = append(intersectIDs, id)
		intersectA = append(intersectA, a)
		intersectB = append(intersectB, b)
	})
	assert.Equal(t, []string{"4", "6", "8"}, intersectIDs)
	assert.Equal(t, []float64{1, 1, 1}, intersectA)
	assert.Equal(t, []float64{2, 2, 2}, intersectB)
}

func TestSparseVector(t *testing.T) {
	vec := NewSparseVector()
	// Add new items
	vec.Add(2, 1)
	vec.Add(1, 0)
	vec.Add(8, 3)
	vec.Add(4, 2)
	assert.Equal(t, []int{2, 1, 8, 4}, vec.Indices)
	assert.Equal(t, []float64{1, 0, 3, 2}, vec.Values)
	// Sort indices
	sort.Sort(vec)
	assert.Equal(t, []int{1, 2, 4, 8}, vec.Indices)
	assert.Equal(t, []float64{0, 1, 2, 3}, vec.Values)
	// Iterates
	vec.ForEach(func(i, index int, value float64) {
		assert.Equal(t, float64(i), value)
		assert.Equal(t, math.Pow(2, value), float64(index))
	})
}

func TestSparseVector_ForIntersection(t *testing.T) {
	a := NewSparseVector()
	a.Add(2, 1)
	a.Add(1, 0)
	a.Add(8, 3)
	a.Add(4, 2)
	b := NewSparseVector()
	b.Add(16, 2)
	b.Add(1, 0)
	b.Add(64, 3)
	b.Add(4, 1)
	intersectIndex := make([]int, 0)
	intersectA := make([]float64, 0)
	intersectB := make([]float64, 0)
	a.ForIntersection(b, func(index int, a, b float64) {
		intersectIndex = append(intersectIndex, index)
		intersectA = append(intersectA, a)
		intersectB = append(intersectB, b)
	})
	assert.Equal(t, []int{1, 4}, intersectIndex)
	assert.Equal(t, []float64{0, 2}, intersectA)
	assert.Equal(t, []float64{0, 1}, intersectB)
}

func TestMaxHeap(t *testing.T) {
	// Test a adjacent vec
	a := NewMaxHeap(3)
	a.Add(10, 2)
	a.Add(20, 8)
	a.Add(30, 1)
	elem, scores := a.ToSorted()
	assert.Equal(t, []interface{}{20, 10, 30}, elem)
	assert.Equal(t, []float64{8, 2, 1}, scores)
	// Test a full adjacent vec
	a.Add(40, 2)
	a.Add(50, 5)
	a.Add(12, 10)
	a.Add(67, 7)
	a.Add(32, 9)
	elem, scores = a.ToSorted()
	assert.Equal(t, []interface{}{12, 32, 20}, elem)
	assert.Equal(t, []float64{10, 9, 8}, scores)
}

func sliceToMapGeneral(a []interface{}) map[interface{}]bool {
	set := make(map[interface{}]bool)
	for _, i := range a {
		set[i] = true
	}
	return set
}

func sliceToMap(a []float64) map[float64]bool {
	set := make(map[float64]bool)
	for _, i := range a {
		set[i] = true
	}
	return set
}

func TestNewDenseSparseMatrix(t *testing.T) {
	a := NewDenseSparseMatrix(3)
	assert.Equal(t, 3, len(a))
}
