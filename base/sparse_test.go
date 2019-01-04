package base

import (
	"github.com/stretchr/testify/assert"
	"sort"
	"testing"
)

func TestSparseIdSet(t *testing.T) {
	// Create a ID set
	set := MakeSparseIdSet()
	assert.Equal(t, set.Len(), 0)
	// Add IDs
	set.Add(1)
	set.Add(2)
	set.Add(4)
	set.Add(8)
	assert.Equal(t, 4, set.Len())
	assert.Equal(t, 0, set.ToDenseId(1))
	assert.Equal(t, 1, set.ToDenseId(2))
	assert.Equal(t, 2, set.ToDenseId(4))
	assert.Equal(t, 3, set.ToDenseId(8))
	assert.Equal(t, NotId, set.ToDenseId(1000))
	assert.Equal(t, 1, set.ToSparseId(0))
	assert.Equal(t, 2, set.ToSparseId(1))
	assert.Equal(t, 4, set.ToSparseId(2))
	assert.Equal(t, 8, set.ToSparseId(3))
}

func TestSparseVector(t *testing.T) {
	vec := NewSparseVector()
	// Add new items
	vec.Add(2, 1)
	vec.Add(0, 0)
	vec.Add(8, 3)
	vec.Add(4, 2)
	assert.Equal(t, []int{2, 0, 8, 4}, vec.Indices)
	assert.Equal(t, []float64{1, 0, 3, 2}, vec.Values)
	// Sort indices
	sort.Sort(vec)
	assert.Equal(t, []int{0, 2, 4, 8}, vec.Indices)
	assert.Equal(t, []float64{0, 1, 2, 3}, vec.Values)
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

func TestKNNHeap(t *testing.T) {
	// Test a adjacent vec
	a := MakeKNNHeap(3)
	a.Add(10, 0, 1)
	a.Add(20, 0, 8)
	a.Add(30, 0, 0)
	assert.Equal(t, sliceToMapInt([]int{10, 20}), sliceToMapInt(a.Indices))
	assert.Equal(t, sliceToMap([]float64{1, 8}), sliceToMap(a.Similarities))
	// Test a full adjacent vec
	a.Add(40, 0, 2)
	a.Add(50, 0, 5)
	a.Add(12, 0, 10)
	a.Add(67, 0, 7)
	a.Add(32, 0, 9)
	assert.Equal(t, sliceToMapInt([]int{12, 32, 20}), sliceToMapInt(a.Indices))
	assert.Equal(t, sliceToMap([]float64{8, 9, 10}), sliceToMap(a.Similarities))
}

func sliceToMapInt(a []int) map[int]bool {
	set := make(map[int]bool)
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
